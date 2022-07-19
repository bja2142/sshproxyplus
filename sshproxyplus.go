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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"

)

var (
	Logger *log.Logger
	// keys are "listen_ip:listen_port:client_ip:client:port"
	SshSessions map[string]*sessionContext 
)




func main() {

	cur_proxy := parseArgsForNewProxyContext()

	cur_proxy.addProxyUser(&proxyUser{"bobbytables","","127.0.0.1:22","ben","password"})

	SshSessions = map[string]*sessionContext{}

	go cur_proxy.startProxy()

	go ServeWebSocketSessionServer("0.0.0.0:"+strconv.Itoa(*cur_proxy.web_listen_port),*cur_proxy.tls_cert, *cur_proxy.tls_key)

	var input string
	for {
		fmt.Scanln(&input)
		if input == "q" {
			break;
		} else if input == "a" {
			cur_proxy.activate()
			Logger.Println("activating")
		}  else if input == "d" {
			cur_proxy.deactivate()
			Logger.Println("deactivating")
			
		}
		fmt.Println("Enter q to quit.")
		log.Printf("Proxy Active:%v\n",cur_proxy.active)
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

	server_ssh_port := flag.Int("dport",22, "destination ssh server port; this field is only used if the the (Users proxyUser) field is empty")
	server_ssh_ip := flag.String("dip","127.0.0.1", "destination ssh server ip; this field is only used if the the (Users proxyUser) field is empty")
	proxy_listen_port := flag.Int("lport", 2222, "proxy listen port")
	proxy_listen_ip   := flag.String("lip", "0.0.0.0", "ip for proxy to bind to")
	proxy_key   := flag.String("lkey", "autogen", "private key for proxy to use; defaults to autogen new key")
	log_file := flag.String("log", "-", "file to log to; defaults to stdout")
	session_folder := flag.String("sess-dir", ".", "directory to write sessions to and to read from; defaults to the current directory")
	tls_cert := flag.String("tls_cert", ".", "TLS certificate to use for web; defaults to plaintext")
	tls_key := flag.String("tls_key", ".", "TLS key to use for web; defaults to plaintext")
	override_user := flag.String("override-user", "", "Override client-supplied username when proxying to remote server; this field is only used if the the (Users proxyUser) field is empty")
	override_password := flag.String("override-pass","","Overrides client-supplied password when proxying to remote server; this field is only used if the the (Users proxyUser) field is empty")
	require_valid_password := flag.Bool("require-valid-password",false, "requires a valid password to authenticate; if this field is false then the presented credentials will be passed to the port and server provided in dport and dip; this field is ignored if (Users proxy) field is not empty")
	web_listen_port := flag.Int("web-port", 8080, "web server listen port; defaults to 8080")
	server_version := flag.String("server-version", "SSH-2.0-OpenSSH_7.9p1 Raspbian-10", "server version to use")
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
	if *session_folder != "." {
		if _, err := os.Stat(*session_folder); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(*session_folder, os.ModePerm)
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

	if _, err := os.Stat(*tls_key); errors.Is(err, os.ErrNotExist) {
		dot := "."
		tls_key = &dot
		
	}
	if _, err := os.Stat(*tls_cert); errors.Is(err, os.ErrNotExist) {
		dot := "."
		tls_cert = &dot
	}

	return &proxyContext{
		server_ssh_port: server_ssh_port,
		server_ssh_ip: server_ssh_ip,
		listen_ip: proxy_listen_ip,
		listen_port: proxy_listen_port,
		private_key: proxy_private_key,
		log: Logger,
		session_folder: session_folder,
		tls_cert: tls_cert,
		tls_key: tls_key,
		override_password: *override_password,
		override_user: *override_user,
		web_listen_port: web_listen_port,
		server_version: server_version,
		RequireValidPassword: *require_valid_password,
		Users: map[string]*proxyUser{},
		sessions: map[string]map[string]*sessionContext{},
		viewers: map[string]*proxySessionViewer{}}
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



func ListSessions() []string {
	session_keys := make([]string, len(SshSessions))

	index := 0
	for cur_key := range SshSessions {
		session_keys[index] = cur_key
		index++
	}

	return session_keys
}


func ListActiveSessions() []string {
	session_keys := make([]string, 0)

	for cur_key := range SshSessions {
		if SshSessions[cur_key].active == true {
			session_keys = append(session_keys, cur_key)
		}
	}

	return session_keys
}

