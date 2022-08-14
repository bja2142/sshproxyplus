package sshproxyplus


import (
	"net"
	"crypto/tls"
	"sync"
	"fmt"
	"bufio"
	"github.com/gorilla/websocket"
	"net/http"
)


type ProxyControllerSocketTCP struct {
	listener net.Listener
	active	bool
	clients []net.Conn
	clientMutex sync.Mutex
	TLSCert		   	string
	TLSKey			string
	plaintext	bool
}

type ProxyControllerSocketTCPClient struct {
	net.Conn
}



func (client *ProxyControllerSocketTCPClient) 	SendLine(data []byte) error {
	_, err := client.Write(data)
	if(err != nil)	{
		fmt.Println("Error writing to plain socket: ", err.Error())
	}
	return err
}

func (client *ProxyControllerSocketTCPClient) 	ReadLine() ([]byte, error) {
	reader := bufio.NewReader(client)
	return reader.ReadBytes('\n')
} 



func (socket *ProxyControllerSocketTCP) ListenAndServe(host string, handler ProxyControllerSocketHandler) error {
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
			go func(client net.Conn, socket *ProxyControllerSocketTCP) {
				defer socket.clientClose(client)
				handler(&ProxyControllerSocketTCPClient{client},socket)	
			}(client,socket)
			
		}
	}
	return nil
}

func (socket *ProxyControllerSocketTCP) clientClose(client net.Conn) {
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

func (socket *ProxyControllerSocketTCP) Stop() {


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

func (socket *ProxyControllerSocketTCP) IsPlaintext() bool {
	return socket.plaintext
}


type ProxyControllerSocketWeb struct {
	TLSCert		   	string
	TLSKey			string
	plaintext		bool
	handler 		ProxyControllerSocketHandler
	server			http.Server
	active			bool
}

type ProxyControllerSocketWebClient struct {
	websocket.Conn
}

func (client *ProxyControllerSocketWebClient) SendLine(data []byte) error {
	err := client.WriteMessage(websocket.TextMessage,data)
	if(err != nil)	{
		fmt.Println("Error writing to web socket: ", err.Error())
	}
	return err
}

func (client *ProxyControllerSocketWebClient) ReadLine() ([]byte, error) {
	_, data, err := client.ReadMessage()
	if err != nil {
		fmt.Println("Error reading from web socket:", err.Error())
	}
	return data, err
} 

func (socket *ProxyControllerSocketWeb) handleWebRequest(writer http.ResponseWriter, reader *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: socket.originChecker,
	}
	client, err := upgrader.Upgrade(writer, reader, nil)
    if err != nil {
        fmt.Println("Error during connection upgrade:", err.Error())
        return
    }
    defer client.Close()
	socket.handler(&ProxyControllerSocketWebClient{*client},socket)	
}

func (socket *ProxyControllerSocketWeb) originChecker(r *http.Request) bool {
	fmt.Printf("%v\n",r.Header.Get("Origin"))
	return true
	//TODO: verify origin
}

func (socket *ProxyControllerSocketWeb) ListenAndServe(host string, handler ProxyControllerSocketHandler) error {
	var err error
	socket.handler = handler
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/", socket.handleWebRequest)
	
	socket.server = http.Server{
		Handler: serverMux,
		Addr:	host,
	}


	socket.active = true

	if(socket.plaintext) {
		err = socket.server.ListenAndServe()
	} else {
		err = socket.server.ListenAndServeTLS(socket.TLSCert, socket.TLSKey)
	}
	
	
	if (err != nil)	{
		fmt.Println("Error creating web server:",err.Error())
		socket.active = false
		return err
	}

	
	return nil
}

func (socket *ProxyControllerSocketWeb) Stop() {
	if(socket.active) {
		socket.active = false
		socket.server.Close()
	}
}

func (socket *ProxyControllerSocketWeb) IsPlaintext() bool {
	return socket.plaintext
}


type ProxyControllerSocketHandler func(ProxyControllerSocketClient, ProxyControllerSocket)

type ProxyControllerSocketClient interface {
	SendLine(data []byte) error
	ReadLine() ([]byte, error)
}

type ProxyControllerSocket interface {
	ListenAndServe(host string, handler ProxyControllerSocketHandler) error
	Stop()
	IsPlaintext() bool
}

