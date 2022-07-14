package main

// inspiration from: https://github.com/cmoog/sshproxy/blob/master/_examples/main.go
// inspiration from: https://github.com/dutchcoders/sshproxy

// ugh i hate camel case but i guess i need to get over it 
// some day, at least. 

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

const SIGNAL_SESSION_END int = 0
const SIGNAL_NEW_MESSAGE int = 1
const SESSION_LIST_FN	string = ".session_list"
//const SIGNAL_RESIZE_WINDOW int = 2






func main() {

	cur_proxy := parseArgsForNewProxyContext()

	SshSessions = map[string]*sessionContext{}

	go cur_proxy.startProxy()

	go ServeWebSocketSessionServer("0.0.0.0:"+strconv.Itoa(*cur_proxy.web_listen_port),*cur_proxy.tls_cert, *cur_proxy.tls_key)

	var input string
	for {
		fmt.Scanln(&input)
		if input == "q" {
			break;
		}
		fmt.Println("Enter q to quit.")
	}

}



// TODO: add methods to sessionContext so I can dump session data to file (json) or to stdout``
// TODO: add list of handlers to session to be called whenever new data is added to a session
//			--> implied: for each session there is a thread that monitors some queue of new
//			messages and forwards them to any active observers 
// TODO: (after the above) add ability to query active sessions and get data for a session using
// 		a web socket











// TODO switch fmt to context




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

func parseArgsForNewProxyContext() proxyContext {

	server_ssh_port := flag.Int("dport",22, "destination ssh server port")
	server_ssh_ip := flag.String("dip","127.0.0.1", "destination ssh server ip")
	proxy_listen_port := flag.Int("lport", 2222, "proxy listen port")
	proxy_listen_ip   := flag.String("lip", "0.0.0.0", "ip for proxy to bind to")
	proxy_key   := flag.String("lkey", "autogen", "private key for proxy to use; defaults to autogen new key")
	log_file := flag.String("log", "-", "file to log to; defaults to stdout")
	session_folder := flag.String("sess-dir", ".", "directory to write sessions to; defaults to the current directory")
	tls_cert := flag.String("tls_cert", ".", "TLS certificate to use for web; defaults to plaintext")
	tls_key := flag.String("tls_key", ".", "TLS key to use for web; defaults to plaintext")
	override_user := flag.String("override-user", "", "Override client-supplied username when proxying to remote server")
	override_password := flag.String("override-pass","","Overrides client-supplied password when proxying to remote server")
	web_listen_port := flag.Int("web-port", 8080, "web server listen port; defaults to 8080")
	server_version := flag.String("server-version", "SSH-2.0-OpenSSH_7.9p1 Raspbian-10", "server version to use")

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

	return proxyContext{
		server_ssh_port,
		server_ssh_ip,
		proxy_listen_ip,
		proxy_listen_port,
		proxy_private_key,
		Logger,
		session_folder,
		tls_cert,
		tls_key,
		*override_password,
		*override_user,
		web_listen_port,
		server_version}
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

