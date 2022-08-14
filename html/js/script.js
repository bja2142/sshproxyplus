
function get_base_host_addr()
{
    base_host =  window.location.host.indexOf(":")
    if (base_host != -1) {
        base_host = window.location.host.substring(0,base_host)
    } else {
        base_host = window.location.host
    }
    base_host = base_host +":8080"
    return base_host
}

function init_query_socket() {
    let socket = new WebSocket("ws://"+get_base_host_addr()+"/socket")
    socket.onmessage = (event) => {
    
        add_session_to_list(JSON.parse(event.data))
      };
    
    socket.onopen = function() {
        socket.send('list');
        setInterval(function() {
            jQuery("#session_list").empty();
            socket.send('list');},5000
            )
        
    } 
}

var global_terminal;
var global_row_height;
var global_col_width;
var global_fit_addon;
jQuery.noConflict()
jQuery(document).ready(function() {
    init_query_socket();
    global_terminal = new Terminal();
    const fitAddon = new FitAddon.FitAddon();
    global_terminal.loadAddon(fitAddon);
    global_terminal.open(document.getElementById('terminal'));
    fitAddon.fit();
    console.log(fitAddon.proposeDimensions())

    global_col_width = 
        parseInt(jQuery("#terminal").css("width").slice(0,-2)) / global_terminal.cols 
    
    global_row_height = 
        parseInt(jQuery("#terminal").css("height").slice(0,-2)) / global_terminal.rows 
    
    global_fit_addon = fitAddon;
});


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
        anchor.attr("href","javascript:init_session_tty('"+session.key+"')")
        anchor.text(session.key + " (" + seconds_to_str(session.length) +")")
        li.append(anchor)
        jQuery("#session_list").append(li)
    }
}

function resize_terminal(rows, columns)
{
    new_height = parseInt(rows*global_row_height + 1)+ "px";
    new_width = parseInt(columns*global_col_width + 1) + "px";
    console.log("new dimensions",new_height,new_width)
    jQuery("#terminal").css("width", new_width)
    jQuery("#terminal").css("height", new_height)
    global_fit_addon.fit()
}

function init_session_tty(keyname)
{
    let socket = new WebSocket("ws://"+get_base_host_addr()+"/socket")
    socket.onopen = function() {
        socket.send('get');
        socket.send(keyname);
    }
    socket.onmessage = (event) => {
        chunk = JSON.parse(event.data)
        if (chunk.type == "window-size") {
            resize_terminal(chunk.rows, chunk.columns)
            socket.send('ack');
        } else {
            if (chunk.direction == "incoming") {
                decoded_data = atob (chunk.data)
                console.log(decoded_data)
                global_terminal.write(decoded_data, () => { socket.send('ack'); })
            } else {
                socket.send('ack');
            }
        }
        
    }
}