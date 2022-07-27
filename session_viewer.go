package main


import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
)

const SESSION_VIEWER_TYPE_SINGLE = 0
const SESSION_VIEWER_TYPE_LIST 	 = 1

const SESSION_VIEWER_SECRET_LEN	 = 64

const SESSION_VIEWER_EXPIRATION = -1

type proxySessionViewer struct {
	ViewerType int
	Secret string
	proxy *proxyContext
	User  *proxyUser
	SessionKey string
	expiration int64
}

func (viewer *proxySessionViewer) buildSignedURL(proxyID uint64) string {
	return fmt.Sprintf("%v/?id=%v#signed-viewer&%v", viewer.proxy.BaseURI,proxyID,viewer.Secret)
}

func createNewSessionViewer(ViewerType int, proxy *proxyContext, user *proxyUser) *proxySessionViewer {
	viewer := &proxySessionViewer{}
	var err error
	viewer.ViewerType = ViewerType
	viewer.Secret, err = GenerateRandomString(SESSION_VIEWER_SECRET_LEN)
	viewer.User = user
	viewer.proxy = proxy

	viewer.expiration = SESSION_VIEWER_EXPIRATION
	if err != nil {
		panic(fmt.Sprintf("error creating secret: %#v", err))
		return nil
	}
	return viewer
}

func (viewer *proxySessionViewer) typeIsSingle() bool {
	return viewer.ViewerType == SESSION_VIEWER_TYPE_SINGLE
}

func (viewer *proxySessionViewer) typeIsList() bool {
	return viewer.ViewerType == SESSION_VIEWER_TYPE_LIST
}

func (viewer *proxySessionViewer) getSessions() (map[string]*sessionContext, []string) {
	session_keys := make([]string, 0)
	user_key := viewer.User.getKey()
	if  _, ok := viewer.proxy.userSessions[user_key]; ok {
		if viewer.typeIsSingle() {
			finalMap := make(map[string]*sessionContext)
			if _, ok := viewer.proxy.userSessions[user_key][viewer.SessionKey]; ok {
				finalMap[viewer.SessionKey] = viewer.proxy.userSessions[user_key][viewer.SessionKey]
				session_keys = append(session_keys,viewer.SessionKey)
			}
			return finalMap, session_keys
		} else {
			return viewer.proxy.userSessions[user_key], viewer.proxy.ListAllUserSessions(user_key)
		}
	} else {
		//viewer.proxy.log.Println("could not find user_key in user session", user_key, viewer.proxy.userSessions)
		return make(map[string]*sessionContext), session_keys
	}
}

func (viewer *proxySessionViewer) isExpired() bool {
	return false
}


// referencing https://gist.github.com/dopey/c69559607800d2f2f90b1b1ed4e550fb
// MIT license per: https://gist.github.com/dopey/c69559607800d2f2f90b1b1ed4e550fb?permalink_comment_id=3603953#gistcomment-3603953

func init() {
	assertAvailablePRNG()
}

func assertAvailablePRNG() {
	// Assert that a cryptographically secure PRNG is available.
	// Panic otherwise.
	buf := make([]byte, 1)

	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		panic(fmt.Sprintf("crypto/rand is unavailable: Read() failed with %#v", err))
	}
}

// GenerateRandomBytes returns securely generated random bytes.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString returns a securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_."
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}

// GenerateRandomStringURLSafe returns a URL-safe, base64 encoded
// securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomStringURLSafe(n int) (string, error) {
	b, err := GenerateRandomBytes(n)
	return base64.RawURLEncoding.EncodeToString(b), err
}