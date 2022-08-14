package sshproxyplus


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
type ProxyContext struct {
	running				bool
	listener			net.Listener
	DefaultRemotePort	int
	DefaultRemoteIP		string
	ListenIP			string
	ListenPort			int
	private_key			ssh.Signer
	Log					LoggerInterface `json:"-"`
	SessionFolder		string
	TLSCert				string
	TLSKey				string
	OverridePassword	string
	OverrideUser		string
	WebListenPort		int
	ServerVersion		string
	Users 				map[string]*ProxyUser
	userSessions		map[string]map[string]*SessionContext
	allSessions			map[string]*SessionContext 
	RequireValidPassword	bool
	active				bool
	PublicAccess		bool
	Viewers				map[string]*proxySessionViewer
	BaseURI				string
	// when there are new sessions, block forwarding until this is true
}

type LoggerInterface interface {
	Printf(format string, v ...any)
	Println(v ...any)
}

// TODO: update authentication routine to 
// check Users list and only authorize
// if user is in list
// should also include default user option


func MakeNewProxy(signer ssh.Signer) *ProxyContext {
	return &ProxyContext{
		Log: log.Default(), 
		Users: map[string]*ProxyUser{},
		userSessions: map[string]map[string]*SessionContext{},
		allSessions: map[string]*SessionContext{},
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

func (proxy *ProxyContext) StartProxy() {

	proxy.Log.Printf("Starting proxy on socket %v:%v\n", proxy.ListenIP, proxy.ListenPort)
	config := &ssh.ServerConfig{
	NoClientAuth: false,
	MaxAuthTries: 3,
	//TODO: move this function into the session class so it doesn't
	// need the session key
	PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		
		proxy.Log.Printf("Got client (%s) using creds (%s:%s)\n",
		conn.RemoteAddr(),
		conn.User(),
		password)

		//TODO: make session_key unique with a counter

		err, user := proxy.AuthenticateUser(conn.User(),string(password))

		if(err != nil) {
			proxy.Log.Printf("authentication failed: %v\n",err)
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
	
	listener, err := net.Listen("tcp",  proxy.ListenIP +":"+strconv.Itoa(proxy.ListenPort))
	if err != nil {
		panic(err)
	}
	proxy.listener = listener
	proxy.running = true
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
		proxy.allSessions[sess_key] = new(SessionContext)
		proxy.allSessions[sess_key].client_host = conn.RemoteAddr().String()
		proxy.allSessions[sess_key].mutex.Lock()
		

		ssh_conn, channels, reqs, err:= ssh.NewServerConn(conn, config)
		if err != nil {
			continue
		}
		
		//go ssh.DiscardRequests(reqs)
		// maybe we *can* discard requests?
		go proxy.HandleClientConn(ssh_conn, channels, reqs, proxy.allSessions[sess_key])
	}
}

func (proxy *ProxyContext) Stop() {
	proxy.running = false
	proxy.listener.Close()
	for _, session := range proxy.allSessions { 
		session.End()
	}
}

func (proxy *ProxyContext) AddSessionToUserList(session *SessionContext) {
	user := session.user.GetKey()
	if  _, ok := proxy.userSessions[user]; !ok {
		proxy.userSessions[user] = make(map[string]*SessionContext)
	}
	session_id := session.GetID()
	proxy.userSessions[user][session_id] = session
}

func (proxy *ProxyContext) Activate() {
	proxy.active = true
}

func (proxy *ProxyContext) Deactivate() {
	proxy.active = false
}

func (proxy *ProxyContext) IsActive() bool {
	return proxy.active 
}

func (proxy *ProxyContext) AddSessionToSessionList(session * SessionContext) {
	filename := SESSION_LIST_FN
	fd, err := os.OpenFile(proxy.SessionFolder + "/" + filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		proxy.Log.Println("error opening session list file:", err)
	}
	if _, err := fd.WriteString(session.InfoAsJSON() + "\n"); err != nil {
		proxy.Log.Println("error writing to session list file:", err)
	}
	if err := fd.Close(); err != nil {
		proxy.Log.Println("error closing session list file:", err)
	}
	
}



func (proxy *ProxyContext) GetProxyUser(username, password string, cloneUser bool) (error, *ProxyUser,bool) {
	err := errors.New("not a valid user")
	key := buildProxyUserKey(username,password)
	if  val, ok := proxy.Users[key]; ok {
		if(cloneUser) {
			return_val := *val
			return nil, &return_val, false
		}
		return nil, val, false
	} else if  val, ok := proxy.Users[buildProxyUserKey(username,"")]; ok {
		if val.Password == "" {
			if (cloneUser) {
				return_val := *val
				return nil, &return_val, true
			}
			return nil, val, true
		} else {
			return err, nil, false
		}
	} else {
		return err, nil, false
	}
}

func (proxy *ProxyContext) AddProxyUser(user *ProxyUser) string {
	key := buildProxyUserKey(user.Username,user.Password)
	proxy.Users[key] = user
	return key
}

func (proxy *ProxyContext) RemoveProxyUser(username string, password string) error {
	key := buildProxyUserKey(username,password)
	var err error
	if _, ok := proxy.Users[key]; ok {
		delete(proxy.Users, key)
	} else {
		err = errors.New("That ProxyUser does not exist")
	}
	return err
}

func (proxy *ProxyContext) AuthenticateUser(username,password string) (error, *ProxyUser) {
	
	default_user := &ProxyUser{
		Username: username,
		Password: password,
		RemoteHost: proxy.GetDefaultRemoteHost(),
		RemoteUsername: username,
		RemotePassword: password,
		EventCallbacks: make([]*EventCallback, 0),
		channelFilters: make([]*ChannelFilterFunc,0),
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
		err, user,password_blank := proxy.GetProxyUser(username, password,true)
		if (err != nil) {
			if ! proxy.RequireValidPassword {
				return nil, default_user
			} else {
				return err, nil
			}
			
		} else {
			if password_blank {
				proxy.Log.Printf("allowing any password for user: %v\n", username)
			} 
			return nil, user
		}
	} else {
		return nil, default_user
	}
}

func (proxy *ProxyContext) GetDefaultRemoteHost() string {
	return proxy.DefaultRemoteIP +":"+strconv.Itoa(proxy.DefaultRemotePort)
}

func makeProxyFromJSON(data []byte, signer ssh.Signer) (error, *ProxyContext) {
	var err error
	proxy := &ProxyContext{}
	err = json.Unmarshal(data, proxy)
	if err == nil {
		proxy.Initialize(signer)
	}
	return err, proxy
}

func (proxy *ProxyContext) Initialize(defaultSigner ssh.Signer) {

	if (proxy.Log == nil) {
		proxy.Log = log.Default()
	}

	if proxy.Users == nil {
		proxy.Users = map[string]*ProxyUser{}
	}
	if proxy.userSessions == nil {
		proxy.userSessions = map[string]map[string]*SessionContext{}
	}

	if proxy.allSessions == nil {
		proxy.allSessions = map[string]*SessionContext{}
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
			err, user, _ := proxy.GetProxyUser(viewer.User.Username, viewer.User.Password,false)
			if err == nil && user != nil{
				viewer.User = user
			} else {
				proxy.AddProxyUser(viewer.User )
			}
		}
	}
}

func (proxy *ProxyContext) HandleClientConn(client_conn *ssh.ServerConn, client_channels <-chan ssh.NewChannel, client_requests <-chan *ssh.Request, curSession *SessionContext) {


	//sess_key := client_conn.LocalAddr().String()+":"+client_conn.RemoteAddr().String()
	//curSession := proxy.allSessions[sess_key]
	//sess_key := curSession.sessionID
	curSession.markThreadStarted()
	curSession.initializeLog()
	defer curSession.markThreadStopped()
	proxy.Log.Printf("i can see password: %s\n",curSession.client_password)
	
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
		proxy.Log.Printf("Error: cannot connect to remote server %s\n",curSession.user.RemoteHost)
		return
	}

	defer remote_sock.Close()

	remote_conn, remote_channels, remote_requests, err := ssh.NewClientConn(remote_sock, curSession.user.RemoteHost, remote_server_conf)

	if err != nil {
		proxy.Log.Printf("Error creating new ssh client conn %v\n", err)
		return 
	}

	curSession.mutex.Lock()
	curSession.remote_conn = &remote_conn
	curSession.mutex.Unlock()

	shutdown_err := make(chan error, 1)
	go func() {
		shutdown_err <- remote_conn.Wait()
	}()

	proxy.AddSessionToUserList(curSession)

	start_event := SessionEvent{
		Type: EVENT_SESSION_START,
		Key: curSession.sessionID,
		ServHost: curSession.user.RemoteHost,
		ClientHost: client_conn.RemoteAddr().String(),
		Username: curSession.client_username ,
		Password: curSession.client_password,
		StartTime: curSession.getStartTimeAsUnix(),
		TimeOffset: 0,
	}
	curSession.HandleEvent(&start_event)
		
	proxy.Log.Printf("New session starting: %v\n",start_event.ToJSON())
	for {
		if (proxy.active) {
			break;
		}
		time.Sleep(ACTIVE_POLLING_DELAY)
	}
	
	

	//curSession.log_session_data()
	go curSession.HandleChannels(remote_conn, client_channels)
	go curSession.HandleChannels(client_conn, remote_channels)
	go curSession.handleRequests(client_conn, remote_requests, 0)
	go curSession.handleRequests(remote_conn, client_requests, 0)
	
	proxy.Log.Println("New session started")

	<-shutdown_err
	remote_conn.Close()
}
/*

func (proxy *ProxyContext) getSessionsKeys() []string {
	session_keys := make([]string, 0)
	for key := range proxy.userSessions {
		session_keys = append(session_keys,key)
	}
	return session_keys
}

func (proxy *ProxyContext) getUserSessionsKeys(user string) []string {
	session_keys := make([]string, 0)
	if val, ok := proxy.userSessions[user]; ok {
		for key := range val {
			session_keys = append(session_keys,key)
		}
	}
	return session_keys
}


func (proxy *ProxyContext) getUsers() []string {
	users := make([]string, 0)
	for key := range Users {
		users = append(users,users[key].Username)
	}
	return Users
}*/

func (proxy *ProxyContext) MakeSessionViewerForUser(username,password string) (error, *proxySessionViewer) {

	err,user,_ := proxy.GetProxyUser(username, password,true)

	if user != nil {
		viewer := createNewSessionViewer(SESSION_VIEWER_TYPE_LIST,proxy, user)
		proxy.AddSessionViewer(viewer)
		return err, viewer
	} 
	return err, nil
}


func (proxy *ProxyContext) MakeSessionViewerForSession(user_key string, password string, session string) (error, *proxySessionViewer) {
	err,user,_ := proxy.GetProxyUser(user_key, password,true)

	if user != nil {
		viewer := createNewSessionViewer(SESSION_VIEWER_TYPE_SINGLE, proxy, user)
		viewer.SessionKey = session
		proxy.AddSessionViewer(viewer)
		return err, viewer
	} else {
		return err, nil
	}
}


func (proxy *ProxyContext) AddSessionViewer(viewer *proxySessionViewer) {
	key := viewer.Secret
	proxy.Viewers[key] = viewer
}

func (proxy *ProxyContext) RemoveSessionViewer(key string) {
	if _, ok := proxy.Viewers[key]; ok {
		delete(proxy.Viewers, key)
	}
}

func (proxy *ProxyContext) RemoveExpiredSessions() {
	for key, val:= range proxy.Viewers {
		if val.isExpired() {
			proxy.RemoveSessionViewer(key)
		} 
	}
}

func (proxy *ProxyContext) GetSessionViewer(key string) *proxySessionViewer {
	if  val, ok := proxy.Viewers[key]; ok {
		if val.isExpired() {
			proxy.RemoveSessionViewer(key)
		} else {
			return val
		}
	}
	return nil
}

func makeListOfSessionKeys(sessions map[string]*SessionContext, include_inactive bool) []string {
	session_keys := make([]string, 0)
	
	for cur_key := range sessions {
		if sessions[cur_key].active == true || include_inactive {
			session_keys = append(session_keys, cur_key)
		}
	}

	return session_keys
}

func (proxy *ProxyContext) ListAllUserSessions(user string) []string {
	return makeListOfSessionKeys(proxy.userSessions[user],true)
}

func (proxy *ProxyContext) ListAllActiveUserSessions(user string) []string {
	return makeListOfSessionKeys(proxy.userSessions[user],false)
}



func (proxy *ProxyContext) ListAllSessions() []string {
	return makeListOfSessionKeys(proxy.allSessions,true)
}


func (proxy *ProxyContext) ListAllActiveSessions() []string {
	return makeListOfSessionKeys(proxy.allSessions,false)
}





// note: username/password combo is a 
// unique key here
type ProxyUser struct {
	Username	string
	Password	string
	RemoteHost	string
	RemoteUsername string
	RemotePassword string
	EventCallbacks []*EventCallback
	channelFilters []*ChannelFilterFunc
}


func buildProxyUserKey(user,pass string) string {
	return user + ":" + pass
}

func (user *ProxyUser) GetKey() string {
	return buildProxyUserKey(user.Username, "")
}

func (user *ProxyUser) AddEventCallback(callback *EventCallback) int {
	if user.EventCallbacks == nil {
		user.EventCallbacks = make([]*EventCallback,0)
	}
	user.EventCallbacks = append(user.EventCallbacks, callback)
	return len(user.EventCallbacks) - 1
}

func (user *ProxyUser) RemoveEventCallback(callback *EventCallback) {
	if user.EventCallbacks != nil {
		for index, value := range user.EventCallbacks {
			if value == callback {
				user.EventCallbacks[index] = user.EventCallbacks[len(user.EventCallbacks)-1]
				user.EventCallbacks = user.EventCallbacks[:len(user.EventCallbacks)-1]
			}
		}
	}
}

func (user *ProxyUser) AddChannelFilter(function *ChannelFilterFunc) int {
	if user.channelFilters == nil {
		user.channelFilters = make([]*ChannelFilterFunc,0)
	}
	user.channelFilters = append(user.channelFilters, function)
	return len(user.channelFilters) -1
}

func (user *ProxyUser) RemoveChannelFilter(function *ChannelFilterFunc) {
	if user.channelFilters != nil {
		for index, value := range user.channelFilters {
			if value == function {
				user.channelFilters[index] = user.channelFilters[len(user.channelFilters)-1]
				user.channelFilters = user.channelFilters[:len(user.channelFilters)-1]
			}
		}
	}
}


