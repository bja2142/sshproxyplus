
let dummy_socket = {
    send: function() {return;}, 
    close: function() { return;}}
var active_queue = ""
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

//TODO: handle sessions that aren't active as a replay
// should involve rewriting the interface to query for active
// sessions and have it put inactive sessions in a separate spot
// on the screen
// but maybe this is better only for non-live?

function get_viewer_session_from_socket(session)
{
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onopen = function() {
        if(session.active) {
            terminal_reader.set_session_mode_live()
        }
        socket.send('viewer-get');
        socket.send(session.secret)
        socket.send(session.key);
        
    }
    //TODO: move next function to its own spot to allow code reuse

    socket.onmessage = (event) => {
        try {
            chunk = JSON.parse(event.data)
            
        } catch {
            console.log("error parsing event",event)
            return
        }
        console.log("loading",chunk,terminal_reader.events.length)
        terminal_reader.load_events(new TerminalEvent(chunk))
        console.log(terminal_reader.events.length)

        var pass_socket = socket
        terminal_reader.process_next_event(function() {
            pass_socket.send('ack');
        });
        if(chunk.type=="session-stop")
        {
            socket.close()
        }
    }
    socket.onclose = (event) =>
    {
        console.log("session socket closing")
        terminal_reader.set_session_mode_disconnected()
    }
    active_session_sockets[session.key] = socket
}

function get_public_session_from_socket(session)
{
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onopen = function() {
        if(session.active) {
            terminal_reader.set_session_mode_live()
        }
        socket.send('get');
        socket.send(session.key);
        
    }
    socket.onmessage = (event) => {
        try {
            chunk = JSON.parse(event.data)
            
        } catch {
            filename =session.key+".log.json";
            url_hash = "#replay&"+filename
            window.location.hash=url_hash
            read_hashes()
            return
        }
        console.log("loading",chunk,terminal_reader.events.length)
        terminal_reader.load_events(new TerminalEvent(chunk))
        console.log(terminal_reader.events.length)

        var pass_socket = socket
        terminal_reader.process_next_event(function() {
            pass_socket.send('ack');
        });
        if(chunk.type=="session-stop")
        {
            socket.close()
        }
    }
    socket.onclose = (event) =>
    {
        console.log("session socket closing")
        terminal_reader.set_session_mode_disconnected()
    }
    active_session_sockets[session.key] = socket
}

function get_session_from_socket(session,clickTarget)
{
    console.log(session)
    mark_selected(clickTarget);
    terminal_reader.reset()
    if(session.secret != undefined)
    {
        get_viewer_session_from_socket(session)
    } else {
        get_public_session_from_socket(session)
    }
}




function list_viewer_session(viwer_key)
{
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onmessage = (event) => {
    
        console.log(event)
        add_viewer_session_to_list(JSON.parse(event.data))
        };

    socket.onclose = (event) => {
        console.log(event)
        //jQuery("#active_session_list_container").removeClass("live").addClass("inactive")
    }
    
    
    socket.onopen = function() {
        //jQuery("#active_session_list_container").removeClass("inactive").addClass("live")
        socket.send('viewer-list');
        socket.send(viewer_key);
        setInterval(function() {
            jQuery("#session_list").empty();
            jQuery("#old_session_list").empty();
            socket.send('viewer-list');
            socket.send(viewer_key);
            },5000 )
        
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
        } else if (action_type == "signed-viewer") {
            if (action_key.indexOf("/") != -1) {
                viewer_elements = action_key.split("/")
                viewer_key = viewer_elements[0]
                viewer_session = viewer_elements[1]
                session_object = {secret: viewer_key, key: viewer_session, active: true}
                get_session_from_socket(session_object,jQuery("a[href*='"+window.location.hash+"']")[0])
            } else {
                viewer_key = action_key
                list_viewer_session(viewer_key)
            }

        } else if (action_type == "signed-session") {
            fetch_viewer_single_session(action_key,jQuery("a[href*='"+window.location.hash+"']")[0])
        }
    } else {
        //TODO: store interval
        //clear interval when resetting it
        setInterval(fetch_old_session_list,10*1000);
        init_query_socket();
        fetch_old_session_list();
    }
    
}


function seconds_to_str(sec)
{
    // https://stackoverflow.com/a/25279340
    return new Date(sec * 1000).toISOString().substring(11, 19);
}

function add_link_to_list(session, href, onclick_function,display_text,list_id)
{
        
        li = jQuery("<li />")
        anchor = jQuery("<a />")
        anchor.attr("href",href)
        anchor.click(onclick_function)
        //anchor.attr("onclick",onclick_text) // TODO: replace this with function
        // and add ability to capture the session item as an object
        // in function() {}
        anchor.text(display_text)
        if(session.key in active_session_sockets)
        {
            anchor.addClass("selected")
        }
        li.append(anchor)
        jQuery(list_id).prepend(li)
}

function add_viewer_session_to_list(sessions)
{
    for(i=0; i<sessions.length; i++)
    {
        session = sessions[i]
        if(session.active)
        {
            add_link_to_list(
                session, 
                "#signed-viewer&"+session.secret+"/"+session.key, 
                function(event) {get_session_from_socket(session,event.target);},
                session.key + " (" + seconds_to_str(session.length) +")",
                "#session_list"
            )
        } else {
            add_link_to_list(
                session, 
                "#signed-viewer&"+session.secret+"/"+session.key, 
                function(event) {get_session_from_socket(session,event.target);},
                session.key + " (" + seconds_to_str(session.length) +")",
                "#old_session_list"
            )
        }
       
    }
}
function add_session_to_list(sessions)
{
    for(i=0; i<sessions.length; i++)
    {
        session = sessions[i]
        add_link_to_list(
            session, 
            "#live&"+session.key, 
            function(event) {init_session_tty(session.key,event.target);},
            session.key + " (" + seconds_to_str(session.length) +")",
            "#session_list"
        )
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
        //socket.close()
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
    terminal_reader.reset()
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onopen = function() {
        terminal_reader.set_session_mode_live()
        socket.send('get');
        socket.send(keyname);
        
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
        console.log("loading",chunk,terminal_reader.events.length)
        terminal_reader.load_events(new TerminalEvent(chunk))
        console.log(terminal_reader.events.length)

        var pass_socket = socket
        terminal_reader.process_next_event(function() {
            pass_socket.send('ack');
        });
        if(chunk.type=="session-stop")
        {
            socket.close()
        }
    }
    socket.onclose = (event) =>
    {
        console.log("session socket closing")
        terminal_reader.set_session_mode_disconnected()
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

    cur_queue = active_queue
    while(cur_queue.length > 0)
    {
        cur_queue.pop()
    }
    active_queue = ""
     
    Object.keys(active_session_sockets).forEach(function(key) {
        console.log("resetting terminal")
        active_session_sockets[key].close();
        delete active_session_sockets[key]
     });
    global_terminal.reset()
}

function replay_session(data)
{
    console.log(this.url)
    console.log(data)
    terminal_reader.reset()
    var session = {requests: [],feed_type: "old"}
    var event_queue = []
    for(index=0; index<data.length; index++)
    {
        blob = data[index]
        switch(blob.type) {
            case "session-start":
                session = Object.assign({}, session, blob);
                break;
            default:
                event_queue.push(new TerminalEvent(blob))
                break;
        }
    }
    active_queue = this.url
    console.log(session)
    //statusbar_start(`From: ${session.client_host}; To: ${session.server_host};`,"old")
    //process_event_queue(event_queue)
    terminal_reader.load_events(event_queue)
    terminal_reader.update_session(session)
    terminal_reader.play()
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
    if(path == active_queue)
    {
        anchor.addClass("selected");
    }
    li.append(anchor)
    jQuery("#old_session_list").prepend(li)

}