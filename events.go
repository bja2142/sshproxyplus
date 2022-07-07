package main


import (
	"encoding/json"
)

const EVENT_SESSION_START 	string = "session-start"
const EVENT_SESSION_STOP 	string = "session-stop"
const EVENT_NEW_REQUEST 	string = "new-request"
const EVENT_NEW_CHANNEL 	string = "new-channel"
const EVENT_WINDOW_RESIZE 	string = "window-resize"
const EVENT_MESSAGE	 		string = "new-message"



type sessionEvent struct {
	Type 			string 		`json:"type"`
	Key  			string     	`json:"key,omitempty"`
	StartTime		int64 		`json:"start,omitempty"`
	StopTime		int64 		`json:"stop,omitempty"`
	Length			int64 		`json:"length,omitempty"`
	TimeOffset		int64		`json:"offset,omitempty"`
	Direction 		string 		`json:"direction,omitempty"`
	Size 			int 		`json:"size,omitempty"`
	Data 			[]byte 		`json:"data,omitempty"`
	ClientHost		string 		`json:"client_host,omitempty"`
	ServHost   		string 		`json:"server_host,omitempty"`
	Username		string 		`json:"username,omitempty"`
	Password    	string 		`json:"password,omitempty"`
	TermRows		uint32 		`json:"term_rows,omitempty"`
	TermCols		uint32 		`json:"term_cols,omitempty"`
	ChannelType		string		`json:"channel_type,omitempty"`
	RequestType		string		`json:"request_type,omitempty"`
	RequestPayload	[]byte		`json:"request_payload,omitempty"`
}

func (event sessionEvent) toJSON() []byte {
	data, err := json.Marshal(event)
	if err == nil {
		data = []byte("")
	}
	return data
}

func (context * session_context) getStartTimeAsUnix() int64 {
	return context.start_time.Unix()
}

func (context * session_context) getStopTimeAsUnix() int64 {
	return context.start_time.Unix()
}

func (session * session_context) addEvent(event *sessionEvent) *sessionEvent{
	event.TimeOffset = session.get_time_offset()
	session.event_mutex.Lock()
	session.events = append(session.events, event)
	session.event_mutex.Unlock()
	return event
}

func (session * session_context) logEvent(event *sessionEvent) {
	json_data, err := json.Marshal(event)
	if err != nil {
		session.proxy.log.Println("Error during marshaling json: ", err)
		return 
	}
	var data []byte

	if event.Type == EVENT_SESSION_START {
		data = json_data
	}  else {
		data = []byte(",\n" + string(json_data))
	}
	
	session.append_to_log(data)
}

func (session * session_context) handleEvent(event *sessionEvent) {
	updated_event := session.addEvent(event)
	session.logEvent(updated_event)
	session.signal_new_message()
}