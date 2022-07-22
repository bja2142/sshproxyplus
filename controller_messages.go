package main


import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
)

type controllerHMAC struct {
	Message []byte
	HMAC	[]byte
}

type controllerMessage struct {
	MessageType	string
	
}

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