//const { Terminal } = require("xterm");
//const { FitAddon } = require("xterm-addon-fit");

class TerminalEvent {
    blob

    constructor(blob)
    {
        this.blob = blob
        for (const [key, value] of Object.entries(blob)) {
            this[key] = value
          }
    }
    get time_offset()
    {
        return this.offset
    }
}
 
class TerminalReader {
    #speed = 1.0
    #paused = true
    initial_width = 800
    initial_height = 300
    #terminal_col_width = 0
    #terminal_row_height = 0
    #terminal
    #fit_addon
    #events =[]
    #event_index
    #cur_event
    #timeout = -1
    #timeout_delay = 0
    #timeout_timestamp = 0
    #timeout_delay_saved = 0

    #statusbar
    #terminal_element
    #keystrokes
    #reader_element

    #session = {}


    

    constructor(element_id) {
        this.element_id = element_id
        this.clear_events()
    }

    initialize()
    {
        this.reset_session()
        this.build_reader()
        this.initialize_terminal()
    }

    build_reader()
    {
        this.#reader_element = jQuery("#"+this.element_id)
        this.reader_element.empty()
        this.reader_element.addClass("terminal_reader")

        this.#statusbar = jQuery('<h4 class="statusbar inactive">Currently Displaying: <span>Nothing</span></h4>')
        this.reader_element.append(this.statusbar)

        this.#terminal_element = jQuery('<div></div>')
        this.terminal_element.attr("id",this.terminal_id)
        this.terminal_element.addClass("terminal-box")
        this.reader_element.append(this.terminal_element)

        this.#keystrokes = jQuery('<div></div>')
        this.keystrokes.addClass("keystrokes")
        this.keystrokes.attr("placeholder","Session input goes here...")
        this.reader_element.append(this.keystrokes)
        
    }

    get ended()
    {
        return this.#event_index >= this.#events.length
    }
    get timeout_delay_saved()
    {
        return this.#timeout_delay_saved
    }

    get reader_element()
    {
        return this.#reader_element
    }
    get keystrokes()
    {
        return this.#keystrokes
    }
    get terminal_element()
    {
        return this.#terminal_element
    }
    get statusbar()
    {
        return this.#statusbar
    }

    get terminal_id()
    {
        return this.element_id+"_terminal"
    }

    update_col_width()
    {
        this.#terminal_col_width = parseInt(this.terminal_element.css("width").slice(0,-2) ) / this.terminal.cols
    }
    update_col_height()
    {
        this.#terminal_row_height = parseInt(this.terminal_element.css("height").slice(0,-2)) / this.terminal.rows
    }

    fit_terminal()
    {
        this.#fit_addon.fit()
        this.update_col_width()
        this.update_col_height()
    }

    initialize_terminal()
    {
        this.#terminal = new Terminal()
        this.#fit_addon = new FitAddon.FitAddon()
        this.#terminal.loadAddon(this.#fit_addon)
        this.#terminal.open(document.getElementById(this.terminal_id));
        this.fit_terminal()
    }

    get events()
    {
        return this.#events
    }
    load_events(events)
    {
        this.#events = this.events.concat(events)
    }

    resize_by_pixels(width,height)
    {
        var new_height = height+ "px";
        var new_width = width + "px";
        this.reader_element.css("width", new_width)
        this.terminal_element.css("width", new_width)
        this.terminal_element.css("height", new_height)
        this.#fit_addon.fit()
        console.log("new dimensions",new_height,new_width)
    }

    resize(rows,cols)
    {
        var new_height = parseInt(rows*this.terminal_row_height + 1)
        var new_width = parseInt(cols*this.terminal_col_width + 1)
        this.resize_by_pixels(new_width,new_height)
    }

    update_session(obj)
    {
        this.#session = Object.assign({}, this.#session, obj);
    }

    reset_session()
    {
        this.#session = {}
    }

    reset()
    {
        this.#speed = 1;
        this.reset_timeout()
        this.reset_session()
        this.refresh_statusbar()
        this.terminal.reset()
        this.keystrokes.empty()
        this.clear_events()
        this.resize_by_pixels(this.initial_width,this.initial_height)
    }

    clear_events()
    {
        this.#events = []
        this.#event_index = -1;
        this.#cur_event = undefined
    }

    start_timer_for_next_event(in_delay=-1)
    {
        console.log("start timer")
        if(in_delay == -1) {       
            if (this.events.length > this.#event_index+1)
            {
                console.log("doit")
                var next_event_index = this.#event_index+1
                var next_event = this.#events[next_event_index]
                console.log(next_event) 
                console.log(this.#cur_event)
                var delay = next_event.time_offset - this.#cur_event.time_offset
                delay = delay / this.speed
                delay = parseInt(delay.toFixed())
                
            } else {
                delay = 0;
            }
        } else {
            delay = in_delay
        }
        var self = this
        self.#timeout = setTimeout(function() {
            self.process_next_event(); },delay);
        this.#timeout_delay = delay
        this.#timeout_timestamp = Date.now()
        
    }

    get time_to_next_event()
    {
        var difference = Date.now() - this.#timeout_timestamp
        return Math.max((this.#timeout_delay-difference),0)
    }

    process_keystrokes(in_data)
    {
        var signal_chars="ABCDEFGHIJKLMNOPQRSTUVWXYZ"
        var data = in_data
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
        return data
    }

    get client()
    {
        return this.#session.client_host
    }

    get host()
    {
        return this.#session.server_host
    }
    
    refresh_statusbar()
    {
        var session_type = this.#session.feed_type
        var statusbar_text = `From: ${this.client};  To: ${this.host};`
        if(this.ended)
        {
            statusbar_text += " (ended)"
            this.statusbar.removeClass("live old inactive").addClass("inactive")

        } else {
            this.statusbar.removeClass("live old inactive").addClass(session_type)
            if (this.paused)
             {
                statusbar_text += " (paused)"
            } else {
                statusbar_text += " (playing)"
            }
        }
        this.statusbar.empty()
        this.statusbar.text(statusbar_text)
        
        // draw statusbar here
    }

    write_keystrokes(in_data)
    {
        var data = this.process_keystrokes(in_data)
        var new_text = document.createTextNode(data)
        this.keystrokes.append(new_text);
        this.keystrokes.scrollTop(this.keystrokes[0].scrollHeight);
    }
    action_event(event,callback)
    {
        console.log(event)
        
        if (event.type == "window-resize") {
            this.resize(event.term_rows, event.term_cols)
            callback()
        } else if (event.type == "new-message") {
            if (event.direction == "incoming") {
                var decoded_data = atob (event.data)
                decoded_data = decoded_data.replace(/[\r\n]+/g, "\n").replace(/\n/g, "\r\n") 
                this.terminal.write(decoded_data, callback)            
            } else if (event.direction == "outgoing") {
                var decoded_data = atob (event.data)
                this.write_keystrokes(decoded_data)
                callback()
            }
        } else if (event.type == "session-stop") {
            callback()
        } else if (event.type == "new-request") {
            
            if (event.request_type == "exec") 
            {
                this.#session.terminal_type = "exec"
                this.write_keystrokes(atob(event.request_payload))
            }
            callback()
        } else {
            callback()
        }

    }
    process_next_event(callback=undefined)
    {
        
        var ack = function() {
            if (callback != undefined)
            {
                callback()
            }
        }
        if (this.event_index == -1)
        {
            this.#event_index = 0;
        }
        if (this.#events.length > this.#event_index)
        {
            this.#cur_event = this.#events[this.#event_index]
            this.action_event(this.#cur_event, ack)            
            this.start_timer_for_next_event()
            this.#event_index++
        } else {
            this.pause()
        }
        this.refresh_statusbar()
    }

    get terminal() 
    {
        return this.#terminal
    }

    redo_next_event()
    {
        var delay = self.time_to_next_event
        this.reset_timeout()
        this.start_timer_for_next_event(this.timeout_delay_saved)
        this.#timeout_delay_saved= 0
    }

    increase_speed(amount=0.25)
    {
        this.#speed += amount
        this.redo_next_event()
    }

    decrease_speed(amount=0.25)
    {
        this.#speed -= amount
        this.redo_next_event()
    }

    get speed()
    {
        return this.#speed
    }

    get event_index()
    {
        return this.#event_index
    }

    reset_timeout()
    {
        if(this.#timeout != -1)
        {
            clearTimeout(this.#timeout)
            this.#timeout = -1
        }
        this.#timeout_delay_saved = this.time_to_next_event
        this.#timeout_delay = 0
        this.#timeout_timestamp = 0
    }
    next()
    {
        this.reset_timeout()
        
        
        this.process_next_event()
    }

    pause()
    {
        this.#paused = true
        this.reset_timeout()
        
    }

    play()
    {
        this.#paused = false
        this.start_timer_for_next_event(this.timeout_delay_saved)
        this.#timeout_delay_saved= 0
    }
    get paused()
    {
        return this.#paused
    }
}