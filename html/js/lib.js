
let dummy_socket = {
    send: function() {return;}, 
    close: function() { return;}}
var active_queues = {}
var active_session_sockets = {}

function build_sock_addr()
{
    addr = window.location.origin

    addr = addr.replace("https://","wss://")
    addr = addr.replace("http://","ws://")
    return addr
}

function init_query_socket() {
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onmessage = (event) => {
    
        add_session_to_list(JSON.parse(event.data))
      };

    socket.onclose = (event) => {
        jQuery("#active_session_list_container").removeClass("live").addClass("inactive")
    }
    
    
    socket.onopen = function() {
        jQuery("#active_session_list_container").removeClass("inactive").addClass("live")
        socket.send('list-active');
        setInterval(function() {
            jQuery("#session_list").empty();
            socket.send('list-active');},5000
            )
        
    }
}

function read_hashes()
{
    if(window.location.hash != "" && window.location.hash.indexOf("&") != -1)
    {
        hash_elements = window.location.hash.slice(1).split("&")
        action_type = hash_elements[0]
        action_key = hash_elements[1]
        console.log(action_type,action_key)
        if(action_type == "live") {
            init_session_tty(action_key,jQuery("a[href*='"+window.location.hash+"']")[0])
        } else if (action_type == "replay")
        {
            fetch_session('sessions/'+action_key,jQuery("a[href*='"+window.location.hash+"']")[0])
        }
    }
    
}


function seconds_to_str(sec)
{
    // https://stackoverflow.com/a/25279340
    return new Date(sec * 1000).toISOString().substring(11, 19);
}

function add_session_to_list(sessions)
{
    for(i=0; i<sessions.length; i++)
    {
        session = sessions[i]
        
        li = jQuery("<li />")
        anchor = jQuery("<a />")
        anchor.attr("href","#live&"+session.key)
        anchor.attr("onclick","javascript:init_session_tty('"+session.key+"',this)")
        anchor.text(session.key + " (" + seconds_to_str(session.length) +")")
        if(session.key in active_session_sockets)
        {
            anchor.addClass("selected")
        }
        li.append(anchor)
        jQuery("#session_list").prepend(li)
    }
}

function resize_terminal(rows, columns)
{
    new_height = parseInt(rows*global_row_height + 1)
    new_width = parseInt(columns*global_col_width + 1)
    if(new_height != NaN && new_width != NaN)
    {
        new_height = new_height+ "px";
        new_width = new_width + "px";
        console.log("new dimensions",new_height,new_width)
        jQuery("#terminal_wrapper").css("width", new_width)
        jQuery("#terminal").css("width", new_width)
        jQuery("#terminal").css("height", new_height)
        global_fit_addon.fit()
    }   
}


function convertToHex(str) {
    var hex = '';
    for(var i=0;i<str.length;i++) {
        hex += ''+str.charCodeAt(i).toString(16);
    }
    return hex;
}
var replace_list = [
    [/\x1b.\x44/,'[LEFT ARROW]'],
    [/\x1b.\x43/,'[RIGHT ARROW]'],
    [/\x1b.\x41/,'[UP ARROW]'],
    [/\x1b.\x42/,'[DOWN ARROW]'],
    [/\x1b.\x46/,'[END]'],
    [/\x1b.\x48/,'[HOME]'],
    [/\x1b\x5b\x33\x7e/,'[DELETE]'],
    [/\x1b\x5b\x36\x7e/,'[PAGE DOWN]'],
    [/\x1b\x5b\x35\x7e/,'[PAGE UP]'],
    [/\x1b\x5b\x32\x7e/,'[INSERT]'],
    [/\x1b\x4f\x50/,'[F1]'],
    [/\x1b\x4f\x51/,'[F2]'],
    [/\x1b\x4f\x52/,'[F3]'],
    [/\x1b\x4f\x53/,'[F4]'],
    [/\x1b\x5b\x31\x35\x7e/,'[F5]'],
    [/\x1b\x5b\x31\x37\x7e/,'[F6]'],
    [/\x1b\x5b\x31\x38\x7e/,'[F7]'],
    [/\x1b\x5b\x31\x39\x7e/,'[F8]'],
    [/\x1b\x5b\x32\x30\x7e/,'[F9]'],
    [/\x1b\x5b\x32\x31\x7e/,'[F10]'],
    [/\x1b\x5b\x32\x33\x7e/,'[F11]'],
    [/\x1b\x5b\x32\x34\x7e/,'[F12]'],
    [/\x09/,'[TAB]'],
    [/\x1b/,'[ESC]'],
    [/\r/,'[CARRIAGE RETURN]'],
    [/\n/,'[LINE FEED]'],
    [/\x7f/,'[BACKSPACE]'],
]

function process_outgoing(in_data)
{
    signal_chars="ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    data = in_data
    console.log(convertToHex(data))
    for(index=0; index<replace_list.length; index++)
    {
        data = data.replace(replace_list[index][0],replace_list[index][1])
    }
    for(index=1; index<signal_chars.length; index++)
    {
        data = data.replace(String.fromCharCode(index),"[CTRL+"+signal_chars[index-1]+"]")

    }
    for(index=0; index<data.length; index++)
    {
        if (data.charCodeAt(index) < 0x20 && data.charCodeAt(index) != 9 && data.charCodeAt(index) != 10) {
            data = data.substring(0,index) + "[\\" + data.charCodeAt(index) + "]" + data.substring(index+1,data.length)
        }
    }
    jQuery("#keystrokes").append(data);
    jQuery("#keystrokes").scrollTop(jQuery("#keystrokes")[0].scrollHeight);
}

function process_event(chunk,socket, callback = undefined)
{
    console.log("process_event",chunk)
    ack = function() {
        if (callback != undefined)
        {
            callback()
        }
        socket.send('ack');
    }
    
    if (chunk.type == "window-resize") {
        resize_terminal(chunk.term_rows, chunk.term_cols)
        ack()
    } else if (chunk.type == "new-message") {
        if (chunk.direction == "incoming") {
            decoded_data = atob (chunk.data)
            console.log(decoded_data)
            decoded_data = decoded_data.replace(/[\r\n]+/g, "\n").replace(/\n/g, "\r\n") 
            global_terminal.write(decoded_data, () => { ack() })            
        } else if (chunk.direction == "outgoing") {
            decoded_data = atob (chunk.data)
            process_outgoing(decoded_data)
            ack()
        }
    } else if (chunk.type == "session-stop") {
        ack()
        statusbar_end()
        socket.close()
    } else if (chunk.type == "new-request") {
        console.log(chunk)
        if (chunk.request_type == "exec") 
        {
            update_statusbar(" exec:"+atob(chunk.request_payload),true)
            
        }
        ack()
    } else {
        ack()
    }
}

function update_statusbar(text,append)
{
    if(!append)
    {
        jQuery("#terminal_statusbar span").empty()
    }
    new_text = document.createTextNode(text)
    jQuery("#terminal_statusbar span").append(new_text)
}

function mark_selected(obj)
{
    jQuery(".selected").removeClass("selected")
    jQuery(obj).addClass("selected")
}

function init_session_tty(keyname,obj)
{
    mark_selected(obj);
    reset_terminal()
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onopen = function() {
        socket.send('get');
        socket.send(keyname);
        statusbar_start(keyname,"live")
    }
    socket.onmessage = (event) => {
        try {
            chunk = JSON.parse(event.data)
            
        } catch {
            filename =keyname+".log.json";
            url_hash = "#replay&"+filename
            window.location.hash=url_hash
            read_hashes()
            return
        }
        process_event(chunk,socket)
    }
    socket.onclose = (event) =>
    {
        statusbar_end()
    }
    active_session_sockets[keyname] = socket
}

function statusbar_end()
{
    if(jQuery("#terminal_statusbar").text().indexOf("(ended)") != -1) {
        update_statusbar(" (ended)",true);
    }
    jQuery("#terminal_statusbar").removeClass("live old inactive").addClass("inactive")
}

function statusbar_start(msg,color)
{
    update_statusbar(msg,false);
    jQuery("#terminal_statusbar").removeClass("live old inactive").addClass(color)
}

function process_event_queue(queue)
{
    if (queue.length >0)
    {
        cur_event = queue.shift()
        if (queue.length > 0)
        {
            delay = queue[0].offset - cur_event.offset
        } else {
            delay = 0
        }
        
        process_event(cur_event,dummy_socket, function() {
            setTimeout(function() {process_event_queue(queue);}, delay);
        })
        
    }
}

function reset_terminal()
{
    jQuery("#keystrokes").empty();
    console.log("cleared")
    Object.keys(active_queues).forEach(function(key) {
        cur_queue = active_queues[key]
        while(cur_queue.length > 0)
        {
            cur_queue.pop()
        }
        delete active_queues[key]
     });
    Object.keys(active_session_sockets).forEach(function(key) {
        active_session_sockets[key].close();
        delete active_session_sockets[key]
     });
    global_terminal.reset()
}

function replay_session(data)
{
    console.log(this.url)
    console.log(data)
    reset_terminal()
    var session = {requests: []}
    var event_queue = []
    for(index=0; index<data.length; index++)
    {
        blob = data[index]
        switch(blob.type) {
            case "session-start":
                session = Object.assign({}, session, blob);
                break;
            default:
                event_queue.push(blob)
                break;
        }
    }
    resize_terminal(session.term_rows, session.term_columns)
    active_queues[this.url]= event_queue
    console.log(session)
    statusbar_start(`From: ${session.client_host}; To: ${session.server_host};`,"old")
    process_event_queue(event_queue)
}

function fetch_session(filepath,obj)
{
    mark_selected(obj)
    jQuery.getJSON(filepath, replay_session)
}

function fetch_old_session_list()
{
    jQuery.get("/sessions/.session_list",update_old_session_list);
}

function update_old_session_list(data)
{
    jQuery("#old_session_list").empty();
    lines = data.split("\n")
    for (index=0; index<lines.length; index++)
    {
        line = lines[index]
        if(line == "")
        {
            continue
        }
        session = JSON.parse(line)
        if(session.filename.slice(-5)==".scan")
        {
            continue
        }
        add_old_session_to_list(session)
    }
    
}

function add_old_session_to_list(obj)
{
    filename = obj.filename
    anchor_text = `${obj.client_host} - ${obj.length}s`
    if(obj.requests)
    {
        if(obj.requests.includes("pty-req"))
        {
            anchor_text += " (pty)"
        } else if (obj.requests.includes("exec"))
        {
            anchor_text += " (exec)"
        }
    }

    li = jQuery("<li />")
    anchor = jQuery("<a />")
    path = "sessions/"+filename
    anchor.attr("href","#replay&"+filename)
    anchor.attr("onclick","javascript:fetch_session('sessions/"+filename+"',this)")
    anchor.text(anchor_text)
    if(path in active_queues)
    {
        anchor.addClass("selected");
    }
    li.append(anchor)
    jQuery("#old_session_list").prepend(li)

}