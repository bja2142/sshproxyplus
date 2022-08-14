package sshproxyplus


import (
	"github.com/gorilla/websocket"
	"encoding/json"
	"net/url"
	"time"
	"testing"
	"log"
)



// TODO: write tests for each web route in the proxyWebServer
// those test should probably go in web_test.go
// or something
// this should make fake sessions that can be verified without
// requiring a full proxy session


func TestWebServerRouteListActive(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	controller.AddExistingProxy(proxy)

	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
	}
	inactiveSession :=  &SessionContext{
		proxy: proxy,
		active: false,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("list-active"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from websocket failed: %s", err)
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
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = false
	controller.AddExistingProxy(proxy)

	proxySessionActiveKey := "active-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("list-active"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from websocket failed: %s", err)
    } else {
		if string(reply) != "public query disabled" {
			t.Errorf("(server *proxyWebServer) socketHandler should have responded with error message but send this instead: %s", string(reply))
		}
	}
}

func TestWebServerRouteListAll(t *testing.T) {

	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	controller.AddExistingProxy(proxy)

	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
	}
	inactiveSession :=  &SessionContext{
		proxy: proxy,
		active: false,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
	err = conn.WriteMessage(websocket.TextMessage, []byte("list-all"))
	if err != nil {
		t.Fatalf("Write to websocket failed: %s", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Read from websocket failed: %s", err)
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
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = false
	controller.AddExistingProxy(proxy)

	proxySessionActiveKey := "active-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("list-all"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from websocket failed: %s", err)
    } else {
		if string(reply) != "public query disabled" {
			t.Errorf("(server *proxyWebServer) socketHandler should have responded with error message but send this instead: %s", string(reply))
		}
	}
}

func TestWebServerRouteViewerList(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = false
	controller.AddExistingProxy(proxy)

	testUser1 := &ProxyUser{
		Username: "testuser1",
		Password: "testPassword1",
	}

	testUser2 := &ProxyUser{
		Username: "testuser2",
		Password: "testPassword2",
	}

	proxy.AddProxyUser(testUser1)
	proxy.AddProxyUser(testUser2)



	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
		user: testUser1,
	}
	inactiveSession :=  &SessionContext{
		proxy: proxy,
		active: false,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
		user: testUser2,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	proxy.AddSessionToUserList(activeSession)
	proxy.AddSessionToUserList(inactiveSession)

	log.Println(proxy.Users)
	err, viewer := proxy.MakeSessionViewerForUser(testUser2.Username, testUser2.Password)

	if(err != nil) {
		t.Fatalf("Failed to create session viewer during setup: %s",err)
	}



	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("viewer-list"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	err = conn.WriteMessage(websocket.TextMessage, []byte(viewer.Secret))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, reply, err := conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from websocket failed: %s", err)
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
		if key != proxySessionInactiveKey {
			t.Fatalf("(server *proxyWebServer) socketHandler() provided incorrect session key when querying for active sessions. Got %s, expected %s", key, proxySessionActiveKey)
		}
	}
}



// may need proxy test for viewer list on inactive

func TestWebServerRouteViewerGet(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = true
	controller.AddExistingProxy(proxy)

	testUser1 := &ProxyUser{
		Username: "testuser1",
		Password: "testPassword1",
	}

	testUser2 := &ProxyUser{
		Username: "testuser2",
		Password: "testPassword2",
	}

	proxy.AddProxyUser(testUser1)
	proxy.AddProxyUser(testUser2)



	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
		user: testUser1,
		msg_signal: make([]chan int,0),
	}
	inactiveSession :=  &SessionContext{
		proxy: proxy,
		active: false,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
		user: testUser2,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	proxy.AddSessionToUserList(activeSession)
	proxy.AddSessionToUserList(inactiveSession)

	err, viewer := proxy.MakeSessionViewerForUser(testUser1.Username, testUser1.Password)

	if(err != nil) {
		t.Fatalf("Failed to create session viewer during setup: %s",err)
	}

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("viewer-get"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	err = conn.WriteMessage(websocket.TextMessage, []byte(viewer.Secret))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	err = conn.WriteMessage(websocket.TextMessage, []byte(proxySessionActiveKey))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	eventsToSend := []*SessionEvent{
		&SessionEvent{
			Type: EVENT_SESSION_START,
			Key: proxySessionActiveKey,
			StartTime: time.Now().Unix(),
			TimeOffset: 0,
		},
		&SessionEvent{
			Type: EVENT_SESSION_STOP,
			StopTime: time.Now().Unix(),
		},
	}

	go func(session * SessionContext, events []*SessionEvent) {
		for _, event := range events {
			session.AddEvent(event)
			session.signalNewMessage()
			time.Sleep(time.Millisecond*100)
		}
	}(activeSession, eventsToSend)

	count := 0

	for count < len(eventsToSend) {
		time.Sleep(time.Millisecond*100)
		_, reply, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Read from websocket failed: %s", err)
		}
		log.Println(reply)
		err = conn.WriteMessage(websocket.TextMessage, []byte("ack"))
		if err != nil {
			t.Fatalf("Write to websocket failed: %s", err)
		}
		replyObj :=  &SessionEvent{}
		err = json.Unmarshal(reply, &replyObj)
		if err != nil {
			t.Fatalf("(server *proxyWebServer) playSession() did not craft valid json reply: %s", err)
		}
		if replyObj.Type != eventsToSend[count].Type {
			t.Errorf("(server *proxyWebServer) playSession() provided incorrect event type. Got %s, expected %s", replyObj.Type, eventsToSend[count].Type)
		}
		count += 1
	}
	activeSession.active = false
	activeSession.signalSessionEnd()

}


func TestWebServerRouteViewerGetSingle(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = true
	controller.AddExistingProxy(proxy)

	testUser1 := &ProxyUser{
		Username: "testuser1",
		Password: "testPassword1",
	}

	testUser2 := &ProxyUser{
		Username: "testuser2",
		Password: "testPassword2",
	}

	proxy.AddProxyUser(testUser1)
	proxy.AddProxyUser(testUser2)



	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
		user: testUser1,
		msg_signal: make([]chan int,0),
	}
	inactiveSession :=  &SessionContext{
		proxy: proxy,
		active: false,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
		user: testUser2,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	proxy.AddSessionToUserList(activeSession)
	proxy.AddSessionToUserList(inactiveSession)

	err, viewer := proxy.MakeSessionViewerForSession(testUser1.Username, testUser1.Password,proxySessionActiveKey)

	if(err != nil) {
		t.Fatalf("Failed to create session viewer during setup: %s",err)
	}

	viewer.getSessions()

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("viewer-get"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	err = conn.WriteMessage(websocket.TextMessage, []byte(viewer.Secret))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	err = conn.WriteMessage(websocket.TextMessage, []byte(proxySessionActiveKey))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	eventsToSend := []*SessionEvent{
		&SessionEvent{
			Type: EVENT_SESSION_START,
			Key: proxySessionActiveKey,
			StartTime: time.Now().Unix(),
			TimeOffset: 0,
		},
		&SessionEvent{
			Type: EVENT_SESSION_STOP,
			StopTime: time.Now().Unix(),
		},
	}

	go func(session * SessionContext, events []*SessionEvent) {
		for _, event := range events {
			session.AddEvent(event)
			session.signalNewMessage()
			time.Sleep(time.Millisecond*100)
		}
	}(activeSession, eventsToSend)

	count := 0

	for count < len(eventsToSend) {
		time.Sleep(time.Millisecond*100)
		_, reply, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Read from websocket failed: %s", err)
		}
		log.Println(reply)
		err = conn.WriteMessage(websocket.TextMessage, []byte("ack"))
		if err != nil {
			t.Fatalf("Write to websocket failed: %s", err)
		}
		replyObj :=  &SessionEvent{}
		err = json.Unmarshal(reply, &replyObj)
		if err != nil {
			t.Fatalf("(server *proxyWebServer) playSession() did not craft valid json reply: %s", err)
		}
		if replyObj.Type != eventsToSend[count].Type {
			t.Errorf("(server *proxyWebServer) playSession() provided incorrect event type. Got %s, expected %s", replyObj.Type, eventsToSend[count].Type)
		}
		count += 1
	}
	activeSession.active = false
	activeSession.signalSessionEnd()

}



func TestWebServerRouteGet(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.defaultSigner)
	proxy.PublicAccess = false
	controller.AddExistingProxy(proxy)

	testUser1 := &ProxyUser{
		Username: "testuser1",
		Password: "testPassword1",
	}

	testUser2 := &ProxyUser{
		Username: "testuser2",
		Password: "testPassword2",
	}

	proxy.AddProxyUser(testUser1)
	proxy.AddProxyUser(testUser2)



	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
		user: testUser1,
		msg_signal: make([]chan int,0),
	}
	otherSession :=  &SessionContext{
		proxy: proxy,
		active: true,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
		user: testUser2,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = otherSession

	proxy.AddSessionToUserList(activeSession)
	proxy.AddSessionToUserList(otherSession)

	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("get"))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	err = conn.WriteMessage(websocket.TextMessage, []byte(proxySessionActiveKey))
    if err != nil {
        t.Fatalf("Write to websocket failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	eventsToSend := []*SessionEvent{
		&SessionEvent{
			Type: EVENT_SESSION_START,
			Key: proxySessionActiveKey,
			StartTime: time.Now().Unix(),
			TimeOffset: 0,
		},
		&SessionEvent{
			Type: EVENT_SESSION_STOP,
			StopTime: time.Now().Unix(),
		},
	}

	go func(session * SessionContext, events []*SessionEvent) {
		for _, event := range events {
			session.AddEvent(event)
			session.signalNewMessage()
			time.Sleep(time.Millisecond*100)
		}
	}(activeSession, eventsToSend)

	count := 0

	for count < len(eventsToSend) {
		time.Sleep(time.Millisecond*100)
		_, reply, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Read from websocket failed: %s", err)
		}
		log.Println(reply)
		err = conn.WriteMessage(websocket.TextMessage, []byte("ack"))
		if err != nil {
			t.Fatalf("Write to websocket failed: %s", err)
		}
		replyObj :=  &SessionEvent{}
		err = json.Unmarshal(reply, &replyObj)
		if err != nil {
			t.Fatalf("(server *proxyWebServer) playSession() did not craft valid json reply: %s", err)
		}
		if replyObj.Type != eventsToSend[count].Type {
			t.Errorf("(server *proxyWebServer) playSession() provided incorrect event type. Got %s, expected %s", replyObj.Type, eventsToSend[count].Type)
		}
		count += 1
	}
	activeSession.active = false
	activeSession.signalSessionEnd()

}