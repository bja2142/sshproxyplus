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
    #timeout_delay_speed = 1
    #timeout_delay = 0
    #timeout_timestamp = 0
    #timeout_delay_saved = 0
    #terminal_buffer = ""
    #keystroke_buffer = ""

    #statusbar
    #terminal_element
    #keystrokes
    #reader_element
    #controlbar
    #controlbar_status
    #controlbar_interval

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
        this.reader_element.addClass("inactive")

        this.#statusbar = jQuery('<h4 class="statusbar">Currently Displaying: <span>Nothing</span></h4>')
        this.reader_element.append(this.statusbar)

        this.#terminal_element = jQuery('<div></div>')
        this.terminal_element.attr("id",this.terminal_id)
        this.terminal_element.addClass("terminal-box")
        this.reader_element.append(this.terminal_element)

        this.#controlbar = jQuery('<div class="controlbar"></div>')
        //buttons

        var self = this

        // add dropdown menu for input events

        // add a slider on top

        var media_buttons = jQuery('<div></div>').addClass("media_buttons")
        
        //slow
        media_buttons.append(jQuery('<span></span>')
            .addClass("slower").addClass("button")
            .click(function() {self.decrease_speed()})
        )

        //prev
        media_buttons.append(jQuery('<span></span>')
            .addClass("prev").addClass("button")
            .click(function() {self.prev()})
        )

        //pause
        media_buttons.append(jQuery('<span></span>')
            .addClass("pause").addClass("button")
            .click(function() {self.pause()})
        )

        //play
        media_buttons.append(jQuery('<span></span>')
            .addClass("play").addClass("button")
            .click(function() {self.play()})
        )

        //next
        media_buttons.append(jQuery('<span></span>')
            .addClass("next").addClass("button")
            .click(function() {self.next()})
        )

        //fast
        media_buttons.append(jQuery('<span></span>')
            .addClass("faster").addClass("button")
            .click(function() {self.increase_speed()})
        )
        
        this.controlbar.append(media_buttons)

        this.controlbar.addClass("controlbar")

        this.#controlbar_status = jQuery('<div></div>')
            .addClass("statusbar")
        this.controlbar.append(this.controlbar_status)

        this.reader_element.append(this.controlbar)

        this.#keystrokes = jQuery('<div></div>')
        this.keystrokes.addClass("keystrokes")
        this.keystrokes.attr("placeholder","Session input goes here...")
        this.reader_element.append(this.keystrokes)
        
        this.#controlbar_interval = setInterval(function(){self.update_controlbar_status()},250)
    }

    update_controlbar_status()
    {
        var msg = `Event: ${this.event_index}; Next: ${this.time_to_next_event}ms; Speed: ${((this.speed*1.0)+"").slice(0,5)}`
        this.controlbar_status.empty().text(msg)

        //update slider
    }

    get ended()
    {
        return (this.#event_index >= this.#events.length && this.is_live == false)
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

    get controlbar_status()
    {
        return this.#controlbar_status
    }

    get terminal_element()
    {
        return this.#terminal_element
    }
    get statusbar()
    {
        return this.#statusbar
    }

    get controlbar()
    {
        return this.#controlbar
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

    get terminal_row_height()
    {
        return this.#terminal_row_height
    }

    get terminal_col_width()
    {
        return this.#terminal_col_width
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
        this.#events = this.#events.concat(events)
    }

    resize_by_pixels(width,height)
    {
        var new_height = height+ "px";
        var new_width = width + "px";
        this.reader_element.css("width", new_width)
        this.terminal_element.css("width", new_width)
        this.terminal_element.css("height", new_height)
        this.#fit_addon.fit()
        //console.log("new dimensions",new_height,new_width)
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
        this.#session = { live: false, feed_type: "old"}
    }

    reset()
    {
        this.#paused = false
        this.#speed = 1;
        this.reset_timeout()
        this.reset_session()
        this.refresh_statusbar()
        this.terminal.reset()
        this.keystrokes.empty()
        this.clear_events()
        this.resize_by_pixels(this.initial_width,this.initial_height)
        this.clear_buffers()
    }

    clear_buffers()
    {
        this.#terminal_buffer = ""
        this.#keystroke_buffer = ""
    }

    clear_events()
    {
        this.#events = []
        this.#event_index = -1;
        this.#cur_event = undefined
    }

    start_timer_for_next_event(in_delay=-1)
    {
        //console.log("start timer")
        if(in_delay == -1) {       
            if (this.events.length > this.#event_index+1)
            {
                //console.log("doit")
                var next_event_index = this.#event_index+1
                var next_event = this.#events[next_event_index]
                //console.log(next_event) 
                //console.log(this.#cur_event)
                var delay = next_event.time_offset - this.#cur_event.time_offset
                delay = delay / this.speed
                delay = parseInt(delay.toFixed())
                this.#timeout_delay_speed = this.speed
                
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
            this.reader_element.removeClass("live old inactive").addClass("inactive")

        } else {
            this.reader_element.removeClass("live old inactive").addClass(session_type)
            if (this.paused)
             {
                statusbar_text += " (paused)"
            } else {
                statusbar_text += " (playing)"
            }
        }
        if(this.session.terminal_type != undefined)
        {
            statusbar_text += ` - (${this.session.terminal_type})`
        }
        this.statusbar.empty()
        this.statusbar.text(statusbar_text)
        
        // draw statusbar here
    }

    write_keystroke_buffer(callback=undefined)
    {
        var in_data = this.#keystroke_buffer
        var data = this.process_keystrokes(in_data)
        var new_text = document.createTextNode(data)
        this.keystrokes.append(new_text);
        if(this.session.terminal_type == "exec")
        {
            this.keystrokes.append(jQuery("<br />"))
        }
        this.keystrokes.scrollTop(this.keystrokes[0].scrollHeight);
        if(callback!=undefined)
        {
            callback();
        }
        this.#keystroke_buffer = ""
    }
    write_terminal_buffer(callback=undefined)
    {
        if(callback == undefined)
        {
            callback = function() {}
        }
        this.terminal.write(this.#terminal_buffer,callback());
        this.#terminal_buffer = ""
    }
    action_event(event)
    {
      
        if(event.type == "session-start")
        {
            delete event.type
            this.update_session(event)
        } else if (event.type == "window-resize") {
            delete event.type
            this.update_session(event)
            this.resize(event.term_rows, event.term_cols)
        } else if (event.type == "new-message") {
            if (event.direction == "incoming") {
                var decoded_data = atob (event.data)
                decoded_data = decoded_data.replace(/[\r\n]+/g, "\n").replace(/\n/g, "\r\n") 
                this.#terminal_buffer += decoded_data      
            } else if (event.direction == "outgoing") {
                var decoded_data = atob (event.data)
                this.#keystroke_buffer += decoded_data
            }
        } else if (event.type == "new-request") {
            
            if (event.request_type == "exec") 
            {
                this.#session.terminal_type = "exec"
                this.#keystroke_buffer += atob(event.request_payload) 
            } else if (event.request_type == "pty-req") {
                this.#session.terminal_type = "pty"
            }
        } 

    }
    write_buffers(callback=undefined,always_callback=true)
    {
        //console.log(this.keystroke_buffer,this.terminal_buffer,always_callback)
        if(callback == undefined)
        {
            callback = function() {}
        }
        if(this.#terminal_buffer != "")
        {
            if(this.#keystroke_buffer != "")
            {
                this.write_keystroke_buffer()
            }
            this.write_terminal_buffer(callback)
        } else if(this.#keystroke_buffer != "") {
            if(always_callback)
            {
                this.write_keystroke_buffer(callback)
            } else {
                this.write_keystroke_buffer()
            }
            
        } else {
            callback()            
        }
    }
    set_session_mode_live()
    {
        this.#session["live"] = true
        this.#session["feed_type"] = "live"

    }
    set_session_mode_disconnected()
    {
        this.#session["live"] = false
    }
    process_next_event(callback=undefined)
    {
        if (callback == undefined)
        {
            callback = function() {}
        }
        //console.log("paused:",this.paused)
        if(this.paused && ! this.is_live)
        {
            return
        }
        var self = this
        var callback = callback
            
        var internal_callback = function() {
            self.start_timer_for_next_event()
            self.#event_index++
            callback()
            
        }
        if (this.event_index == -1)
        {
            this.#event_index = 0;
        }
        if (this.#events.length > this.#event_index)
        {
            this.#cur_event = this.#events[this.#event_index]
            //console.log(this.#cur_event)
            this.action_event(this.#cur_event)
            if(!this.paused)
            {
                this.write_buffers(internal_callback,true)
            } else {
                callback()
                this.clear_buffers()
            }
            
            
        } else {
            if(this.is_live == false)
            {
                //console.log("pausing")
                this.pause()
            } else {
                callback()
            }
        }
        this.refresh_statusbar()
    }

    get session() {
        return this.#session
    }

    get is_live() {
        return this.session.live
    }
    get terminal() 
    {
        return this.#terminal
    }

    get timeout_delay_speed() 
    {
        return this.#timeout_delay_speed
    }
    
    resume_with_new_delay()
    {
        var new_delay = parseInt(this.timeout_delay_speed * this.timeout_delay_saved / this.speed)
        this.start_timer_for_next_event(new_delay)
        this.#timeout_delay_saved= new_delay
        this.#timeout_delay_speed = this.speed
    }

    redo_next_event()
    {
        this.reset_timeout(false)
        this.resume_with_new_delay()
    }

    increase_speed(amount=0.3)
    {
        if(!this.paused)
        {
            this.pause()
            this.#speed = amount+this.speed
            this.play()
        }
        else
        {
            this.#speed = amount+this.speed
        }

    }

    get keystroke_buffer()
    {
        return this.#keystroke_buffer
    }

    get terminal_buffer()
    {
        return this.#terminal_buffer
    }

    decrease_speed(amount=0.3)
    {
        if(this.paused)
        {
            this.#speed = Math.max(this.speed-amount,0.1)
        } else {
            this.pause()
            this.#speed = Math.max(this.speed-amount,0.1)
            this.play()
        }
        
    }

    get speed()
    {
        return this.#speed
    }

    get event_index()
    {
        return this.#event_index
    }

    reset_timeout(clear_delay_speed=true)
    {
        if(this.#timeout != -1)
        {
            //clearTimeout(this.#timeout)
            this.#timeout = -1
        }
        this.#timeout_delay_saved = this.time_to_next_event
        this.#timeout_delay = 0
        this.#timeout_timestamp = 0
        if(clear_delay_speed)
        {
            this.#timeout_delay_speed = 1
        }
        
    }

    next()
    {
        if(this.#event_index< this.events.length)
        {
        this.reset_timeout()
        if(this.paused)
        {
           
                var cur_event = this.events[this.#event_index]
                this.action_event(cur_event,function(){})
                this.write_buffers()
                this.#event_index++
                
                
            } else {
                this.process_next_event()
            }
        }
        this.refresh_statusbar()
    }

    pause()
    {
        this.#paused = true
        this.reset_timeout(false)
        this.refresh_statusbar()
        
    }

    prev()
    {
        this.pause()
        var scroll_Y= parseInt(this.terminal.buffer._normal.viewportY)
        this.terminal.reset()
        this.#keystrokes.empty()
        var new_index = Math.max(0,this.event_index-2)
        //console.log
        for (var event_index = 0; event_index <= new_index; event_index++)
        {
            var cur_event = this.events[event_index]
            var self= this
            this.action_event(cur_event)
        }
        var self = this
        var scroll_Y= parseInt(this.terminal.buffer._normal.viewportY)

        this.write_buffers(function(){
            self.terminal.scrollToBottom();
            setTimeout(function(){self.terminal.scrollToBottom();},25);
        },true);
        this.#event_index = new_index+1;
        this.refresh_statusbar()
    }

    play()
    {
        this.#paused = false
        this.resume_with_new_delay()
        this.refresh_statusbar()
    }
    get paused()
    {
        return this.#paused
    }
}