package main

	

import (
    "testing"
	"fmt"
	"log"
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

func makeNewTestProxy() *proxyContext {

	
	return &proxyContext{
		log: log.Default(),
		Users: map[string]*proxyUser{},
		userSessions: map[string]map[string]*sessionContext{},
		allSessions: map[string]*sessionContext{},
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

func makeNewTestProxyUser() *proxyUser {
	return &proxyUser{
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


	expected_host := proxy.getDefaultRemoteHost()

	err, user := proxy.authenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid proxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
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


	expected_host := proxy.getDefaultRemoteHost()
	
	err, user := proxy.authenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid proxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
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


	expected_host := proxy.getDefaultRemoteHost()
	
	err, user := proxy.authenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid proxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
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

	proxy.addProxyUser(proxy_user)

	err, user := proxy.authenticateUser(test_user,test_pass)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid proxyUser object`, test_user,test_pass, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user,test_pass, user, err,expected_host, expected_user, expected_pass)
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

	proxy.addProxyUser(proxy_user)
	proxy.addProxyUser(proxy_user2)

	err, user := proxy.authenticateUser(test_user1,test_pass1)

	err2, user2 := proxy.authenticateUser(test_user2,test_pass2)


	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid proxyUser object`, test_user1,test_pass1, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user1,test_pass1, user, err,expected_host, expected_user, expected_pass)
    }

	if err2 != nil || user2 == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want no error and a valid proxyUser object`, test_user2,test_pass2, user2, err2)
    }

	if user2.RemoteHost != expected_host || user2.RemoteUsername != expected_user || user2.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user2,test_pass2, user2, err2,expected_host, expected_user, expected_pass)
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

	proxy.addProxyUser(proxy_user)

	test_pass2 := "pass"

	err, user := proxy.authenticateUser(test_user1,test_pass1)
	err2, user2 := proxy.authenticateUser(test_user1,test_pass2)

	if err != nil || user == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving blank password, want no error and a valid proxyUser object`, test_user1,test_pass1, user, err)
    }

	if user.RemoteHost != expected_host || user.RemoteUsername != expected_user || user.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving blank password, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user1,test_pass1, user, err,expected_host, expected_user, expected_pass)
    }

	if err2 != nil || user2 == nil {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving non-blank password, want no error and a valid proxyUser object`, test_user1,test_pass2, user2, err2)
    }

	if user2.RemoteHost != expected_host || user2.RemoteUsername != expected_user || user2.RemotePassword != expected_pass {
        t.Fatalf(`authenticateUser(%v,%v) = %v, %v, when giving non-blank password, want proxyUser object to have host, user, pass values %v, %v, %v`, test_user1,test_pass2, user2, err2,expected_host, expected_user, expected_pass)
    }
}


