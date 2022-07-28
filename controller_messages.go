package main


import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

type controllerHMAC struct {
	Message []byte
	HMAC	[]byte
}

type controllerMessage struct {
	MessageType	string
	MessageData []byte `json:"omitempty"`
	ProxyID		uint64 `json:"omitempty"`
}

const CONTROLLER_MESSAGE_CREATE_PROXY		string = "create-proxy"
const CONTROLLER_MESSAGE_START_PROXY		string = "start-proxy"
const CONTROLLER_MESSAGE_STOP_PROXY			string = "stop-proxy"
const CONTROLLER_MESSAGE_DESTROY_PROXY		string = "destroy-proxy"
const CONTROLLER_MESSAGE_ACTIVATE_PROXY		string = "activate-proxy"
const CONTROLLER_MESSAGE_DEACTIVATE_PROXY	string = "deactivate-proxy"
const CONTROLLER_MESSAGE_LIST_PROXIES		string = "list-proxies"
const CONTROLLER_MESSAGE_GET_PROXY_INFO		string = "get-proxy-info"
const CONTROLLER_MESSAGE_GET_PROXY_VIEWERS	string = "get-proxy-viewers"
const CONTROLLER_MESSAGE_NEW_PROXY_VIEWER	string = "new-proxy-viewer"
const CONTROLLER_MESSAGE_ADD_PROXY_USER		string = "add-proxy-user"
const CONTROLLER_MESSAGE_REMOVE_PROXY_USER	string = "remove-proxy-user"
const CONTROLLER_MESSAGE_ADD_CHANNEL_FILTER	string = "add-channel-filter"
const CONTROLLER_MESSAGE_ADD_USER_CALLBACK	string = "add-user-callback"

// TODO: !!! TODO TODO TODO
// - define message types
// - integrate HMAC into message protocol
// - test

/*
make_new_proxy (json blob)
start_proxy(id)
stop_proxy(id)
destroy_proxy(id)
activate_proxy(id)
deactivate_proxy(id)
list_proxy()
get_proxy_info(id)
get_existing_proxy_viewers(id)
make_new_proxy_viewer(proxy_id,user)
add_user_to_proxy(user)
remove_user_from_proxy(user,proxy)
add_filter_to_user(user,proxy)
add_string_callback_to_user(string,hook_url,user, proxy)



*/


func (messageWrapper *controllerHMAC) verify(key []byte) (error,controllerMessage) {
	var err error = nil
	mac := hmac.New(sha256.New, key)
	out_message := controllerMessage{}
	mac.Write(messageWrapper.Message)
	expectedMAC := mac.Sum(nil)
	if (hmac.Equal(messageWrapper.HMAC, expectedMAC)) {
		err = json.Unmarshal(messageWrapper.Message, &out_message)
	} else {
		err = errors.New("hmac does not match")
	}
	return err, out_message
}

func (message *controllerMessage) sign(key []byte) (error,controllerHMAC) {
	var err error = nil
	mac := hmac.New(sha256.New, key)
	messageWrapper := controllerHMAC{}
	messageData, err := json.Marshal(message)
	if (err == nil) {
		mac.Write(messageData)
		messageWrapper.Message = messageData
		messageWrapper.HMAC = mac.Sum(nil)
	}
	return err, messageWrapper
}


func (message *controllerMessage) handleMessage(controller *proxyController) []byte {
	reply:= make(map[string]interface{})

	reply["MessageType"] = message.MessageType + "-reply"

	switch message.MessageType {
	case CONTROLLER_MESSAGE_CREATE_PROXY:
		err,newProxyID := controller.addProxyFromJSON(message.MessageData)
		if (err == nil)	{
			reply["ProxyID"] = newProxyID
		} else {
			reply["Error"] = fmt.Sprintf("%s", err)
		}
	case CONTROLLER_MESSAGE_START_PROXY:
	case CONTROLLER_MESSAGE_STOP_PROXY:
	case CONTROLLER_MESSAGE_DESTROY_PROXY:
	case CONTROLLER_MESSAGE_ACTIVATE_PROXY:
	case CONTROLLER_MESSAGE_DEACTIVATE_PROXY:
	case CONTROLLER_MESSAGE_LIST_PROXIES:
		data,err := json.Marshal(controller.Proxies)
		if (err == nil)	{
			reply["Proxies"] = data
		}
	case CONTROLLER_MESSAGE_GET_PROXY_INFO:
	case CONTROLLER_MESSAGE_GET_PROXY_VIEWERS:
	case CONTROLLER_MESSAGE_NEW_PROXY_VIEWER:
	case CONTROLLER_MESSAGE_ADD_PROXY_USER:
	case CONTROLLER_MESSAGE_REMOVE_PROXY_USER:
	case CONTROLLER_MESSAGE_ADD_CHANNEL_FILTER:
	case CONTROLLER_MESSAGE_ADD_USER_CALLBACK:
	default:
		reply["Error"] = "unsupported message type"
	}
	replyData, err := json.Marshal(reply)
	if (err == nil)	{
		return replyData
	} else {
		return []byte("")
	}
	

}