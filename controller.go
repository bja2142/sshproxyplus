package main


import (
	"net"
	"sync"
	"fmt"
	"bufio"
)

const PROXY_CONTROLLER_SOCKET_PLAIN				uint16 = 0
const PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET 	uint16 = 1
const PROXY_CONTROLLER_SOCKET_TLS				uint16 = 2
const PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET		uint16 = 3


type proxyController struct {
	proxies			map[uint64]*proxyContext
	proxyCounter	uint64
	presharedKey	string
	socketType		uint16
	socketHost		string
	socket			proxyControllerSocket
}

type proxyControllerSocketPlain struct {
	listener net.Listener
	active	bool
	clients []net.Conn
	clientMutex sync.Mutex
}

type proxyControllerSocketPlainClient struct {
	net.Conn
}

func (client *proxyControllerSocketPlainClient) 	SendLine(data []byte) error {
	_, err := client.Write(data)
	if(err != nil)	{
		fmt.Println("Error writing to plain socket: ", err.Error())
	}
	return err
}

func (client *proxyControllerSocketPlainClient) 	ReadLine() ([]byte, error) {
	reader := bufio.NewReader(client)
	return reader.ReadBytes('\n')
} 



func (socket *proxyControllerSocketPlain) ListenAndServe(host string, handler proxyControllerSocketHandler) error {
	var err error
	socket.listener, err = net.Listen("tcp", host)
	socket.active = true
	if (err != nil)	{
		fmt.Println("Error creating plaint TCP socket:",err.Error())
		return err
	}
	defer socket.Stop()

	for socket.active {
		client, err := socket.listener.Accept()
		if err != nil {
				fmt.Println("Error accepting: ", err.Error())
				break;
		} else {
			socket.clientMutex.Lock()
				socket.clients = append(socket.clients, client)
			socket.clientMutex.Unlock()
			go func(client net.Conn, socket *proxyControllerSocketPlain) {
				defer socket.clientClose(client)
				handler(&proxyControllerSocketPlainClient{client},socket)	
			}(client,socket)
			
		}
	}
	return nil
}

func (socket *proxyControllerSocketPlain) clientClose(client net.Conn) {
	socket.clientMutex.Lock()
		for index, curClient := range socket.clients {
			if curClient == client {
				if(len(socket.clients) > index+1) {
					socket.clients = append(socket.clients[:index], socket.clients[index+1:]...)
				} else {
					socket.clients = socket.clients[:index]
				}
				client.Close();
				break;
			}
		}
	socket.clientMutex.Unlock()
}

func (socket *proxyControllerSocketPlain) Stop() {
	if(socket.active) {
		socket.active = false
		socket.clientMutex.Lock()
			for _, client := range socket.clients {
				client.Close();
			}
			socket.clients = make([]net.Conn,0)
		socket.clientMutex.Unlock()
	}
}
/*
type proxyControllerSocketTLS struct {
	receiveHandler 	proxyControllerSocketHandler
	TLSCert		   	string
	TLSKey			string
}
type proxyControllerSocketWeb struct {
	receiveHandler *func
}
type proxyControllerSocketWebTLS struct {
	receiveHandler *func
	TLSCert		   	string
	TLSKey			string
}*/


type proxyControllerSocketHandler func(proxyControllerSocketClient, proxyControllerSocket)

type proxyControllerSocketClient interface {
	SendLine(data []byte) error
	ReadLine() ([]byte, error)
}

type proxyControllerSocket interface {
	ListenAndServe(host string, handler proxyControllerSocketHandler) error
	Stop()
}




func (controller *proxyController) clientHandler(client proxyControllerSocketClient, socket proxyControllerSocket) {
	for {
		data, _ := client.ReadLine()
		client.SendLine(data)
	}
}

func (controller *proxyController) listen() {
	switch controller.socketType {
	case PROXY_CONTROLLER_SOCKET_PLAIN:
		controller.socket = &proxyControllerSocketPlain{}
	case PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET:

	case PROXY_CONTROLLER_SOCKET_TLS:

	case PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET:

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