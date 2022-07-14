package main


import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"net"
	"strconv"
	"os"
)


// a proxy context should be modular
// so that multiple proxies can be run in the same
// program with separate contexts, including
// logging
type proxyContext struct {
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


func (proxy proxyContext) startProxy() {
	proxy.log.Printf("Starting proxy on socket %v:%v\n", *proxy.listen_ip, *proxy.listen_port)
	config := &ssh.ServerConfig{
	NoClientAuth: false,
	MaxAuthTries: 3,
	PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		proxy.log.Printf("Got client (%s) using creds (%s:%s)\n",
		conn.RemoteAddr(),
		conn.User(),
		password)
		sess_key := conn.LocalAddr().String()+":"+conn.RemoteAddr().String()
		curSession := SshSessions[sess_key]
		curSession.client_password = string(password)
		curSession.client_username = conn.User()
		curSession.proxy = proxy
		curSession.channels = make([]*channel_data, 0)
		curSession.channel_count = 0
		curSession.requests = make([]*request_data, 0)
		curSession.request_count = 0
		curSession.active = true
		curSession.thread_count = 0
		curSession.start_time = time.Now()
		curSession.msg_signal = make([]chan int,0)
		curSession.filename = sess_key + ".log.json"
		curSession.mutex.Unlock()
		return nil, nil
	},
	ServerVersion: *proxy.server_version,
	BannerCallback: func(conn ssh.ConnMetadata) string {
		return "bannerCallback"
	},
	}
	config.AddHostKey(proxy.private_key)

	listener, err := net.Listen("tcp",  *proxy.listen_ip +":"+strconv.Itoa(*proxy.listen_port))
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		sess_key := conn.LocalAddr().String()+":"+conn.RemoteAddr().String()
		SshSessions[sess_key] = new(sessionContext)
		SshSessions[sess_key].client_host = conn.RemoteAddr().String()
		SshSessions[sess_key].mutex.Lock()
		

		ssh_conn, channels, reqs, err:= ssh.NewServerConn(conn, config)
		if err != nil {
			continue
		}
		//go ssh.DiscardRequests(reqs)
		// maybe we *can* discard requests?
		go proxy.handleClientConn(ssh_conn, channels ,reqs)
	}
}

func (proxy proxyContext) addSessionToSessionList(session * sessionContext) {
	filename := SESSION_LIST_FN
	fd, err := os.OpenFile(*proxy.session_folder + "/" + filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		proxy.log.Println("error opening session list file:", err)
	}
	if _, err := fd.WriteString(session.infoAsJSON() + "\n"); err != nil {
		proxy.log.Println("error writing to session list file:", err)
	}
	if err := fd.Close(); err != nil {
		proxy.log.Println("error closing session list file:", err)
	}
	
}


func (proxy proxyContext) handleClientConn(client_conn *ssh.ServerConn, client_channels <-chan ssh.NewChannel, client_requests <-chan *ssh.Request) {


	sess_key := client_conn.LocalAddr().String()+":"+client_conn.RemoteAddr().String()
	curSession := SshSessions[sess_key]
	curSession.markThreadStarted()
	curSession.initializeLog()
	defer curSession.markThreadStopped()
	proxy.log.Printf("i can see password: %s\n",curSession.client_password)
	
	// connect to client.

	remote_server_host := *proxy.server_ssh_ip +":"+strconv.Itoa(*proxy.server_ssh_port)

	client_password := curSession.client_password
	if proxy.override_password != "" {
		client_password = proxy.override_password
	}

	client_user     := client_conn.User()

	if proxy.override_user != "" {
		client_user = proxy.override_user
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
		proxy.log.Printf("Error: cannot connect to remote server %s\n",remote_server_host)
		return
	}

	defer remote_sock.Close()

	remote_conn, remote_channels, remote_requests, err := ssh.NewClientConn(remote_sock, remote_server_host, remote_server_conf)

	if err != nil {
		proxy.log.Printf("Error creating new ssh client conn %w\n", err)
		return 
	}

	curSession.mutex.Lock()
	curSession.remote_conn = &remote_conn
	curSession.mutex.Unlock()

	shutdown_err := make(chan error, 1)
	go func() {
		shutdown_err <- remote_conn.Wait()
	}()

	curSession.handleEvent(
		&sessionEvent{
			Type: EVENT_SESSION_START,
			Key: sess_key,
			ServHost: remote_server_host,
			ClientHost: client_conn.RemoteAddr().String(),
			Username: curSession.client_username ,
			Password: curSession.client_password,
			StartTime: curSession.getStartTimeAsUnix(),
			TimeOffset: 0,
		})

	//curSession.log_session_data()
	go curSession.handleChannels(remote_conn, client_channels)
	go curSession.handleChannels(client_conn, remote_channels)
	go curSession.handleRequests(client_conn, remote_requests)
	go curSession.handleRequests(remote_conn, client_requests)
	

	<-shutdown_err
	remote_conn.Close()
}