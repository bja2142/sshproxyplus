
var terminal_reader;
var global_row_height;
var global_col_width;
var global_fit_addon;
jQuery.noConflict()
function resize_terminal_to_window()
{
    var width = max_terminal_width()
    var height = max_terminal_height()
    terminal_reader.resize_by_pixels(width,height)
}
jQuery(document).ready(function() {
    terminal_reader = new TerminalReader("terminal_reader")
    terminal_reader.disable_resize()
    terminal_reader.initialize()    
    jQuery( window ).on( 'hashchange', function( e ) {
        read_hashes(true);
    } );
    read_hashes(true);

    setTimeout(resize_terminal_to_window, 100);

    jQuery(window).resize(function(){
        resize_terminal_to_window();
    })
});

