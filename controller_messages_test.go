package main

import (
	"testing"
	"encoding/json"
	"encoding/hex"
	"encoding/base64"
	"bytes"
	"time"
	"net/http"
)

func TestMessageWrapperVerifyValid(t *testing.T) {
	key := []byte("key")
	messageJson := []byte(`{"Message":"eyJNZXNzYWdlVHlwZSI6Imxpc3QtcHJveGllcyJ9","HMAC":"ax2pX7hEbL29TIquZL7JQ+wtSTMTI9xEKIAtoKORKYQ="}`)
	wrapper := &controllerHMAC{}
	json.Unmarshal(messageJson,wrapper)

	err, message := wrapper.verify(key)

	if(err != nil) {
		t.Fatalf(`*messageWrapper verify() encountered an error while verifying a valid message: %s`,err)
	}
	if message.MessageType != "list-proxies" {
		t.Errorf(`*messageWrapper verify() did not generate valid MessageType. Wanted "list-proxies"; got: %v`, message.MessageType)
	}
}

func TestMessageWrapperVerifyInvalid(t *testing.T) {
	key := []byte("key")
	messageJson := []byte(`{"Message":"eyJNZXNzYWdlVHlwZSI6Imxpc3QtcHJveGllcyJ9","HMAC":"ax2pX7hEbL2fTIquZL7JQ+wtSTMTI9fEKIAtoKORKYQ="}`)
	wrapper := &controllerHMAC{}
	json.Unmarshal(messageJson,wrapper)

	err, _ := wrapper.verify(key)

	if(err == nil) {
		t.Fatalf(`*messageWrapper verify() verified a message when it shouldn't have`)
	}
}

func TestMessageWrapperVerifyInvalidBlank(t *testing.T) {
	key := []byte("key")
	messageJson := []byte(`{"Message":"eyJNZXNzYWdlVHlwZSI6Imxpc3QtcHJveGllcyJ9","HMAC":""}`)
	wrapper := &controllerHMAC{}
	json.Unmarshal(messageJson,wrapper)

	err, _ := wrapper.verify(key)

	if(err == nil) {
		t.Fatalf(`*messageWrapper verify() verified a message when it shouldn't have`)
	}
}

func TestMessageWrapperSign(t *testing.T) {
	key := []byte("key")
	messageJson := []byte(`{"Message":"eyJNZXNzYWdlVHlwZSI6Imxpc3QtcHJveGllcyJ9","HMAC":"ax2pX7hEbL29TIquZL7JQ+wtSTMTI9xEKIAtoKORKYQ="}`)
	expected := &controllerHMAC{}
	json.Unmarshal(messageJson,expected)

	inputMessage := controllerMessage{MessageType: "list-proxies"}

	err, outWrapper := inputMessage.sign(key)

	if(err != nil) {
		t.Fatalf(`*controllerMessage sign() encountered an error while signing a valid message: %s`,err)
	}
	if ! bytes.Equal(outWrapper.Message, expected.Message) {
		t.Errorf(`*controllerMessage sign() did not generate expected Message. Wanted "%s"; got: %s`, expected.Message, outWrapper.Message)
	}

	if ! bytes.Equal(outWrapper.Message, expected.Message) {
		t.Errorf(`*controllerMessage sign() did not generate expected HMAC. Wanted "%s"; got: %s`, hex.EncodeToString(expected.Message), hex.EncodeToString(outWrapper.Message))
	}

}

func TestMessageUnsupported(t *testing.T) {
	controller := makeNewController()

	message := &controllerMessage{
		MessageType: "this is a fake message",
		}	
	
	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() failed to error when it should have.")
	}


}

func TestMessageCreateProxy(t *testing.T) {

	messageJson := []byte(`{"MessageType":"create-proxy","ProxyData":"e30="}`)
	message := &controllerMessage{}
	json.Unmarshal(messageJson,message)

	controller := makeNewController()

	expectedMessageReplyType := "create-proxy-reply"
	var expectedID uint64 = 10
	controller.ProxyCounter = expectedID


	replyObj :=  simulateMessage(message, controller, t)


	_, ProxyIDFound := replyObj["ProxyID"]

	actualID :=  uint64(replyObj["ProxyID"].(float64))

	if(  !ProxyIDFound) {
		t.Errorf(`*controllerMessage handleMessage() did have one of the expected keys in its reply. Wanted ProxyID. Got: %+v`, replyObj)
	} else {
		if actualID != expectedID {
			t.Fatalf("Did not get expected proxy ID. Wanted %v, got %v",expectedID,actualID)
		}
		if replyObj["MessageType"] != expectedMessageReplyType {
			t.Fatalf("Did not get expected MessageType. Wanted %s, got %v",expectedMessageReplyType,replyObj["MessageType"])
		}
	}
	_, err := controller.getProxy(actualID)
	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() error when getting proxy: %s",err)
	}

}

func TestMessageStartProxy(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}
	proxy, err := controller.getProxy(proxyID)
	if err != nil {
		t.Fatalf("*controller getProxy() error when getting proxy just created: %s",err)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_START_PROXY,
		ProxyID: proxyID,
		}	

	simulateMessage(message, controller, t)


	for proxy.running == false {
		time.Sleep(100)
	}
	controller.stopProxy(proxyID)
}

func simulateMessage(message *controllerMessage, controller *proxyController, t *testing.T) map[string]interface{} {
	expectedMessageReplyType := message.MessageType + "-reply"
	reply := message.handleMessage(controller)
	replyObj :=  make(map[string]interface{})
	err := json.Unmarshal(reply, &replyObj)
	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() did not craft valid json reply: %s", err)
	}
	messageType, MessageTypeFound := replyObj["MessageType"]

	if( !MessageTypeFound) {
		t.Errorf("*controllerMessage handleMessage() did not craft correct reply; did not find MessageType")
	}

	if messageType != expectedMessageReplyType {
		t.Errorf("*controllerMessage handleMessage() did not craft correct reply; expected MessageType %s, got %s", expectedMessageReplyType, messageType)
	}
	//controller.log.Println(string(reply))
	return replyObj
}

func TestMessageStopProxy(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}
	proxy, err := controller.getProxy(proxyID)
	if err != nil {
		t.Fatalf("*controller getProxy() error when getting proxy just created: %s",err)
	}

	err = controller.startProxy(proxyID)
	for proxy.running == false {
		time.Sleep(100)
	}
	if (err != nil) {
		t.Fatalf("*controller startProxy() error when starting proxy: %s",err)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_STOP_PROXY,
		ProxyID: proxyID,
		}	
	simulateMessage(message, controller, t)

	if proxy.running  {
		t.Errorf("*controllerMessage handleMessage() did not correctly stop the proxy.")
		controller.stopProxy(proxyID)
	}
	
}



func TestMessageDestroyProxy(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_DESTROY_PROXY,
		ProxyID: proxyID,
		}	
	simulateMessage(message, controller, t)

	_, err = controller.getProxy(proxyID)
	if err == nil  {
		t.Errorf("*controllerMessage handleMessage() did not correctly destroy the proxy.")
	}
	
}


func TestMessageActivateProxy(t *testing.T) {

	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	proxy, err := controller.getProxy(proxyID)
	if err != nil  {
		t.Errorf("*controllerMessage getProxy() had an error: %s",err)
	}

	proxy.active = false

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_ACTIVATE_PROXY,
		ProxyID: proxyID,
		}	
	simulateMessage(message, controller, t)


	if proxy.active == false {
		t.Errorf("*controllerMessage handleMessage() failed to activate proxy.")

	}

}

func TestMessageDeactivateProxy(t *testing.T) {

	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	proxy, err := controller.getProxy(proxyID)
	if err != nil  {
		t.Errorf("*controllerMessage getProxy() had an error: %s",err)
	}

	proxy.active = true

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_DEACTIVATE_PROXY,
		ProxyID: proxyID,
		}	
	simulateMessage(message, controller, t)


	if proxy.active == true {
		t.Errorf("*controllerMessage handleMessage() failed to deactivate proxy.")

	}
}

func TestMessageListProxies(t *testing.T) {
	controller := makeNewController()

	proxyCount := 3

	for i:=0; i < proxyCount; i++  {
		proxy0 := makeNewProxy(controller.defaultSigner)
		controller.addExistingProxy(proxy0)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}	

	replyObj := simulateMessage(message, controller, t)

	Proxies, ProxiesFound := replyObj["Proxies"]


	if ! ProxiesFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return list of proxies.")
	}

	ProxiesObj, err := base64.StdEncoding.DecodeString(Proxies.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Proxies: %s", Proxies.(string))
	}
	decodedProxies := make(map[uint64]*proxyContext)

	err = json.Unmarshal(ProxiesObj, &decodedProxies)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Proxies: %s", err)
	}

	proxiesLength := len(decodedProxies)
	if proxiesLength != proxyCount {
		t.Fatalf("*controllerMessage handleMessage() returned %v objects; expected %v.", proxiesLength, proxyCount)
	}
	
}

func TestMessageGetProxyInfo(t *testing.T) {
	controller := makeNewController()

	var proxyCount uint64 = 3
	var proxyIDToSend uint64 = 1
	testString := "1.1.1.1"

	for i := uint64(0); i < proxyCount; i++  {
		proxy := makeNewProxy(controller.defaultSigner)
		if i == proxyIDToSend {
			proxy.ListenIP = testString
		}
		controller.addExistingProxy(proxy)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_INFO,
		ProxyID: proxyIDToSend,
		}	

	replyObj := simulateMessage(message, controller, t)

	Proxy, ProxyFound := replyObj["Proxy"]


	if ! ProxyFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return Proxy.")
	}

	ProxyObj, err := base64.StdEncoding.DecodeString(Proxy.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Proxy: %s", Proxy.(string))
	}
	var decodedProxy *proxyContext

	err = json.Unmarshal(ProxyObj, &decodedProxy)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Proxy: %s", err)
	}

	if decodedProxy.ListenIP != testString {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy with an unexpected ListenIP.  Expected %v; got: %v", testString, decodedProxy.ListenIP)
	}

}

func TestMessageGetProxyViewerUsingSecret(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, viewer := controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller createUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}

	viewerKey := viewer.Secret


	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWER,
		ProxyID: proxyID,
		ViewerSecret: viewerKey,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewerString, ViewerFound := replyObj["Viewer"]

	if ! ViewerFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return an existing Viewer.")
	}

	ViewerObj, err := base64.StdEncoding.DecodeString(ViewerString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewer: %s", ViewerString.(string))
	}
	var decodedViewer *proxySessionViewer

	err = json.Unmarshal(ViewerObj, &decodedViewer)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewer: %s", err)
	}

	if decodedViewer.User == nil {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer without a user object")
	}

	if decodedViewer.User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewer.User.Username)
	}

}

func TestMessageGetProxyViewerUsingSessionKey(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	testSessionKey := "myfake-session-key.json"
	proxy.addProxyUser(user)

	err, _ := controller.createSessionViewer(proxyID, user.Username, testSessionKey)

	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}

	


	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWER,
		ProxyID: proxyID,
		SessionKey: testSessionKey,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewerString, ViewerFound := replyObj["Viewer"]

	if ! ViewerFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return an existing Viewer.")
	}

	ViewerObj, err := base64.StdEncoding.DecodeString(ViewerString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewer: %s", ViewerString.(string))
	}
	var decodedViewer *proxySessionViewer

	err = json.Unmarshal(ViewerObj, &decodedViewer)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewer: %s", err)
	}

	if decodedViewer.User == nil {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer without a user object")
	}

	if ! decodedViewer.typeIsSingle(){
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer that should have been a single session viewer but wasn't.")
	}

	if decodedViewer.User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewer.User.Username)
	}

	if decodedViewer.SessionKey != testSessionKey {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected session key.  Expected %v; got: %v", decodedViewer.SessionKey, testSessionKey)
	}
	
}

func TestMessageGetProxyViewerUsingUsername(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, viewer := controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller createUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}


	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWER,
		ProxyID: proxyID,
		Username: user.Username,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewerString, ViewerFound := replyObj["Viewer"]

	if ! ViewerFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return an existing Viewer.")
	}

	ViewerObj, err := base64.StdEncoding.DecodeString(ViewerString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewer: %s", ViewerString.(string))
	}
	var decodedViewer *proxySessionViewer

	err = json.Unmarshal(ViewerObj, &decodedViewer)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewer: %s", err)
	}

	if decodedViewer.User == nil {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer without a user object")
	}

	if decodedViewer.User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewer.User.Username)
	}
}

func TestMessageGetProxyViewerErrorCondition(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, viewer := controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller createUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}


	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWER,
		ProxyID: proxyID,
		}	

	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() failed to throw error when it should have: %v", replyObj)
	}

}

func TestMessageGetProxyViewersUsingSessionKey(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	testSessionKey := "myfake-session-key.json"
	proxy.addProxyUser(user)

	err, _ := controller.createSessionViewer(proxyID, user.Username, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}
	
	err, _ = controller.createSessionViewer(proxyID, user.Username, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWERS,
		ProxyID: proxyID,
		SessionKey: testSessionKey,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewersString, ViewersFound := replyObj["Viewers"]

	if ! ViewersFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return viewers.")
	}

	ViewersObj, err := base64.StdEncoding.DecodeString(ViewersString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewers: %s", ViewersString.(string))
	}
	var decodedViewers []*proxySessionViewer

	err = json.Unmarshal(ViewersObj, &decodedViewers)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewers: %s", err)
	}

	if len(decodedViewers) != 2 {
		t.Fatalf("*controllerMessage handleMessage() returned the wrong number of proxy viewers")
	}

	if decodedViewers[0].User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewers[0].User.Username)
	}
}


func TestMessageGetProxyViewersUsingUsername(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, _ := controller.createUserSessionViewer(proxyID, user.Username)
	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWERS,
		ProxyID: proxyID,
		Username: user.Username,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewersString, ViewersFound := replyObj["Viewers"]

	if ! ViewersFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return viewers.")
	}

	ViewersObj, err := base64.StdEncoding.DecodeString(ViewersString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewers: %s", ViewersString.(string))
	}
	var decodedViewers []*proxySessionViewer

	err = json.Unmarshal(ViewersObj, &decodedViewers)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewers: %s", err)
	}

	if len(decodedViewers) != 2 {
		t.Fatalf("*controllerMessage handleMessage() returned the wrong number of proxy viewers")
	}

	if decodedViewers[0].User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewers[0].User.Username)
	}
}


func TestMessageGetProxyViewers(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, _ := controller.createUserSessionViewer(proxyID, user.Username)
	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createUserSessionViewer(proxyID, user.Username)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}
	
	err, _ = controller.createUserSessionViewer(proxyID, user.Username)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_GET_PROXY_VIEWERS,
		ProxyID: proxyID,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewersString, ViewersFound := replyObj["Viewers"]

	if ! ViewersFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return viewers.")
	}

	ViewersObj, err := base64.StdEncoding.DecodeString(ViewersString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewers: %s", ViewersString.(string))
	}
	var decodedViewers []*proxySessionViewer

	err = json.Unmarshal(ViewersObj, &decodedViewers)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewers: %s - for string `%s`", err, ViewersObj)
	}

	if len(decodedViewers) != 4 {
		t.Fatalf("*controllerMessage handleMessage() returned the wrong number of proxy viewers")
	}

	if decodedViewers[0].User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewers[0].User.Username)
	}
}



func TestMessageCreateNewSessionProxyViewer(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)
	testSessionKey := "myfake-session-key.json"

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_NEW_PROXY_VIEWER,
		ProxyID: proxyID,
		Username: user.Username,
		SessionKey: testSessionKey,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewerString, ViewerFound := replyObj["Viewer"]

	if ! ViewerFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return an existing Viewer.")
	}

	ViewerObj, err := base64.StdEncoding.DecodeString(ViewerString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewer: %s", ViewerString.(string))
	}
	var decodedViewer *proxySessionViewer

	err = json.Unmarshal(ViewerObj, &decodedViewer)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewer: %s", err)
	}

	if ! decodedViewer.typeIsSingle(){
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer that should have been a single session viewer but wasn't.")
	}

	if decodedViewer.User == nil {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer without a user object")
	}

	if decodedViewer.User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewer.User.Username)
	}

	if decodedViewer.SessionKey != testSessionKey {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected session key.  Expected %v; got: %v", decodedViewer.SessionKey, testSessionKey)
	}
}


func TestMessageCreateNewUserProxyViewer(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_NEW_PROXY_VIEWER,
		ProxyID: proxyID,
		Username: user.Username,
		}	

	replyObj := simulateMessage(message, controller, t)

	ViewerString, ViewerFound := replyObj["Viewer"]

	if ! ViewerFound {
		t.Fatalf("*controllerMessage handleMessage() failed to return an existing Viewer.")
	}

	ViewerObj, err := base64.StdEncoding.DecodeString(ViewerString.(string))

	if (err != nil ) {
		t.Fatalf("*controllerMessage handleMessage() returned a non-binary object in Viewer: %s", ViewerString.(string))
	}
	var decodedViewer *proxySessionViewer

	err = json.Unmarshal(ViewerObj, &decodedViewer)

	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() returned an invalid json object for Viewer: %s", err)
	}

	if decodedViewer.User == nil {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer without a user object")
	}

	if ! decodedViewer.typeIsList(){
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer that should have been a user session viewer but wasn't.")
	}

	if decodedViewer.User.Username != user.Username {
		t.Fatalf("*controllerMessage handleMessage() returned a proxy viewer with an unexpected username.  Expected %v; got: %v", user.Username, decodedViewer.User.Username)
	}

}


func TestMessageCreateNewViewerErrorCondition(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_NEW_PROXY_VIEWER,
		ProxyID: proxyID,
		}	

	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() failed to throw error when it should have: %v", replyObj)
	}

}

func TestMessageAddProxyUser(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	expectedKey := user.Username + ":" + user.Password

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_ADD_PROXY_USER,
		ProxyID: proxyID,
		ProxyUser: user,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	ErrorString, ErrorFound := replyObj["Error"]

	if ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() threw an unexpected error: %v", ErrorString)
	}

	UserKey, UserKeyFound := replyObj["UserKey"]

	if ! UserKeyFound {
		t.Fatalf("*controllerMessage handleMessage() did not return a UserKey when it should have.")
	}

	if UserKey != expectedKey {
		t.Errorf("*controllerMessage handleMessage() did not return expected UserKey. Expected %v, got %v", expectedKey, UserKey)
	}

}

func TestMessageAddProxyUserErrorCondition(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_ADD_PROXY_USER,
		ProxyID: proxyID,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() did not throw an error when it should have: %v", replyObj)
	}

}

func TestMessageRemoveProxyUser(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_REMOVE_PROXY_USER,
		ProxyID: proxyID,
		Username: user.Username,
		Password: user.Password,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	ErrorString, ErrorFound := replyObj["Error"]

	if ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() threw an unexpected error: %v", ErrorString)
	}

	if len(proxy.Users) != 0 {
		t.Errorf("*controllerMessage handleMessage() failed to remove proxy user.")
	}

}


func TestMessageAddChannelFilter(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_ADD_CHANNEL_FILTER,
		ProxyID: proxyID,
		FindString: []byte("ls"),
		ReplaceString: []byte("exit"),
		Username: user.Username,
		Password: user.Password,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	ErrorString, ErrorFound := replyObj["Error"]

	if ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() threw an unexpected error: %v", ErrorString)
	}

	_, FilterKeyFound := replyObj["FilterKey"]

	if ! FilterKeyFound {
		t.Fatalf("*controllerMessage handleMessage() failed to provide a FilterKey: %v", replyObj)
	}

	if user.channelFilters == nil {
		t.Fatalf("*controllerMessage handleMessage() failed to populate user's channelFilters.")
	}

	if len(user.channelFilters) == 0 {
		t.Errorf("*controllerMessage handleMessage() failed to populate user's channelFilters.")
	}

	fn := user.channelFilters[0].fn

	outData := string(fn([]byte("this is a test;\nls\n"),nil))
	expectedData := "this is a test;\nexit\n"

	if outData != expectedData {
		t.Errorf("*controllerMessage handleMessage() filter function failed to perform as expected. got `%s`, wanted `%s`.", outData, expectedData)
	}

}

func TestMessageAddChannelFilterErrorCondition(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_ADD_CHANNEL_FILTER,
		ProxyID: proxyID,
		FindString: []byte("ls"),
		ReplaceString: []byte("exit"),
		}	
	
	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() did not throw an error when it should have: %v", replyObj)
	}
}

func TestMessageRemoveChannelFilter(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, key := controller.addChannelFilterToUser(proxyID, user.Username, user.Password, &channelFilterFunc{fn: 
		func(in_data []byte, wrapper *channelWrapper) []byte {
			return in_data
		}})
	
	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() called addChannelFilterToUser() and it threw an unexpected error: %s",err)
	}


	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_REMOVE_CHANNEL_FILTER,
		FilterKey: key,
		Username: user.Username,
		Password: user.Password,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	ErrorString, ErrorFound := replyObj["Error"]

	if ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() threw an unexpected error: %v", ErrorString)
	}

	if _, ok := controller.channelFilters[key]; ok {
		t.Fatalf("*controllerMessage handleMessage() failed to remove filter from user: %s", key)
	}

}

func TestMessageRemoveChannelFilterErrorCondition(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_REMOVE_CHANNEL_FILTER,
		ProxyID: proxyID,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() did not throw an error when it should have: %v", replyObj)
	}
}



type callbackHandler struct {
	triggered bool
}
func (me *callbackHandler) catchCallback(writer http.ResponseWriter, reader *http.Request) {
	me.triggered = true
}
func TestMessageAddUserCallback(t *testing.T) {

	callback  := &callbackHandler{false}
	callbackHost := "127.0.0.1:11989"

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/", callback.catchCallback)
	
	callbackServer := http.Server{
		Handler: serverMux,
		Addr:	callbackHost,
	}
	go callbackServer.ListenAndServe()

	defer callbackServer.Close()
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_ADD_USER_CALLBACK,
		ProxyID: proxyID,
		FindString: []byte("ls"),
		Username: user.Username,
		Password: user.Password,
		CallbackURL: "http://" +callbackHost + "/" ,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	ErrorString, ErrorFound := replyObj["Error"]

	if ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() threw an unexpected error: %v", ErrorString)
	}

	_, CallbackKeyFound := replyObj["CallbackKey"]

	if ! CallbackKeyFound {
		t.Fatalf("*controllerMessage handleMessage() failed to provide a CallbackKey: %v", replyObj)
	}

	if user.eventCallbacks == nil {
		t.Fatalf("*controllerMessage handleMessage() failed to populate user's eventCallbacks.")
	}

	if len(user.eventCallbacks) == 0 {
		t.Errorf("*controllerMessage handleMessage() failed to populate user's eventCallbacks.")
	}

	fn := user.eventCallbacks[0].handler

	event := sessionEvent{Type: EVENT_MESSAGE, Data: []byte("this is a test;\nls\n")}
	fn(event)
	

	// check if web server got reply
	if callback.triggered != true {
		t.Errorf("*controllerMessage handleMessage() callback function failed to callback as expected")
	}

}



func TestMessageRemoveUserCallback(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, key := controller.addEventCallbackToUser(proxyID, user.Username, user.Password, &eventCallback{})
	
	if err != nil {
		t.Fatalf("*controllerMessage handleMessage() called addCallbackToUser() and it threw an unexpected error: %s",err)
	}


	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_REMOVE_USER_CALLBACK,
		CallbackKey: key,
		Username: user.Username,
		Password: user.Password,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	ErrorString, ErrorFound := replyObj["Error"]

	if ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() threw an unexpected error: %v", ErrorString)
	}

	if _, ok := controller.eventCallbacks[key]; ok {
		t.Fatalf("*controllerMessage handleMessage() failed to remove filter from user: %s", key)
	}

}

func TestMessageRemoveUserCallbackErrorCondition(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)

	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_REMOVE_USER_CALLBACK,
		ProxyID: proxyID,
		}	
	
	replyObj := simulateMessage(message, controller, t)

	_, ErrorFound := replyObj["Error"]

	if ! ErrorFound {
		t.Fatalf("*controllerMessage handleMessage() did not throw an error when it should have: %v", replyObj)
	}
}
