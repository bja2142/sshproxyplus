package main


import (
	"sync"
	"net/http"
	"strconv"
	"log"
)

type proxyController struct {
	Proxies			map[uint64]*proxyContext
	ProxyCounter	uint64
	PresharedKey	string
	SocketType		uint16
	SocketHost		string
	socket			proxyControllerSocket
	TLSKey			string
	TLSCert			string
	mutex			sync.Mutex
	webServer		*http.Server
	WebHost			string
	WebStaticDir	string
	BaseURI			string
	log				loggerInterface
}


const PROXY_CONTROLLER_SOCKET_PLAIN				uint16 = 0
const PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET 	uint16 = 1
const PROXY_CONTROLLER_SOCKET_TLS				uint16 = 2
const PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET		uint16 = 3


// TODO:
// - define message types
// - integrate HMAC into message protocol
// - test

func (controller *proxyController) clientHandler(client proxyControllerSocketClient, socket proxyControllerSocket) {
	for {
		data, err := client.ReadLine()
		if (err != nil) {
			break;
		}
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
		proxy := controller.getProxy(numericID)
		if proxy != nil {
			proxyWebHandler := &proxyWebServer{
				proxy:proxy,
				BaseURI: controller.BaseURI,
			}
			controller.log.Printf("handling")
			proxyWebHandler.socketHandler(w,r)
		}
	}
}

func (controller *proxyController) startWebServer() error {
	controller.initializeLogger()
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

func (controller *proxyController) stopWebServer() {
	if controller.webServer != nil {
		controller.webServer.Close()
		controller.webServer = nil
	}
}

func (controller *proxyController) initializeLogger() {
	if nil == controller.log {
		controller.log = log.Default()
	}
}


func (controller *proxyController) listen() {
	controller.initializeLogger()
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
	controller.socket.ListenAndServe(controller.SocketHost, controller.clientHandler)
}

func (controller *proxyController) Stop() {
	controller.socket.Stop()
}

func (controller *proxyController) getNextProxyID() uint64 {
	controller.mutex.Lock()
		proxy_id := controller.ProxyCounter
		controller.ProxyCounter+=1
	controller.mutex.Unlock()
	return proxy_id
}

func (controller *proxyController) createProxy() uint64 {
	proxy_id := controller.getNextProxyID()
	controller.Proxies[proxy_id] = makeNewProxy()
	controller.Proxies[proxy_id].log = controller.log
	return proxy_id
} 

func (controller *proxyController) destroyProxy(proxyID uint64) {
	controller.mutex.Lock()
		if _, ok := controller.Proxies[proxyID]; ok {
			delete(controller.Proxies, proxyID)
		}
	controller.mutex.Unlock()
}

func (controller *proxyController) activateProxy(proxyID uint64) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.activate()
	}
}

func (controller *proxyController) deactivateProxy(proxyID uint64) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.deactivate()
	}
}

func (controller *proxyController) getProxy(proxyID uint64) (proxy *proxyContext) {
	proxy = nil
	controller.mutex.Lock()
		if val, ok := controller.Proxies[proxyID]; ok {
			proxy = val
		}
	controller.mutex.Unlock()
	return proxy
}

func (controller *proxyController) addExistingProxy(proxy *proxyContext) uint64 {
	proxy_id := controller.getNextProxyID()
	controller.Proxies[proxy_id] = proxy
	return proxy_id
}


func (controller *proxyController) startProxy(proxyID uint64) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.startProxy()
	}
}

func (controller *proxyController) addUserToProxy(proxyID uint64, user *proxyUser) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.addProxyUser(user)
	}
}

func (controller *proxyController) removeUserFromProxy(proxyID uint64, user *proxyUser) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		proxy.removeProxyUser(user.Username, user.Password)
	}
}

func (controller *proxyController) addEventCallbackToUser(proxyID uint64, user *proxyUser, callback *eventCallback) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		err, user, _ := proxy.getProxyUser(user.Username,user.Password)
		if (err != nil) {
			user.addEventCallback(callback)
		}
	}
}

func (controller *proxyController) removeEventCallbackFromUser(proxyID uint64, user *proxyUser, callback *eventCallback) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		err, user, _ := proxy.getProxyUser(user.Username,user.Password)
		if (err != nil) {
			user.removeEventCallback(callback)
		}
	}
}


func (controller *proxyController) addChannelFilterToUser(proxyID uint64, user *proxyUser, function *channelFilterFunc) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		err, user, _ := proxy.getProxyUser(user.Username,user.Password)
		if (err != nil) {
			user.addChannelFilter(function)
		}
	}
}

func (controller *proxyController) removeChannelFilterFromUser(proxyID uint64, user *proxyUser, function *channelFilterFunc) {
	proxy := controller.getProxy(proxyID)
	if proxy != nil {
		err, user, _ := proxy.getProxyUser(user.Username,user.Password)
		if (err != nil) {
			user.removeChannelFilter(function)
		}
	}
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