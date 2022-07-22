package main


import (

)

type proxyController struct {
	proxies			map[uint64]*proxyContext
	proxyCounter	uint64
	presharedKey	string
	socketType		uint16
	socketHost		string
	socket			proxyControllerSocket
	TLSKey			string
	TLSCert			string
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
		err = client.SendLine(data)
		if (err != nil) {
			break;
		}
	}
}

func (controller *proxyController) listen() {
	switch controller.socketType {
	case PROXY_CONTROLLER_SOCKET_PLAIN:
		controller.socket = &proxyControllerSocketTCP{plaintext: true}
	case PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET:
		controller.socket = &proxyControllerSocketWeb{plaintext: true}

	case PROXY_CONTROLLER_SOCKET_TLS:
		controller.socket = &proxyControllerSocketTCP{TLSCert: controller.TLSCert, TLSKey: controller.TLSKey}

	case PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET:
		controller.socket = &proxyControllerSocketWeb{TLSCert: controller.TLSCert, TLSKey: controller.TLSKey}

	}
	controller.socket.ListenAndServe(controller.socketHost, controller.clientHandler)
}

func (controller *proxyController) Stop() {
	controller.socket.Stop()
}

func (controller *proxyController) createProxy() uint64 {
	return 0
} 

func (controller *proxyController) destroyProxy(proxyID uint64) {

}

func (controller *proxyController) activateProxy(proxyID uint64) {

}

func (controller *proxyController) deactivateProxy(proxyID uint64) {
	
}

func (controller *proxyController) getProxy(proxyID uint64) *proxyContext {
	return nil
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