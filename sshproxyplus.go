package main

// inspiration from: https://github.com/cmoog/sshproxy/blob/master/_examples/main.go
// inspiration from: https://github.com/dutchcoders/sshproxy

// ugh i hate camel case but i guess i need to get over it 
// some day, at least. 

// thanks to these tutorials and references:
// - https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
// - https://scalingo.com/blog/writing-a-replacement-to-openssh-using-go-12
// - https://github.com/helloyi/go-sshclient/blob/master/sshclient.go
// - https://gist.github.com/denji/12b3a568f092ab951456
// for future reference: https://elliotchance.medium.com/how-to-create-an-ssh-tunnel-in-go-b63722d682aa
import (
	"fmt"
	"flag"
	"log"
	"os"
	"strconv"
	"io/ioutil"
	"errors"
	"net"
	"sync"
	"io"
	"time"
	"golang.org/x/crypto/ssh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"encoding/binary"
	"encoding/json"
)

var (
	Logger *log.Logger
	// keys are "listen_ip:listen_port:client_ip:client:port"
	SshSessions map[string]*session_context 
)

const SIGNAL_SESSION_END int = 0
const SIGNAL_NEW_MESSAGE int = 1
const SESSION_LIST_FN	string = ".session_list"
//const SIGNAL_RESIZE_WINDOW int = 2

// a proxy context should be modular
// so that multiple proxies can be run in the same
// program with separate contexts, including
// logging
type proxy_context struct {
	server_ssh_port		*int
	server_ssh_ip		*string
	listen_ip			*string
	listen_port			*int
	private_key			ssh.Signer
	log					*log.Logger
	session_folder		*string
	tls_cert			*string
	tls_key				*string
	override_password	string
	override_user		string
	web_listen_port		*int
	server_version		*string
}

// session context
type session_context struct {
	proxy				proxy_context
	mutex				sync.Mutex
	log_mutex			sync.Mutex
	event_mutex			sync.Mutex
	log_fd				*os.File
	client_host			string
	client_username		string
	client_password		string	
	remote_conn			*ssh.Conn
	channels			[]*channel_data
	channel_count		int
	requests			[]*request_data
	request_count		int
	active				bool
	thread_count		int
	start_time   		time.Time
	stop_time 	 		time.Time
	msg_signal			[]chan int
	term_rows			uint32
	term_cols			uint32
	filename			string
	events				[]*sessionEvent
}
// TODO: create a routine to remove a signal
// so the signal list doesn't get crowded.


func main() {

	cur_proxy := init_proxy_context()

	SshSessions = map[string]*session_context{}

	go start_proxy(cur_proxy)

	go ServeWebSocketSessionServer("0.0.0.0:"+strconv.Itoa(*cur_proxy.web_listen_port),*cur_proxy.tls_cert, *cur_proxy.tls_key)

	var input string
	for {
		fmt.Scanln(&input)
		if input == "q" {
			break;
		}
		fmt.Println("Enter q to quit.")
	}

}

func channelTypeSupported(channelType string) bool {
    switch channelType {
    case
        "session":
        	return true
    }
    return false
}

// TODO: add methods to session_context so I can dump session data to file (json) or to stdout``
// TODO: add list of handlers to session to be called whenever new data is added to a session
//			--> implied: for each session there is a thread that monitors some queue of new
//			messages and forwards them to any active observers 
// TODO: (after the above) add ability to query active sessions and get data for a session using
// 		a web socket


// TODO: track all session events in a single list
// similar to the logging rather than storing them in channels, etc
func start_proxy(context proxy_context) {
	context.log.Printf("Starting proxy on socket %v:%v\n", *context.listen_ip, *context.listen_port)
	config := &ssh.ServerConfig{
	NoClientAuth: false,
	MaxAuthTries: 3,
	PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		context.log.Printf("Got client (%s) using creds (%s:%s)\n",
		conn.RemoteAddr(),
		conn.User(),
		password)
		sess_key := conn.LocalAddr().String()+":"+conn.RemoteAddr().String()
		cur_session := SshSessions[sess_key]
		cur_session.client_password = string(password)
		cur_session.client_username = conn.User()
		cur_session.proxy = context
		cur_session.channels = make([]*channel_data, 0)
		cur_session.channel_count = 0
		cur_session.requests = make([]*request_data, 0)
		cur_session.request_count = 0
		cur_session.active = true
		cur_session.thread_count = 0
		cur_session.start_time = time.Now()
		cur_session.msg_signal = make([]chan int,0)
		cur_session.filename = sess_key + ".log.json"
		cur_session.mutex.Unlock()
		return nil, nil
	},
	ServerVersion: *context.server_version,
	BannerCallback: func(conn ssh.ConnMetadata) string {
		return "bannerCallback"
	},
	}
	config.AddHostKey(context.private_key)

	listener, err := net.Listen("tcp",  *context.listen_ip +":"+strconv.Itoa(*context.listen_port))
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		sess_key := conn.LocalAddr().String()+":"+conn.RemoteAddr().String()
		SshSessions[sess_key] = new(session_context)
		SshSessions[sess_key].client_host = conn.RemoteAddr().String()
		SshSessions[sess_key].mutex.Lock()
		

		ssh_conn, channels, reqs, err:= ssh.NewServerConn(conn, config)
		if err != nil {
			continue
		}
		//go ssh.DiscardRequests(reqs)
		// maybe we *can* discard requests?
		go handle_client_conn(context, ssh_conn, channels ,reqs)
	}
}

func handle_channels(context * session_context, dest_conn ssh.Conn, channels <-chan ssh.NewChannel) {
	context.thread_start()
	defer context.thread_stop()
	defer dest_conn.Close()
	for this_channel := range channels {
		// reset the var scope for each goroutine; taken from https://github.com/cmoog/sshproxy/blob/master/reverseproxy.go#L87
		cur_channel := this_channel
		go func() {
			forward_channel(context, dest_conn, cur_channel)
		}()
	}
}


func forward_channel(context * session_context, dest_conn ssh.Conn, cur_channel ssh.NewChannel) {
	context.thread_start()
	defer context.thread_stop()
	context.handleEvent(
		&sessionEvent{
			Type: EVENT_NEW_CHANNEL,
			ChannelType: cur_channel.ChannelType(),
			ChannelData: cur_channel.ExtraData(),
		})
	if ! channelTypeSupported(cur_channel.ChannelType()) {
		_ = cur_channel.Reject(ssh.ConnectionFailed, "Unable to open channel.")
		context.proxy.log.Printf("Rejecting channel type: %v\n", cur_channel.ChannelType())
		return
	}
	outgoing_channel, outgoing_requests, err := dest_conn.OpenChannel(cur_channel.ChannelType(), cur_channel.ExtraData())
	if err != nil {
		if openChanErr, ok := err.(*ssh.OpenChannelError); ok {
			_ = cur_channel.Reject(openChanErr.Reason, openChanErr.Message)
		} else {
			_ = cur_channel.Reject(ssh.ConnectionFailed, err.Error())
		}
		
		context.proxy.log.Printf("error open channel: t:%v p:%v - %v\n", cur_channel.ChannelType(), cur_channel.ExtraData(),err)
		return
	}
	context.proxy.log.Printf("Opening channel of type: %v\n", cur_channel.ChannelType())
	defer outgoing_channel.Close()

	incoming_channel, incoming_requests, err := cur_channel.Accept()
	if err != nil {
		context.proxy.log.Printf("error accept new channel: %w\n", err)
		return
	}
	defer incoming_channel.Close()

	dest_requests_completed := make(chan struct{})
	// https://github.com/cmoog/sshproxy/blob/47ea68e82eaa4d43250d2a93c18fb26806cd67eb/reverseproxy.go#L127
	go func() {
		defer close(dest_requests_completed)
		handle_requests(context, channelRequestDest{incoming_channel}, outgoing_requests)
	}()

	// This request channel does not get closed
	// by the client causing this function to hang if we wait on it.
	// https://github.com/cmoog/sshproxy/blob/master/reverseproxy.go#L134
	go handle_requests(context, channelRequestDest{outgoing_channel}, incoming_requests)

	bidirectional_channel_clone(context, incoming_channel, outgoing_channel, cur_channel.ChannelType());
	<-dest_requests_completed
}

//TODO inject some wrapper for the reader so data gets stored
func copy_channel(context * session_context, write_channel ssh.Channel, read_channel ssh.Channel, direction string, channel_type string) {
	context.thread_start()
	defer context.thread_stop()
	defer write_channel.CloseWrite()
	done_copying := make(chan struct{})

	go func() {
		defer close(done_copying)
		_, err := io.Copy(write_channel, newChannelWrapper(read_channel,context, direction, "stdout", time.Now(), channel_type))
		if err != nil && !errors.Is(err, io.EOF) {
			context.proxy.log.Printf("channel copy error: %v\n", err)
		}
	}()
	_, err := io.Copy(write_channel.Stderr(), newChannelWrapper(read_channel.Stderr(),context, direction,"stderr", time.Now(), channel_type))
	if err != nil && !errors.Is(err, io.EOF) {
		context.proxy.log.Printf("channel copy error: %v\n", err)
	}
	<-done_copying
}

func bidirectional_channel_clone(context * session_context, incoming_channel ssh.Channel, outgoing_channel ssh.Channel,channel_type string) {
	context.thread_start()
	defer context.thread_stop()
	incoming_write_done := make(chan struct{})
	go func() {
		defer close(incoming_write_done)
		copy_channel(context, incoming_channel,outgoing_channel, "incoming",channel_type)
	}()
	go copy_channel(context, outgoing_channel, incoming_channel, "outgoing",channel_type)

	<-incoming_write_done
}


func handle_requests(context * session_context, outgoing_conn requestDest, incoming_requests <-chan *ssh.Request ) {
	context.thread_start()
	defer context.thread_stop()
	for cur_request := range incoming_requests {
		err := forward_request(context, outgoing_conn, cur_request)
		if err != nil && !errors.Is(err, io.EOF) {
			context.proxy.log.Printf("handle request error: %v", err)
		}
	}
}

// parseDims extracts two uint32s from the provided buffer.
// https://github.com/Scalingo/go-ssh-examples/blob/ae24797273aa9fcd3a8fa6c624af1b068a81d58b/server_complex.go#L229
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

// TODO switch fmt to context
func forward_request(context * session_context, outgoing_channel requestDest, request *ssh.Request) error {
	
	context.thread_start()
	defer context.thread_stop()
	
	
	context.request_count += 1
	
	request_entry := &request_data{Req_type: request.Type, Req_payload: request.Payload, Msg_type: "request-data", Offset: context.get_time_offset() }
	context.handleEvent(
		&sessionEvent{
			Type: EVENT_NEW_REQUEST,
			RequestType:  request.Type,
			RequestPayload: request.Payload,
		})
	
	context.requests = append(context.requests, request_entry)
	if request.Type == "env" || request.Type == "shell" || request.Type == "exec" {
		context.proxy.log.Printf("req.Type:%v, req.Payload:%v\n",request.Type,string(request.Payload))
	} else {
		context.proxy.log.Printf("req.Type:%v, req.Payload:%v\n",request.Type,request.Payload)
	}
	
	if request.Type == "pty-req" {
		//https://github.com/Scalingo/go-ssh-examples/blob/ae24797273aa9fcd3a8fa6c624af1b068a81d58b/server_complex.go#L206
		if(len(request.Payload)>4) {
			termLen := uint(request.Payload[3])
			if len(request.Payload) >= (4 + int(termLen)+ 8) {
				width, height := parseDims(request.Payload[termLen+4:])
				context.term_rows = height
				context.term_cols = width
				go context.handleEvent(
					&sessionEvent{
						Type: EVENT_WINDOW_RESIZE,
						TermRows: context.term_rows,
						TermCols: context.term_cols,
					})
				context.proxy.log.Printf("Window row:%v, col:%v\n", height,width)
			}
		}					
	} else if request.Type == "window-change" && len(request.Payload) >= 8 {
		width, height := parseDims(request.Payload)
		context.term_rows = height
		context.term_cols = width
		go context.handleEvent(
			&sessionEvent{
				Type: EVENT_WINDOW_RESIZE,
				TermRows: context.term_rows,
				TermCols: context.term_cols,
			})
		context.proxy.log.Printf("New window row:%v, col:%v\n", height,width)
	} 
	ok, product, err := outgoing_channel.SendRequest(request.Type, request.WantReply, request.Payload)
	if err != nil {
		if request.WantReply {
			if err := request.Reply(ok, product); err != nil {
				return fmt.Errorf("reply after send failure: %w", err)
			}
		}
		return fmt.Errorf("send request: %w", err)
	}

	if request.WantReply {
		if err := request.Reply(ok, product); err != nil {
			return fmt.Errorf("reply: %w", err)
		}
	}
	return nil
}




func handle_client_conn(context proxy_context, client_conn *ssh.ServerConn, client_channels <-chan ssh.NewChannel, client_requests <-chan *ssh.Request) {


	sess_key := client_conn.LocalAddr().String()+":"+client_conn.RemoteAddr().String()
	cur_session := SshSessions[sess_key]
	cur_session.thread_start()
	cur_session.init_log()
	defer cur_session.thread_stop()
	context.log.Printf("i can see password: %s\n",cur_session.client_password)
	
	// connect to client.

	remote_server_host := *context.server_ssh_ip +":"+strconv.Itoa(*context.server_ssh_port)

	client_password := cur_session.client_password
	if context.override_password != "" {
		client_password = context.override_password
	}

	client_user     := client_conn.User()

	if context.override_user != "" {
		client_user = context.override_user
	}
	remote_server_conf := &ssh.ClientConfig{
		User:            client_user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password(client_password),
		},

	}
	remote_sock, err := net.DialTimeout("tcp", remote_server_host, 5000000000)
	if err != nil {
		context.log.Printf("Error: cannot connect to remote server %s\n",remote_server_host)
		return
	}

	defer remote_sock.Close()

	remote_conn, remote_channels, remote_requests, err := ssh.NewClientConn(remote_sock, remote_server_host, remote_server_conf)

	if err != nil {
		context.log.Printf("Error creating new ssh client conn %w\n", err)
		return 
	}

	cur_session.mutex.Lock()
	cur_session.remote_conn = &remote_conn
	cur_session.mutex.Unlock()

	shutdown_err := make(chan error, 1)
	go func() {
		shutdown_err <- remote_conn.Wait()
	}()

	cur_session.handleEvent(
		&sessionEvent{
			Type: EVENT_SESSION_START,
			Key: sess_key,
			ServHost: remote_server_host,
			ClientHost: client_conn.RemoteAddr().String(),
			Username: cur_session.client_username ,
			Password: cur_session.client_password,
			StartTime: cur_session.getStartTimeAsUnix(),
			TimeOffset: 0,
		})

	//cur_session.log_session_data()
	go handle_channels(cur_session, remote_conn, client_channels)
	go handle_channels(cur_session, client_conn, remote_channels)
	go handle_requests(cur_session, client_conn, remote_requests)
	go handle_requests(cur_session, remote_conn, client_requests)
	

	<-shutdown_err
	remote_conn.Close()
	/*for cur_chan := range channels {
		if cur_chan.ChannelType() != "session" {
			context.log.Printf("rejecting channel of type: %v\n",cur_chan.ChannelType())
			cur_chan.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		ch, reqs, err := cur_chan.Accept()
		if err != nil {
			context.log.Printf("error accepting current channel\n")
			continue
		}



		go func (in <-chan *ssh.Request) {
			defer ch.Close()
			for req := range in {
				context.log.Printf("req.Type:%v, req.Payload:%v\n",req.Type,string(req.Payload))
				msg := []byte("Hullo\n")
				ch.Write(msg)
				for {
					in_msg := make([]byte, 1024)
					_, err := ch.Read(in_msg)
					if err != nil {
						break;
					}
					context.log.Printf("%s\n",string(in_msg))
					ch.Write(in_msg)
					if string(in_msg[:4]) == "exit"{
						context.log.Println("Got exit message. exiting channel loop.")
						break;
					}
				}
				continue
			}
		}(reqs)

	} */
}

func init_logging(filename *string) {
	var file *os.File
	if *filename != "-" {
		var err error
		file, err = os.OpenFile(*filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		file = os.Stdout
	}
	Logger = log.New(file, "LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func init_proxy_context() proxy_context {

	server_ssh_port := flag.Int("dport",22, "destination ssh server port")
	server_ssh_ip := flag.String("dip","127.0.0.1", "destination ssh server ip")
	proxy_listen_port := flag.Int("lport", 2222, "proxy listen port")
	proxy_listen_ip   := flag.String("lip", "0.0.0.0", "ip for proxy to bind to")
	proxy_key   := flag.String("lkey", "autogen", "private key for proxy to use; defaults to autogen new key")
	log_file := flag.String("log", "-", "file to log to; defaults to stdout")
	session_folder := flag.String("sess-dir", ".", "directory to write sessions to; defaults to the current directory")
	tls_cert := flag.String("tls_cert", ".", "TLS certificate to use for web; defaults to plaintext")
	tls_key := flag.String("tls_key", ".", "TLS key to use for web; defaults to plaintext")
	override_user := flag.String("override-user", "", "Override client-supplied username when proxying to remote server")
	override_password := flag.String("override-pass","","Overrides client-supplied password when proxying to remote server")
	web_listen_port := flag.Int("web-port", 8080, "web server listen port; defaults to 8080")
	server_version := flag.String("server-version", "SSH-2.0-OpenSSH_7.9p1 Raspbian-10", "server version to use")

	flag.Parse()

	init_logging(log_file)
	Logger.Println("sshproxyplus has started.")

	var proxy_private_key ssh.Signer
	var err error

	if *proxy_key != "autogen" {
		var proxy_key_bytes []byte
		proxy_key_bytes, err = ioutil.ReadFile(*proxy_key)
		if err == nil {
			proxy_private_key, err = ssh.ParsePrivateKey(proxy_key_bytes)
		}
		if err == nil {
			Logger.Printf("Successfully loaded key: %v\n",*proxy_key)
		} else {
			Logger.Printf("Failed to load key: %v\n", *proxy_key)
		}
	} else {
		err = errors.New("must autogen")
	}

	// https://freshman.tech/snippets/go/create-directory-if-not-exist/
	if *session_folder != "." {
		if _, err := os.Stat(*session_folder); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(*session_folder, os.ModePerm)
			if err != nil {
				Logger.Println(err)
			}
		}
	}


	if err != nil {
		proxy_private_key,err = generate_signer()
		Logger.Printf("Generating new key.")
		if err != nil {
			log.Fatal("Unable to load or generate a public key")
		}
	}
	Logger.Printf("Proxy using key with public key: %v\n",proxy_private_key.PublicKey().Marshal())

	if _, err := os.Stat(*tls_key); errors.Is(err, os.ErrNotExist) {
		dot := "."
		tls_key = &dot
		
	}
	if _, err := os.Stat(*tls_cert); errors.Is(err, os.ErrNotExist) {
		dot := "."
		tls_cert = &dot
	}

	return proxy_context{
		server_ssh_port,
		server_ssh_ip,
		proxy_listen_ip,
		proxy_listen_port,
		proxy_private_key,
		Logger,
		session_folder,
		tls_cert,
		tls_key,
		*override_password,
		*override_user,
		web_listen_port,
		server_version}
}


// taken from: https://github.com/cmoog/sshproxy/blob/5448845f823a2e6ec7ba6bb7ddf1e5db786410d4/_examples/main.go
func generate_signer() (ssh.Signer, error) { 
	const blockType = "EC PRIVATE KEY"
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate rsa private key: %w", err)
	}

	byt, err := x509.MarshalECPrivateKey(pkey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	pb := pem.Block{
		Type:    blockType,
		Headers: nil,
		Bytes:   byt,
	}
	p, err := ssh.ParsePrivateKey(pem.EncodeToMemory(&pb))
	if err != nil {
		return nil, err
	}
	return p, nil
}


type channelWrapper struct {
	io.ReadWriter
	context * session_context 
	direction string
	data_type string
	start_time   time.Time
	channel_id int
}

func (channel * channelWrapper) Read(buff []byte) (bytes_read int, err error) {
	bytes_read, err = channel.ReadWriter.Read(buff)

	if err == nil {
		
		//time_elapsed := channel.context.get_time_offset()
		// reference: https://github.com/dutchcoders/sshproxy/blob/9b3ed8f5f5a7018c8dc09b4266e7f630683a9596/readers.go#L35
		/*channel.context.proxy.log.Printf("dir: %v, type: %v, msec: %v, data(%v)\n", 
		channel.direction, 
		channel.data_type, 
		time_elapsed, 
		bytes_read, 
		//string(buff[:bytes_read])
	)*/

		data_copy := make([]byte, bytes_read)
	
		copy(data_copy, buff)

		/*new_chunk := block_chunk{
			Direction: channel.direction, 
			Channel_type: channel.data_type, 
			Time_offset: time_elapsed, 
			Size: bytes_read, 
			Data: data_copy,
		}*/
		//channel.context.channels[channel.channel_id].chunks = append(channel.context.channels[channel.channel_id].chunks, new_chunk)
		go channel.context.handleEvent(
			&sessionEvent{
				Type: EVENT_MESSAGE,
				Direction: channel.direction,
				ChannelType: channel.data_type,
				Size:	bytes_read,
				Data:	data_copy,
			})
	}

	return bytes_read, err
}


func (context * session_context) send_signal_to_clients(signal int) {
	for _, cur_signal := range context.msg_signal {
		cur_signal <- signal
	}
}

func (context * session_context) signal_new_message() {
	context.send_signal_to_clients(SIGNAL_NEW_MESSAGE)
}

func (context * session_context) signal_session_end() {
	context.send_signal_to_clients(SIGNAL_SESSION_END)
}
/*
func (context * session_context) signal_resize_window() {
	context.send_signal_to_clients(SIGNAL_RESIZE_WINDOW)
}
*/


func (context * session_context) thread_start() {
	context.mutex.Lock()
	context.thread_count += 1
	context.mutex.Unlock()
}

func (context * session_context) add_session_to_session_list() {
	filename := SESSION_LIST_FN
	fd, err := os.OpenFile(*context.proxy.session_folder + "/" + filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		context.proxy.log.Println("error opening session list file:", err)
	}
	if _, err := fd.WriteString(context.session_info_as_JSON() + "\n"); err != nil {
		context.proxy.log.Println("error writing to session list file:", err)
	}
	if err := fd.Close(); err != nil {
		context.proxy.log.Println("error closing session list file:", err)
	}
	
}

func (context * session_context) end_session() {
	context.active = false
	context.stop_time = time.Now()
	context.handleEvent(
		&sessionEvent{
			Type: EVENT_SESSION_STOP,
			StopTime: context.getStopTimeAsUnix(),
		})
	context.signal_session_end()
	context.finalize_log()
	context.add_session_to_session_list()
}

func (context * session_context) thread_stop() {
	context.mutex.Lock()
	context.thread_count -= 1
	if context.thread_count < 1 {
		context.end_session()
	}
	context.mutex.Unlock()
}


func (context * session_context) session_info_as_JSON() string {
	session_info := session_info_extended{
		Start_time: 	context.start_time.Unix(),
		Stop_time: 		context.stop_time.Unix(),
		Length:			int64(context.stop_time.Sub(context.start_time).Seconds()),
		Client_host:	context.client_host,
		Serv_host:		*context.proxy.server_ssh_ip +":"+strconv.Itoa(*context.proxy.server_ssh_port),
		Username:		context.client_username,
		Password: 		context.client_password,
		Term_rows:		context.term_rows,
		Term_cols:		context.term_cols,
		Filename:		context.filename}
	for _, request := range context.requests {
		session_info.Requests = append(session_info.Requests, request.Req_type)
	}
	data, err := json.Marshal(session_info)
	if err != nil {
		context.proxy.log.Println("Error during marshaling json: ", err)
		return ""
	}
	return string(data)
}



func (context * session_context) get_time_offset() int64 {
	time_now := time.Now()
	return time_now.Sub(context.start_time).Milliseconds()
}

/*
func (context * session_context) log_session_data()  {
	data := context.session_info_as_JSON()
	context.append_to_log([]byte(data))
}

func (context * session_context) log_chunk(chunk *block_chunk)  {
	json_data, err := json.Marshal(chunk)
	if err != nil {
		context.proxy.log.Println("Error during marshaling json: ", err)
		return 
	}
	data := []byte(",\n" + string(json_data))
	context.append_to_log(data)
}
func (context * session_context) log_resize()  {
	msg := window_message_extended{Rows:int64(context.term_rows), Columns: int64(context.term_cols), Type: "window-size", Offset: context.get_time_offset()}
	json_data, err := json.Marshal(msg)
	if err != nil {
		context.proxy.log.Println("Error during marshaling json: ", err)
		return 
	}
	data := []byte(",\n" + string(json_data))
	context.append_to_log(data)

}

func (context * session_context) log_request(msg *request_data)  {
	json_data, err := json.Marshal(msg)
	if err != nil {
		context.proxy.log.Println("Error during marshaling json: ", err)
		return 
	}
	data := []byte(",\n" + string(json_data))
	context.append_to_log(data)
}
*/

func (context * session_context) init_log()  {
	f, err := os.OpenFile(*context.proxy.session_folder + "/" + context.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		context.proxy.log.Println("error opening session log file:", err)
	}
	context.mutex.Lock()
		context.log_fd = f
	context.mutex.Unlock()
	context.append_to_log([]byte("[\n")); 
}

func (context * session_context) append_to_log(data []byte) {
	context.log_mutex.Lock()
	if _, err := context.log_fd.Write(data); err != nil {
		context.log_fd.Close() // ignore error; Write error takes precedence
		context.proxy.log.Println("error writing to log file:", err)
	}
	context.log_mutex.Unlock()
}

func (context * session_context) finalize_log()  {
	context.append_to_log([]byte("\n]"))
	if err := context.log_fd.Close(); err != nil {
		context.proxy.log.Println("error closing log file:", err)
	}
	if(len(context.events)<10) {
		old_file := *context.proxy.session_folder + "/" + context.filename
		new_file := old_file + ".scan"
		err := os.Rename(old_file,new_file)
		if err != nil {
			context.proxy.log.Printf("Error moving log file from %v to %v: %v",old_file, new_file, err)
		}
		context.filename = new_file
	}
}


func (context * session_context) new_signal() chan int {
	new_msg_signal := make(chan int)
	context.mutex.Lock()
	context.msg_signal = append(context.msg_signal,new_msg_signal)
	context.mutex.Unlock()
	return new_msg_signal
}


func ListSessions() []string {
	session_keys := make([]string, len(SshSessions))

	index := 0
	for cur_key := range SshSessions {
		session_keys[index] = cur_key
		index++
	}

	return session_keys
}


func ListActiveSessions() []string {
	session_keys := make([]string, 0)

	for cur_key := range SshSessions {
		if SshSessions[cur_key].active == true {
			session_keys = append(session_keys, cur_key)
		}
	}

	return session_keys
}


func newChannelWrapper(
	in_channel io.ReadWriter, 
	context * session_context, 
	direction string, 
	data_type string, 
	start_time time.Time,
	channel_type string,
	) io.ReadWriter {
	channel_index := context.channel_count
	context.channels = append(context.channels, &channel_data{chunks: make([]block_chunk, 0), channel_type: channel_type})
	context.channel_count += 1
	
	return &channelWrapper{ReadWriter: in_channel, context: context, direction: direction, data_type: data_type, start_time: start_time, channel_id: channel_index}
}

type request_data struct {
	Req_type string	`json:"request_type"`
	Req_payload	[]byte `json:"request_data"`
	Msg_type	string  `json:"type"`
	Offset		int64	`json:"offset"`
}

type channel_data struct {
	chunks []block_chunk
	channel_type string
}
type block_chunk struct {
	Direction string `json:"direction"`
	Channel_type string `json:"type"`
	Time_offset int64 `json:"offset"`
	Size int `json:"size"`
	Data []byte `json:"data"`
}

type window_message_extended struct {
	Rows int64 `json:"rows"`
	Columns int64 `json:"columns"`
	Type string `json:"type"`
	Offset int64 `json:"offset"`
}


type session_info_extended struct {
	Start_time	int64 `json:"start"`
	Stop_time	int64 `json:"stop"`
	Length		int64 `json:"length"`
	Client_host	string `json:"client_host"`
	Serv_host   string `json:"server_host"`
	Username	string `json:"username"`
	Password    string `json:"password"`
	Term_rows	uint32 `json:"term_rows"`
	Term_cols	uint32 `json:"term_cols"`
	Filename	string  `json:"filename"`
	Requests	[]string `json:"requests"`
}

// taken from 192-208: https://github.com/cmoog/sshproxy/blob/47ea68e82eaa4d43250d2a93c18fb26806cd67eb/reverseproxy.go#L192
// channelRequestDest wraps the ssh.Channel type to conform with the standard SendRequest function signiture.
// This allows for convenient code re-use in piping channel-level requests as well as global, connection-level
// requests.
type channelRequestDest struct {
	ssh.Channel
}

func (c channelRequestDest) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	ok, err := c.Channel.SendRequest(name, wantReply, payload)
	return ok, nil, err
}

// requestDest defines a resource capable of receiving requests, (global or channel).
type requestDest interface {
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
}