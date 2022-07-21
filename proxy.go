package main


import (
	"golang.org/x/crypto/ssh"
	"time"
	"net"
	"strconv"
	"os"
	"errors"
)

const SESSION_LIST_FN	string = ".session_list"
const ACTIVE_POLLING_DELAY time.Duration = 500* time.Millisecond


// a proxy runs on a single port
// it can support username/password
// combinations and redirect each
// combination to a different remote
// host
type proxyContext struct {
	server_ssh_port		*int
	server_ssh_ip		*string
	listen_ip			*string
	listen_port			*int
	private_key			ssh.Signer
	log					loggerInterface
	session_folder		*string
	tls_cert			*string
	tls_key				*string
	override_password	string
	override_user		string
	web_listen_port		*int
	server_version		*string
	Users 				map[string]*proxyUser
	userSessions			map[string]map[string]*sessionContext
	allSessions			map[string]*sessionContext 
	RequireValidPassword	bool
	active				bool
	publicAccess		bool
	viewers				map[string]*proxySessionViewer
	baseURI				string
	// when there are new sessions, block forwarding until this is true
}

type loggerInterface interface {
	Printf(format string, v ...any)
	Println(v ...any)
}

// TODO: update authentication routine to 
// check users list and only authorize
// if user is in list
// should also include default user option


func (proxy *proxyContext) startProxy() {

	proxy.log.Printf("Starting proxy on socket %v:%v\n", *proxy.listen_ip, *proxy.listen_port)
	config := &ssh.ServerConfig{
	NoClientAuth: false,
	MaxAuthTries: 3,
	PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		
		proxy.log.Printf("Got client (%s) using creds (%s:%s)\n",
		conn.RemoteAddr(),
		conn.User(),
		password)

		//TODO: make session_key unique with a counter

		err, user := proxy.authenticateUser(conn.User(),string(password))

		if(err != nil) {
			proxy.log.Printf("authentication failed: %v\n",err)
			return nil, err
		}

		sess_key := conn.LocalAddr().String()+":"+conn.RemoteAddr().String() // +string(getNextSessionIdCounter())
		curSession := proxy.allSessions[sess_key]
		
		curSession.user = user
		curSession.client_password = string(password)
		curSession.client_username = conn.User()
		curSession.proxy = proxy
		curSession.channels = make([]*channel_data, 0)
		curSession.channel_count = 1
		curSession.active = true
		curSession.requests = make([]*request_data, 0)
		curSession.request_count = 1
		curSession.thread_count = 0
		curSession.start_time = time.Now()
		curSession.msg_signal = make([]chan int,0)
		curSession.filename = sess_key + ".log.json"
		curSession.sessionID = sess_key
		
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
		// if a client reuses socket ports, let's preserve the old session
		// they shouldn't be able to do this concurrently because of how TCP works.
		if  val, ok := proxy.allSessions[sess_key]; ok {
			proxy.allSessions[sess_key+"_old"] = val
			delete(proxy.allSessions, sess_key)
		}
		proxy.allSessions[sess_key] = new(sessionContext)
		proxy.allSessions[sess_key].client_host = conn.RemoteAddr().String()
		proxy.allSessions[sess_key].mutex.Lock()
		

		ssh_conn, channels, reqs, err:= ssh.NewServerConn(conn, config)
		if err != nil {
			continue
		}
		//go ssh.DiscardRequests(reqs)
		// maybe we *can* discard requests?
		go proxy.handleClientConn(ssh_conn, channels, reqs, proxy.allSessions[sess_key])
	}
}

func (proxy *proxyContext) addSessionToUserList(session *sessionContext) {
	user := session.user.getKey()
	if  _, ok := proxy.userSessions[user]; !ok {
		proxy.userSessions[user] = make(map[string]*sessionContext)
	}
	session_id := session.getID()
	proxy.userSessions[user][session_id] = session
}

func (proxy *proxyContext) activate() {
	proxy.active = true
}

func (proxy *proxyContext) deactivate() {
	proxy.active = false
}

func (proxy *proxyContext) addSessionToSessionList(session * sessionContext) {
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



func (proxy *proxyContext) getProxyUser(username, password string) (error, *proxyUser,bool) {
	err := errors.New("not a valid user")
	key := buildProxyUserKey(username,password)
	if  val, ok := proxy.Users[key]; ok {
		return_val := *val
		return nil, &return_val, false
	} else if  val, ok := proxy.Users[buildProxyUserKey(username,"")]; ok {
		if val.Password == "" {
			return_val := *val
			return nil, &return_val, true
		} else {
			return err, nil, false
		}
	} else {
		return err, nil, false
	}
}

func (proxy *proxyContext) addProxyUser(user *proxyUser) {
	key := buildProxyUserKey(user.Username,user.Password)
	proxy.Users[key] = user
}

func (proxy *proxyContext) removeProxyUser(username string, password string) {
	key := buildProxyUserKey(username,password)
	if _, ok := proxy.Users[key]; ok {
		delete(proxy.Users, key)
	}
	
}

func (proxy *proxyContext) authenticateUser(username,password string) (error, *proxyUser) {
	
	default_user := &proxyUser{
		Username: username,
		Password: password,
		RemoteHost: proxy.getDefaultRemoteHost(),
		RemoteUsername: username,
		RemotePassword: password,
	}

	// override password if it is provided
	if proxy.override_password != "" {
		default_user.RemotePassword = proxy.override_password
	}
	// override user if it is provided
	if proxy.override_user != "" {
		default_user.RemoteUsername = proxy.override_user
	}

	if(len(proxy.Users)>0) {
		err, user,password_blank := proxy.getProxyUser(username, password)
		if (err != nil) {
			//creds are not valid 
			return err, nil
		} else {
			if password_blank {
				proxy.log.Printf("allowing any password for user: %v\n", username)
			} 
			return nil, user
		}
	} else {
		return nil, default_user
	}
}

func (proxy *proxyContext) getDefaultRemoteHost() string {
	return *proxy.server_ssh_ip +":"+strconv.Itoa(*proxy.server_ssh_port)
}

func (proxy *proxyContext) handleClientConn(client_conn *ssh.ServerConn, client_channels <-chan ssh.NewChannel, client_requests <-chan *ssh.Request, curSession *sessionContext) {


	//sess_key := client_conn.LocalAddr().String()+":"+client_conn.RemoteAddr().String()
	//curSession := proxy.allSessions[sess_key]
	//sess_key := curSession.sessionID
	curSession.markThreadStarted()
	curSession.initializeLog()
	defer curSession.markThreadStopped()
	proxy.log.Printf("i can see password: %s\n",curSession.client_password)
	
	// connect to client.

	remote_server_conf := &ssh.ClientConfig{
		User:            curSession.user.RemoteUsername,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password(curSession.user.RemotePassword),
		},

	}
	remote_sock, err := net.DialTimeout("tcp", curSession.user.RemoteHost, 5000000000)
	if err != nil {
		proxy.log.Printf("Error: cannot connect to remote server %s\n",curSession.user.RemoteHost)
		return
	}

	defer remote_sock.Close()

	remote_conn, remote_channels, remote_requests, err := ssh.NewClientConn(remote_sock, curSession.user.RemoteHost, remote_server_conf)

	if err != nil {
		proxy.log.Printf("Error creating new ssh client conn %v\n", err)
		return 
	}

	curSession.mutex.Lock()
	curSession.remote_conn = &remote_conn
	curSession.mutex.Unlock()

	shutdown_err := make(chan error, 1)
	go func() {
		shutdown_err <- remote_conn.Wait()
	}()

	proxy.addSessionToUserList(curSession)

	start_event := sessionEvent{
		Type: EVENT_SESSION_START,
		Key: curSession.sessionID,
		ServHost: curSession.user.RemoteHost,
		ClientHost: client_conn.RemoteAddr().String(),
		Username: curSession.client_username ,
		Password: curSession.client_password,
		StartTime: curSession.getStartTimeAsUnix(),
		TimeOffset: 0,
	}
	curSession.handleEvent(&start_event)
		
	proxy.log.Printf("New session starting: %v\n",start_event.toJSON())
	for {
		if (proxy.active) {
			break;
		}
		time.Sleep(ACTIVE_POLLING_DELAY)
	}
	
	

	//curSession.log_session_data()
	go curSession.handleChannels(remote_conn, client_channels)
	go curSession.handleChannels(client_conn, remote_channels)
	go curSession.handleRequests(client_conn, remote_requests, 0)
	go curSession.handleRequests(remote_conn, client_requests, 0)
	
	proxy.log.Println("New session started")

	<-shutdown_err
	remote_conn.Close()
}
/*

func (proxy *proxyContext) getSessionsKeys() []string {
	session_keys := make([]string, 0)
	for key := range proxy.userSessions {
		session_keys = append(session_keys,key)
	}
	return session_keys
}*/

func (proxy *proxyContext) getUserSessionsKeys(user string) []string {
	session_keys := make([]string, 0)
	if val, ok := proxy.userSessions[user]; ok {
		for key := range val {
			session_keys = append(session_keys,key)
		}
	}
	return session_keys
}


func (proxy *proxyContext) getUsers() []string {
	users := make([]string, 0)
	for key := range proxy.Users {
		users = append(users,proxy.Users[key].Username)
	}
	return users
}

func (proxy *proxyContext) makeSessionViewerForUser(user_key string) (error, *proxySessionViewer) {

	err,user,_ := proxy.getProxyUser(user_key, "")

	if user != nil {
		viewer := createNewSessionViewer(SESSION_VIEWER_TYPE_LIST)
		viewer.proxy = proxy
		viewer.user = user
		proxy.addSessionViewer(viewer)
		return err, viewer
	} 
	return err, nil
}


func (proxy *proxyContext) makeSessionViewerForSession(user_key string, session string) *proxySessionViewer {
	_,user,_ := proxy.getProxyUser(user_key, "")

	if user != nil {
		viewer := createNewSessionViewer(SESSION_VIEWER_TYPE_LIST)
		viewer.proxy = proxy
		viewer.user = user
		viewer.sessionKey = session
		proxy.addSessionViewer(viewer)
		return viewer
	} else {
		return nil
	}
}


func (proxy *proxyContext) addSessionViewer(viewer *proxySessionViewer) {
	key := viewer.secret
	proxy.viewers[key] = viewer
}

func (proxy *proxyContext) removeSessionViewer(key string) {
	if _, ok := proxy.viewers[key]; ok {
		delete(proxy.viewers, key)
	}
}


func (proxy *proxyContext) getSessionViewer(key string) *proxySessionViewer {
	proxy.log.Printf("key is:%v\n",key)
	if  val, ok := proxy.viewers[key]; ok {
		if val.isExpired() {
			proxy.removeSessionViewer(key)
		} else {
			return val
		}
	}
	return nil
}

func makeListOfSessionKeys(sessions map[string]*sessionContext, include_inactive bool) []string {
	session_keys := make([]string, 0)
	
	for cur_key := range sessions {
		if sessions[cur_key].active == true || include_inactive {
			session_keys = append(session_keys, cur_key)
		}
	}

	return session_keys
}

func (proxy *proxyContext) ListAllUserSessions(user string) []string {
	return makeListOfSessionKeys(proxy.userSessions[user],true)
}

func (proxy *proxyContext) ListAllActiveUserSessions(user string) []string {
	return makeListOfSessionKeys(proxy.userSessions[user],false)
}



func (proxy *proxyContext) ListAllSessions() []string {
	return makeListOfSessionKeys(proxy.allSessions,true)
}


func (proxy *proxyContext) ListAllActiveSessions() []string {
	return makeListOfSessionKeys(proxy.allSessions,false)
}





// note: username/password combo is a 
// unique key here
type proxyUser struct {
	Username	string
	Password	string
	RemoteHost	string
	RemoteUsername string
	RemotePassword string
}
// eventHooks	[]*eventHook

func buildProxyUserKey(user,pass string) string {
	return user + ":" + pass
}

func (user *proxyUser) getKey() string {
	return buildProxyUserKey(user.Username, "")
}


