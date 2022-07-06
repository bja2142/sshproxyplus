# SSH Proxy+

## Purpose
The goal of this project is to create an ssh proxy that can provide real-time logging,
mirroring, and monitoring of ssh sessions as they occur. 

## Dependencies

For xterm.js to work, you'll need to pull 
down the source files. 

Go to html/js and run:

```
npm install --save xterm.js
npm install --save xterm-addon-fit
```

For me, on stock Ubuntu server 20.04, this places the xterm source in:

`html/js/node_modules/xterm/` 

and

`html/js/nod_modules/xterm-addon-fit/`


## Usage:

### Launching Go Server

```
go mod tidy
go run .
```

For usage:

```
go run . --help
```

### Launching Static HTML server

``` 
cd html
python3 -m http.server
```


## Supported Channel Types:
-exec
-tty

## Unsupported Channel Types:
-tunnels. not the goal of this project

## design needs:

* be able to start and stop proxy on demand 
* be able to query active sessions for a given proxy
* be able to query past sessions for a given proxy
* be able to view data for a session


organized in microservices:
start and stop proxies based on messages
--> wraps the actual proxy that stores data as it runs
--> writes session data to disk
--> also opens port and will track a list of clients
    to send copies of data to in real time. 

database of some sort to track active proxies and view previous data
--> database should just be flat file structure

interface to view data that has been recorded
--> interacts with sqlite database and flat files

ability to replay old data 
--> interface with sqlite database and flat files

