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
}


func getSessionInfo(active_only bool) []session_info {
	var sessions_keys []string
	if active_only {
		sessions_keys = ListActiveSessions()
	} else {
		sessions_keys = ListSessions()
	}
	session_list := make([]session_info, len(sessions_keys))
	
	index := 0
	for index < len(sessions_keys) {
		
		session_key := sessions_keys[index]
		session_list[index].Key = session_key
		session_list[index].Active = SshSessions[session_key].active
		session_list[index].Start_time = SshSessions[session_key].getStartTimeAsUnix()
		stop_time := SshSessions[session_key].stop_time
		if SshSessions[session_key].active {
			stop_time = time.Now()
		}
		session_list[index].Length = int64(stop_time.Sub(SshSessions[session_key].start_time).Seconds())
		index++
	}
	return session_list
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

func getActiveSession(conn *websocket.Conn) bool {
	_, session_keyname, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error during message reading:", err)
		return true
	}
	session := string(session_keyname)
	fmt.Printf("selecting %v\n",session)
	
	if context, ok := SshSessions[session]; ok {
		
		last_event_index := 0
		new_event_index := len(context.events)
		fmt.Println("found session")
		client_signal :=  context.makeNewSignal()
		fmt.Println("have signal")
		//send_window_update(context.term_rows,context.term_cols, conn)
		send_latest_events(last_event_index, 
				new_event_index, conn, context.events)
			
		last_event_index = new_event_index				
		for true {
			switch <-client_signal {
				case SIGNAL_SESSION_END:
					fmt.Println("session ended")
					return true
				case SIGNAL_NEW_MESSAGE: 
					fmt.Println("new message signal")
					new_event_index := len(context.events)
					send_latest_events(last_event_index, 
						new_event_index, conn, context.events)
					
					last_event_index = new_event_index	
				
			}
		}
		fmt.Println("no more messages")
	} else {
		log.Println("could not find session %v\n",session)
		conn.WriteMessage(websocket.TextMessage,[]byte("could not find session"))
		return true
	}

	return false
}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	
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
				sessions_json, err := json.Marshal(getSessionInfo(true))
				if err != nil {
					log.Println("Error during marshaling json: ", err)
					break
				}
				err = conn.WriteMessage(websocket.TextMessage,sessions_json)
				if err != nil {
					log.Println("Error during message writing:", err)
					break
				}
			case "get":
				if getActiveSession(conn) {
					log.Println("breaking from loop")
					break
				}
				
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
func ServeWebSocketSessionServer(host string, tls_cert string, tls_key string) {

	fs := http.FileServer(http.Dir("./html"))
	http.Handle("/", fs)
	http.HandleFunc("/socket", socketHandler)
    //http.HandleFunc("/", home)

	Logger.Printf("starting web socket server on %v\n",host)
	if tls_cert != "." || tls_key != "." {
		http.ListenAndServeTLS(host, tls_cert, tls_key,nil)
	} else {
		http.ListenAndServe(host, nil)
	}
    
}