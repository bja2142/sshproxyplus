
/*

TODO:
-rework how sessions are queried
  -- make read_hashes the intro function
    -- track intervals, and clear them when it is recalled
  -- integrate live, inactive sessions with recorded session URL (if available)
    -- maybe? or maybe it's better to view in live mode or in replay mode
    -- in either case I don't want two things updating the same list at the same time
    and clearing the other stuff

*/

/*
let dummy_socket = {
    send: function() {return;}, 
    close: function() { return;}}
var active_queue = ""*/
var active_session_sockets = {}
var active_query_sockets = []
var timers = []

function build_sock_addr()
{
    addr = window.location.origin

    addr = addr.replace("https://","wss://")
    addr = addr.replace("http://","ws://")
    return addr
}

function init_query_socket() {
    let socket = new WebSocket(build_sock_addr()+"/socket")
    var this_timer = -1
    socket.onmessage = (event) => {
    
        add_session_to_list(JSON.parse(event.data))
      };

    socket.onclose = (event) => {
        jQuery("#active_session_list_container").removeClass("live").addClass("inactive")
        close_and_remove_timer_from_list(this_timer)
    }
    
    
    socket.onopen = function() {
        jQuery("#session_list").empty();
        //jQuery("#old_session_list").empty();
        jQuery("#active_session_list_container").removeClass("inactive").addClass("live")
        this_timer = run_timer(function() {
            jQuery("#session_list").empty();
            //jQuery("#old_session_list").empty();
            socket.send('list-active');
            fetch_old_session_list()},
            5000)
        
    }
    active_query_sockets.push(socket)
}

function close_and_remove_timer_from_list(timer)
{
    if (timer != -1)
    {
        clearInterval(timer)
        timers = timers.filter(function(element) { return element !== timer })
    }
}

function get_viewer_session_from_socket(session)
{
    run_session_from_socket(session,true)
}

function run_session_from_socket(session,viewer_mode)
{
    //console.log("getting session",session)
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onopen = function() {
        if(session.active) {
            terminal_reader.set_session_mode_live()
        } else {
            terminal_reader.set_session_mode_disconnected()
            terminal_reader.pause()
        }
        if(viewer_mode)
        {
            socket.send('viewer-get');
            socket.send(session.secret)
        } else {
            socket.send('get');
        }
        
        socket.send(session.key);
        
    }
    //TODO: move next function to its own spot to allow code reuse

    socket.onmessage = (event) => {
        try {
            chunk = JSON.parse(event.data)
            
        } catch {
            console.log("error parsing event",event.data)
            return
        }
        //console.log("loading",chunk,terminal_reader.events.length)
        terminal_reader.load_events(new TerminalEvent(chunk))
        //console.log(terminal_reader.events.length)

        if(session.active)
        {
            terminal_reader.process_next_event(function() {
                socket.send('ack');
            });
        } else {
            socket.send('ack');
        }

        if(chunk.type=="session-stop")
        {
            socket.close()
            if(session.active)
            {
                socket.close();
                terminal_reader.set_session_mode_disconnected()
            } else {
                terminal_reader.play()
            }
            
        }
    }
    socket.onclose = (event) =>
    {
        //console.log("session socket closing")
        
        
    }
    active_session_sockets[session.key] = socket
}

function get_public_session_from_socket(session)
{
    run_session_from_socket(session,false)
    /*
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
        //console.log("loading",chunk,terminal_reader.events.length)
        terminal_reader.load_events(new TerminalEvent(chunk))
        //console.log(terminal_reader.events.length)

        terminal_reader.process_next_event(function() {
            socket.send('ack');
        });
        if(chunk.type=="session-stop")
        {
            socket.close()
        }
    }
    socket.onclose = (event) =>
    {
        //console.log("session socket closing")
        terminal_reader.set_session_mode_disconnected()
    }
    active_session_sockets[session.key] = socket
    */
}

function get_session_from_socket(session,clickTarget)
{
    //console.log(session)
    mark_selected(clickTarget);
    close_session_sockets();
    terminal_reader.reset()
    if(session.secret != undefined)
    {
        get_viewer_session_from_socket(session)
    } else {
        get_public_session_from_socket(session)
    }
}

function list_viewer_session_and_add_to_list(viewer_key)
{
    list_viewer_session(viewer_key,false,"");
}

function list_viewer_session_and_play_first_item(viewer_key)
{
    list_viewer_session(viewer_key,true,"");
}

function list_viewer_session_and_play_key(viewer_key,session_key)
{
    list_viewer_session(viewer_key,true,session_key);
}

function list_viewer_session(viewer_key,auto_play_item,session_key)
{
    list_session(viewr_key, auto_play_item, session_key, false)
}

function list_and_play_session(session_key)
{
    list_session("",true,session_key,false)
}

function list_session(viewer_key, auto_play_item, session_key, use_viewer)
{
    var this_timer = -1
    
    let socket = new WebSocket(build_sock_addr()+"/socket")
    socket.onmessage = (event) => {
    
        console.log(event)
        data = JSON.parse(event.data);
        if(auto_play_item)
        {
            if(data.length>0)
            {
                var session_object = data[0]
                if(session_key != "")
                {
                    for(index=0; index<data.length; index++)
                    {
                        if (data[index].key == session_key)
                        {
                            session_object = data[index]
                            break;
                        }
                    }
                } 
                get_session_from_socket(session_object,jQuery("a[href*='"+window.location.hash+"']")[0])
                socket.close();
            } else {
                if(session_key != "" && ! use_viewer)
                {
                    filename =session_key+".log.json";
                    url_hash = "#replay&"+filename
                    window.location.hash=url_hash
                    read_hashes()
                    socket.close()
                }
                
            }
        } else {
            add_viewer_session_to_list(data)
        }
    };

    socket.onclose = (event) => {
        ////console.log(event)
        close_and_remove_timer_from_list(this_timer)
        //jQuery("#active_session_list_container").removeClass("live").addClass("inactive")
    }
    
    
    socket.onopen = function() {
        //jQuery("#active_session_list_container").removeClass("inactive").addClass("live")
        if(use_viewer)
        {
            socket.send('viewer-list');
            socket.send(viewer_key);
        } else {
            socket.send('list-active');
        }

        jQuery("#session_list").empty();
        //jQuery("#old_session_list").empty();
        if(! auto_play_item)
        {
            this_timer = setInterval(function() {
                jQuery("#session_list").empty();
                //jQuery("#old_session_list").empty();
                socket.send('viewer-list');
                socket.send(viewer_key);
                },5000 )
            active_query_sockets.push(socket)
        }        
    }
}

function reset_timers_and_query_sockets()
{
    close_query_sockets()
    remove_timers()
}

function remove_timers() 
{
    while(timers.length > 0)
    {
        timer = timers.pop()
        clearInterval(timer)
    }
}

function close_query_sockets()
{
    while(active_query_sockets.length >0)
    {
        socket = active_query_sockets.pop()
        scoket.close()
    }
}

function close_session_sockets()
{
    Object.keys(active_session_sockets).forEach(function(key) {
        active_session_sockets[key].close();
        delete active_session_sockets[key]
     });
}

function read_hashes()
{
    
    if(window.location.hash != "" && window.location.hash.indexOf("&") != -1)
    {
        hash_elements = window.location.hash.slice(1).split("&")
        action_type = hash_elements[0]
        action_key = hash_elements[1]
        //console.log(action_type,action_key)
        if(action_type == "live") {
            list_and_play_session(action_key)
        } else if (action_type == "replay")
        {
            fetch_session_json_and_update_link('sessions/'+action_key,jQuery("a[href*='"+window.location.hash+"']")[0])
        } else if (action_type == "signed-viewer") {
            
            var viewer_key = "";
            if (action_key.indexOf("/") != -1) {
                viewer_elements = action_key.split("/")
                viewer_key = viewer_elements[0]
                viewer_session = viewer_elements[1]
                list_viewer_session_and_play_key(viewer_key,viewer_session)
                //get_session_from_socket(session_object,jQuery("a[href*='"+window.location.hash+"']")[0])
            } else {
                viewer_key = action_key
            }
            list_viewer_session_and_add_to_list(viewer_key);
            return

        } else if (action_type == "signed-session") {
            list_viewer_session_and_play_first_item(action_key)
            return
        }
    } 
    run_timer(fetch_old_session_list);
    init_query_socket();
}

function run_timer(fn,interval=10*1000)
{
    fn();
    timer_obj = setInterval(fn,interval);
    timers.push(timer_obj);
    return timer_obj;
}


function seconds_to_str(sec)
{
    // https://stackoverflow.com/a/25279340
    return new Date(sec * 1000).toISOString().substring(11, 19);
}

function add_link_to_list(session, href, list_id)
{
    existing_anchors = jQuery("a[href*='"+href+"']")
    if(existing_anchors.length>0)
    {
        if (href == window.location.hash)
        {
            existing_anchors.addClass("selected")
        }
    }
    else {
        existing_anchor = jQuery
        //add_session_to_list
        li = jQuery("<li />")
        anchor = jQuery("<a />")
        anchor.attr("href",href)
        //anchor.click(onclick_function)
        //anchor.attr("onclick",onclick_text) // TODO: replace this with function
        // and add ability to capture the session item as an object
        // in function() {}
        anchor.text(session.display_text)
        if(href == window.location.hash)
        {
            anchor.addClass("selected")
        }
        li.append(anchor)
        jQuery(list_id).prepend(li)
    }
}

function add_viewer_session_to_list(sessions)
{
    for(i=0; i<sessions.length; i++)
    {
        session = sessions[i]
        if(session.active)
        {
            fn = function() {
                var cur_session = session;
                cur_session.display_text = cur_session.key + " (" + seconds_to_str(cur_session.length) +")";
                add_link_to_list(
                    session, 
                    "#signed-viewer&"+session.secret+"/"+session.key, 
                    //function(event) {get_session_from_socket(cur_session,event.target);},
                    "#session_list"
                )
            }
            fn();
        } else {
            fn = function() {
                var cur_session = session;
                cur_session.display_text = cur_session.key + " (" + seconds_to_str(cur_session.length) +")"
                add_link_to_list(
                    session, 
                    "#signed-viewer&"+session.secret+"/"+session.key, 
                    //function(event) {get_session_from_socket(cur_session,event.target);},
                    "#old_session_list"
                )
            }
            fn();
        }
       
    }
}
function add_session_to_list(sessions)
{

    for(i=0; i<sessions.length; i++)
    {
        var session = sessions[i]
        if(session.active)
        {
            list_id = "#session_list"
            hash_path = "#live&"+session.key
        } else {
            if(session.filename == undefined)
            {
                hash_path = "#live&"+session.key
                return;

                // NOTE: if we want to only show replay data
                // we can remove the return here and change the type of query
                // to "list-all" instead of "list-active"
                
            } else {
                hash_path = "#replay&"+session.filename
            }
            list_id = "#old_session_list"
        }
        if(session.display_text == undefined)
        {
            session.display_text = session.key + " (" + seconds_to_str(session.length) +")"
        }
        add_link_to_list(
            session, 
            hash_path, 
            //function(event) {get_session_from_socket(session,event.target);},
            list_id
        )
    }
}
/*
function resize_terminal(rows, columns)
{
    new_height = parseInt(rows*global_row_height + 1)
    new_width = parseInt(columns*global_col_width + 1)
    if(new_height != NaN && new_width != NaN)
    {
        new_height = new_height+ "px";
        new_width = new_width + "px";
        //console.log("new dimensions",new_height,new_width)
        jQuery("#terminal_wrapper").css("width", new_width)
        jQuery("#terminal").css("width", new_width)
        jQuery("#terminal").css("height", new_height)
        global_fit_addon.fit()
    }   
}*/
/*

function convertToHex(str) {
    var hex = '';
    for(var i=0;i<str.length;i++) {
        hex += ''+str.charCodeAt(i).toString(16);
    }
    return hex;
}*/


/*
function process_outgoing(in_data)
{
    signal_chars="ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    data = in_data
    //console.log(convertToHex(data))
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
}*/
/*
function process_event(chunk,socket, callback = undefined)
{
    //console.log("process_event",chunk)
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
            //console.log(decoded_data)
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
        //console.log(chunk)
        if (chunk.request_type == "exec") 
        {
            update_statusbar(" exec:"+atob(chunk.request_payload),true)
            
        }
        ack()
    } else {
        ack()
    }
}*/

/*
function update_statusbar(text,append)
{
    if(!append)
    {
        jQuery("#terminal_statusbar span").empty()
    }
    new_text = document.createTextNode(text)
    jQuery("#terminal_statusbar span").append(new_text)
}*/

function mark_selected(obj)
{
    jQuery(".selected").removeClass("selected")
    jQuery(obj).addClass("selected")
}

/*
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
        //console.log("loading",chunk,terminal_reader.events.length)
        terminal_reader.load_events(new TerminalEvent(chunk))
        //console.log(terminal_reader.events.length)

        terminal_reader.process_next_event(function() {
            socket.send('ack');
        });
        if(chunk.type=="session-stop")
        {
            socket.close()
        }
    }
    socket.onclose = (event) =>
    {
        //console.log("session socket closing")
        terminal_reader.set_session_mode_disconnected()
    }
    active_session_sockets[keyname] = socket
}*/
/*
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
}*/

/*
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
}*/

/*
function reset_terminal()
{
    jQuery("#keystrokes").empty();
    //console.log("cleared")

    cur_queue = active_queue
    while(cur_queue.length > 0)
    {
        cur_queue.pop()
    }
    active_queue = ""
     
    Object.keys(active_session_sockets).forEach(function(key) {
        //console.log("resetting terminal")
        active_session_sockets[key].close();
        delete active_session_sockets[key]
     });
    global_terminal.reset()
}*/

function replay_session_event_list(data)
{
    //console.log(this.url)
    //console.log(data)
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
    //active_queue = this.url
    //console.log(session)
    //statusbar_start(`From: ${session.client_host}; To: ${session.server_host};`,"old")
    //process_event_queue(event_queue)
    terminal_reader.load_events(event_queue)
    terminal_reader.update_session(session)
    terminal_reader.play()
}

function fetch_session_json_and_update_link(filepath,obj)
{
    mark_selected(obj)
    jQuery.getJSON(filepath, replay_session_event_list)
}

function fetch_old_session_list()
{
    jQuery.get("/sessions/.session_list",update_old_session_list);
}

function update_old_session_list(data)
{
    //jQuery("#old_session_list").empty();
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
        session.active = false

        
        add_old_session_to_list(session)
    }
    
}

function add_old_session_to_list(obj)
{
    filename = obj.filename
    path = "sessions/"+filename
    cur_href = "#replay&"+filename
    existing_anchors = jQuery("a[href*='"+cur_href+"']")
    if(existing_anchors.length>0)
    {
        if (path == window.location.hash)
        {
            existing_anchors.addClass("selected")
        }
    }
    else {
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
        session.display_text = anchor_text
        add_session_to_list([session])
        /*
        li = jQuery("<li />")
        anchor = jQuery("<a />")
        
        anchor.attr("href",cur_href)
        //anchor.attr("onclick","javascript:fetch_session_json_and_update_link('sessions/"+filename+"',this)")
        anchor.text(anchor_text)
        if(path == window.location.hash)
        {
            anchor.addClass("selected");
        }
        li.append(anchor)
        jQuery("#old_session_list").prepend(li)*/
    }
    

}