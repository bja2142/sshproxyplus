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
go run _example/sshproxyplus.go --help
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

## References

* https://www.ssh.com/academy/ssh/protocol#the-core-protocol
* https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
* https://scalingo.com/blog/writing-a-replacement-to-openssh-using-go-12
* https://github.com/helloyi/go-sshclient/blob/master/sshclient.go
* https://gist.github.com/denji/12b3a568f092ab951456


