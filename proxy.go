package main


import (
	"golang.org/x/crypto/ssh"
	"time"
	"net"
	"strconv"
	"os"
	"errors"
	"log"
	"encoding/json"
)

const SESSION_LIST_FN	string = ".session_list"
const ACTIVE_POLLING_DELAY time.Duration = 500* time.Millisecond


// a proxy runs on a single port
// it can support username/password
// combinations and redirect each
// combination to a different remote
// host
type proxyContext struct {
	running				bool
	listener			net.Listener
	DefaultRemotePort	int
	DefaultRemoteIP		string
	ListenIP			string
	ListenPort			int
	private_key			ssh.Signer
	log					loggerInterface
	SessionFolder		string
	TLSCert				string
	TLSKey				string
	OverridePassword	string
	OverrideUser		string
	WebListenPort		int
	ServerVersion		string
	Users 				map[string]*proxyUser
	userSessions		map[string]map[string]*sessionContext
	allSessions			map[string]*sessionContext 
	RequireValidPassword	bool
	active				bool
	PublicAccess		bool
	Viewers				map[string]*proxySessionViewer
	BaseURI				string
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


func makeNewProxy(signer ssh.Signer) *proxyContext {
	return &proxyContext{
		log: log.Default(),
		Users: map[string]*proxyUser{},
		userSessions: map[string]map[string]*sessionContext{},
		allSessions: map[string]*sessionContext{},
		Viewers: map[string]*proxySessionViewer{},
		DefaultRemotePort: 22,
		DefaultRemoteIP: "127.0.0.1",
		ListenIP: "0.0.0.0",
		ListenPort: 2222,
		SessionFolder: "html/sessions",
		WebListenPort: 8080,
		ServerVersion: "SSH-2.0-OpenSSH_7.9p1 Raspbian-10",
		PublicAccess:true,
		private_key: signer,
	}
}

func (proxy *proxyContext) startProxy() {

	proxy.log.Printf("Starting proxy on socket %v:%v\n", proxy.ListenIP, proxy.ListenPort)
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
	ServerVersion: proxy.ServerVersion,
	BannerCallback: func(conn ssh.ConnMetadata) string {
		return "bannerCallback"
	},
	}
	config.AddHostKey(proxy.private_key)
	proxy.running = true
	listener, err := net.Listen("tcp",  proxy.ListenIP +":"+strconv.Itoa(proxy.ListenPort))
	if err != nil {
		panic(err)
	}
	proxy.listener = listener
	for proxy.running {
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

func (proxy *proxyContext) Stop() {
	proxy.running = false
	proxy.listener.Close()
	for _, value := range proxy.allSessions { 
		value.end()
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
	fd, err := os.OpenFile(proxy.SessionFolder + "/" + filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
		eventCallbacks: make([]*eventCallback, 0),
		channelFilters: make([]*channelFilterFunc,0),
	}

	// override password if it is provided
	if proxy.OverridePassword != "" {
		default_user.RemotePassword = proxy.OverridePassword
	}
	// override user if it is provided
	if proxy.OverrideUser != "" {
		default_user.RemoteUsername = proxy.OverrideUser
	}

	if(len(proxy.Users)>0) {
		err, user,password_blank := proxy.getProxyUser(username, password)
		if (err != nil) {
			if ! proxy.RequireValidPassword {
				return nil, default_user
			} else {
				return err, nil
			}
			
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
	return proxy.DefaultRemoteIP +":"+strconv.Itoa(proxy.DefaultRemotePort)
}

func makeProxyFromJSON(data []byte, signer ssh.Signer) (error, *proxyContext) {
	var err error
	proxy := &proxyContext{}
	err = json.Unmarshal(data, proxy)
	if err == nil {
		proxy.initialize(signer)
	}
	return err, proxy
}

func (proxy *proxyContext) initialize(defaultSigner ssh.Signer) {
	if proxy.Users == nil {
		proxy.Users = map[string]*proxyUser{}
	}
	if proxy.userSessions == nil {
		proxy.userSessions = map[string]map[string]*sessionContext{}
	}

	if proxy.allSessions == nil {
		proxy.allSessions = map[string]*sessionContext{}
	}

	if proxy.Viewers == nil {
		proxy.Viewers = map[string]*proxySessionViewer{}
	}

	if proxy.private_key == nil {
		proxy.private_key = defaultSigner
	}

	for _, viewer := range proxy.Viewers {
		viewer.proxy = proxy
		if(viewer.User != nil) {
			err, user, _ := proxy.getProxyUser(viewer.User.Username, viewer.User.Password)
			if err == nil {
				viewer.User = user
			}
		}
	}
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
		viewer := createNewSessionViewer(SESSION_VIEWER_TYPE_LIST,proxy, user)
		proxy.addSessionViewer(viewer)
		return err, viewer
	} 
	return err, nil
}


func (proxy *proxyContext) makeSessionViewerForSession(user_key string, session string) *proxySessionViewer {
	_,user,_ := proxy.getProxyUser(user_key, "")

	if user != nil {
		viewer := createNewSessionViewer(SESSION_VIEWER_TYPE_LIST, proxy, user)
		viewer.SessionKey = session
		proxy.addSessionViewer(viewer)
		return viewer
	} else {
		return nil
	}
}


func (proxy *proxyContext) addSessionViewer(viewer *proxySessionViewer) {
	key := viewer.Secret
	proxy.Viewers[key] = viewer
}

func (proxy *proxyContext) removeSessionViewer(key string) {
	if _, ok := proxy.Viewers[key]; ok {
		delete(proxy.Viewers, key)
	}
}


func (proxy *proxyContext) getSessionViewer(key string) *proxySessionViewer {
	if  val, ok := proxy.Viewers[key]; ok {
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
	eventCallbacks []*eventCallback
	channelFilters []*channelFilterFunc
}


func buildProxyUserKey(user,pass string) string {
	return user + ":" + pass
}

func (user *proxyUser) getKey() string {
	return buildProxyUserKey(user.Username, "")
}

func (user *proxyUser) addEventCallback(callback *eventCallback) {
	if user.eventCallbacks == nil {
		user.eventCallbacks = make([]*eventCallback,0)
	}
	user.eventCallbacks = append(user.eventCallbacks, callback)
}

func (user *proxyUser) removeEventCallback(callback *eventCallback) {
	if user.eventCallbacks != nil {
		for index, value := range user.eventCallbacks {
			if value == callback {
				user.eventCallbacks[index] = user.eventCallbacks[len(user.eventCallbacks)-1]
				user.eventCallbacks = user.eventCallbacks[:len(user.eventCallbacks)-1]
			}
		}
	}
}

func (user *proxyUser) addChannelFilter(function *channelFilterFunc) {
	if user.channelFilters == nil {
		user.channelFilters = make([]*channelFilterFunc,0)
	}
	user.channelFilters = append(user.channelFilters, function)
}

func (user *proxyUser) removeChannelFilter(function *channelFilterFunc) {
	if user.channelFilters != nil {
		for index, value := range user.channelFilters {
			if value == function {
				user.channelFilters[index] = user.channelFilters[len(user.channelFilters)-1]
				user.channelFilters = user.channelFilters[:len(user.channelFilters)-1]
			}
		}
	}
}


