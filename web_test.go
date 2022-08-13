package main


import (
	"github.com/gorilla/websocket"
	"encoding/json"
	"net/url"
	"time"
	"testing"
)



// TODO: write tests for each web route in the proxyWebServer
// those test should probably go in web_test.go
// or something
// this should make fake sessions that can be verified without
// requiring a full proxy session


func TestWebServerRouteListActive(t *testing.T) {
	controller := makeNewController()
	controller.initializeSocket()
	proxy := makeNewProxy(controller.defaultSigner)
	controller.addExistingProxy(proxy)

	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &sessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*sessionEvent,0),
		sessionID: proxySessionActiveKey,
	}
	inactiveSession :=  &sessionContext{
		proxy: proxy,
		active: false,
		events: make([]*sessionEvent,0),
		sessionID: proxySessionInactiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	go controller.startWebServer()
	defer controller.stopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("list-active"))
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    } else {
		
		replyObj :=  make([]map[string]interface{},0)
		err := json.Unmarshal(reply, &replyObj)
		if err != nil {
			t.Fatalf("(server *proxyWebServer) socketHandler() did not craft valid json reply: %s", err)
		}
		if len(replyObj) != 1 {
			t.Fatalf("(server *proxyWebServer) socketHandler() did not provide json with correct length of responses: %s", string(reply))
		}
		key, keyFound := replyObj[0]["key"]
		if ! keyFound {
			t.Fatalf("(server *proxyWebServer) socketHandler() did not provide json with expected 'key' during a list-active query: %s", string(reply))
		}
		if key != proxySessionActiveKey {
			t.Fatalf("(server *proxyWebServer) socketHandler() provided incorrect session key when querying for active sessions. Got %s, expected %s", key, proxySessionActiveKey)
		}
	}
}

func TestWebServerRouteListActivePublicAccessDisabled(t *testing.T) {
	controller := makeNewController()
	controller.initializeSocket()
	proxy := makeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = false
	controller.addExistingProxy(proxy)

	proxySessionActiveKey := "active-session"

	activeSession := &sessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*sessionEvent,0),
		sessionID: proxySessionActiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession

	go controller.startWebServer()
	defer controller.stopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("list-active"))
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    } else {
		if string(reply) != "public query disabled" {
			t.Errorf("(server *proxyWebServer) socketHandler should have responded with error message but send this instead: %s", string(reply))
		}
	}
}

func TestWebServerRouteListAll(t *testing.T) {

	controller := makeNewController()
	controller.initializeSocket()
	proxy := makeNewProxy(controller.defaultSigner)
	controller.addExistingProxy(proxy)

	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &sessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*sessionEvent,0),
		sessionID: proxySessionActiveKey,
	}
	inactiveSession :=  &sessionContext{
		proxy: proxy,
		active: false,
		events: make([]*sessionEvent,0),
		sessionID: proxySessionInactiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	go controller.startWebServer()
	defer controller.stopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
	err = conn.WriteMessage(websocket.TextMessage, []byte("list-all"))
	if err != nil {
		t.Fatalf("Write to controller failed: %s", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Read from controller failed: %s", err)
	} else {
		
		replyObj :=  make([]map[string]interface{},0)
		err := json.Unmarshal(reply, &replyObj)
		if err != nil {
			t.Fatalf("(server *proxyWebServer) socketHandler() did not craft valid json reply: %s", err)
		}
		if len(replyObj) != 2 {
			t.Fatalf("(server *proxyWebServer) socketHandler() did not provide json with correct length of responses: %s", string(reply))
		}
	}
}
	


func TestWebServerRouteListAllPublicAccessDisabled(t *testing.T) {
	controller := makeNewController()
	controller.initializeSocket()
	proxy := makeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = false
	controller.addExistingProxy(proxy)

	proxySessionActiveKey := "active-session"

	activeSession := &sessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*sessionEvent,0),
		sessionID: proxySessionActiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession

	go controller.startWebServer()
	defer controller.stopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("list-all"))
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    } else {
		if string(reply) != "public query disabled" {
			t.Errorf("(server *proxyWebServer) socketHandler should have responded with error message but send this instead: %s", string(reply))
		}
	}
}

func TestWebServerRouteViewerList(t *testing.T) {

}

// may need proxy test for viewer list on inactive

func TestWebServerRouteViewerGet(t *testing.T) {


	// will need some goroutine to play session for a tick or so
	// should add a new message and then end session


}


func TestWebServerRouteGet(t *testing.T) {

}