package main


import (
	"sync"
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


// add event callback

// add event filter


//TODO: add option to start or stop web interface for controller
// update web view to add routes for individual proxies by ID


//make web view resizable box


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