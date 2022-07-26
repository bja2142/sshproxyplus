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
	//"strconv"
	"io/ioutil"
	"errors"
	"golang.org/x/crypto/ssh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"

)

var (
	Logger *log.Logger
)

//TODO: deprecate SshSessions



func main() {

	cur_proxy := parseArgsForNewProxyContext()

	//cur_proxy.addProxyUser(&proxyUser{"testuser","","127.0.0.1:22","ben","password"})


	controller := proxyController{
		SocketType: PROXY_CONTROLLER_SOCKET_PLAIN,
		SocketHost: "0.0.0.0:9999",
		PresharedKey: "key",
		TLSKey: "TLSKeys/server.key",
		TLSCert: "TLSKeys/server.crt",
		Proxies: make(map[uint64]*proxyContext),
		WebHost: "0.0.0.0:8000",
		WebStaticDir: "./html",
		log: Logger,
	}

	controller.listen()


	defer controller.Stop()

	proxyID := controller.addExistingProxy(cur_proxy)

	controller.log.Printf("Proxy has id:%v\n",proxyID)

	go controller.startProxy(proxyID)
	go controller.startWebServer()

	/*webServer := &proxyWebServer{
		proxy: cur_proxy, 
		listenHost: "0.0.0.0:"+strconv.Itoa(cur_proxy.WebListenPort),
	}

	go webServer.ServeWebSocketSessionServer()*/

	var input string
	for {
		fmt.Scanln(&input)
		if input == "q" {
			break;
		} else if input == "a" {
			controller.activateProxy(proxyID)
			Logger.Println("activating")
		}  else if input == "d" {
			controller.deactivateProxy(proxyID)
			Logger.Println("deactivating")
			
		} else if input == "k" {
			getKeysForAllUsers(cur_proxy)
		}
		fmt.Println("Enter q to quit.")
		log.Printf("Proxy Active:%v\n",cur_proxy.active)
	}

}

func getKeysForAllUsers(proxy * proxyContext) {
	users := proxy.getUsers()
	proxy.log.Printf("%v\n",users)
	for _,key := range users {
		proxy.log.Println(key)
		err, viewer := proxy.makeSessionViewerForUser(key)
		if (err == nil) {
			proxy.log.Printf("%v:%v\n", key,viewer.buildSignedURL())
		}
	}
}


// TODO: add methods to sessionContext so I can dump session data to file (json) or to stdout``
// TODO: add list of handlers to session to be called whenever new data is added to a session












// TODO switch fmt to session




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
	Logger = log.New(file, "LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func parseArgsForNewProxyContext() *proxyContext {

	DefaultRemotePort := flag.Int("dport",22, "destination ssh server port; this field is only used if the the (Users proxyUser) field is empty")
	DefaultRemoteIP := flag.String("dip","127.0.0.1", "destination ssh server ip; this field is only used if the the (Users proxyUser) field is empty")
	proxy_listen_port := flag.Int("lport", 2222, "proxy listen port")
	proxy_listen_ip   := flag.String("lip", "0.0.0.0", "ip for proxy to bind to")
	proxy_key   := flag.String("lkey", "autogen", "private key for proxy to use; defaults to autogen new key")
	log_file := flag.String("log", "-", "file to log to; defaults to stdout")
	SessionFolder := flag.String("sess-dir", ".", "directory to write sessions to and to read from; defaults to the current directory")
	TLSCert := flag.String("tls_cert", ".", "TLS certificate to use for web; defaults to plaintext")
	TLSKey := flag.String("tls_key", ".", "TLS key to use for web; defaults to plaintext")
	override_user := flag.String("override-user", "", "Override client-supplied username when proxying to remote server; this field is only used if the the (Users proxyUser) field is empty")
	OverridePassword := flag.String("override-pass","","Overrides client-supplied password when proxying to remote server; this field is only used if the the (Users proxyUser) field is empty")
	require_valid_password := flag.Bool("require-valid-password",false, "requires a valid password to authenticate; if this field is false then the presented credentials will be passed to the port and server provided in dport and dip; this field is ignored if (Users proxy) field is not empty")
	WebListenPort := flag.Int("web-port", 8080, "web server listen port; defaults to 8080")
	server_version := flag.String("server-version", "SSH-2.0-OpenSSH_7.9p1 Raspbian-10", "server version to use")
	base_URI_option	:= flag.String("base-uri","auto","override base URI when crafting signed URLs; default is to auto-detect")
	public_access := flag.Bool("public-view", false, "allow viewers to query sessions without secret URL")
	//TODO: add ability to load and save from json config file
	flag.Parse()

	initLogging(log_file)
	Logger.Println("sshproxyplus has started.")

	var proxy_private_key ssh.Signer
	var err error



	if *proxy_key != "autogen" {
		var proxy_key_bytes []byte
		proxy_key_bytes, err = ioutil.ReadFile(*proxy_key)
		if err == nil {
			proxy_private_key, err = ssh.ParsePrivateKey(proxy_key_bytes)
		}
		if err == nil {
			Logger.Printf("Successfully loaded key: %v\n",*proxy_key)
		} else {
			Logger.Printf("Failed to load key: %v\n", *proxy_key)
		}
	} else {
		err = errors.New("must autogen")
	}

	// https://freshman.tech/snippets/go/create-directory-if-not-exist/
	if *SessionFolder != "." {
		if _, err := os.Stat(*SessionFolder); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(*SessionFolder, os.ModePerm)
			if err != nil {
				Logger.Println(err)
			}
		}
	}


	if err != nil {
		proxy_private_key,err = generateSigner()
		Logger.Printf("Generating new key.")
		if err != nil {
			log.Fatal("Unable to load or generate a public key")
		}
	}
	Logger.Printf("Proxy using key with public key: %v\n",proxy_private_key.PublicKey().Marshal())

	if _, err := os.Stat(*TLSKey); errors.Is(err, os.ErrNotExist) {
		dot := "."
		TLSKey = &dot
		
	}
	if _, err := os.Stat(*TLSCert); errors.Is(err, os.ErrNotExist) {
		dot := "."
		TLSCert = &dot
	}

	var base_URI string
	if *base_URI_option == "auto" {
		var protocol, hostname string
		var err error
		if *TLSCert != "." && *TLSKey != "." {
			protocol = "https"
			hostname, err = os.Hostname()
			if err != nil {
				hostname = "localhost"
			}
		} else {
			protocol = "http"
			hostname, err = GetLocalIP()
			if err != nil {
				hostname = "127.0.0.1"
			}
		}
		base_URI = fmt.Sprintf("%v://%v:%v",protocol,hostname,*WebListenPort)
	} else {
		base_URI = *base_URI_option
	}

	proxy := makeNewProxy()
	proxy.DefaultRemotePort = *DefaultRemotePort
	proxy.DefaultRemoteIP = *DefaultRemoteIP
	proxy.ListenIP = *proxy_listen_ip
	proxy.ListenPort = *proxy_listen_port
	proxy.private_key = proxy_private_key
	proxy.SessionFolder = *SessionFolder
	proxy.TLSCert = *TLSCert
	proxy.TLSKey = *TLSKey
	proxy.OverridePassword = *OverridePassword
	proxy.override_user = *override_user
	proxy.WebListenPort = *WebListenPort
	proxy.ServerVersion = *server_version
	proxy.RequireValidPassword = *require_valid_password
	proxy.BaseURI = base_URI	
	proxy.PublicAccess = *public_access
	proxy.log = Logger

	return proxy
}


// taken from: https://github.com/cmoog/sshproxy/blob/5448845f823a2e6ec7ba6bb7ddf1e5db786410d4/_examples/main.go
func generateSigner() (ssh.Signer, error) { 
	const blockType = "EC PRIVATE KEY"
	pkey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate rsa private key: %w", err)
	}

	byt, err := x509.MarshalECPrivateKey(pkey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	pb := pem.Block{
		Type:    blockType,
		Headers: nil,
		Bytes:   byt,
	}
	p, err := ssh.ParsePrivateKey(pem.EncodeToMemory(&pb))
	if err != nil {
		return nil, err
	}
	return p, nil
}

// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go/31551220#31551220

// GetLocalIP returns the first non loopback local IP of the host
func GetLocalIP() (string,error) {
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