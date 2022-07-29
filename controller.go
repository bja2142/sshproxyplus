package main


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

type proxyController struct {
	Proxies				map[uint64]*proxyContext
	ProxyCounter		uint64
	PresharedKey		string
	SocketType			uint16
	SocketHost			string
	socket				proxyControllerSocket
	TLSKey				string
	TLSCert				string
	mutex				sync.Mutex
	webServer			*http.Server
	WebHost				string
	WebStaticDir		string
	BaseURI				string
	log					loggerInterface
	defaultSigner		ssh.Signer
	channelFilters		map[string]*channelFilterFunc
	eventCallbacks		map[string]*eventCallback
}


const PROXY_CONTROLLER_SOCKET_PLAIN				uint16 = 0
const PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET 	uint16 = 1
const PROXY_CONTROLLER_SOCKET_TLS				uint16 = 2
const PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET		uint16 = 3




func (controller *proxyController) clientHandler(client proxyControllerSocketClient, socket proxyControllerSocket) {
	for {
		data, err := client.ReadLine()
		if (err != nil) {
			break;
		}
		messageWrapper :=controllerHMAC{}
		err = json.Unmarshal(data,&messageWrapper)
		if(err == nil) {
			err, message := messageWrapper.verify([]byte(controller.PresharedKey))
			if (err == nil) {
				data = message.handleMessage(controller)
			} else {
				controller.log.Println("error during verify", err)
			}
		}  else {
			controller.log.Println("error during unmarshal",err)
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
func (controller *proxyController) handleWebProxyRequest(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	numericID,err := strconv.ParseUint(id, 10, 64)
	controller.log.Printf("Got websocket request for proxy with id: %v\n",numericID)
	if err == nil {
		proxy, _ := controller.getProxy(numericID)
		if proxy != nil {
			proxyWebHandler := &proxyWebServer{
				proxy:proxy,
				BaseURI: controller.BaseURI,
			}
			proxyWebHandler.socketHandler(w,r)
		}
	}
}

func (controller *proxyController) startWebServer() error {
	
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
		controller.log.Printf("Starting plaintext web server: %v\n",controller.WebHost)
		err = controller.webServer.ListenAndServe()
	} else {
		controller.log.Printf("Starting TLS web server: %v\n",controller.WebHost)
		err = controller.webServer.ListenAndServeTLS(controller.TLSCert, controller.TLSKey)
	}

	if (err != nil)	{
		controller.log.Println("Error creating web server:",err.Error())
		return err
	} 
	return nil
}

func (controller *proxyController) exportControllerAsJSON() ([]byte, error) {
	data, err := json.MarshalIndent(controller,"","    ")
	if err != nil {
		controller.log.Println("Error during marshaling json: ", err)
		return []byte(""), err
	}
	return data, err
}

func (controller *proxyController) writeControllerConfigToFile(filepath string) error {
	data, err := controller.exportControllerAsJSON()
	if err == nil {
		err = os.WriteFile(filepath, data, 0600)
		if(err != nil) {
			controller.log.Println("Error writing to file:",err)
		} else {
			controller.log.Printf("Wrote config to file:%v\n",filepath)
		}
	}
	return err
}

// TODO: write test case for this function 
func loadControllerConfigFromFile(filepath string, signer ssh.Signer) (error, *proxyController) {
	data, err := os.ReadFile(filepath)
	if(err != nil) {
		return err,nil
	}
	controller := &proxyController{}
	err = json.Unmarshal(data,controller)

	if err == nil {
		if signer != nil {
			controller.defaultSigner = signer
		}

		controller.initialize()

		
	}
	return err, controller
}

func (controller *proxyController) updateProxiesWithCurrentLogger(overwrite bool) {
	for _,proxy := range controller.Proxies {
		if proxy.log == nil || overwrite {
			proxy.log = controller.log
		}
		
	}
}

func (controller *proxyController) useNewLogger(logger loggerInterface) {
	controller.log = logger
	controller.updateProxiesWithCurrentLogger(true)

}


func (controller *proxyController) stopWebServer() {
	if controller.webServer != nil {
		controller.webServer.Close()
		controller.webServer = nil
	}
}

func (controller *proxyController) initialize() {

	if nil == controller.log {
		controller.useNewLogger(log.Default())
	}

	if controller.channelFilters == nil {
		controller.channelFilters = make(map[string]*channelFilterFunc)
	}
	if controller.Proxies == nil {
		controller.Proxies = make(map[uint64]*proxyContext)
	}

	if controller.defaultSigner == nil {
		var err error
		controller.defaultSigner, err = generateSigner()
		if err != nil {
			controller.log.Println("unable generate signer: ", err)
		}
	}

	for _, proxy := range controller.Proxies {
		proxy.initialize(controller.defaultSigner)
	}
	controller.updateProxiesWithCurrentLogger(false)
}


func (controller *proxyController) listen() {
	
	switch controller.SocketType {
	case PROXY_CONTROLLER_SOCKET_PLAIN:
		controller.socket = &proxyControllerSocketTCP{plaintext: true}
	case PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET:
		controller.socket = &proxyControllerSocketWeb{plaintext: true}
	case PROXY_CONTROLLER_SOCKET_TLS:
		controller.socket = &proxyControllerSocketTCP{TLSCert: controller.TLSCert, TLSKey: controller.TLSKey}
	case PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET:
		controller.socket = &proxyControllerSocketWeb{TLSCert: controller.TLSCert, TLSKey: controller.TLSKey}
	}
	go controller.socket.ListenAndServe(controller.SocketHost, controller.clientHandler)
}

func (controller *proxyController) Stop() {
	if controller.socket != nil {
		controller.socket.Stop()
	}
	controller.stopProxies()
	controller.stopWebServer()
	
}

func (controller *proxyController) getNextProxyID() uint64 {
	controller.mutex.Lock()
		proxy_id := controller.ProxyCounter
		controller.ProxyCounter+=1
	controller.mutex.Unlock()
	return proxy_id
}

func (controller *proxyController) createProxy() uint64 {
	return controller.addProxy(makeNewProxy(controller.defaultSigner))
}

func (controller *proxyController) addProxyFromJSON(data []byte) (error,uint64) {
	err,newProxy := makeProxyFromJSON(data, controller.defaultSigner)
	var proxyID uint64
	if (err == nil) {
		proxyID = controller.addProxy(newProxy)
	}
	return err, proxyID
}

func (controller *proxyController) destroyProxy(proxyID uint64) (err error) {
	controller.mutex.Lock()
		if proxy, ok := controller.Proxies[proxyID]; ok {
			delete(controller.Proxies, proxyID)
			proxy.Stop()
		} else {
			err = errors.New(fmt.Sprintf("No proxy with this ID exists: %v", proxyID))
		}
	controller.mutex.Unlock()
	return err
}

func (controller *proxyController) activateProxy(proxyID uint64) error {
	proxy, err := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.activate()
	}
	return err
}

func (controller *proxyController) addUserToProxy(proxyID uint64, user *proxyUser) (error, string) {
	proxy, err := controller.getProxy(proxyID)
	var key string
	if (proxy != nil) {
		key = proxy.addProxyUser(user)
	}
	return err, key
}

func (controller *proxyController) removeUserFromProxy(proxyID uint64, username, password string) (error) {
	proxy, err := controller.getProxy(proxyID)
	if (proxy != nil) {
		err = proxy.removeProxyUser(username,password)
	}
	return err
}

func (controller *proxyController) deactivateProxy(proxyID uint64) error {
	proxy, err := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.deactivate()
	}
	return err
}

func (controller *proxyController) getProxy(proxyID uint64) (proxy *proxyContext, err error) {
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

func (controller *proxyController) addProxy(proxy *proxyContext) uint64 {
	proxy_id := controller.addExistingProxy(proxy)
	controller.Proxies[proxy_id].log = controller.log
	return proxy_id
}

func (controller *proxyController) addExistingProxy(proxy *proxyContext) uint64 {
	proxy_id := controller.getNextProxyID()
	controller.mutex.Lock()
	controller.Proxies[proxy_id] = proxy
	controller.mutex.Unlock()
	return proxy_id
}


func (controller *proxyController) startProxy(proxyID uint64) error {
	proxy, err := controller.getProxy(proxyID)
	if proxy != nil {
		go proxy.startProxy()
	}
	return err
}

func (controller *proxyController) stopProxy(proxyID uint64) error {
	proxy, err := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.Stop()
	}
	return err
}

func (controller *proxyController) stopProxies() {
	controller.mutex.Lock()
	for _,proxy := range controller.Proxies {
		proxy.Stop()
	}
	controller.mutex.Unlock()
}

func (controller *proxyController) addEventCallbackToUser(proxyID uint64, username, password string, callback *eventCallback) (error, string) {
	var key string
	proxy, err := controller.getProxy(proxyID)
	if proxy != nil {
		var user *proxyUser
		err, user, _ = proxy.getProxyUser(username,password,false)
		if (err != nil) {
			index := user.addEventCallback(callback)
			key = fmt.Sprintf("callback-proxy%v-%s-%s-%v",proxyID,username,password,index)
			_, ok := controller.eventCallbacks[key];
			for ! ok {
				key = key + "."
				_, ok = controller.eventCallbacks[key];
			}
			controller.eventCallbacks[key] = callback
		}
	}
	return err, key
}

func (controller *proxyController) removeEventCallbackFromUserByKey(proxyID uint64, username, password, key string) error  {
	var err error
	if _, ok := controller.eventCallbacks[key]; ok {
		err = controller.removeEventCallbackFromUser(proxyID, username, password, controller.eventCallbacks[key])
		delete(controller.eventCallbacks, key)
	} else {
		err = errors.New("could not find channel filter key")
	}
	return err
}

func (controller *proxyController) removeEventCallbackFromUser(proxyID uint64, username, password string, callback *eventCallback) error {
	var err error 
	proxy, _ := controller.getProxy(proxyID)
	if proxy != nil {
		var user *proxyUser
		err, user, _ = proxy.getProxyUser(username, password,false)
		if (err != nil) {
			user.removeEventCallback(callback)
		}
	}
	return err
}

func (controller *proxyController) getProxyViewers(proxyID uint64) (error, map[string]*proxySessionViewer) {
	proxy, err := controller.getProxy(proxyID)
	if err == nil {
		proxy.removeExpiredSessions()
		return err, proxy.Viewers
	} else {
		return err, make(map[string]*proxySessionViewer)
	}
}

func (controller *proxyController) getProxyViewerByViewerKey(proxyID uint64, viewerKey string) (error, *proxySessionViewer) {
	proxy, err := controller.getProxy(proxyID)
	var viewer *proxySessionViewer
	if (err == nil) {
		viewer = proxy.getSessionViewer(viewerKey)
		if (viewer == nil) {
			err = errors.New("Could not find viewer with that key.")
		}
	}
	return err, viewer
}

func (controller *proxyController) getProxyViewerBySessionKey(proxyID uint64, sessionKey string) (error, *proxySessionViewer) {
	var finalViewer *proxySessionViewer
	err, allViewers := controller.getProxyViewers(proxyID)
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

func (controller *proxyController) getProxyViewersBySessionKey(proxyID uint64, sessionKey string) (error, []*proxySessionViewer) {
	finalViewers := make([]*proxySessionViewer,0)
	err, allViewers := controller.getProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			if sessionKey == viewer.SessionKey {
				finalViewers = append(finalViewers, viewer)
			}
		}
	}
	return err, finalViewers
}

func (controller *proxyController) getProxyViewerByUsername(proxyID uint64, username string) (error, *proxySessionViewer) {
	var finalViewer *proxySessionViewer
	err, allViewers := controller.getProxyViewers(proxyID)
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

func (controller *proxyController) getProxyViewersByUsername(proxyID uint64, username string) (error, []*proxySessionViewer) {
	finalViewers := make([]*proxySessionViewer,0)
	err, allViewers := controller.getProxyViewers(proxyID)
	if(err == nil) {
		for _,viewer := range allViewers {
			if username == viewer.User.Username {
				finalViewers = append(finalViewers, viewer)
			}
		}
	}
	return err, finalViewers
}

func (controller *proxyController) createSessionViewer(proxyID uint64, username, sessionKey string) (error, *proxySessionViewer) {
	var viewer *proxySessionViewer
	proxy, err := controller.getProxy(proxyID)
	if(proxy != nil) {
		err, viewer = proxy.makeSessionViewerForSession(username, sessionKey)
	}
	return err, viewer
}

func (controller *proxyController) createUserSessionViewer(proxyID uint64, username string) (error, *proxySessionViewer) {
	var viewer *proxySessionViewer
	proxy, err := controller.getProxy(proxyID)
	if(proxy != nil) {
		err, viewer = proxy.makeSessionViewerForUser(username)
	}
	return err, viewer
}

//TODO add getProxyUserClone


func (controller *proxyController) addChannelFilterToUser(proxyID uint64, username, password string, function *channelFilterFunc) (error,string) {
	proxy, _ := controller.getProxy(proxyID)
	var err error
	var user *proxyUser
	var key string
	if proxy != nil {
		err, user, _ = proxy.getProxyUser(username, password,true)
		if (err != nil) {
			index := user.addChannelFilter(function)
			key = fmt.Sprintf("filter-proxy%v-%s-%s-%v",proxyID,username,password,index)
			_, ok := controller.channelFilters[key];
			for  ! ok {
				key = key + "."
				_, ok = controller.channelFilters[key];
			}
			controller.channelFilters[key] = function
		}
	}
	return err, key
}
func (controller *proxyController) removeChannelFilterFromUserByKey(proxyID uint64, username, password, key string) error {
	var err error
	if _, ok := controller.channelFilters[key]; ok {
		err = controller.removeChannelFilterFromUser(proxyID, username, password, controller.channelFilters[key])
		delete(controller.channelFilters, key)
	} else {
		err = errors.New("could not find channel filter key")
	}
	return err
}

func (controller *proxyController) removeChannelFilterFromUser(proxyID uint64, username, password string, function *channelFilterFunc) error {
	var err error 
	proxy, _ := controller.getProxy(proxyID)
	if proxy != nil {
		var user *proxyUser
		err, user, _ = proxy.getProxyUser(username,password,false)
		if (err != nil) {
			user.removeChannelFilter(function)
		}
	}
	return err 
}



// taken from: https://github.com/cmoog/sshproxy/blob/5448845f823a2e6ec7ba6bb7ddf1e5db786410d4/_examples/main.go
func generateSigner() (ssh.Signer, error) { 
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
by session ID OR by proxyUser. In either case, proxyUser
will track it the eventHook and session will look
there for hooks.
(event hooks can either be intercept or not intercept,
if they intercept, then they block and can modify the
value of the data. if they do not intercept, they
do not block but cannot modify data)


*/