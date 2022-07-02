package main

// inspiration from: https://github.com/cmoog/sshproxy/blob/master/_examples/main.go
// inspiration from: https://github.com/dutchcoders/sshproxy

// ugh i hate camel case but i guess i need to get over it 
// some day, at least. 

// thanks to these tutorials and references:
// - https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
// - https://scalingo.com/blog/writing-a-replacement-to-openssh-using-go-12
// - https://github.com/helloyi/go-sshclient/blob/master/sshclient.go
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
	//"github.com/cmoog/sshproxy"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
)

var (
	Logger *log.Logger
	// keys are "listen_ip:listen_port:client_ip:client:port"
	SshSessions map[string]*session_context 
)

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
}

// session context
type session_context struct {
	proxy				proxy_context
	mutex				sync.Mutex
	client_host			string
	client_username		string
	client_password		string	
	remote_conn			*ssh.Conn
	channels			[]*channel_data
	channel_count		int
	requests			[]*request_data
	request_count		int
}


func main() {

	cur_proxy := init_proxy_context()

	SshSessions = map[string]*session_context{}

	go start_proxy(cur_proxy)

	var input string
	for {
		fmt.Scanln(&input)
		if input == "q" {
			break;
		}
		fmt.Println("Enter q to quit.")
	}

}

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
		SshSessions[sess_key].client_password = string(password)
		SshSessions[sess_key].client_username = conn.User()
		SshSessions[sess_key].proxy = context
		SshSessions[sess_key].channels = make([]*channel_data, 0)
		SshSessions[sess_key].channel_count = 0
		SshSessions[sess_key].requests = make([]*request_data, 0)
		SshSessions[sess_key].request_count = 0
		SshSessions[sess_key].mutex.Unlock()
		return nil, nil
	},
//	ServerVersion: "OpenSSH_8.2p1 Ubuntu-4ubuntu0.5",
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
	defer dest_conn.Close()
	for cur_channel := range channels {
		// reset the var scope for each goroutine; taken from https://github.com/cmoog/sshproxy/blob/master/reverseproxy.go#L87
		cur_channel := cur_channel
		go func() {
			forward_channel(context, dest_conn, cur_channel)
		}()
	}
}


func forward_channel(context * session_context, dest_conn ssh.Conn, cur_channel ssh.NewChannel) {
	outgoing_channel, outgoing_requests, err := dest_conn.OpenChannel(cur_channel.ChannelType(), cur_channel.ExtraData())
	if err != nil {
		if openChanErr, ok := err.(*ssh.OpenChannelError); ok {
			_ = cur_channel.Reject(openChanErr.Reason, openChanErr.Message)
		} else {
			_ = cur_channel.Reject(ssh.ConnectionFailed, err.Error())
		}
		
		context.proxy.log.Printf("error open channel: %w\n", err)
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
	defer func() { _ = write_channel.CloseWrite() }()
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
	incoming_write_done := make(chan struct{})
	go func() {
		defer close(incoming_write_done)
		copy_channel(context, incoming_channel,outgoing_channel, "incoming",channel_type)
	}()
	go copy_channel(context, outgoing_channel, incoming_channel, "outgoing",channel_type)

	<-incoming_write_done
}


func handle_requests(context * session_context, outgoing_conn requestDest, incoming_requests <-chan *ssh.Request ) {
	for cur_request := range incoming_requests {
		err := forward_request(context, outgoing_conn, cur_request)
		if err != nil && !errors.Is(err, io.EOF) {
			context.proxy.log.Printf("handle request error: %v", err)
		}
	}
}

// TODO switch fmt to context
func forward_request(context * session_context, outgoing_channel requestDest, request *ssh.Request) error {
	
	context.request_count += 1
	context.requests = append(context.requests, &request_data{req_type: request.Type, req_payload: request.Payload})
	if request.Type == "env" || request.Type == "shell" || request.Type == "exec" {
		context.proxy.log.Printf("req.Type:%v, req.Payload:%v\n",request.Type,string(request.Payload))
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
	SshSessions[sess_key].mutex.Lock()
	SshSessions[sess_key].mutex.Unlock()
	context.log.Printf("i can see password: %s\n",SshSessions[sess_key].client_password)
	
	// connect to client.

	remote_server_host := *context.server_ssh_ip +":"+strconv.Itoa(*context.server_ssh_port)

	remote_server_conf := &ssh.ClientConfig{
		User:            client_conn.User(),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password(SshSessions[sess_key].client_password),
		},

	}
	remote_sock, err := net.DialTimeout("tcp", remote_server_host, 3000000000)
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

	SshSessions[sess_key].mutex.Lock()
	SshSessions[sess_key].remote_conn = &remote_conn
	SshSessions[sess_key].mutex.Unlock()

	shutdown_err := make(chan error, 1)
	go func() {
		shutdown_err <- remote_conn.Wait()
	}()

	go handle_channels(SshSessions[sess_key], remote_conn, client_channels)
	go handle_channels(SshSessions[sess_key], client_conn, remote_channels)
	go handle_requests(SshSessions[sess_key], client_conn, remote_requests)
	go handle_requests(SshSessions[sess_key], remote_conn, client_requests)
	

	<-shutdown_err
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

	if err != nil {
		proxy_private_key,err = generate_signer()
		Logger.Printf("Generating new key.")
		if err != nil {
			log.Fatal("Unable to load or generate a public key")
		}
	}
	Logger.Printf("Proxy using key with public key: %v\n",proxy_private_key.PublicKey().Marshal())

	return proxy_context{
		server_ssh_port,
		server_ssh_ip,
		proxy_listen_ip,
		proxy_listen_port,
		proxy_private_key,
		Logger}
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
		time_now := time.Now()
		time_elapsed := time_now.Sub(channel.start_time).Milliseconds()
		// reference: https://github.com/dutchcoders/sshproxy/blob/9b3ed8f5f5a7018c8dc09b4266e7f630683a9596/readers.go#L35
		channel.context.proxy.log.Printf("dir: %v, type: %v, msec: %v, data(%v): %v\n", 
		channel.direction, 
		channel.data_type, 
		time_elapsed, 
		bytes_read, 
		string(buff[:bytes_read]))
		channel.context.channels[channel.channel_id].chunks = append(channel.context.channels[channel.channel_id].chunks, 
			block_chunk{
				direction: channel.direction, 
				channel_type: channel.data_type, 
				time_offset: time_elapsed, 
				size: bytes_read, 
				data: buff[:bytes_read],
			})
	}

	return bytes_read, err
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
	//context.channels[channel_index].chunks = make([]block_chunk, 0)
	context.channel_count += 1
	return &channelWrapper{ReadWriter: in_channel, context: context, direction: direction, data_type: data_type, start_time: start_time, channel_id: channel_index}
}

type request_data struct {
	req_type string
	req_payload	[]byte
}

type channel_data struct {
	chunks []block_chunk
	channel_type string
}
type block_chunk struct {
	direction string
	channel_type string
	time_offset int64
	size int
	data []byte
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