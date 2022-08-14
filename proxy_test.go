package sshproxyplus

	

import (
    "testing"
	"fmt"
	"log"
	"math/big"
	"golang.org/x/crypto/ssh"
	"net"
	"time"
	"bytes"
	"strconv"
	"strings"
	"os"

)

type testLogger struct {
	messages []string
}

func (logger testLogger) Printf(format string, v ...any) {
	logger.messages = append(logger.messages, fmt.Sprintf(format, v...))
}

func (logger testLogger) Println(v ...any) {
	logger.messages = append(logger.messages, fmt.Sprintln(v...) )
}

func makeNewTestProxy() *ProxyContext {

	
	return &ProxyContext{
		Log: log.Default(), 
		Users: map[string]*ProxyUser{},
		userSessions: map[string]map[string]*SessionContext{},
		allSessions: map[string]*SessionContext{},
		Viewers: map[string]*proxySessionViewer{},
		DefaultRemotePort: 22,
		DefaultRemoteIP: "127.0.0.1",
		ListenIP: "0.0.0.0",
		ListenPort: 2222,
		SessionFolder: "html/sessions",
		WebListenPort: 8080,
		ServerVersion: "SSH-2.0-OpenSSH_7.9p1 Raspbian-10",
		PublicAccess:true,
	}
}

func makeNewTestProxyUser() *ProxyUser {
	return &ProxyUser{
		Username: "test", 
		Password: "pass", 
		RemoteHost: "remote:port", 
		RemoteUsername: "expected_user", 
		RemotePassword: "expected_pass"}
}

func TestAuthenticateUserDefault(t *testing.T) {

	proxy := makeNewTestProxy()

	expected_user := "OverrideUser"
	expected_pass := "OverridePassword"

	proxy.OverrideUser = expected_user
	proxy.OverridePassword = expected_pass

	test_user := expected_user
	test_pass := expected_pass


	expected_host := proxy.GetDefaultRemoteHost()

	err, user := proxy.AuthenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid ProxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
    }
}

func TestAuthenticateUserAnyValue(t *testing.T) {

	proxy := makeNewTestProxy()

	expected_user := "OverrideUser"
	expected_pass := "OverridePassword"

	proxy.OverrideUser = expected_user
	proxy.OverridePassword = expected_pass

	test_user := "something_else"
	test_pass := "literally_anything"


	expected_host := proxy.GetDefaultRemoteHost()
	
	err, user := proxy.AuthenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid ProxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
    }
}

func TestAuthenticateUserAnyUserBlankPassword(t *testing.T) {

	proxy := makeNewTestProxy()

	expected_user := "OverrideUser"
	expected_pass := "OverridePassword"

	proxy.OverrideUser = expected_user
	proxy.OverridePassword = expected_pass

	test_user := "something_else"
	test_pass := ""


	expected_host := proxy.GetDefaultRemoteHost()
	
	err, user := proxy.AuthenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid ProxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
    }
}

func TestAuthenticateUserAuthorized(t *testing.T) {

	proxy := makeNewTestProxy()
	proxy_user := makeNewTestProxyUser()

	test_user := proxy_user.Username
	test_pass := proxy_user.Password

	expected_user := proxy_user.RemoteUsername
	expected_pass := proxy_user.RemotePassword

	expected_host := proxy_user.RemoteHost

	proxy.AddProxyUser(proxy_user)

	err, user := proxy.AuthenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid ProxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
    }
}

func TestAuthenticateUsersAuthorized(t *testing.T) {

	proxy := makeNewTestProxy()
	proxy_user := makeNewTestProxyUser()
	proxy_user2 := makeNewTestProxyUser()

	test_user1 := proxy_user.Username
	test_pass1 := proxy_user.Password

	test_user2 := proxy_user2.Username
	test_pass2 := proxy_user2.Password

	expected_user := proxy_user.RemoteUsername
	expected_pass := proxy_user.RemotePassword

	expected_host := proxy_user.RemoteHost

	proxy.AddProxyUser(proxy_user)
	proxy.AddProxyUser(proxy_user2)

	err, user := proxy.AuthenticateUser(test_user1,test_pass1)

	err2, user2 := proxy.AuthenticateUser(test_user2,test_pass2)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid ProxyUser object`, test_user1,test_pass1, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user1,test_pass1, user, err,expected_host, expected_user, expected_pass)
    }

	if err2 != nil || user2 == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid ProxyUser object`, test_user2,test_pass2, user2, err2)
    }

	if user2.RemoteHost != expected_host || user2.RemoteUsername != expected_user || user2.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user2,test_pass2, user2, err2,expected_host, expected_user, expected_pass)
    }
}

func TestAuthenticateUserBlankAuthorized(t *testing.T) {

	proxy := makeNewTestProxy()
	proxy_user := makeNewTestProxyUser()

	test_user1 := proxy_user.Username
	test_pass1 := proxy_user.Password

	expected_user := proxy_user.RemoteUsername
	expected_pass := proxy_user.RemotePassword

	expected_host := proxy_user.RemoteHost

	proxy.AddProxyUser(proxy_user)

	test_pass2 := "pass"

	err, user := proxy.AuthenticateUser(test_user1,test_pass1)
	err2, user2 := proxy.AuthenticateUser(test_user1,test_pass2)

	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving blank password, want no error and a valid ProxyUser object`, test_user1,test_pass1, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving blank password, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user1,test_pass1, user, err,expected_host, expected_user, expected_pass)
    }

	if err2 != nil || user2 == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving non-blank password, want no error and a valid ProxyUser object`, test_user1,test_pass2, user2, err2)
    }

	if user2.RemoteHost != expected_host || user2.RemoteUsername != expected_user || user2.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving non-blank password, want ProxyUser object to have host, user, pass values %v, %v, %v`, test_user1,test_pass2, user2, err2,expected_host, expected_user, expected_pass)
    }
}



type testSSHServer struct {
	port *big.Int
	key ssh.Signer
	SSHConn ssh.Conn
	t *testing.T
	listener net.Listener
	active bool
	messages [][]byte
}


func (self *testSSHServer) stop() {
	self.active = false
	if self.listener != nil {
		self.listener.Close()
	}
}

func (self *testSSHServer) listen() {
		var err error
		if self.messages == nil {
			self.messages = make([][]byte,0)
		}
		if self.key == nil {
			self.key, err = GenerateSigner()
			if err != nil {
				self.t.Fatalf("Cannot generate ssh key for test: %s",err)
			}
		}
		config := &ssh.ServerConfig{
			PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
				log.Printf("Got client (%s) using creds (%s:%s)\n",
					conn.RemoteAddr(),
					conn.User(),
					password)

				return &ssh.Permissions{}, nil
			},
			NoClientAuth: false,
			MaxAuthTries: 3,
			BannerCallback: func(conn ssh.ConnMetadata) string {
				return "bannerCallback"
			},
		}
		config.AddHostKey(self.key)
		listener, err := net.Listen("tcp", "0.0.0.0:"+self.port.Text(10))
		if err != nil {
			self.t.Fatalf("Cannot start server listener: %s", err)
		}
		self.listener = listener
		log.Printf("Starting dummy SSH server on :%s\n",self.port)
		for self.active {
			// Once a ServerConfig has been configured, connections can be accepted.
			serverConnection, err := listener.Accept()
			
			if err != nil {
				log.Printf("Failed to accept client: %s",err)
				continue
			}
			// Before use, a handshake must be performed on the incoming net.Conn.
			SSHConn, SSHChannels, SSHRequests, err := ssh.NewServerConn(serverConnection, config)
			self.SSHConn = SSHConn
			if err != nil {
				self.t.Errorf("Failed to start ssh connection: %s", err)
				continue
			}

			handleRequests := func(inRequests <-chan *ssh.Request) {
				for request := range inRequests {
					log.Printf("Got new request: %s\n", request.Type)
					if request.WantReply {
						log.Printf("Giving request reply\n")
						if err := request.Reply(true, []byte("")); err != nil {
							self.t.Errorf("Error giving server reply: %s", err)
						}
					}
				}
			}

			go handleRequests(SSHRequests)
			go func(channels <-chan ssh.NewChannel) {
				for newChannel := range channels {
					if newChannel.ChannelType() != "session" {

						newChannel.Reject(ssh.UnknownChannelType, "unsupported channel")
						continue
					} else {
						log.Printf("Got channel: %s", newChannel.ChannelType())
					}
					log.Println("new channel accept start")
					channel, channelRequests, err := newChannel.Accept()
					log.Println("new channel accepted")
					if err != nil {
						self.t.Errorf("Failed to accept new channel")
						continue
					}
					go handleRequests(channelRequests)
					///https://gist.github.com/jpillora/b480fde82bff51a06238
					/*storage := make([]byte,1024)
					reader := bytes.NewReader(storage)
					pipe := bytes.NewBuffer(storage)
					go func() {
						io.Copy(channel, reader)
					}()
					go func() {
						io.Copy(pipe, channel)
					}()*/
					log.Println("start looping")
					for {
						data := make([]byte,1024)
						numBytes,err := channel.Read(data)
						if (err == nil) {
							self.messages = append(self.messages,data[:numBytes])
							_,err = channel.Write(data[:numBytes])
						}
						log.Printf("data: %s\n",data)
						if err != nil {
							log.Println("there was an error:", err)
							break
						}
					}
					log.Println("done looping")
					channel.Close()
				}
			}(SSHChannels)
		}

}

func sendCommandToTestServer(host, user, password, command string) (error, string) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: time.Second *3,
	}
	var replyString string
	client, err :=  ssh.Dial("tcp", host, config) 
	if err == nil {
		defer client.Close() 
		session, newErr := client.NewSession()
		err = newErr
		if err == nil {
			defer session.Close()
			var reply bytes.Buffer
			var input bytes.Buffer
			session.Stdout = &reply
			session.Stdin = &input
			//session.Stdout = &input
			//session.Stdin = &reply
			input.Write([]byte(command))
			log.Printf("sending command:%s\n", command)
			err = session.Shell();
			time.Sleep(time.Millisecond*500)
			replyString = reply.String()
		}
	}
	/*
	remote_sock, err := net.DialTimeout("tcp", host, time.Second *1)
	defer remote_sock.Close()
	if (err == nil) {
		clientConn, clientChannels, clientRequests, newErr := ssh.NewClientConn(remote_sock, host, config)
		err = newErr

		if err == nil {
			go ssh.DiscardRequests(clientRequests)
			go func(channels <-chan NewChannel) {
				for channel := range channels {
						channel.Reject(ssh.UnknownChannelType, "unsupported channel")
						continue
				}
			}(clientChannels)
			clientSession, newErr := clientConn.NewSession()
			defer clientSession.Close()

			err = newErr
			if err == nil {
				err, _ = clientSession.Stdout.Write([]byte(command))
				if (err == nil ){
					replyBytes := make([]byte,1024)
					err, _ = client.Session.Stdin.Read(replyBytes)
					replyString = string(replyBytes)
				}
				
			}
		}
	}*/
	return err, replyString
}


// move this test to proxy
func TestProxy(t *testing.T) {

	testString := "echo this is a test string"
	testUser := "user"
	testPassword := "password"
	dummyServer := testSSHServer{
		port: newRandomPort(),
		t: t,
		active: true,
	}
	go dummyServer.listen()

	time.Sleep(500*time.Millisecond)
	
	defer dummyServer.stop()
// start dummy ssh server
// https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
	signer, err := GenerateSigner()
	proxy := MakeNewProxy(signer)
	proxy.DefaultRemotePort = int(dummyServer.port.Int64())
	proxy.ListenPort =  int(newRandomPort().Int64())
	proxy.active = true

	// create proxy connecting to it


	go proxy.StartProxy()
	time.Sleep(500*time.Millisecond)

	err, testReply := sendCommandToTestServer("127.0.0.1:"+strconv.Itoa(proxy.ListenPort), testUser, testPassword, testString)
	if (err != nil) {
		t.Errorf("Error when sending command to proxy: %s\n", err)
	}
	log.Println("reply:", testReply)

	if strings.Compare(testReply, testString) != 0 {
		t.Errorf("Failed to get test string back from dummy echo server. Expected `%s`, got `%s`", testString, testReply)
	}

	if len(proxy.allSessions) != 1 {
		t.Errorf("Proxy did not store session.")
	} else {
		for testSessionKey, _ := range proxy.allSessions {
			testSession := proxy.allSessions[testSessionKey]
			if (strings.Compare(testSession.client_username, testUser) != 0) {
				t.Errorf("Proxy session does not have expected username. Expected %s, got %s", testUser, testSession.client_username)
			}

			if (strings.Compare(testSession.client_password, testPassword) != 0) {
				t.Errorf("Proxy session does not have expected password. Expected %s, got %s", testUser, testSession.client_password)
			}
		}
	}

	proxy.Stop()

	for testSessionKey, _ := range proxy.allSessions {
		testSession := proxy.allSessions[testSessionKey]
		err := os.Remove(testSession.filename)
		if err != nil {
			log.Printf("Failed to remove file during cleanup: %s\n", err)
		}
	}

}

func requestWindowChangeToTestServer(host, user, password string, height, width int) (error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: time.Second *3,
	}
	client, err :=  ssh.Dial("tcp", host, config) 
	if err == nil {
		defer client.Close() 
		session, newErr := client.NewSession()
		err = newErr
		if err == nil {
			defer session.Close()
			// Set up terminal modes; from example in docs: https://pkg.go.dev/golang.org/x/crypto/ssh#Session
			modes := ssh.TerminalModes{
				ssh.ECHO:          0,     // disable echoing
				ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
				ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
			}
			// Request pseudo terminal
			err = session.RequestPty("xterm", 50, 50, modes);
			if err == nil {
				time.Sleep(time.Millisecond*50)
				err = session.WindowChange(height, width)
				time.Sleep(time.Millisecond*50)
			}
		}
	}

	return err
}

func requestPtyToTestServer(host, user, password string, height, width int) (error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout: time.Second *3,
	}
	client, err :=  ssh.Dial("tcp", host, config) 
	if err == nil {
		defer client.Close() 
		session, newErr := client.NewSession()
		err = newErr
		if err == nil {
			defer session.Close()
			// Set up terminal modes; from example in docs: https://pkg.go.dev/golang.org/x/crypto/ssh#Session
			modes := ssh.TerminalModes{
				ssh.ECHO:          0,     // disable echoing
				ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
				ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
			}
			// Request pseudo terminal
			err = session.RequestPty("xterm", height, width, modes)
		}
	}

	return err
}



func TestProxyRequestPty(t *testing.T) {

	testWidth := uint32(10)
	testHeight := uint32(20)
	testUser := "user"
	testPassword := "password"
	dummyServer := testSSHServer{
		port: newRandomPort(),
		t: t,
		active: true,
	}
	go dummyServer.listen()

	time.Sleep(500*time.Millisecond)
	
	defer dummyServer.stop()
// start dummy ssh server
// https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
	signer, err := GenerateSigner()
	proxy := MakeNewProxy(signer)
	proxy.DefaultRemotePort = int(dummyServer.port.Int64())
	proxy.ListenPort =  int(newRandomPort().Int64())
	proxy.active = true

	// create proxy connecting to it


	go proxy.StartProxy()
	time.Sleep(500*time.Millisecond)

	err = requestPtyToTestServer("127.0.0.1:"+strconv.Itoa(proxy.ListenPort), testUser, testPassword, int(testHeight), int(testWidth))
	if (err != nil) {
		t.Errorf("Error when sending PtyRequest to proxy: %s\n", err)
	}
	
	

	if len(proxy.allSessions) != 1 {
		t.Errorf("Proxy did not store session.")
	} else {
		for testSessionKey, _ := range proxy.allSessions {
			testSession := proxy.allSessions[testSessionKey]
			if testSession.term_rows != testHeight {
				t.Errorf("Proxy session does not have expected term_rows. Expected %v, got %v", testHeight, testSession.term_rows )
			}

			if testSession.term_cols != testWidth {
				t.Errorf("Proxy session does not have expected term_cols. Expected %v, got %v", testWidth, testSession.term_cols )
			}
		}
	}

	proxy.Stop()

	for testSessionKey, _ := range proxy.allSessions {
		testSession := proxy.allSessions[testSessionKey]
		err := os.Remove(testSession.filename)
		if err != nil {
			log.Printf("Failed to remove file during cleanup: %s\n", err)
		}
	}

}


func TestProxyWindowChange(t *testing.T) {

	testWidth := uint32(10)
	testHeight := uint32(20)
	testUser := "user"
	testPassword := "password"
	dummyServer := testSSHServer{
		port: newRandomPort(),
		t: t,
		active: true,
	}
	go dummyServer.listen()

	time.Sleep(500*time.Millisecond)
	
	defer dummyServer.stop()
// start dummy ssh server
// https://blog.gopheracademy.com/advent-2015/ssh-server-in-go/
	signer, err := GenerateSigner()
	proxy := MakeNewProxy(signer)
	proxy.DefaultRemotePort = int(dummyServer.port.Int64())
	proxy.ListenPort =  int(newRandomPort().Int64())
	proxy.active = true

	// create proxy connecting to it


	go proxy.StartProxy()
	time.Sleep(500*time.Millisecond)

	err = requestWindowChangeToTestServer("127.0.0.1:"+strconv.Itoa(proxy.ListenPort), testUser, testPassword, int(testHeight), int(testWidth))
	if (err != nil) {
		t.Errorf("Error when sending window change to proxy: %s\n", err)
	}
	
	

	if len(proxy.allSessions) != 1 {
		t.Errorf("Proxy did not store session.")
	} else {
		for testSessionKey, _ := range proxy.allSessions {
			testSession := proxy.allSessions[testSessionKey]
			if testSession.term_rows != testHeight {
				t.Errorf("Proxy session does not have expected term_rows. Expected %v, got %v", testHeight, testSession.term_rows )
			}

			if testSession.term_cols != testWidth {
				t.Errorf("Proxy session does not have expected term_cols. Expected %v, got %v", testWidth, testSession.term_cols )
			}
		}
	}

	proxy.Stop()

	for testSessionKey, _ := range proxy.allSessions {
		testSession := proxy.allSessions[testSessionKey]
		err := os.Remove(testSession.filename)
		if err != nil {
			log.Printf("Failed to remove file during cleanup: %s\n", err)
		}
	}

}

func TestListSessionsSingle(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxy.PublicAccess = true
	controller.AddExistingProxy(proxy)

	testUser1 := &ProxyUser{
		Username: "testuser1",
		Password: "testPassword1",
	}

	testUser2 := &ProxyUser{
		Username: "testuser2",
		Password: "testPassword2",
	}

	proxy.AddProxyUser(testUser1)
	proxy.AddProxyUser(testUser2)



	proxySessionActiveKey := "active-session"
	proxySessionInactiveKey := "inactive-session"

	activeSession := &SessionContext{
		proxy: proxy,
		active: true,
		start_time: time.Now(),
		stop_time:time.Now(),
		events: make([]*SessionEvent,0),
		sessionID: proxySessionActiveKey,
		user: testUser1,
		msg_signal: make([]chan int,0),
	}
	inactiveSession :=  &SessionContext{
		proxy: proxy,
		active: false,
		events: make([]*SessionEvent,0),
		sessionID: proxySessionInactiveKey,
		user: testUser2,
	}

	proxy.allSessions[proxySessionActiveKey] = activeSession
	proxy.allSessions[proxySessionInactiveKey] = inactiveSession

	proxy.AddSessionToUserList(activeSession)
	proxy.AddSessionToUserList(inactiveSession)

	err, viewer := proxy.MakeSessionViewerForSession(testUser1.Username, testUser1.Password,proxySessionActiveKey)

	if(err != nil) {
		t.Fatalf("Failed to create session viewer during setup: %s",err)
	}

	viewer.getSessions()

	

}

// test with callback events

// test with unsupported channels

// test get proxyuser in the case to find the blank password

// test get user session keys
// test get Users


// expired sessions