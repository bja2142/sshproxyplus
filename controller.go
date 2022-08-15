package sshproxyplus


import (
	"sync"
	"net/http"
	"strconv"
	"log"
	"encoding/json"
	"os"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"golang.org/x/crypto/ssh"
	"fmt"
	"encoding/pem"
	"errors"
)


/*
 The ProxyController object
 can be managed directly via the
 software API, or over a controller
 socket. 

 It can be used to create  and
 destroy proxies,
 start and stop proxies, add ProxyUsers
 to proxies, create filters and callbacks
 for users, and to host a web interface
 which can be used to view sessions
 in real time. 

 It also provides a socket that can be
 used to remotely manage the controller.
 The remote tool must have the same
 preshared key. The key is used to send
 HMAC-signed JSON blobs for execution by
 the controller.


*/
type ProxyController struct {
	Proxies				map[uint64]*ProxyContext
	ProxyCounter		uint64
	// Used to authenticate commands 
	// sent over the controller socket
	// from a remote server
	PresharedKey		string 
	SocketType			uint16
	SocketHost			string
	socket				ProxyControllerSocket
	TLSKey				string
	TLSCert				string
	mutex				sync.Mutex
	webServer			*http.Server
	WebHost				string
	WebStaticDir		string
	BaseURI				string
	Log					LoggerInterface	`json:"-"`
	DefaultSigner		ssh.Signer	`json:"-"`
	channelFilters		map[string]*ChannelFilterFunc
	EventCallbacks		map[string]*EventCallback
}







func (controller *ProxyController) clientHandler(client ProxyControllerSocketClient, socket ProxyControllerSocket) {
	for {
		data, err := client.ReadLine()
		if (err != nil) {
			break;
		}
		messageWrapper :=ControllerHMAC{}
		err = json.Unmarshal(data,&messageWrapper)
		if(err == nil) {
			err, message := messageWrapper.Verify([]byte(controller.PresharedKey))
			if (err == nil) {
				data = message.HandleMessage(controller)
			} else {
				controller.Log.Println("error during verify", err)
			}
		}  else {
			controller.Log.Println("error during unmarshal",err)
		}
		// echo back commands on error
		err = client.SendLine(data)
		if (err != nil) {
			break;
		}
		
	}
}


// if performance is an issue, don't need to make a new handler every time.
// reuse after first go.
func (controller *ProxyController) handleWebProxyRequest(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	numericID,err := strconv.ParseUint(id, 10, 64)
	controller.Log.Printf("Got websocket request for proxy with id: %v\n",numericID)
	if err != nil {
		numericID = 0
	}
	proxy, _ := controller.GetProxy(numericID)
	if proxy != nil {
		proxyWebHandler := &proxyWebServer{
			proxy:proxy,
			BaseURI: controller.BaseURI,
		}
		proxyWebHandler.socketHandler(w,r)
	}
	
}

func (controller *ProxyController) StartWebServer() error {
	
	if controller.webServer == nil {
		serverMux := http.NewServeMux()
		fileServe := http.FileServer(http.Dir(controller.WebStaticDir))

		serverMux.Handle("/",fileServe)
		serverMux.HandleFunc("/proxysocket/", controller.handleWebProxyRequest)
		controller.webServer = &http.Server{
			Handler: serverMux,
			Addr:	controller.WebHost,
		}
	}
	var err error
	if controller.socket.IsPlaintext() {
		controller.Log.Printf("Starting plaintext web server: %v\n",controller.WebHost)
		err = controller.webServer.ListenAndServe()
	} else {
		controller.Log.Printf("Starting TLS web server: %v\n",controller.WebHost)
		err = controller.webServer.ListenAndServeTLS(controller.TLSCert, controller.TLSKey)
	}

	if (err != nil)	{
		controller.Log.Println(err.Error())
		return err
	} 
	return nil
}

func (controller *ProxyController) ExportControllerAsJSON() ([]byte, error) {
	data, err := json.MarshalIndent(controller,"","    ")
	if err != nil {
		controller.Log.Println("Error during marshaling json: ", err)
		return []byte(""), err
	}
	return data, err
}

func (controller *ProxyController) WriteControllerConfigToFile(filepath string) error {
	data, err := controller.ExportControllerAsJSON()
	if err == nil {
		err = os.WriteFile(filepath, data, 0600)
		if(err != nil) {
			controller.Log.Println("Error writing to file:",err)
		} else {
			controller.Log.Printf("Wrote config to file:%v\n",filepath)
		}
	}
	return err
}

// TODO: write test case for this function 


func LoadControllerConfigFromFile(filepath string, signer ssh.Signer) (error, *ProxyController) {
	data, err := os.ReadFile(filepath)
	if(err != nil) {
		return err,nil
	}
	controller := &ProxyController{}
	err = json.Unmarshal(data,controller)

	if err == nil {
		if signer != nil {
			controller.DefaultSigner = signer
		}

		controller.Initialize()

		
	}
	return err, controller
}

//

func (controller *ProxyController) UpdateProxiesWithCurrentLogger(overwrite bool) {
	for _,proxy := range controller.Proxies {
		if proxy.Log == nil || overwrite {
			proxy.Log = controller.Log
		}
		
	}
}

func (controller *ProxyController) UseNewLogger(logger LoggerInterface) {
	controller.Log = logger
	controller.UpdateProxiesWithCurrentLogger(true)

}


func (controller *ProxyController) StopWebServer() {
	if controller.webServer != nil {
		controller.webServer.Close()
		controller.webServer = nil
	}
}

func (controller *ProxyController) Initialize() {

	if nil == controller.Log {
		controller.UseNewLogger(log.Default())
	}

	if controller.channelFilters == nil {
		controller.channelFilters = make(map[string]*ChannelFilterFunc)
	}
	if controller.EventCallbacks == nil {
		controller.EventCallbacks = make(map[string]*EventCallback)
	}
	if controller.Proxies == nil {
		controller.Proxies = make(map[uint64]*ProxyContext)
	}

	if controller.DefaultSigner == nil {
		var err error
		controller.DefaultSigner, err = GenerateSigner()
		if err != nil {
			controller.Log.Println("unable generate signer: ", err)
		}
	}

	for _, proxy := range controller.Proxies {
		proxy.Initialize(controller.DefaultSigner)
	}
	controller.UpdateProxiesWithCurrentLogger(false)
	
}

func (controller *ProxyController) InitializeSocket() {
	if(controller.socket == nil) {
		switch controller.SocketType {
		case PROXY_CONTROLLER_SOCKET_PLAIN:
			controller.socket = &ProxyControllerSocketTCP{plaintext: true}
		case PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET:
			controller.socket = &ProxyControllerSocketWeb{plaintext: true}
		case PROXY_CONTROLLER_SOCKET_TLS:
			controller.socket = &ProxyControllerSocketTCP{TLSCert: controller.TLSCert, TLSKey: controller.TLSKey}
		case PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET:
			controller.socket = &ProxyControllerSocketWeb{TLSCert: controller.TLSCert, TLSKey: controller.TLSKey}
		default:
			return
		}
	}
}
func (controller *ProxyController) Listen() {
	controller.InitializeSocket()
	go controller.socket.ListenAndServe(controller.SocketHost, controller.clientHandler)
}

func (controller *ProxyController) Stop() {
	if controller.socket != nil {
		controller.socket.Stop()
	}
	controller.StopProxies()
	controller.StopWebServer()
	
}

func (controller *ProxyController) GetNextProxyID() uint64 {
	controller.mutex.Lock()
		proxy_id := controller.ProxyCounter
		controller.ProxyCounter+=1
	controller.mutex.Unlock()
	return proxy_id
}

func (controller *ProxyController) CreateProxy() uint64 {
	return controller.AddProxy(MakeNewProxy(controller.DefaultSigner))
}

func (controller *ProxyController) AddProxyFromJSON(data []byte) (error,uint64) {
	err,newProxy := makeProxyFromJSON(data, controller.DefaultSigner)
	var proxyID uint64
	if (err == nil) {
		proxyID = controller.AddProxy(newProxy)
	}
	return err, proxyID
}

func (controller *ProxyController) DestroyProxy(proxyID uint64) (err error) {
	controller.mutex.Lock()
		if proxy, ok := controller.Proxies[proxyID]; ok {
			if(proxy.running) {
				proxy.Stop()
			}
			
			delete(controller.Proxies, proxyID)
		} else {
			err = errors.New(fmt.Sprintf("No proxy with this ID exists: %v", proxyID))
		}
	controller.mutex.Unlock()
	return err
}

func (controller *ProxyController) ActivateProxy(proxyID uint64) error {
	proxy, err := controller.GetProxy(proxyID)
	if proxy != nil {
		proxy.Activate()
	}
	return err
}

func (controller *ProxyController) AddUserToProxy(proxyID uint64, user *ProxyUser) (error, string) {
	proxy, err := controller.GetProxy(proxyID)
	var key string
	if (proxy != nil) {
		key = proxy.AddProxyUser(user)
	}
	return err, key
}


func (controller *ProxyController) RemoveUserFromProxy(proxyID uint64, username, password string) (error) {
	proxy, err := controller.GetProxy(proxyID)
	if (proxy != nil) {
		err = proxy.RemoveProxyUser(username,password)
	}
	return err
}

func (controller *ProxyController) DeactivateProxy(proxyID uint64) error {
	proxy, err := controller.GetProxy(proxyID)
	if proxy != nil {
		proxy.Deactivate()
	}
	return err
}

func (controller *ProxyController) GetProxy(proxyID uint64) (proxy *ProxyContext, err error) {
	proxy = nil
	err = nil
	controller.mutex.Lock()
		if val, ok := controller.Proxies[proxyID]; ok {
			proxy = val
		} else {
			err = errors.New(fmt.Sprintf("Cannot find proxy with ID: %v", proxyID))
		}
	controller.mutex.Unlock()
	return proxy, err
}

func (controller *ProxyController) AddProxy(proxy *ProxyContext) uint64 {
	proxy_id := controller.AddExistingProxy(proxy)
	controller.Proxies[proxy_id].Log = controller.Log
	return proxy_id
}

func (controller *ProxyController) AddExistingProxy(proxy *ProxyContext) uint64 {
	proxy_id := controller.GetNextProxyID()
	controller.mutex.Lock()
	controller.Proxies[proxy_id] = proxy
	controller.mutex.Unlock()
	return proxy_id
}


func (controller *ProxyController) StartProxy(proxyID uint64) error {
	proxy, err := controller.GetProxy(proxyID)
	if proxy != nil {
		go proxy.StartProxy()
	}
	return err
}

func (controller *ProxyController) StopProxy(proxyID uint64) error {
	proxy, err := controller.GetProxy(proxyID)
	if proxy != nil {
		proxy.Stop()
	}
	return err
}

func (controller *ProxyController) StopProxies() {
	controller.mutex.Lock()
	for _,proxy := range controller.Proxies {
		proxy.Stop()
	}
	controller.mutex.Unlock()
}

func (controller *ProxyController) AddEventCallbackToUser(proxyID uint64, username, password string, callback *EventCallback) (error, string) {
	var key string
	proxy, err := controller.GetProxy(proxyID)
	if proxy != nil {
		var user *ProxyUser
		err, user, _ = proxy.GetProxyUser(username,password,false)
		if (err == nil) {
			index := user.AddEventCallback(callback)
			key = fmt.Sprintf("callback-proxy%v-%s-%s-%v",proxyID,username,password,index)
			_, ok := controller.EventCallbacks[key];
			for ok {
				key = key + "."
				_, ok = controller.EventCallbacks[key];
			}
			controller.EventCallbacks[key] = callback
		}
	}
	return err, key
}

func (controller *ProxyController) RemoveEventCallbackFromUserByKey(proxyID uint64, username, password, key string) error  {
	var err error
	if _, ok := controller.EventCallbacks[key]; ok {
		err = controller.RemoveEventCallbackFromUser(proxyID, username, password, controller.EventCallbacks[key])
		delete(controller.EventCallbacks, key)
	} else {
		err = errors.New("could not find channel filter key")
	}
	return err
}

func (controller *ProxyController) RemoveEventCallbackFromUser(proxyID uint64, username, password string, callback *EventCallback) error {
	var err error 
	proxy, _ := controller.GetProxy(proxyID)
	if proxy != nil {
		var user *ProxyUser
		err, user, _ = proxy.GetProxyUser(username, password,false)
		if (err == nil) {
			user.RemoveEventCallback(callback)
		}
	}
	return err
}

func (controller *ProxyController) GetProxyViewers(proxyID uint64) (error, map[string]*proxySessionViewer) {
	proxy, err := controller.GetProxy(proxyID)
	if err == nil {
		proxy.RemoveExpiredSessions()
		return err, proxy.Viewers
	} else {
		return err, make(map[string]*proxySessionViewer)
	}
}

func (controller *ProxyController) GetProxyViewerByViewerKey(proxyID uint64, viewerKey string) (error, *proxySessionViewer) {
	proxy, err := controller.GetProxy(proxyID)
	var viewer *proxySessionViewer
	if (err == nil) {
		viewer = proxy.GetSessionViewer(viewerKey)
		if (viewer == nil) {
			err = errors.New("Could not find viewer with that key.")
		}
	}
	return err, viewer
}

/*
Note that if multiple viewers exist for the same session key, this will
only return the first one and stop looking
*/

func (controller *ProxyController) GetProxyViewerBySessionKey(proxyID uint64, sessionKey string) (error, *proxySessionViewer) {
	var finalViewer *proxySessionViewer
	err, allViewers := controller.GetProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			if sessionKey == viewer.SessionKey {
				finalViewer = viewer
				break
			}
		}
	}
	return err, finalViewer
}



func (controller *ProxyController) GetProxyViewersBySessionKey(proxyID uint64, sessionKey string) (error, []*proxySessionViewer) {
	finalViewers := make([]*proxySessionViewer,0)
	err, allViewers := controller.GetProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			if sessionKey == viewer.SessionKey {
				finalViewers = append(finalViewers, viewer)
			}
		}
	}
	return err, finalViewers
}


/*
Note that if multiple viewers exist for the same user, this will
only return the first one and stop looking
*/

func (controller *ProxyController) GetProxyViewerByUsername(proxyID uint64, username string) (error, *proxySessionViewer) {
	var finalViewer *proxySessionViewer
	err, allViewers := controller.GetProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			if username == viewer.User.Username {
				finalViewer = viewer
				break
			}
		}
	}
	return err, finalViewer
}

func (controller *ProxyController) GetProxyViewersByUsername(proxyID uint64, username string) (error, []*proxySessionViewer) {
	finalViewers := make([]*proxySessionViewer,0)
	err, allViewers := controller.GetProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			if username == viewer.User.Username {
				finalViewers = append(finalViewers, viewer)
			}
		}
	}
	return err, finalViewers
}

func (controller *ProxyController) GetProxyViewersAsList(proxyID uint64) (error, []*proxySessionViewer) {
	finalViewers := make([]*proxySessionViewer,0)
	err, allViewers := controller.GetProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			finalViewers = append(finalViewers, viewer)
		}
	}
	return err, finalViewers
}

func (controller *ProxyController) CreateSessionViewer(proxyID uint64, username, password, sessionKey string) (error, *proxySessionViewer) {
	var viewer *proxySessionViewer
	proxy, err := controller.GetProxy(proxyID)
	if(proxy != nil) {
		err, viewer = proxy.MakeSessionViewerForSession(username, password, sessionKey)
	}
	return err, viewer
}

func (controller *ProxyController) CreateUserSessionViewer(proxyID uint64, username, password string) (error, *proxySessionViewer) {
	var viewer *proxySessionViewer
	proxy, err := controller.GetProxy(proxyID)
	if(proxy != nil) {
		err, viewer = proxy.MakeSessionViewerForUser(username,password)
	}
	return err, viewer
}

//TODO add getProxyUserClone


func (controller *ProxyController) AddChannelFilterToUser(proxyID uint64, username, password string, function *ChannelFilterFunc) (error,string) {
	proxy, err := controller.GetProxy(proxyID)
	var user *ProxyUser
	var key string
	if proxy != nil {
		err, user, _ = proxy.GetProxyUser(username, password,false)
		if (err == nil) {
			index := user.AddChannelFilter(function)
			key = fmt.Sprintf("filter-proxy%v-%s-%s-%v",proxyID,username,password,index)
			_, ok := controller.channelFilters[key];
			for  ok {
				key = key + "."
				_, ok = controller.channelFilters[key];
			}
			controller.channelFilters[key] = function
		}
	}
	return err, key
}
func (controller *ProxyController) RemoveChannelFilterFromUserByKey(proxyID uint64, username, password, key string) error {
	var err error
	if _, ok := controller.channelFilters[key]; ok {
		err = controller.RemoveChannelFilterFromUser(proxyID, username, password, controller.channelFilters[key])
		delete(controller.channelFilters, key)
	} else {
		err = errors.New("could not find channel filter key")
	}
	return err
}

func (controller *ProxyController) RemoveChannelFilterFromUser(proxyID uint64, username, password string, function *ChannelFilterFunc) error {
	var err error 
	proxy, _ := controller.GetProxy(proxyID)
	if proxy != nil {
		var user *ProxyUser
		err, user, _ = proxy.GetProxyUser(username,password,false)
		if (err == nil) {
			user.RemoveChannelFilter(function)
		}
	}
	return err 
}

// TODO:
func (controller *ProxyController) removeViewerFromProxy(proxyID uint64, viewerKey string) error {
	return nil
}



// taken from: https://github.com/cmoog/sshproxy/blob/5448845f823a2e6ec7ba6bb7ddf1e5db786410d4/_examples/main.go
func GenerateSigner() (ssh.Signer, error) { 
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


/*

The controller should provide a third-party application 
the ability to create and manage proxies over a socket.

Each request is signed using HMAC and a pre-shared key

Sockets can either be plaintext or TLS,
or can be a web socket or a plain socket

Controller should enable hooking specific user sessions 
by session ID OR by ProxyUser. In either case, proxyUser
will track it the eventHook and session will look
there for hooks.
(event hooks can either be intercept or not intercept,
if they intercept, then they block and can modify the
value of the data. if they do not intercept, they
do not block but cannot modify data)


*/