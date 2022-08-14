
var terminal_reader;
var global_row_height;
var global_col_width;
var global_fit_addon;
jQuery.noConflict()
jQuery(document).ready(function() {
    terminal_reader = new TerminalReader("terminal_reader")
    terminal_reader.initialize()
    //init_query_socket();
    //fetch_old_session_list();
    /*global_terminal = new Terminal({convertEol: true});
    const fitAddon = new FitAddon.FitAddon();
    global_terminal.loadAddon(fitAddon);
    global_terminal.open(document.getElementById('terminal'));
    fitAddon.fit();
    console.log(fitAddon.proposeDimensions())

    global_col_width = 
        parseInt(jQuery("#terminal").css("width").slice(0,-2)) / global_terminal.cols 
    
    global_row_height = 
        parseInt(jQuery("#terminal").css("height").slice(0,-2)) / global_terminal.rows 
    
    global_fit_addon = fitAddon;*/
    //setInterval(fetch_old_session_list,10*1000);
    
    jQuery( window ).on( 'hashchange', function( e ) {
        read_hashes();
    } );
    read_hashes();
});

