package main

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
)

func newRandomPort() *big.Int {
	port, _ := rand.Int(rand.Reader,big.NewInt(65534-1024))
	port.Add(port,big.NewInt(1025))
	return port
}

func makeNewController() *proxyController {
	port := newRandomPort()
	controller := &proxyController{
		SocketType: PROXY_CONTROLLER_SOCKET_PLAIN,
		SocketHost: "127.0.0.1:"+port.Text(10),
		PresharedKey: "key",
		Proxies: make(map[uint64]*proxyContext),
		WebHost: "127.0.0.1:"+port.Add(port,big.NewInt(1)).Text(10),
		WebStaticDir: ".",
		log: log.Default(),
	}
	controller.initialize()
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
	keypairString, _ := GenerateRandomString(8)
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

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	proxy, err := controller.getProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when getting proxy just created: %s",err)
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
		t.Errorf("*controller addProxyFromJSON() error populating values in proxy object")
	}

	testProxyUser, testProxyUserFound := proxy.Users["testuser:"]
	if ( ! testProxyUserFound ) {
		t.Fatalf("*controller addProxyFromJSON() generated proxy did not get test user: %v", proxy.Users)
	}
	if ( testProxyUser.Username!= "testuser" || 
		 testProxyUser.RemoteHost != "127.0.0.1:22" || 
		 testProxyUser.RemoteUsername != "ben" || 
		 testProxyUser.RemotePassword != "password" ) {
			t.Errorf("*controller addProxyFromJSON() test proxy user did not correctly populate")
		 }

	testProxyViewer, testProxyViewerFound := proxy.Viewers["2r3K4wQZegCz6i-6k1SMx1kb5x2BntDZpQsWXYxFDEMAsbY-tb0MshRzmuXLL.IZ"]
	if ( ! testProxyViewerFound ) {
		t.Fatalf("*controller addProxyFromJSON() generated proxy did not get test viewer: %v", proxy.Viewers)
	}
	if ( testProxyViewer.ViewerType != 1 || // SESSION_VIEWER_TYPE_LIST
		testProxyViewer.Secret != "2r3K4wQZegCz6i-6k1SMx1kb5x2BntDZpQsWXYxFDEMAsbY-tb0MshRzmuXLL.IZ" ||
		testProxyViewer.SessionKey != "" ) {
		t.Errorf("*controller addProxyFromJSON() test proxy viewer did not correctly populate: %v", testProxyViewer)
	}
	if (testProxyViewer.User != proxy.Users["testuser:"] ) {
		t.Errorf("*controller addProxyFromJSON() test proxy viewer user (%p) does not point to expected user: %p", testProxyViewer.User, proxy.Users["testuser:"])
	}

}

func TestControllerAddProxyFromJSONEmptyCase(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}

	_, err = controller.getProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when getting proxy just created: %s",err)
	}
}

func TestControllerStartAndStopProxy(t *testing.T) {
	controller := makeNewController()

	proxyJSON := []byte(`{}`)

	err,proxyID := controller.addProxyFromJSON(proxyJSON)
	if err != nil {
		t.Fatalf("*controller addProxyFromJSON() error when parsing valid proxy JSON blob: %s",err)
	}
	proxy, err := controller.getProxy(proxyID)
	if err != nil {
		t.Fatalf("*controller getPRoxy() error when getting proxy just created: %s",err)
	}

	err = controller.startProxy(proxyID)
	for proxy.running == false {
		time.Sleep(100)
	}
	controller.stopProxy(proxyID)

	if (err != nil) {
		t.Fatalf("*controller startProxy() error when starting proxy: %s",err)
	}

	

	
}


func TestControllerAddAndGetProxySingle(t *testing.T) {
	controller := makeNewController()

	proxy := makeNewProxy(controller.defaultSigner)

	var expectedProxyID uint64 = 5
	controller.ProxyCounter = expectedProxyID

	proxyID := controller.addExistingProxy(proxy)

	if(proxyID != expectedProxyID){
		t.Fatalf("*controller addExistingProxy() error did not produce expected proxy ID. Expected: %v, Got: %v",expectedProxyID, proxyID)
	}

	gotProxy, err := controller.getProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller getProxy() error when getting proxy just created: %s",err)
	}

	if gotProxy != proxy {
		t.Errorf("*controller getProxy() did not return the expected proxy when given its corresponding ID.")
	}
}

func TestControllerAddAndGetProxyMultiple(t *testing.T) {
	controller := makeNewController()

	proxy0 := makeNewProxy(controller.defaultSigner)
	proxy1 := makeNewProxy(controller.defaultSigner)
	proxy2 := makeNewProxy(controller.defaultSigner)


	proxyID0 := controller.addExistingProxy(proxy0)
	proxyID1 := controller.addExistingProxy(proxy1)
	proxyID2 := controller.addExistingProxy(proxy2)


	if(proxyID0 != 0 || proxyID1 != 1 || proxyID2 != 2 ){
		t.Fatalf("*controller addExistingProxy() error did not produce expected proxy IDs.")
	}

	gotProxy0, err0 := controller.getProxy(proxyID0)
	gotProxy1, err1 := controller.getProxy(proxyID1)
	gotProxy2, err2 := controller.getProxy(proxyID2)

	if err0 != nil  || err1 != nil || err2 != nil {
		t.Fatalf("*controller getProxy() error when getting a proxy just created: %v, %v, %v",err0, err1, err2)
	}

	if gotProxy0 != proxy0 || gotProxy1 != proxy1 || gotProxy2 != proxy2 {
		t.Errorf("*controller getProxy() did not return the expected proxy when given its corresponding ID.")
	}
}

func TestControllerGetProxyErrorConditionWrongID(t *testing.T) {
	controller := makeNewController()

	proxy := makeNewProxy(controller.defaultSigner)

	var fakeProxyID  uint64 = 10
	
	controller.addExistingProxy(proxy)

	_, err := controller.getProxy(fakeProxyID)

	if err == nil {
		t.Fatalf("*controller getProxy() did not return error when trying to get a proxy that doesn't exist: %s",err)
	}
}

func TestControllerDestroyProxy(t *testing.T) {
	controller := makeNewController()

	proxy0 := makeNewProxy(controller.defaultSigner)
	proxy1 := makeNewProxy(controller.defaultSigner)
	proxy2 := makeNewProxy(controller.defaultSigner)


	proxyID0 := controller.addExistingProxy(proxy0)
	proxyID1 := controller.addExistingProxy(proxy1)
	proxyID2 := controller.addExistingProxy(proxy2)

	controller.destroyProxy(proxyID1)

	gotProxy1, err1 := controller.getProxy(proxyID1)

	if err1 == nil {
		t.Fatalf("*controller getProxy() did not throw error when it should have")
	}

	if gotProxy1 != nil {
		t.Errorf("*controller getProxy() returned a destroyed proxy when it shouldn't have")
	}

	gotProxy0, err0 := controller.getProxy(proxyID0)
	gotProxy2, err2 := controller.getProxy(proxyID2)

	if err0 != nil  || err2 != nil {
		t.Errorf("*controller getProxy() error when getting a proxy just created: %v, %v, %v",err0, err1, err2)
	}

	if gotProxy0 != proxy0 || gotProxy2 != proxy2 {
		t.Errorf("*controller getProxy() did not return the expected proxy when given its corresponding ID.")
	}

	err := controller.destroyProxy(proxyID1)

	if err == nil {
		t.Errorf("*controller destroyProxy didn't error out when it should have after destroying the same proxy a second time.")
	}
}

func TestControllerActivateProxy(t *testing.T) {
	controller := makeNewController()

	proxy := makeNewProxy(controller.defaultSigner)
	
	proxyID := controller.addExistingProxy(proxy)

	proxy.active = false

	err := controller.activateProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller activateProxy() had an error: %s",err)
	}

	if proxy.active == false {
		t.Fatalf("*controller actiateProxy() did not activate the proxy")
	}
}

func TestControllerDeactivateProxy(t *testing.T) {
	controller := makeNewController()

	proxy := makeNewProxy(controller.defaultSigner)
	
	proxyID := controller.addExistingProxy(proxy)

	proxy.active = true

	err := controller.deactivateProxy(proxyID)

	if err != nil {
		t.Fatalf("*controller deactivateProxy() had an error: %s",err)
	}

	if proxy.active == true {
		t.Fatalf("*controller deactivateProxy() did not deactivate the proxy")
	}
}


func TestControllerCreateAndGetProxyViewerByViewerKey(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, viewer := controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller createUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}

	viewerKey := viewer.Secret

	err, testViewer := controller.getProxyViewerByViewerKey(proxyID, viewerKey)

	if err != nil {
		t.Fatalf("*controller getProxyViewerByViewerKey() threw an error getting the viewer: %s",err)
	}

	if testViewer != viewer {
		t.Errorf("controller getProxyViewerByViewerKey() did not return the correct viewer.")

	}

}

func TestControllerCreateAndGetProxyViewerBySessionKey(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	testSessionKey := "myfake-session-key.json"
	proxy.addProxyUser(user)

	err, viewer := controller.createSessionViewer(proxyID, user.Username, testSessionKey)

	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsSingle()) {
		t.Errorf("controller createSessionViewer() created a viewer of the wrong type. Expected single, but this was not so.")
	}

	err, testViewer := controller.getProxyViewerBySessionKey(proxyID, testSessionKey)

	if err != nil {
		t.Fatalf("*controller getProxyViewerBySessionKey() threw an error getting the viewer: %s",err)
	}

	if testViewer != viewer {
		t.Errorf("controller getProxyViewerBySessionKey() did not return the correct viewer.")

	}
}

func TestControllerCreateAndGetProxyViewerByUsername(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	proxy.addProxyUser(user)

	err, viewer := controller.createUserSessionViewer(proxyID, user.Username)

	if (err != nil) {
		t.Fatalf("*controller createUserSessionViewer() threw an error when creating new viewer: %s",err)
	}

	if (! viewer.typeIsList()) {
		t.Errorf("controller createUserSessionViewer() created a viewer of the wrong type. Expected list, but this was not so.")
	}
	err, testViewer := controller.getProxyViewerByUsername(proxyID, user.Username)

	if err != nil {
		t.Fatalf("*controller getProxyViewerByUsername() threw an error getting the viewer: %s",err)
	}

	if testViewer != viewer {
		t.Errorf("controller getProxyViewerByUsername() did not return the correct viewer.")

	}

}



func TestControllerCreateAndGetProxyViewersBySessionKey(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	testSessionKey := "myfake-session-key.json"
	proxy.addProxyUser(user)

	err, _ := controller.createSessionViewer(proxyID, user.Username, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createSessionViewer(proxyID, user.Username, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createSessionViewer(proxyID, user.Username, "not the test key")
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}

	err, viewers := controller.getProxyViewersBySessionKey(proxyID, testSessionKey)

	if err != nil {
		t.Fatalf("*controller getProxyViewersBySessionKey() threw an error getting the viewers: %s",err)
	}

	if len(viewers) != 2 {
		t.Errorf("controller getProxyViewersBySessionKey() did not return the correct number of viewers")
	}
}

func TestControllerCreateAndGetProxyViewersByUsername(t *testing.T) {
	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	user2:= &proxyUser{
		Username: "user2",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	user3:= &proxyUser{
		Username: user2.Username,
		Password: "with password",
		RemoteHost: "127.0.0.2:22",
		RemoteUsername: "ben2",
		RemotePassword: "password2"}
	testSessionKey := "myfake-session-key.json"
	proxy.addProxyUser(user)
	proxy.addProxyUser(user2)
	proxy.addProxyUser(user3)

	err, _ := controller.createSessionViewer(proxyID, user.Username, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createSessionViewer(proxyID, user2.Username, testSessionKey)
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}
	err, _ = controller.createSessionViewer(proxyID, user3.Username, "not the test key")
	if (err != nil) {
		t.Fatalf("*controller createSessionViewer() threw an error when creating new viewer: %s",err)
	}

	err, viewers := controller.getProxyViewersByUsername(proxyID, user2.Username)

	if err != nil {
		t.Fatalf("*controller getProxyViewersByUsername() threw an error getting the viewers: %s",err)
	}

	if len(viewers) != 2 {
		t.Errorf("controller getProxyViewersByUsername() did not return the correct number of viewers")
	}

}


func TestAddAndRemoveProxyUser(t *testing.T) {

	controller := makeNewController()
	proxy := makeNewProxy(controller.defaultSigner)
	proxyID := controller.addExistingProxy(proxy)
	user:= &proxyUser{
		Username: "testuser",
		Password: "",
		RemoteHost: "127.0.0.1:22",
		RemoteUsername: "ben",
		RemotePassword: "password"}
	expectedKey := user.Username + ":" + user.Password
	err, key := controller.addUserToProxy(proxyID,user)

	if err != nil {
		t.Fatalf("*controller addUserToProxy() threw an error adding a proxyUser: %s",err)
	}
	if  key != expectedKey {
		t.Fatalf("*controller addUserToProxy() did not create the expected proxy user key. Expected: %s, got: %s",expectedKey, key)
	}

	if len(proxy.Users) != 1 {
		t.Fatalf("*controller addUserToProxy() did not actually add user to the proxy. proxy.Users is: %v", proxy.Users)

	}

	err = controller.removeUserFromProxy(proxyID, user.Username, user.Password)

	if err != nil {
		t.Fatalf("*controller removeUserFromProxy() threw an error when removing user: %s",err)
	}

	if len(proxy.Users) != 0 {
		t.Fatalf("*controller removeUserFromProxy() did not actually remove user from proxy. proxy.Users is: %v", proxy.Users)

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
	go controller.listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.sign([]byte(controller.PresharedKey))
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
	go controller.listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.sign([]byte(controller.PresharedKey))
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
	go controller.listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.sign([]byte(controller.PresharedKey))
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
	go controller.listen()
	defer controller.Stop()
	time.Sleep(100* time.Millisecond)
	message := &controllerMessage{
		MessageType: CONTROLLER_MESSAGE_LIST_PROXIES,
		}
	_, signedMessage := message.sign([]byte(controller.PresharedKey))
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
	controller.initializeSocket()
	proxy := makeNewProxy(controller.defaultSigner)
	controller.addExistingProxy(proxy)
	go controller.startWebServer()
	defer controller.stopWebServer()
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
	controller.initializeSocket()
	proxy := makeNewProxy(controller.defaultSigner)
	controller.addExistingProxy(proxy)
	go controller.startWebServer()
	defer controller.stopWebServer()
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

	
// verify web server is able to see events occurring






// test start and stop of web server
// test getting session 