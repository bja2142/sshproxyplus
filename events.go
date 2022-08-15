package sshproxyplus


import (
	"encoding/json"
)

const EVENT_SESSION_START 	string = "session-start"
const EVENT_SESSION_STOP 	string = "session-stop"
const EVENT_NEW_REQUEST 	string = "new-request"
const EVENT_NEW_CHANNEL 	string = "new-channel"
const EVENT_WINDOW_RESIZE 	string = "window-resize"
const EVENT_MESSAGE	 		string = "new-message"


/*
SessionEvents are the meat of an SSH Session.
They track the start and stop of a session,
when new requests or channels are created,
when a window is resized, and when
data is transmitted as a message. 
*/
type SessionEvent struct {
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
	ChannelData		[]byte		`json:"channel_data,omitempty"`
	RequestType		string		`json:"request_type,omitempty"`
	RequestPayload	[]byte		`json:"request_payload,omitempty"`
	ChannelID		int			`json:"channel_id,omitempty"`
	RequestID		int			`json:"request_id,omitempty"`
}

func (event *SessionEvent) ToJSON() string {
	data, err := json.Marshal(*event)
	if err != nil {
		data = []byte("")
	}
	return string(data)
}

func (context * SessionContext) getStartTimeAsUnix() int64 {
	return context.start_time.Unix()
}

func (context * SessionContext) getStopTimeAsUnix() int64 {
	return context.start_time.Unix()
}

func (session * SessionContext) AddEvent(event *SessionEvent) *SessionEvent{
	event.TimeOffset = session.GetTimeOffset()
	session.event_mutex.Lock()
	session.events = append(session.events, event)
	session.event_mutex.Unlock()
	return event
}

func (session * SessionContext) LogEvent(event *SessionEvent) {
	json_data, err := json.Marshal(event)
	if err != nil {
		session.proxy.Log.Println("Error during marshaling json: ", err)
		return 
	}
	var data []byte

	if event.Type == EVENT_SESSION_START {
		data = json_data
	}  else {
		data = []byte(",\n" + string(json_data))
	}
	
	session.appendToLog(data)
}

func (session * SessionContext) HandleEvent(event *SessionEvent) {
	go func(session * SessionContext, event *SessionEvent) {
		if(session.user.EventCallbacks != nil)	{
			for _, callback := range session.user.EventCallbacks  {
				if callback.events != nil {
					if triggerEvent, ok :=  callback.events[event.Type]; ok {
						if triggerEvent {
							go callback.handler(*event)
						}
					}
				}
			}
		}
	}(session,event)
	updated_event := session.AddEvent(event)
	session.LogEvent(updated_event)
	session.signalNewMessage()
}

type EventCallbackFunc func(SessionEvent)

type EventCallback struct {
	events map[string]bool
	handler EventCallbackFunc
}

type ChannelFilterFunc	struct {
	fn func([]byte, *channelWrapper) []byte
}
// has to be hooked in the reader
