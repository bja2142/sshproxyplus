package main

import (
	"testing"
	"log"
	"time"
)

func makeNewController() *proxyController {
	controller := &proxyController{
		SocketType: PROXY_CONTROLLER_SOCKET_PLAIN,
		SocketHost: "127.0.0.1:8000",
		PresharedKey: "key",
		Proxies: make(map[uint64]*proxyContext),
		WebHost: "127.0.0.1:8001",
		WebStaticDir: ".",
		log: log.Default(),
	}
	controller.initialize()
	return controller
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

