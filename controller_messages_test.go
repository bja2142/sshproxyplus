package main

import (
	"testing"
	"encoding/json"
	"encoding/hex"
	"bytes"
	"time"
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

func TestMessageCreateProxy(t *testing.T) {

	messageJson := []byte(`{"MessageType":"create-proxy","ProxyData":"e30="}`)
	message := &controllerMessage{}
	json.Unmarshal(messageJson,message)

	controller := makeNewController()

	expectedMessageReplyType := "create-proxy-reply"
	var expectedID uint64 = 10
	controller.ProxyCounter = expectedID
	reply := message.handleMessage(controller)

	replyObj :=  make(map[string]interface{})

	json.Unmarshal(reply, &replyObj)

	_, MessageTypeFound := replyObj["MessageType"]
	_, ProxyIDFound := replyObj["ProxyID"]

	actualID :=  uint64(replyObj["ProxyID"].(float64))

	if( !MessageTypeFound || !ProxyIDFound) {
		t.Errorf(`*controllerMessage handleMessage() did have one of the expected keys in its reply. Wanted MessageType and ProxyID. Got: %s`, string(reply))
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

	expectedMessageReplyType := message.MessageType + "-reply"
	reply := message.handleMessage(controller)
	replyObj :=  make(map[string]interface{})
	json.Unmarshal(reply, &replyObj)
	messageType, MessageTypeFound := replyObj["MessageType"]

	if( !MessageTypeFound) {
		t.Errorf("*controllerMessage handleMessage() did not craft correct reply; did not find MessageType")
	}

	if messageType != expectedMessageReplyType {
		t.Errorf("*controllerMessage handleMessage() did not craft correct reply; expected MessageType %s, got %s", expectedMessageReplyType, messageType)
	}


	for proxy.running == false {
		time.Sleep(100)
	}
	controller.stopProxy(proxyID)
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

	expectedMessageReplyType := message.MessageType + "-reply"
	reply := message.handleMessage(controller)
	replyObj :=  make(map[string]interface{})
	json.Unmarshal(reply, &replyObj)
	messageType, MessageTypeFound := replyObj["MessageType"]

	if( !MessageTypeFound) {
		t.Errorf("*controllerMessage handleMessage() did not craft correct reply; did not find MessageType")
	}

	if messageType != expectedMessageReplyType {
		t.Errorf("*controllerMessage handleMessage() did not craft correct reply; expected MessageType %s, got %s", expectedMessageReplyType, messageType)
	}


	if proxy.running  {
		t.Errorf("*controllerMessage handleMessage() did not correctly stop the proxy.")
		controller.stopProxy(proxyID)
	}
	
}

