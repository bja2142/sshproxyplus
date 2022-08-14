package sshproxyplus

import (
	"testing"
	"log"
	"time"
	"encoding/json"
	"net"
//	"io"
	//"net/http"
	"net/url"
	"github.com/gorilla/websocket"
	"crypto/rand"
	"crypto/x509"
	"crypto/ecdsa"
	"crypto/x509/pkix"
	"crypto/elliptic"
	"crypto/tls"
	"math/big"
	"os"
	"encoding/pem"
	//"strconv"
	//"strings"
)

func newRandomPort() *big.Int {
	port, _ := rand.Int(rand.Reader,big.NewInt(65534-1024))
	port.Add(port,big.NewInt(1025))
	return port
}

func makeNewController() *ProxyController {
	port := newRandomPort()
	controller := &ProxyController{
		SocketType: PROXY_CONTROLLER_SOCKET_PLAIN,
		SocketHost: "127.0.0.1:"+port.Text(10),
		PresharedKey: "key",
		Proxies: make(map[uint64]*ProxyContext),
		WebHost: "127.0.0.1:"+port.Add(port,big.NewInt(1)).Text(10),
		WebStaticDir: ".",
		Log: log.Default(),
	}
	controller.Initialize()
	return controller
}

func makeTestKeys() (string, string, func()) {
	//https://go.dev/src/crypto/tls/generate_cert.go
	// https://go.dev/LICENSE
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}
	keyUsage := x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign
	notBefore := time.Now()
	notAfter := notBefore.Add(365*24*time.Hour)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,	
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
	}

	if ip := net.ParseIP("127.0.0.1"); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}
	template.DNSNames = append(template.DNSNames, "localhost")
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}
	keypairString, _ := generateRandomString(8)
	certFilename := "test-cert-" +keypairString + ".pem"
	keyFilename := "test-key-" + keypairString + ".pem"
	
	certOut, err := os.Create(certFilename)
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert: %v", err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.pem: %v", err)
	}
	keyOut, err := os.OpenFile(keyFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)

	if err != nil {

		log.Fatalf("Failed to open key for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key: %v", err)
	}

	cleanup := func() {
		err := os.Remove(certFilename)
		if err != nil {
			log.Println("Failed to clean up test file:",certFilename)
		}
		err = os.Remove(keyFilename)
		if err != nil {
			log.Println("Failed to clean up test file:",keyFilename)
		}
	}
	return certFilename, keyFilename, cleanup
}



func TestControllerAddProxyFromJSON(t *testing.T) {

	controller := makeNewController()

	proxyJSON := []byte(`{
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
		},
		"RequireValidPassword": false,
		"PublicAccess": true,
		"Viewers": {
			"2r3K4wQZegCz6i-6k1SMx1kb5x2BntDZpQsWXYxFDEMAsbY-tb0MshRzmuXLL.IZ": {
				"ViewerType": 1,
				"Secret": "2r3K4wQZegCz6i-6k1SMx1kb5x2BntDZpQsWXYxFDEMAsbY-tb0MshRzmuXLL.IZ",
				"User": {
					"Username": "testuser",
					"Password": "",
					"RemoteHost": "127.0.0.1:22",
					"RemoteUsername": "ben",
					"RemotePassword": "password"
				},
				"SessionKey": ""
			}
		},
		"BaseURI": "https://192.168.122.69:8443"
	}`)

	err,proxyID := controller.AddProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller.AddProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	proxy, err := controller.GetProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller.AddProxyFromJSON() error when getting proxy just created: %s",err)
	}

	if (proxy.DefaultRemotePort != 22 || 
	   proxy.DefaultRemoteIP != "127.0.0.1" || 
	   proxy.ListenIP != "0.0.0.0" ||
	   proxy.ListenPort != 2222 || 
	   proxy.SessionFolder != "html/sessions" || 
	   proxy.TLSCert != "tls_keys/server.crt" || 
	   proxy.TLSKey != "tls_keys/server.key" || 
	   proxy.OverridePassword != "password" ||
	   proxy.OverrideUser != "ben" ||
	   proxy.WebListenPort != 8443 || 
	   proxy.RequireValidPassword != false ||
	   proxy.PublicAccess != true || 
	   proxy.Viewers == nil || 
	   proxy.Users == nil || 
	   proxy.BaseURI != "https://192.168.122.69:8443" ||
	   proxy.ServerVersion != "SSH-2.0-OpenSSH_7.9p1 Raspbian-10") {
		t.Errorf("*controller.AddProxyFromJSON() error populating values in proxy object")
	}

	testProxyUser, testProxyUserFound := proxy.Users["testuser:"]
	if ( ! testProxyUserFound ) {
		t.Fatalf("*controller.AddProxyFromJSON() generated proxy did not get test user: %v", proxy.Users)
	}
	if ( testProxyUser.Username!= "testuser" || 
		 testProxyUser.RemoteHost != "127.0.0.1:22" || 
		 testProxyUser.RemoteUsername != "ben" || 
		 testProxyUser.RemotePassword != "password" ) {
			t.Errorf("*controller.AddProxyFromJSON() test proxy user did not correctly populate")
		 }

	testProxyViewer, testProxyViewerFound := proxy.Viewers["2r3K4wQZegCz6i-6k1SMx1kb5x2BntDZpQsWXYxFDEMAsbY-tb0MshRzmuXLL.IZ"]
	if ( ! testProxyViewerFound ) {
		t.Fatalf("*controller.AddProxyFromJSON() generated proxy did not get test viewer: %v", proxy.Viewers)
	}
	if ( testProxyViewer.ViewerType != 1 || // SESSION_VIEWER_TYPE_LIST
		testProxyViewer.Secret != "2r3K4wQZegCz6i-6k1SMx1kb5x2BntDZpQsWXYxFDEMAsbY-tb0MshRzmuXLL.IZ" ||
		testProxyViewer.SessionKey != "" ) {
		t.Errorf("*controller.AddProxyFromJSON() test proxy viewer did not correctly populate: %v", testProxyViewer)
	}
	if (testProxyViewer.User != proxy.Users["testuser:"] ) {
		t.Errorf("*controller.AddProxyFromJSON() test proxy viewer user (%p) does not point to expected user: %p", testProxyViewer.User, proxy.Users["testuser:"])
	}

}

func TestControllerAddProxyFromJSONEmptyCase(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.AddProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller.AddProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	_, err = controller.GetProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller.AddProxyFromJSON() error when getting proxy just created: %s",err)
	}
}

func TestControllerStartAndStopProxy(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.AddProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller.AddProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}
	proxy, err := controller.GetProxy(proxyID)
	if err != nil {
		t.Fatalf("*controller getPRoxy() error when getting proxy just created: %s",err)
	}

	err = controller.StartProxy(proxyID)
	for proxy.running == false {
		time.Sleep(100)
	}
	controller.StopProxy(proxyID)

	if (err != nil) {
		t.Fatalf("*controller.StartProxy() error when starting proxy: %s",err)
	}

	

	
}


func TestControllerAddAndGetProxySingle(t *testing.T) {
	controller := makeNewController()

	proxy := MakeNewProxy(controller.DefaultSigner)

	var expectedProxyID uint64 = 5
	controller.ProxyCounter = expectedProxyID

	proxyID := controller.AddExistingProxy(proxy)

	if(proxyID != expectedProxyID){
		t.Fatalf("*controller.AddExistingProxy() error did not produce expected proxy ID. Expected: %v, Got: %v",expectedProxyID, proxyID)
	}

	gotProxy, err := controller.GetProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller.GetProxy() error when getting proxy just created: %s",err)
	}

	if gotProxy != proxy {
		t.Errorf("*controller.GetProxy() did not return the expected proxy when given its corresponding ID.")
	}
}

func TestControllerAddAndGetProxyMultiple(t *testing.T) {
	controller := makeNewController()

	proxy0 := MakeNewProxy(controller.DefaultSigner)
	proxy1 := MakeNewProxy(controller.DefaultSigner)
	proxy2 := MakeNewProxy(controller.DefaultSigner)


	proxyID0 := controller.AddExistingProxy(proxy0)
	proxyID1 := controller.AddExistingProxy(proxy1)
	proxyID2 := controller.AddExistingProxy(proxy2)


	if(proxyID0 != 0 || proxyID1 != 1 || proxyID2 != 2 ){
		t.Fatalf("*controller.AddExistingProxy() error did not produce expected proxy IDs.")
	}

	gotProxy0, err0 := controller.GetProxy(proxyID0)
	gotProxy1, err1 := controller.GetProxy(proxyID1)
	gotProxy2, err2 := controller.GetProxy(proxyID2)

	if err0 != nil  || err1 != nil || err2 != nil {
		t.Fatalf("*controller.GetProxy() error when getting a proxy just created: %v, %v, %v",err0, err1, err2)
	}

	if gotProxy0 != proxy0 || gotProxy1 != proxy1 || gotProxy2 != proxy2 {
		t.Errorf("*controller.GetProxy() did not return the expected proxy when given its corresponding ID.")
	}
}

func TestControllerGetProxyErrorConditionWrongID(t *testing.T) {
	controller := makeNewController()

	proxy := MakeNewProxy(controller.DefaultSigner)

	var fakeProxyID  uint64 = 10
	
	controller.AddExistingProxy(proxy)

	_, err := controller.GetProxy(fakeProxyID)

	if err == nil {
		t.Fatalf("*controller.GetProxy() did not return error when trying to get a proxy that doesn't exist: %s",err)
	}
}

func TestControllerDestroyProxy(t *testing.T) {
	controller := makeNewController()

	proxy0 := MakeNewProxy(controller.DefaultSigner)
	proxy1 := MakeNewProxy(controller.DefaultSigner)
	proxy2 := MakeNewProxy(controller.DefaultSigner)


	proxyID0 := controller.AddExistingProxy(proxy0)
	proxyID1 := controller.AddExistingProxy(proxy1)
	proxyID2 := controller.AddExistingProxy(proxy2)

	controller.DestroyProxy(proxyID1)

	gotProxy1, err1 := controller.GetProxy(proxyID1)

	if err1 == nil {
		t.Fatalf("*controller.GetProxy() did not throw error when it should have")
	}

	if gotProxy1 != nil {
		t.Errorf("*controller.GetProxy() returned a destroyed proxy when it shouldn't have")
	}

	gotProxy0, err0 := controller.GetProxy(proxyID0)
	gotProxy2, err2 := controller.GetProxy(proxyID2)

	if err0 != nil  || err2 != nil {
		t.Errorf("*controller.GetProxy() error when getting a proxy just created: %v, %v, %v",err0, err1, err2)
	}

	if gotProxy0 != proxy0 || gotProxy2 != proxy2 {
		t.Errorf("*controller.GetProxy() did not return the expected proxy when given its corresponding ID.")
	}

	err := controller.DestroyProxy(proxyID1)

	if err == nil {
		t.Errorf("*controller.DestroyProxy didn't error out when it should have after destroying the same proxy a second time.")
	}
}

func TestControllerActivateProxy(t *testing.T) {
	controller := makeNewController()

	proxy := MakeNewProxy(controller.DefaultSigner)
	
	proxyID := controller.AddExistingProxy(proxy)

	proxy.active = false

	err := controller.ActivateProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller.ActivateProxy() had an error: %s",err)
	}

	if proxy.active == false {
		t.Fatalf("*controller.ActivateProxy() did not activate the proxy")
	}
}

func TestControllerDeactivateProxy(t *testing.T) {
	controller := makeNewController()

	proxy := MakeNewProxy(controller.DefaultSigner)
	
	proxyID := controller.AddExistingProxy(proxy)

	proxy.active = true

	err := controller.DeactivateProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller.DeactivateProxy() had an error: %s",err)
	}

	if proxy.active == true {
		t.Fatalf("*controller.DeactivateProxy() did not deactivate the proxy")
	}
}


func TestControllerCreateAndGetProxyViewerByViewerKey(t *testing.T) {
	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.AddProxyUser(user)

	err, viewer := controller.CreateUserSessionViewer(proxyID, user.Username, user.Password)

	if (err != nil) {
		t.Fatalf("*controller.CreateUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller.CreateUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}

	viewerKey := viewer.Secret

	err, testViewer := controller.GetProxyViewerByViewerKey(proxyID, viewerKey)

	if err != nil {
		t.Fatalf("*controller.GetProxyViewerByViewerKey() threw an error getting the viewer: %s",err)
	}

	if testViewer != viewer {
		t.Errorf("controller.GetProxyViewerByViewerKey() did not return the correct viewer.")

	}

}

func TestControllerCreateAndGetProxyViewerBySessionKey(t *testing.T) {
	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	testSessionKey := "myfake-session-key.json"
	proxy.AddProxyUser(user)

	err, viewer := controller.CreateSessionViewer(proxyID, user.Username, user.Password, testSessionKey)

	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsSingle()) {
		t.Errorf("controller.CreateSessionViewer() created a viewer of the wrong type. Expected single, but this was not so.")
	}

	err, testViewer := controller.GetProxyViewerBySessionKey(proxyID, testSessionKey)

	if err != nil {
		t.Fatalf("*controller.GetProxyViewerBySessionKey() threw an error getting the viewer: %s",err)
	}

	if testViewer != viewer {
		t.Errorf("controller.GetProxyViewerBySessionKey() did not return the correct viewer.")

	}
}

func TestControllerCreateAndGetProxyViewerByUsername(t *testing.T) {
	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.AddProxyUser(user)

	err, viewer := controller.CreateUserSessionViewer(proxyID, user.Username, user.Password)

	if (err != nil) {
		t.Fatalf("*controller.CreateUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller.CreateUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}
	err, testViewer := controller.GetProxyViewerByUsername(proxyID, user.Username)

	if err != nil {
		t.Fatalf("*controller.GetProxyViewerByUsername() threw an error getting the viewer: %s",err)
	}

	if testViewer != viewer {
		t.Errorf("controller.GetProxyViewerByUsername() did not return the correct viewer.")

	}

}



func TestControllerCreateAndGetProxyViewersBySessionKey(t *testing.T) {
	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	testSessionKey := "myfake-session-key.json"
	proxy.AddProxyUser(user)

	err, _ := controller.CreateSessionViewer(proxyID, user.Username, user.Password, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.CreateSessionViewer(proxyID, user.Username, user.Password, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.CreateSessionViewer(proxyID, user.Username, user.Password, "not the test key")
	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}

	err, viewers := controller.GetProxyViewersBySessionKey(proxyID, testSessionKey)

	if err != nil {
		t.Fatalf("*controller.GetProxyViewersBySessionKey() threw an error getting the viewers: %s",err)
	}

	if len(viewers) != 2 {
		t.Errorf("controller.GetProxyViewersBySessionKey() did not return the correct number of viewers")
	}
}

func TestControllerCreateAndGetProxyViewersByUsername(t *testing.T) {
	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	user2:= &ProxyUser{
		Username: "user2",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	user3:= &ProxyUser{
		Username: user2.Username,
		Password: "with password",
		RemoteHost: "127.0.0.2:22",
		RemoteUsername: "ben2",
		RemotePassword: "password2"}
	testSessionKey := "myfake-session-key.json"
	proxy.AddProxyUser(user)
	proxy.AddProxyUser(user2)
	proxy.AddProxyUser(user3)

	err, _ := controller.CreateSessionViewer(proxyID, user.Username, user.Password, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.CreateSessionViewer(proxyID, user2.Username, user2.Password, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.CreateSessionViewer(proxyID, user3.Username, user3.Password, "not the test key")
	if (err != nil) {
		t.Fatalf("*controller.CreateSessionViewer() threw an error when creating new viewer: %s",err)
	}

	err, viewers := controller.GetProxyViewersByUsername(proxyID, user2.Username)

	if err != nil {
		t.Fatalf("*controller.GetProxyViewersByUsername() threw an error getting the viewers: %s",err)
	}

	if len(viewers) != 2 {
		t.Errorf("controller.GetProxyViewersByUsername() did not return the correct number of viewers")
	}

}


func TestAddAndRemoveProxyUser(t *testing.T) {

	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	expectedKey := user.Username + ":" + user.Password
	err, key := controller.AddUserToProxy(proxyID,user)

	if err != nil {
		t.Fatalf("*controller.AddUserToProxy() threw an error adding a ProxyUser: %s",err)
	}
	if  key != expectedKey {
		t.Fatalf("*controller.AddUserToProxy() did not create the expected proxy user key. Expected: %s, got: %s",expectedKey, key)
	}

	if len(proxy.Users) != 1 {
		t.Fatalf("*controller.AddUserToProxy() did not actually add user to the proxy. Users is: %v", proxy.Users)

	}

	err = controller.RemoveUserFromProxy(proxyID, user.Username, user.Password)

	if err != nil {
		t.Fatalf("*controller.RemoveUserFromProxy() threw an error when removing user: %s",err)
	}

	if len(proxy.Users) != 0 {
		t.Fatalf("*controller.RemoveUserFromProxy() did not actually remove user from proxy. Users is: %v", proxy.Users)

	}

}

//TODO:
func TestRemoveViewerFromProxy(t *testing.T) {
	controller := makeNewController()
	controller.removeViewerFromProxy(0,"")
}

func TestControllerListenPlain(t *testing.T) {
	controller := makeNewController()
	controller.SocketType = PROXY_CONTROLLER_SOCKET_PLAIN
	go controller.Listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &ControllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.Sign([]byte(controller.PresharedKey))
	data, err := json.Marshal(signedMessage)

	if (err != nil) {
		t.Fatalf("Unexpected fatal error from json.Marshal")
	}

	addr, err := net.ResolveTCPAddr("tcp", controller.SocketHost)
    if err != nil {
        t.Fatalf("ResolveTCPAddr failed: %s", err)
    }

    conn, err := net.DialTCP("tcp", nil, addr)
    if err != nil {
        t.Fatalf("Dial failed: %s", err)
    }

    _, err = conn.Write(data)
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }
	_, err = conn.Write([]byte("\n"))
	if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }
    reply := make([]byte, 1024)

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

    _, err = conn.Read(reply)
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    }

	defer conn.Close()
	controller.Stop()
}

func TestControllerListenTLS(t *testing.T) {
	testCert, testKey, cleanupKeys := makeTestKeys()
	defer cleanupKeys()
	controller := makeNewController()
	controller.TLSKey = testKey
	controller.TLSCert = testCert
	controller.SocketType = PROXY_CONTROLLER_SOCKET_TLS
	go controller.Listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &ControllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.Sign([]byte(controller.PresharedKey))
	data, err := json.Marshal(signedMessage)

	if (err != nil) {
		t.Fatalf("Unexpected fatal error from json.Marshal")
	}
	// https://eli.thegreenplace.net/2021/go-socket-servers-with-tls/
	// https://github.com/eliben/code-for-blog/blob/master/LICENSE
	cert, err := os.ReadFile(testCert)
	if err != nil {
	  t.Fatal(err)
	}
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
	  log.Fatalf("unable to parse cert from %s", testCert)
	}
	config := &tls.Config{RootCAs: certPool}
  
	conn, err := tls.Dial("tcp", controller.SocketHost, config)
	if err != nil {
	  log.Fatal(err)
	}
	_, err = conn.Write(data)
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }
	_, err = conn.Write([]byte("\n"))
	if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }
    reply := make([]byte, 1024)

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

    _, err = conn.Read(reply)
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    }

	defer conn.Close()
	controller.Stop()
}
	


func TestControllerListenWeb(t *testing.T) {
	controller := makeNewController()
	controller.SocketType = PROXY_CONTROLLER_SOCKET_PLAIN_WEBSOCKET
	go controller.Listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &ControllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.Sign([]byte(controller.PresharedKey))
	data, err := json.Marshal(signedMessage)

	if (err != nil) {
		t.Fatalf("Unexpected fatal error from json.Marshal")
	}

	connectURL := url.URL{Scheme: "ws", Host: controller.SocketHost, Path: "/"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))

    err = conn.WriteMessage(websocket.TextMessage, data)
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

    _, _, err = conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    }

	controller.Stop()

}

func makeTLSWebsocketDialer(certFile string) *websocket.Dialer {

	cert, err := os.ReadFile(certFile)
	if err != nil {
		log.Fatal(err)
	}
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		log.Fatalf("unable to parse cert from %s", certFile)
	}
	config := &tls.Config{RootCAs: certPool}
	TLSDialer := &websocket.Dialer{
		Subprotocols:     []string{"p1", "p2"},
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		HandshakeTimeout: 3 * time.Second,
		TLSClientConfig: config,
	}
	return TLSDialer
}

func TestControllerListenWebTLS(t *testing.T) {
	testCert, testKey, cleanupKeys := makeTestKeys()
	defer cleanupKeys()
	controller := makeNewController()
	controller.TLSKey = testKey
	controller.TLSCert = testCert
	controller.SocketType = PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET
	go controller.Listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &ControllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.Sign([]byte(controller.PresharedKey))
	data, err := json.Marshal(signedMessage)

	if (err != nil) {
		t.Fatalf("Unexpected fatal error from json.Marshal")
	}
	TLSDialer :=makeTLSWebsocketDialer(testCert)
	connectURL := url.URL{Scheme: "wss", Host: controller.SocketHost, Path: "/"}
	conn, _, err :=TLSDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))

    err = conn.WriteMessage(websocket.TextMessage, data)
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

	

    _, _, err = conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    }

	controller.Stop()
}


func TestControllerStartWebServerPlaintext(t *testing.T) {
	controller := makeNewController()
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.DefaultSigner)
	controller.AddExistingProxy(proxy)
	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s", err)
	}
	defer conn.Close()
    err = conn.WriteMessage(websocket.TextMessage, []byte("unsupported"))
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

    _, _, err = conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    }
}

//TODO: change response to controller handleWebProxyRequest so it 
// provides more meaningful error if no proxies are currently running.

func TestControllerStartWebServerTLS(t *testing.T) {
	testCert, testKey, cleanupKeys := makeTestKeys()
	defer cleanupKeys()
	controller := makeNewController()
	controller.TLSKey = testKey
	controller.TLSCert = testCert
	controller.SocketType = PROXY_CONTROLLER_SOCKET_TLS_WEBSOCKET
	controller.InitializeSocket()
	proxy := MakeNewProxy(controller.DefaultSigner)
	controller.AddExistingProxy(proxy)
	go controller.StartWebServer()
	defer controller.StopWebServer()
	time.Sleep(100* time.Millisecond)
	TLSDialer :=makeTLSWebsocketDialer(testCert)

	connectURL := url.URL{Scheme: "wss", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
	conn, resp, err :=TLSDialer.Dial(connectURL.String(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to websocket: %s, %v", err, resp)
	}
	defer conn.Close()

    err = conn.WriteMessage(websocket.TextMessage, []byte("unsupported"))
    if err != nil {
        t.Fatalf("Write to controller failed: %s", err)
    }

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

    _, _, err = conn.ReadMessage()
    if err != nil {
        t.Fatalf("Read from controller failed: %s", err)
    }
}

func TestAddAndRemoveChannelFilterFromUsery(t *testing.T) {
	controller := makeNewController()
	proxy := MakeNewProxy(controller.DefaultSigner)
	proxyID := controller.AddExistingProxy(proxy)
	user:= &ProxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.AddProxyUser(user)

	err, key := controller.AddChannelFilterToUser(proxyID, user.Username, user.Password, &ChannelFilterFunc{fn: 
		func(in_data []byte, wrapper *channelWrapper) []byte {
			return in_data
		}})
	
	if err != nil {
		t.Fatalf("*controller.AddChannelFilterToUser() threw an unexpected error: %s",err)
	}

	if user.channelFilters == nil {
		t.Fatalf("*controller.AddChannelFilterToUser() failed to allocate list.")
	}
	
	if len( user.channelFilters) == 0 {
		t.Fatalf("*controller.AddChannelFilterToUser() failed to actually add channel filter.")
	}

	err = controller.RemoveChannelFilterFromUserByKey(proxyID, user.Username, user.Password, key)
	if err != nil {
		t.Fatalf("*controller.RemoveChannelFilterFromUserByKey() threw an error: %s", err)
	}
	
	if len( user.channelFilters) != 0 {
		t.Fatalf("*controller.RemoveChannelFilterFromUserByKey() failed to actually remove filter.")
	}
	
}

/*
func TestControllerFullIntegration(t *testing.T) {

	return

	testString := "echo this is a test string"
	newString :=  "echo this is a new string"
	testUsername := "user"
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
	controller := makeNewController()
	controller.InitializeSocket()
	go controller.StartWebServer()
	defer controller.StopWebServer()


	proxy := MakeNewProxy(controller.DefaultSigner)
	proxy.DefaultRemotePort = int(dummyServer.port.Int64())
	proxy.ListenPort =  int(newRandomPort().Int64())
	proxy.active = true
	proxy.PublicAccess = false
	proxyID := controller.AddExistingProxy(proxy)

	testUser1 := &ProxyUser{
		Username: "testuser1",
		Password: "testPassword1",
		RemoteUsername: testUsername,
		RemotePassword: testPassword,
	}

	proxy.AddProxyUser(testUser)	

	err, viewer := controller.CreateUserSessionViewer(proxyID, testUser1.Username, testUser1.Password)

	// create proxy connecting to it

	go proxy.StartProxy()
	
	time.Sleep(500*time.Millisecond)

	// TODO: replace this with something like sendListOfCommandsToTestServer and include option of delay between actions
	// move to goroutine
	err, testReply := sendCommandToTestServer("127.0.0.1:"+strconv.Itoa(proxy.ListenPort), testUser, testPassword, testString)
	if (err != nil) {
		t.Errorf("Error when sending command to proxy: %s\n", err)
	}
	time.Sleep(time.Millisecond*100)

	curUserSessions, userSessionEntryFound := proxy.userSessions[testUser1.getKey()]
	if len(proxy.allSessions) != 1  {
		t.Errorf("Proxy did not store session in allSessions.")
	} else if ! userSessionEntryFound  {
		t.Errorf("Proxy did not create entry in userSessions for user.")
	} else if len(curUserSessions) != 1 {
		t.Errorf("Proxy did not store session in userSessions.")
	} else {

		// use viewer to view this stuff in real time

		connectURL := url.URL{Scheme: "ws", Host: controller.WebHost, Path: "/proxysocket/?id=0"}
		conn, _, err := websocket.DefaultDialer.Dial(connectURL.String(), nil)
		if err != nil {
			t.Fatalf("Failed to connect to websocket: %s", err)
		}
		defer conn.Close()
		err = conn.WriteMessage(websocket.TextMessage, []byte("viewer-get"))
		if err != nil {
			t.Fatalf("Write to websocket failed: %s", err)
		}
	
		err = conn.WriteMessage(websocket.TextMessage, []byte(viewer.Secret))
		if err != nil {
			t.Fatalf("Write to websocket failed: %s", err)
		}
	
		err = conn.WriteMessage(websocket.TextMessage, []byte(proxySessionActiveKey))
		if err != nil {
			t.Fatalf("Write to websocket failed: %s", err)
		}


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

}*/