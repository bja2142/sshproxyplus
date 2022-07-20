package main


import (
	"net/http"
	"github.com/gorilla/websocket"
	"log"
	"fmt"
	"encoding/json"
	"time"
)

/*

Tutorials: 
- https://yalantis.com/blog/how-to-build-websockets-in-go/
- https://golangdocs.com/golang-gorilla-websockets
- https://tutorialedge.net/golang/go-websocket-tutorial/
*/

type window_message struct {
	Rows int64 `json:"rows"`
	Columns int64 `json:"columns"`
	Type string `json:"type"`
}

type session_info struct {
    Key  string      `json:"key"`
    Active bool `json:"active"`
	Start_time	int64 `json:"start"`
	Length		int64 `json:"length"`
	User		string `json:"user,omitempty"`
	Secret		string `json:"secret,omitempty"`
}

func buildWebSessionInfoList(sessions map[string]*sessionContext, sessions_keys []string, user string, secret string) []session_info {
	session_list := make([]session_info, len(sessions_keys))
	log.Printf("%v,%v,%v\n",len(sessions_keys),sessions_keys,sessions)
	for index := 0; index < len(sessions_keys); index ++ {

		session_key := sessions_keys[index]
		if session_key == "" {
			continue
		}
		cur_session := sessions[session_key]
				
		session_list[index].Key = session_key
		session_list[index].User = user
		session_list[index].Secret = secret
		session_list[index].Active = cur_session.active
		session_list[index].Start_time = cur_session.getStartTimeAsUnix()
		
		stop_time := cur_session.stop_time
		if cur_session.active {
			stop_time = time.Now()
		}
		session_list[index].Length = int64(stop_time.Sub(cur_session.start_time).Seconds())
	}
	return session_list

}
func (server *proxyWebServer) getUserSessionInfo(viewer *proxySessionViewer) []session_info {
	sessions, sessions_keys := viewer.getSessions()

	return buildWebSessionInfoList(sessions, sessions_keys, viewer.user.getKey(), viewer.secret)
}

func (server *proxyWebServer) getAllSessionInfo(active_only bool) []session_info {
	var sessions_keys []string

	if active_only {
		sessions_keys = server.proxy.ListAllActiveSessions()
	} else {
		sessions_keys = server.proxy.ListAllSessions()
	}
	sessions := server.proxy.allSessions

	
	return buildWebSessionInfoList(sessions, sessions_keys,"","")
	
	
}

func send_window_update(rows uint32, columns uint32,  conn * websocket.Conn){

	msg := window_message{Rows:int64(rows), Columns: int64(columns), Type: "window-size"}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("Error during marshaling json: ", err)
		return
	}
	log.Println("sending",data)
	ack := []byte("")
	for string(ack) != "ack" {
		conn.WriteMessage(websocket.TextMessage,data)
		_, ack, err = conn.ReadMessage()
		if err != nil {
			log.Println("Error during message reading:", err)
			break
			
		}
		if string(ack) != "ack" {
			log.Println("Error: client did not ack last message.")
			
		}
	}

}


func send_latest_events(prev_index int, new_index int, conn * websocket.Conn, events []*sessionEvent){

	for _, event := range events[prev_index:new_index] {
		data, err := json.Marshal(event)
		if err != nil {
			log.Println("Error during marshaling json: ", err)
			break
		}
		log.Println("sending",data)
		ack := []byte("")
		for string(ack) != "ack" {
			conn.WriteMessage(websocket.TextMessage,data)
			_, ack, err = conn.ReadMessage()
			if err != nil {
				log.Println("Error during message reading:", err)
				break
				
			}
			if string(ack) != "ack" {
				log.Println("Error: client did not ack last message.")
				
			}
		}
	}
}

func (server *proxyWebServer) sessionViewerSocketGet(conn * websocket.Conn) {
	_, viewerKey, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error during message reading:", err)
		return
	}
	viewer := server.proxy.getSessionViewer(string(viewerKey))
	if viewer == nil {
		log.Printf("couldn't find viewer with key:%v\n", "|"+string(viewerKey)+"|")
		return
	}
	_, sessionKey, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error during message reading:", err)
		return
	}
	viewer_sessions, _ := viewer.getSessions()

	if session, ok := viewer_sessions[string(sessionKey)]; ok {
		playSession(conn,session)
	} else {
		log.Printf("could not find session %v\n",sessionKey)
		conn.WriteMessage(websocket.TextMessage,[]byte("could not find session"))
		return 
	}
	
}

func (server *proxyWebServer) sessionViewerSocketList(conn * websocket.Conn) {
	_, viewerKey, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error during message reading:", err)
		return
	}
	viewer := server.proxy.getSessionViewer(string(viewerKey))
	if viewer == nil {
		log.Printf("couldn't find viewer with key:%v\n", "|"+string(viewerKey)+"|")
		return
	}
	sessions_json, err := json.Marshal(server.getUserSessionInfo(viewer))
	if err != nil {
		log.Println("Error during marshaling json: ", err)
		return
	}
	err = conn.WriteMessage(websocket.TextMessage,sessions_json)	
}

func playSession(conn *websocket.Conn, session *sessionContext) {
	last_event_index := 0
	new_event_index := len(session.events)
	fmt.Println("found session")
	client_signal :=  session.makeNewSignal()
	defer session.removeSignal(client_signal)
	fmt.Println("have signal")
	//send_window_update(session.term_rows,session.term_cols, conn)
	send_latest_events(last_event_index, 
			new_event_index, conn, session.events)
		
	last_event_index = new_event_index				
	for true {
		switch <-client_signal {
			case SIGNAL_SESSION_END:
				fmt.Println("session ended")
				return
			case SIGNAL_NEW_MESSAGE: 
				fmt.Println("new message signal")
				new_event_index := len(session.events)
				send_latest_events(last_event_index, 
					new_event_index, conn, session.events)
				
				last_event_index = new_event_index	
			
		}
	}
	fmt.Println("no more messages")
}

func (server *proxyWebServer) getActiveSession(conn *websocket.Conn) {
	_, session_keyname, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error during message reading:", err)
		return 
	}
	session := string(session_keyname)
	fmt.Printf("selecting %v\n",session)
	
	if context, ok := server.proxy.allSessions[session]; ok {
		playSession(conn,context)
	} else {
		log.Printf("could not find session %v\n",session)
		conn.WriteMessage(websocket.TextMessage,[]byte("could not find session"))
		return 
	}

	return 
}


func (server *proxyWebServer) socketHandler(w http.ResponseWriter, r *http.Request) {
	
    // Upgrade our raw HTTP connection to a websocket based one
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Print("Error during connection upgradation:", err)
        return
    }
    defer conn.Close()

	Logger.Printf("got new connection on web socket\n")

    // The event loop
    for {
        _, message, err := conn.ReadMessage()
        if err != nil {
            log.Println("Error during message reading:", err)
            break
        }
		switch string(message) {
			case "list-active":
				sessions_json, err := json.Marshal(server.getAllSessionInfo(true))
				if err != nil {
					log.Println("Error during marshaling json: ", err)
					break
				}
				err = conn.WriteMessage(websocket.TextMessage,sessions_json)
				if err != nil {
					log.Println("Error during message writing:", err)
					break
				}
			case "list-all":
				sessions_json, err := json.Marshal(server.getAllSessionInfo(false))
				if err != nil {
					log.Println("Error during marshaling json: ", err)
					break
				}
				err = conn.WriteMessage(websocket.TextMessage,sessions_json)
				if err != nil {
					log.Println("Error during message writing:", err)
					break
				}
			case "viewer-get":
				server.sessionViewerSocketGet(conn)
				break;
			case "viewer-list":
				server.sessionViewerSocketList(conn)
				break;
			case "get":
				server.getActiveSession(conn) 
				break;
			default:
				err = conn.WriteMessage(websocket.BinaryMessage,[]byte("unsupported message type"))
				if err != nil {
					log.Println("Error during message writing:", err)
					break
				}
		}
	}
	Logger.Printf("ending session with client")
}

func home(w http.ResponseWriter, r *http.Request) {
	// TODO: update this to redirect to some home page
    fmt.Fprintf(w, "")
}


func originChecker(r *http.Request) bool {
	log.Printf("%v\n",r.Header.Get("Origin"))
	return true
	//TODO: verify origin
}
var upgrader = websocket.Upgrader{
	CheckOrigin: originChecker,
} // use default options

// TODO: make the session folder and the html server folders consistent
// with the proxy parameters 
func (server *proxyWebServer) ServeWebSocketSessionServer() {

	host := server.listenHost
	tls_cert := *server.proxy.tls_cert
	tls_key := *server.proxy.tls_key
	
	fs := http.FileServer(http.Dir("./html"))
	http.Handle("/", fs)
	http.HandleFunc("/socket", server.socketHandler)
    //http.HandleFunc("/", home)

	Logger.Printf("starting web socket server on %v\n",host)
	if tls_cert != "." && tls_key != "." {
		http.ListenAndServeTLS(host, tls_cert, tls_key,nil)
	} else {
		http.ListenAndServe(host, nil)
	}
    
}

type proxyWebServer struct {
	proxy	*proxyContext
	listenHost	string
	baseURI		string
}