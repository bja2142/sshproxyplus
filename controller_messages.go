package main


import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"bytes"
	"net/http"
)

type controllerHMAC struct {
	Message []byte
	HMAC	[]byte
}

type controllerMessage struct {
	MessageType		string
	ProxyData 		[]byte `json:"omitempty"`
	ProxyID			uint64 `json:"omitempty"`
	ViewerSecret	string `json:"omitempty"`
	SessionKey		string `json:"omitempty"`
	Username    	string `json:"omitempty"`
	Password    	string `json:"omitempty"`
	FilterKey    	string `json:"omitempty"`
	CallbackKey    	string `json:"omitempty"`
	CallbackURL    	string `json:"omitempty"`
	ProxyUser		*proxyUser `json:"omitempty"`
	FindString		[]byte `json:"omitempty"`
	ReplaceString	[]byte `json:"omitempty`
}

const CONTROLLER_MESSAGE_CREATE_PROXY			string = "create-proxy"
const CONTROLLER_MESSAGE_START_PROXY			string = "start-proxy"
const CONTROLLER_MESSAGE_STOP_PROXY				string = "stop-proxy"
const CONTROLLER_MESSAGE_DESTROY_PROXY			string = "destroy-proxy"
const CONTROLLER_MESSAGE_ACTIVATE_PROXY			string = "activate-proxy"
const CONTROLLER_MESSAGE_DEACTIVATE_PROXY		string = "deactivate-proxy"
const CONTROLLER_MESSAGE_LIST_PROXIES			string = "list-proxies"
const CONTROLLER_MESSAGE_GET_PROXY_INFO			string = "get-proxy-info"
const CONTROLLER_MESSAGE_GET_PROXY_VIEWER		string = "get-proxy-viewer"
const CONTROLLER_MESSAGE_GET_PROXY_VIEWERS		string = "get-proxy-viewers"
const CONTROLLER_MESSAGE_NEW_PROXY_VIEWER		string = "new-proxy-viewer"
const CONTROLLER_MESSAGE_ADD_PROXY_USER			string = "add-proxy-user"
const CONTROLLER_MESSAGE_REMOVE_PROXY_USER		string = "remove-proxy-user"
const CONTROLLER_MESSAGE_ADD_CHANNEL_FILTER		string = "add-channel-filter"
const CONTROLLER_MESSAGE_REMOVE_CHANNEL_FILTER 	string = "remove-channel-filter"
const CONTROLLER_MESSAGE_ADD_USER_CALLBACK		string = "add-user-callback"
const CONTROLLER_MESSAGE_REMOVE_USER_CALLBACK	string = "remove-user-callback"

// TODO: !!! TODO TODO TODO

// - test

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
	var err error
	switch message.MessageType {
	case CONTROLLER_MESSAGE_CREATE_PROXY:
		if (message.ProxyData != nil) {
			var newProxyID uint64 
			err,newProxyID = controller.addProxyFromJSON(message.ProxyData)
			if (err == nil)	{
				reply["ProxyID"] = newProxyID
			}
		} else {
			err = errors.New("No ProxyData provided")
		}
	case CONTROLLER_MESSAGE_START_PROXY:
		err = controller.startProxy(message.ProxyID)
	case CONTROLLER_MESSAGE_STOP_PROXY:
		err = controller.stopProxy(message.ProxyID)
	case CONTROLLER_MESSAGE_DESTROY_PROXY:
		err = controller.destroyProxy(message.ProxyID)
	case CONTROLLER_MESSAGE_ACTIVATE_PROXY:
		err = controller.activateProxy(message.ProxyID)
	case CONTROLLER_MESSAGE_DEACTIVATE_PROXY:
		err = controller.deactivateProxy(message.ProxyID)
	case CONTROLLER_MESSAGE_LIST_PROXIES:
		var data []byte
		data,err =json.Marshal(controller.Proxies)
		if (err == nil)	{
			reply["Proxies"] = data
		}
	case CONTROLLER_MESSAGE_GET_PROXY_INFO:
		var proxy *proxyContext
		proxy, err = controller.getProxy(message.ProxyID)
		if (proxy != nil) {
			var data []byte
			data,err =json.Marshal(proxy)
			if (err == nil)	{
				reply["Proxy"] = data
			}
		}
	case CONTROLLER_MESSAGE_GET_PROXY_VIEWER:
		var viewer *proxySessionViewer
		if message.ViewerSecret != "" {
			err, viewer = controller.getProxyViewerByViewerKey(message.ProxyID, message.ViewerSecret)
		} else if message.SessionKey != "" {
			err, viewer = controller.getProxyViewerBySessionKey(message.ProxyID, message.SessionKey)
		} else if message.Username != "" {
			err, viewer = controller.getProxyViewerByUsername(message.ProxyID, message.Username)
		} else {
			err = errors.New("no viewer secret, session key, nor username provided")
		}
		if err == nil {
			var data []byte
			data, err := json.Marshal(viewer)
			if (err == nil) {
				reply["Viewer"] = data
			}
		}
	case CONTROLLER_MESSAGE_GET_PROXY_VIEWERS:
		var viewers interface{}
		if message.SessionKey != "" {
			err, viewers = controller.getProxyViewersBySessionKey(message.ProxyID, message.SessionKey)
		} else if message.Username != "" {
			err, viewers = controller.getProxyViewersByUsername(message.ProxyID, message.Username)
		} else {
			err, viewers = controller.getProxyViewers(message.ProxyID)			
		}
		if err == nil {
			var data []byte
			data, err := json.Marshal(viewers)
			if (err == nil) {
				reply["Viewers"] = data
			}
		}
	case CONTROLLER_MESSAGE_NEW_PROXY_VIEWER:
		var viewer *proxySessionViewer
		if message.Username != "" {
			if message.SessionKey != "" {
				err, viewer = controller.createSessionViewer(message.ProxyID, message.Username, message.SessionKey)
			} else {
				err, viewer = controller.createUserSessionViewer(message.ProxyID, message.Username)	
			}
			if err == nil {
				var data []byte
				data, err := json.Marshal(viewer)
				if (err == nil) {
					reply["Viewer"] = data
				}
			}
		} else {
			err = errors.New("No Username nor SessionKey provided")
		}
	case CONTROLLER_MESSAGE_ADD_PROXY_USER:
		if message.ProxyUser != nil {
			var key  string
			err, key = controller.addUserToProxy(message.ProxyID, message.ProxyUser)
			if err == nil {
				reply["UserKey"] = key
			}
		} else {
			err = errors.New("No proxyUser provided")
		}
	case CONTROLLER_MESSAGE_REMOVE_PROXY_USER:
		if message.Username != "" {
			err = controller.removeUserFromProxy(message.ProxyID, message.Username, message.Password)
		}
	case CONTROLLER_MESSAGE_ADD_CHANNEL_FILTER:
		if message.FindString != nil && message.ReplaceString != nil && message.Username != "" {
			var key string
			err, key = controller.addChannelFilterToUser(message.ProxyID, message.Username, message.Password, &channelFilterFunc{fn: 
				func(in_data []byte, wrapper *channelWrapper) []byte {
					return bytes.Replace(in_data,message.FindString, message.ReplaceString,-1)
				}})
			reply["FilterKey"] = key
		} else {
			err = errors.New("Missing Username, FindString or ReplaceString")
		}
	case CONTROLLER_MESSAGE_REMOVE_CHANNEL_FILTER:
		if message.FilterKey != "" && message.Username != "" {
			err = controller.removeChannelFilterFromUserByKey(message.ProxyID, message.Username, message.Password, message.FilterKey)
		} else {
			err = errors.New("Missing Username or FilterKey")
		}
	case CONTROLLER_MESSAGE_ADD_USER_CALLBACK:
		var key string
		if message.FindString != nil && message.CallbackURL != "" && message.Username != "" {
			callback := &eventCallback{
				events: map[string]bool{EVENT_SESSION_START:true, EVENT_SESSION_STOP: true},
				handler: func(event sessionEvent) {
					data, err := json.Marshal(&event)
					if err == nil {
						responseBody := bytes.NewBuffer(data)
						resp, _ := http.Post(message.CallbackURL, "application/json",responseBody)
						defer resp.Body.Close()
					}
				},
			}
			err, key = controller.addEventCallbackToUser(message.ProxyID, message.Username, message.Password, callback)
			reply["CallbackKey"] = key
		} else {
			err = errors.New("Missing Username, FindString, or CallbackURL ")
		}
	case CONTROLLER_MESSAGE_REMOVE_USER_CALLBACK:
		if message.CallbackKey != "" && message.Username != "" {
			err = controller.removeEventCallbackFromUserByKey(message.ProxyID, message.Username, message.Password, message.CallbackKey)
		} else {
			err = errors.New("Missing Username or CallbackKey")
		}
	default:
		err = errors.New("unsupported message type")
	}

	if err != nil {
		reply["Error"] = fmt.Sprintf("%s", err)
	}
	
	replyData, err := json.Marshal(reply)
	if (err == nil)	{
		return replyData
	} else {
		return []byte("")
	}
	
}