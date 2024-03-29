package main

// inspiration from: https://github.com/cmoog/sshproxy/blob/master/_examples/main.go
// inspiration from: https://github.com/dutchcoders/sshproxy

// ugh i hate camel case but i guess i need to get over it 
// some day, at least. (edit: slowly phasing it out...)

// thanks to these tutorials and references:
// - https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
// - https://scalingo.com/blog/writing-a-replacement-to-openssh-using-go-12
// - https://github.com/helloyi/go-sshclient/blob/master/sshclient.go
// - https://gist.github.com/denji/12b3a568f092ab951456
// for future reference: https://elliotchance.medium.com/how-to-create-an-ssh-tunnel-in-go-b63722d682aa
import (
	"fmt"
	"flag"
	"log"
	"os"
	"strconv"
	"io/ioutil"
	"errors"
	"golang.org/x/crypto/ssh"
	"net"
	"encoding/json"
	. "github.com/bja2142/sshproxyplus"
)

var (
	logger *log.Logger
)


func main() {

	args := parseArgs()

	var err error
	var controller *ProxyController

	if *args["controller_config_file"].(*string) != "" {
		err, controller = LoadControllerConfigFromFile(*args["controller_config_file"].(*string),args["default_private_key"].(ssh.Signer))
	}

	if err != nil || controller == nil {
		logger.Println("Using Default Controller.")
		controller = &ProxyController{
			SocketType: PROXY_CONTROLLER_SOCKET_PLAIN,
			SocketHost: *args["controller.Listen_host"].(*string),
			PresharedKey: "key",
			TLSKey: *args["tls_key"].(*string),
			TLSCert: *args["tls_cert"].(*string),
			Proxies: make(map[uint64]*ProxyContext),
			WebHost: "0.0.0.0:"+strconv.Itoa(*args["web_listen_port"].(*int)),
			WebStaticDir: *args["controller_web_static_dir"].(*string),
			Log: logger,
			BaseURI: args["base_URI"].(string),
			DefaultSigner: args["default_private_key"].(ssh.Signer),
		}	

		cur_proxy := useArgsForNewProxyContext(args)

		cur_proxy.AddProxyUser(&ProxyUser{
			Username: "testuser",
			Password: "",
			RemoteHost: "127.0.0.1:22",
			RemoteUsername: "ben",
			RemotePassword: "password"})

		proxyID := controller.AddExistingProxy(cur_proxy)
		controller.ActivateProxy(proxyID)
	}

	controller.Listen()
	defer controller.Stop()
	go controller.StartWebServer()

	for index,_ := range controller.Proxies {
		controller.ActivateProxy(index)
		go controller.StartProxy(index)
	}

	var input string
	for {
		fmt.Scanln(&input)
		if input == "q" {
			break;
		} else if input == "a" {
			for index,_ := range controller.Proxies {
				controller.ActivateProxy(index)
			}
			logger.Println("activating")
		}  else if input == "d" {
			for index,_ := range controller.Proxies {
				controller.DeactivateProxy(index)
			}
			logger.Println("deactivating")
			
		} else if input == "k" {
			for index, proxy := range controller.Proxies {
				makeNewViewersForAllUsers(proxy,index)
			}
		} else if input == "x" {
			data, _ := controller.ExportControllerAsJSON()
			logger.Println(string(data))
			controller.WriteControllerConfigToFile("config.json")
		} else if input == "t" {
			message := ControllerMessage{MessageType: CONTROLLER_MESSAGE_LIST_PROXIES}
			err, signedMessage := message.Sign([]byte(controller.PresharedKey))
			data, err := json.Marshal(&signedMessage)
			if (err == nil)	{
				logger.Println(string(data))
			}
		} else if input == "c" {
			message := ControllerMessage{
				MessageType: CONTROLLER_MESSAGE_CREATE_PROXY,
				ProxyData: []byte(`{
					"DefaultRemotePort": 22,
					"DefaultRemoteIP": "127.0.0.1",
					"ListenIP": "0.0.0.0",
					"ListenPort": 2222,
					"SessionFolder": "html/sessions",
					"TLSCert": "tls_keys/server.crt",
					"TLSKey": "tls_keys/server.key",
					"OverridePassword": "password",
					"OverrideUser": "ben",
					"WebListenPort": 8443,
					"ServerVersion": "SSH-2.0-OpenSSH_7.9p1 Raspbian-10",
					"Users": {
						"testuser:": {
							"Username": "testuser",
							"Password": "",
							"RemoteHost": "127.0.0.1:22",
							"RemoteUsername": "ben",
							"RemotePassword": "password"
						}
					}}`),
			}
			err, signedMessage := message.Sign([]byte(controller.PresharedKey))
			data, err := json.Marshal(&signedMessage)
			if (err == nil)	{
				logger.Println(string(data))
			}
		}
		fmt.Println("Enter q to quit.")
		for index, proxy := range controller.Proxies {
			log.Printf("Proxy %v Active: %v\n", index, proxy.IsActive())
		}
	}

}

func makeNewViewersForAllUsers(proxy * ProxyContext, proxyID uint64) {
	for key,user := range proxy.Users {
		logger.Println(key)
		err, viewer := proxy.MakeSessionViewerForUser(user.Username,user.Password)
		if (err == nil) {
			logger.Printf("%v:%v\n", key,viewer.BuildSignedURL(proxyID))
		}
	}
}





// TODO switch fmt to log




func initLogging(filename *string) {
	var file *os.File
	if *filename != "-" {
		var err error
		file, err = os.OpenFile(*filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		file = os.Stdout
	}
	logger = log.New(file, "LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func parseArgs() map[string]interface{} {
	args := make(map[string]interface{})
	args["default_remote_port"] = flag.Int("dport",22, "default destination ssh server port; this field is only used if the the (Users ProxyUser) field is empty")
	args["default_remote_ip"] =  flag.String("dip","127.0.0.1", "default destination ssh server ip; this field is only used if the the (Users ProxyUser) field is empty")
	args["proxy_listen_port"] = flag.Int("lport", 2222, "proxy listen port")
	args["proxy_listen_ip"] = flag.String("lip", "0.0.0.0", "ip for proxy to bind to")
	args["proxy_key"] = flag.String("lkey", "autogen", "private key for proxy to use; defaults to autogen new key")
	args["log_file"] = flag.String("log", "-", "file to log to; defaults to stdout")
	args["session_folder"] = flag.String("sess-dir", "html/sessions", "directory to write sessions to and to read from; defaults to the current directory")
	args["tls_cert"] = flag.String("tls_cert", ".", "TLS certificate to use for web; defaults to plaintext")
	args["tls_key"] = flag.String("tls_key", ".", "TLS key to use for web; defaults to plaintext")
	args["override_user"] = flag.String("override-user", "", "Override client-supplied username when proxying to remote server; this field is only used if the the (Users ProxyUser) field is empty or require-valid-password is false")
	args["override_password"] = flag.String("override-pass","","Overrides client-supplied password when proxying to remote server; this field is only used if the the (Users ProxyUser) field is empty or require-valid-password is false")
	args["require_valid_password"] = flag.Bool("require-valid-password",false, "requires a valid password to authenticate; if this field is false, then when ProxyUsers are provided and a match is not found, the user is directed to the default remote server and port. typically used with override*")
	args["web_listen_port"] = flag.Int("web-port", 8080, "web server listen port; defaults to 8080")
	args["server_version"] = flag.String("server-version", "SSH-2.0-OpenSSH_7.9p1 Raspbian-10", "server version to use")
	args["base_URI_option"] = flag.String("base-uri","auto","override base URI when crafting signed URLs; default is to auto-detect")
	args["public_access"] = flag.Bool("public-view", true, "allow viewers to query sessions without secret URL")
	args["controller_config_file"] = flag.String("config", "", "path to a config file for controller to load. otherwise a hardcoded default is used.")
	args["controller.Listen_host"] = flag.String("controller-listen-host", "127.0.0.1:9999", "host for controller port to listen on.")
	args["controller_web_static_dir"] = flag.String("controller-web-static-dir", "./html", "host for controller port to listen on.")
	flag.Parse()

	var err error

	initLogging(args["log_file"].(*string))
	logger.Println("sshproxyplus has started.")

	if *args["base_URI_option"].(*string)  == "auto" {
		var protocol, hostname string
		var err error
		if *args["tls_cert"].(*string)  != "." && *args["tls_key"].(*string)  != "." {
			protocol = "https"
			hostname, err = os.Hostname()
			if err != nil {
				hostname = "localhost"
			}
		} else {
			protocol = "http"
			hostname, err = getLocalIP()
			if err != nil {
				hostname = "127.0.0.1"
			}
		}
		args["base_URI"] = fmt.Sprintf("%v://%v:%v",protocol,hostname,*args["web_listen_port"].(*int) )
	} else {
		args["base_URI"] = *args["base_URI_option"].(*string)
	}
	
	var default_private_key ssh.Signer
	if *args["proxy_key"].(*string) != "autogen" {
		var proxy_key_bytes []byte
		proxy_key_bytes, err = ioutil.ReadFile(*args["proxy_key"].(*string) )
		if err == nil {
			default_private_key, err = ssh.ParsePrivateKey(proxy_key_bytes)
		}
		if err == nil {
			logger.Printf("Successfully loaded key: %v\n",*args["proxy_key"].(*string) )
		} else {
			logger.Printf("Failed to load key: %v\n", *args["proxy_key"].(*string) )
		}
	} else {
		err = errors.New("must autogen")
	}

	if err != nil {
		default_private_key,err = GenerateSigner()
		logger.Printf("Generating new key.")
		if err != nil {
			log.Fatal("Unable to load or generate a public key")
		}
	}
	args["default_private_key"] = default_private_key
	


	if _, err := os.Stat(*args["tls_key"].(*string) ); errors.Is(err, os.ErrNotExist) {
		dot := "."
		args["tls_key"] = &dot
		
	}
	if _, err := os.Stat(*args["tls_cert"].(*string) ); errors.Is(err, os.ErrNotExist) {
		dot := "."
		args["tls_cert"] = &dot
	}


	return args
}

func useArgsForNewProxyContext(args map[string]interface{}) *ProxyContext {

	// https://freshman.tech/snippets/go/create-directory-if-not-exist/
	if *args["session_folder"].(*string)  != "." {
		if _, err := os.Stat(*args["session_folder"].(*string) ); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(*args["session_folder"].(*string) , os.ModePerm)
			if err != nil {
				logger.Println(err)
			}
		}
	}

	logger.Printf("Proxy using key with public key: %v\n",args["default_private_key"].(ssh.Signer).PublicKey().Marshal())

	proxy := MakeNewProxy(args["default_private_key"].(ssh.Signer))
	proxy.DefaultRemotePort = *args["default_remote_port"].(*int)
	proxy.DefaultRemoteIP = *args["default_remote_ip"].(*string)
	proxy.ListenIP = *args["proxy_listen_ip"].(*string)
	proxy.ListenPort = *args["proxy_listen_port"].(*int)
	proxy.SessionFolder = *args["session_folder"].(*string)
	proxy.TLSCert = *args["tls_cert"].(*string)
	proxy.TLSKey = *args["tls_key"].(*string)
	proxy.OverridePassword = *args["override_password"].(*string)
	proxy.OverrideUser = *args["override_user"].(*string)
	proxy.WebListenPort = *args["web_listen_port"].(*int)
	proxy.ServerVersion = *args["server_version"].(*string)
	proxy.RequireValidPassword = *args["require_valid_password"].(*bool)
	proxy.BaseURI = args["base_URI"].(string)
	proxy.PublicAccess = *args["public_access"].(*bool)
	proxy.Log = logger

	return proxy
}



// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go/31551220#31551220

// getLocalIP returns the first non loopback local IP of the host
func getLocalIP() (string,error) {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return "", err
    }
    for _, address := range addrs {
        // check the address type and if it is not a loopback the display it
        if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                return ipnet.IP.String(), nil
            }
        }
    }
    return "", errors.New("no non-loopback interface found")
}
