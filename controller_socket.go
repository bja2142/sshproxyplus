package main


import (
	"net"
	"crypto/tls"
	"sync"
	"fmt"
	"bufio"
	"github.com/gorilla/websocket"
	"net/http"

)


type proxyControllerSocketTCP struct {
	listener net.Listener
	active	bool
	clients []net.Conn
	clientMutex sync.Mutex
	TLSCert		   	string
	TLSKey			string
	plaintext	bool
}

type proxyControllerSocketTCPClient struct {
	net.Conn
}



func (client *proxyControllerSocketTCPClient) 	SendLine(data []byte) error {
	_, err := client.Write(data)
	if(err != nil)	{
		fmt.Println("Error writing to plain socket: ", err.Error())
	}
	return err
}

func (client *proxyControllerSocketTCPClient) 	ReadLine() ([]byte, error) {
	reader := bufio.NewReader(client)
	return reader.ReadBytes('\n')
} 



func (socket *proxyControllerSocketTCP) ListenAndServe(host string, handler proxyControllerSocketHandler) error {
	var err error
	if(socket.plaintext) {
		socket.listener, err = net.Listen("tcp", host)
	} else {
		var keyPair tls.Certificate 
		keyPair, err = tls.LoadX509KeyPair(socket.TLSCert, socket.TLSKey)
		if (err != nil)	{
			fmt.Println("Error importing keys:",err.Error())
			return err
		}
		TLSConfig := &tls.Config{Certificates: []tls.Certificate{keyPair}}

		socket.listener, err = tls.Listen("tcp", host, TLSConfig)
	}
	
	
	if (err != nil)	{
		fmt.Println("Error creating TLS socket:",err.Error())
		return err
	}
	socket.active = true
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
			go func(client net.Conn, socket *proxyControllerSocketTCP) {
				defer socket.clientClose(client)
				handler(&proxyControllerSocketTCPClient{client},socket)	
			}(client,socket)
			
		}
	}
	return nil
}

func (socket *proxyControllerSocketTCP) clientClose(client net.Conn) {
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

func (socket *proxyControllerSocketTCP) Stop() {


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

type proxyControllerSocketWeb struct {
	TLSCert		   	string
	TLSKey			string
	plaintext		bool
	handler 		proxyControllerSocketHandler
	server			http.Server
	active			bool
}

type proxyControllerSocketWebClient struct {
	websocket.Conn
}

func (client *proxyControllerSocketWebClient) SendLine(data []byte) error {
	err := client.WriteMessage(websocket.TextMessage,data)
	if(err != nil)	{
		fmt.Println("Error writing to web socket: ", err.Error())
	}
	return err
}

func (client *proxyControllerSocketWebClient) ReadLine() ([]byte, error) {
	_, data, err := client.ReadMessage()
	if err != nil {
		fmt.Println("Error reading from web socket:", err.Error())
	}
	return data, err
} 

func (socket *proxyControllerSocketWeb) handleWebRequest(writer http.ResponseWriter, reader *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: socket.originChecker,
	}
	client, err := upgrader.Upgrade(writer, reader, nil)
    if err != nil {
        fmt.Println("Error during connection upgrade:", err.Error())
        return
    }
    defer client.Close()
	socket.handler(&proxyControllerSocketWebClient{*client},socket)	
}

func (socket *proxyControllerSocketWeb) originChecker(r *http.Request) bool {
	fmt.Printf("%v\n",r.Header.Get("Origin"))
	return true
	//TODO: verify origin
}

func (socket *proxyControllerSocketWeb) ListenAndServe(host string, handler proxyControllerSocketHandler) error {
	var err error
	socket.handler = handler
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/", socket.handleWebRequest)
	
	socket.server = http.Server{
		Handler: serverMux,
		Addr:	host,
	}

	if(socket.plaintext) {
		err = socket.server.ListenAndServe()
	} else {
		err = socket.server.ListenAndServeTLS(socket.TLSCert, socket.TLSKey)
	}
	
	
	if (err != nil)	{
		fmt.Println("Error creating web server:",err.Error())
		return err
	}

	socket.active = true
	
	return nil
}

func (socket *proxyControllerSocketWeb) Stop() {
	if(socket.active) {
		socket.active = false
		socket.server.Close()
	}
}



type proxyControllerSocketHandler func(proxyControllerSocketClient, proxyControllerSocket)

type proxyControllerSocketClient interface {
	SendLine(data []byte) error
	ReadLine() ([]byte, error)
}

type proxyControllerSocket interface {
	ListenAndServe(host string, handler proxyControllerSocketHandler) error
	Stop()
}

