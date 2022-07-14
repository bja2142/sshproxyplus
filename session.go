package main


import (
	"sync"
	"time"
	"golang.org/x/crypto/ssh"
	"io"
	"os"
	"errors"
	"fmt"
	"strconv"
	"encoding/binary"
	"encoding/json"
)

// session context
type sessionContext struct {
	proxy				proxyContext
	mutex				sync.Mutex
	log_mutex			sync.Mutex
	event_mutex			sync.Mutex
	log_fd				*os.File
	client_host			string
	client_username		string
	client_password		string	
	remote_conn			*ssh.Conn
	channels			[]*channel_data
	channel_count		int
	requests			[]*request_data
	request_count		int
	active				bool
	thread_count		int
	start_time   		time.Time
	stop_time 	 		time.Time
	msg_signal			[]chan int
	term_rows			uint32
	term_cols			uint32
	filename			string
	events				[]*sessionEvent
}
// TODO: create a routine to remove a signal
// so the signal list doesn't get crowded.


// TODO: track all session events in a single list
// similar to the logging rather than storing them in channels, etc

func (session * sessionContext) handleChannels(dest_conn ssh.Conn, channels <-chan ssh.NewChannel) {
	session.markThreadStarted()
	defer session.markThreadStopped()
	defer dest_conn.Close()
	for this_channel := range channels {
		// reset the var scope for each goroutine; taken from https://github.com/cmoog/sshproxy/blob/master/reverseproxy.go#L87
		cur_channel := this_channel
		go func() {
			session.forwardChannel(dest_conn, cur_channel)
		}()
	}
}


func (session * sessionContext) forwardChannel(dest_conn ssh.Conn, cur_channel ssh.NewChannel) {
	session.markThreadStarted()
	defer session.markThreadStopped()
	session.handleEvent(
		&sessionEvent{
			Type: EVENT_NEW_CHANNEL,
			ChannelType: cur_channel.ChannelType(),
			ChannelData: cur_channel.ExtraData(),
		})
	if ! channelTypeSupported(cur_channel.ChannelType()) {
		_ = cur_channel.Reject(ssh.ConnectionFailed, "Unable to open channel.")
		session.proxy.log.Printf("Rejecting channel type: %v\n", cur_channel.ChannelType())
		return
	}
	outgoing_channel, outgoing_requests, err := dest_conn.OpenChannel(cur_channel.ChannelType(), cur_channel.ExtraData())
	if err != nil {
		if openChanErr, ok := err.(*ssh.OpenChannelError); ok {
			_ = cur_channel.Reject(openChanErr.Reason, openChanErr.Message)
		} else {
			_ = cur_channel.Reject(ssh.ConnectionFailed, err.Error())
		}
		
		session.proxy.log.Printf("error open channel: t:%v p:%v - %v\n", cur_channel.ChannelType(), cur_channel.ExtraData(),err)
		return
	}
	session.proxy.log.Printf("Opening channel of type: %v\n", cur_channel.ChannelType())
	defer outgoing_channel.Close()

	incoming_channel, incoming_requests, err := cur_channel.Accept()
	if err != nil {
		session.proxy.log.Printf("error accept new channel: %w\n", err)
		return
	}
	defer incoming_channel.Close()

	dest_requests_completed := make(chan struct{})
	// https://github.com/cmoog/sshproxy/blob/47ea68e82eaa4d43250d2a93c18fb26806cd67eb/reverseproxy.go#L127
	go func() {
		defer close(dest_requests_completed)
		session.handleRequests(channelRequestDest{incoming_channel}, outgoing_requests)
	}()

	// This request channel does not get closed
	// by the client causing this function to hang if we wait on it.
	// https://github.com/cmoog/sshproxy/blob/master/reverseproxy.go#L134
	go session.handleRequests(channelRequestDest{outgoing_channel}, incoming_requests)

	session.bidirectionalChannelClone(incoming_channel, outgoing_channel, cur_channel.ChannelType());
	<-dest_requests_completed
}

func (session * sessionContext) copyChannel(write_channel ssh.Channel, read_channel ssh.Channel, direction string, channel_type string) {
	session.markThreadStarted()
	defer session.markThreadStopped()
	defer write_channel.CloseWrite()
	done_copying := make(chan struct{})

	go func() {
		defer close(done_copying)
		_, err := io.Copy(write_channel, newChannelWrapper(read_channel,session, direction, "stdout", time.Now(), channel_type))
		if err != nil && !errors.Is(err, io.EOF) {
			session.proxy.log.Printf("channel copy error: %v\n", err)
		}
	}()
	_, err := io.Copy(write_channel.Stderr(), newChannelWrapper(read_channel.Stderr(),session, direction,"stderr", time.Now(), channel_type))
	if err != nil && !errors.Is(err, io.EOF) {
		session.proxy.log.Printf("channel copy error: %v\n", err)
	}
	<-done_copying
}

func (session * sessionContext) bidirectionalChannelClone(incoming_channel ssh.Channel, outgoing_channel ssh.Channel,channel_type string) {
	session.markThreadStarted()
	defer session.markThreadStopped()
	incoming_write_done := make(chan struct{})
	go func() {
		defer close(incoming_write_done)
		session.copyChannel(incoming_channel,outgoing_channel, "incoming",channel_type)
	}()
	go session.copyChannel(outgoing_channel, incoming_channel, "outgoing",channel_type)

	<-incoming_write_done
}


func (session * sessionContext) handleRequests(outgoing_conn requestDest, incoming_requests <-chan *ssh.Request ) {
	session.markThreadStarted()
	defer session.markThreadStopped()
	for cur_request := range incoming_requests {
		err := session.forwardRequest(outgoing_conn, cur_request)
		if err != nil && !errors.Is(err, io.EOF) {
			session.proxy.log.Printf("handle request error: %v", err)
		}
	}
}


func (session * sessionContext) forwardRequest(outgoing_channel requestDest, request *ssh.Request) error {
	
	session.markThreadStarted()
	defer session.markThreadStopped()
	
	
	session.request_count += 1
	
	request_entry := &request_data{Req_type: request.Type, Req_payload: request.Payload, Msg_type: "request-data", Offset: session.getTimeOffset() }
	session.handleEvent(
		&sessionEvent{
			Type: EVENT_NEW_REQUEST,
			RequestType:  request.Type,
			RequestPayload: request.Payload,
		})
	
	session.requests = append(session.requests, request_entry)
	if request.Type == "env" || request.Type == "shell" || request.Type == "exec" {
		session.proxy.log.Printf("req.Type:%v, req.Payload:%v\n",request.Type,string(request.Payload))
	} else {
		session.proxy.log.Printf("req.Type:%v, req.Payload:%v\n",request.Type,request.Payload)
	}
	
	if request.Type == "pty-req" {
		//https://github.com/Scalingo/go-ssh-examples/blob/ae24797273aa9fcd3a8fa6c624af1b068a81d58b/server_complex.go#L206
		if(len(request.Payload)>4) {
			termLen := uint(request.Payload[3])
			if len(request.Payload) >= (4 + int(termLen)+ 8) {
				width, height := parseDims(request.Payload[termLen+4:])
				session.term_rows = height
				session.term_cols = width
				go session.handleEvent(
					&sessionEvent{
						Type: EVENT_WINDOW_RESIZE,
						TermRows: session.term_rows,
						TermCols: session.term_cols,
					})
				session.proxy.log.Printf("Window row:%v, col:%v\n", height,width)
			}
		}					
	} else if request.Type == "window-change" && len(request.Payload) >= 8 {
		width, height := parseDims(request.Payload)
		session.term_rows = height
		session.term_cols = width
		go session.handleEvent(
			&sessionEvent{
				Type: EVENT_WINDOW_RESIZE,
				TermRows: session.term_rows,
				TermCols: session.term_cols,
			})
		session.proxy.log.Printf("New window row:%v, col:%v\n", height,width)
	}
	if (request.Type == "no-more-sessions@openssh.com" || request.Type == "hostkeys-00@openssh.com" ) {
		session.proxy.log.Printf("skipping: %v",request.Type);
	} else {
		ok, product, err := outgoing_channel.SendRequest(request.Type, request.WantReply, request.Payload)
		if err != nil {
			if request.WantReply {
				if err := request.Reply(ok, product); err != nil {
					return fmt.Errorf("reply after send failure: %w", err)
				}
			}
			return fmt.Errorf("send request: %w", err)
		}
	
	

		if request.WantReply {
			if err := request.Reply(ok, product); err != nil {
				return fmt.Errorf("reply: %w", err)
			}
		}
	}
	return nil
}

func (session * sessionContext) sendSignalToClients(signal int) {
	for _, cur_signal := range session.msg_signal {
		cur_signal <- signal
	}
}

func (session * sessionContext) signalNewMessage() {
	session.sendSignalToClients(SIGNAL_NEW_MESSAGE)
}

func (session * sessionContext) signalSessionEnd() {
	session.sendSignalToClients(SIGNAL_SESSION_END)
}



func (session * sessionContext) markThreadStarted() {
	session.mutex.Lock()
	session.thread_count += 1
	session.mutex.Unlock()
}



func (session * sessionContext) end() {
	session.active = false
	session.stop_time = time.Now()
	session.handleEvent(
		&sessionEvent{
			Type: EVENT_SESSION_STOP,
			StopTime: session.getStopTimeAsUnix(),
		})
	session.signalSessionEnd()
	session.finalizeLog()
	session.proxy.addSessionToSessionList(session)
}

func (session * sessionContext) markThreadStopped() {
	session.mutex.Lock()
	session.thread_count -= 1
	if session.thread_count < 1 {
		session.end()
	}
	session.mutex.Unlock()
}


func (session * sessionContext) infoAsJSON() string {
	session_info := session_info_extended{
		Start_time: 	session.start_time.Unix(),
		Stop_time: 		session.stop_time.Unix(),
		Length:			int64(session.stop_time.Sub(session.start_time).Seconds()),
		Client_host:	session.client_host,
		Serv_host:		*session.proxy.server_ssh_ip +":"+strconv.Itoa(*session.proxy.server_ssh_port),
		Username:		session.client_username,
		Password: 		session.client_password,
		Term_rows:		session.term_rows,
		Term_cols:		session.term_cols,
		Filename:		session.filename}
	for _, request := range session.requests {
		session_info.Requests = append(session_info.Requests, request.Req_type)
	}
	data, err := json.Marshal(session_info)
	if err != nil {
		session.proxy.log.Println("Error during marshaling json: ", err)
		return ""
	}
	return string(data)
}



func (session * sessionContext) getTimeOffset() int64 {
	time_now := time.Now()
	return time_now.Sub(session.start_time).Milliseconds()
}


func (session * sessionContext) initializeLog()  {
	f, err := os.OpenFile(*session.proxy.session_folder + "/" + session.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		session.proxy.log.Println("error opening session log file:", err)
	}
	session.mutex.Lock()
		session.log_fd = f
	session.mutex.Unlock()
	session.appendToLog([]byte("[\n")); 
}

func (session * sessionContext) appendToLog(data []byte) {
	session.log_mutex.Lock()
	if _, err := session.log_fd.Write(data); err != nil {
		session.log_fd.Close() // ignore error; Write error takes precedence
		session.proxy.log.Println("error writing to log file:", err)
	}
	session.log_mutex.Unlock()
}

func (session * sessionContext) finalizeLog()  {
	session.appendToLog([]byte("\n]"))
	if err := session.log_fd.Close(); err != nil {
		session.proxy.log.Println("error closing log file:", err)
	}
	if(len(session.events)<10) {
		old_file := *session.proxy.session_folder + "/" + session.filename
		new_file := old_file + ".scan"
		err := os.Rename(old_file,new_file)
		if err != nil {
			session.proxy.log.Printf("Error moving log file from %v to %v: %v",old_file, new_file, err)
		}
		session.filename = new_file
	}
}


func (session * sessionContext) makeNewSignal() chan int {
	new_msg_signal := make(chan int)
	session.mutex.Lock()
	session.msg_signal = append(session.msg_signal,new_msg_signal)
	session.mutex.Unlock()
	return new_msg_signal
}



type channelWrapper struct {
	io.ReadWriter
	session * sessionContext 
	direction string
	data_type string
	start_time   time.Time
	channel_id int
}

func (channel * channelWrapper) Read(buff []byte) (bytes_read int, err error) {
	bytes_read, err = channel.ReadWriter.Read(buff)

	if err == nil {
		
		data_copy := make([]byte, bytes_read)
	
		copy(data_copy, buff)

		go channel.session.handleEvent(
			&sessionEvent{
				Type: EVENT_MESSAGE,
				Direction: channel.direction,
				ChannelType: channel.data_type,
				Size:	bytes_read,
				Data:	data_copy,
			})
	}

	return bytes_read, err
}

func channelTypeSupported(channelType string) bool {
    switch channelType {
    case
        "session":
        	return true
    }
    return false
}

// parseDims extracts two uint32s from the provided buffer.
// https://github.com/Scalingo/go-ssh-examples/blob/ae24797273aa9fcd3a8fa6c624af1b068a81d58b/server_complex.go#L229
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}



func newChannelWrapper(
	in_channel io.ReadWriter, 
	context * sessionContext, 
	direction string, 
	data_type string, 
	start_time time.Time,
	channel_type string,
	) io.ReadWriter {
	channel_index := context.channel_count
	context.channels = append(context.channels, &channel_data{chunks: make([]block_chunk, 0), channel_type: channel_type})
	context.channel_count += 1
	
	return &channelWrapper{ReadWriter: in_channel, session: context, direction: direction, data_type: data_type, start_time: start_time, channel_id: channel_index}
}

type request_data struct {
	Req_type string	`json:"request_type"`
	Req_payload	[]byte `json:"request_data"`
	Msg_type	string  `json:"type"`
	Offset		int64	`json:"offset"`
}

type channel_data struct {
	chunks []block_chunk
	channel_type string
}
type block_chunk struct {
	Direction string `json:"direction"`
	Channel_type string `json:"type"`
	Time_offset int64 `json:"offset"`
	Size int `json:"size"`
	Data []byte `json:"data"`
}

type window_message_extended struct {
	Rows int64 `json:"rows"`
	Columns int64 `json:"columns"`
	Type string `json:"type"`
	Offset int64 `json:"offset"`
}


type session_info_extended struct {
	Start_time	int64 `json:"start"`
	Stop_time	int64 `json:"stop"`
	Length		int64 `json:"length"`
	Client_host	string `json:"client_host"`
	Serv_host   string `json:"server_host"`
	Username	string `json:"username"`
	Password    string `json:"password"`
	Term_rows	uint32 `json:"term_rows"`
	Term_cols	uint32 `json:"term_cols"`
	Filename	string  `json:"filename"`
	Requests	[]string `json:"requests"`
}

// taken from 192-208: https://github.com/cmoog/sshproxy/blob/47ea68e82eaa4d43250d2a93c18fb26806cd67eb/reverseproxy.go#L192
// channelRequestDest wraps the ssh.Channel type to conform with the standard SendRequest function signiture.
// This allows for convenient code re-use in piping channel-level requests as well as global, connection-level
// requests.
type channelRequestDest struct {
	ssh.Channel
}

func (c channelRequestDest) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	ok, err := c.Channel.SendRequest(name, wantReply, payload)
	return ok, nil, err
}

// requestDest defines a resource capable of receiving requests, (global or channel).
type requestDest interface {
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
}