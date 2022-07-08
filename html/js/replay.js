
var global_terminal;
var global_row_height;
var global_col_width;
var global_fit_addon;
jQuery.noConflict()
jQuery(document).ready(function() {
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
    fetch_old_session_list()
});
