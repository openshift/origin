package scmauth

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/util/file"
)

const (
	DefaultUsername       = "builder"
	UsernamePasswordName  = "password"
	UsernameSecret        = "username"
	PasswordSecret        = "password"
	TokenSecret           = "token"
	curlPasswordThreshold = 255
	proxyHost             = "127.0.0.1:8080"
	UserPassGitConfig     = `# credential git config
[credential]
   helper = store --file=%s
`
)

// UsernamePassword implements SCMAuth interface for using Username and Password credentials
type UsernamePassword struct {
	SourceURL url.URL
}

// Setup creates a gitconfig fragment that includes a substitution URL with the username/password
// included in the URL. Returns source URL stripped of username/password credentials.
func (u UsernamePassword) Setup(baseDir string, context SCMAuthContext) error {
	// Only apply to https and http URLs
	if scheme := strings.ToLower(u.SourceURL.Scheme); scheme != "http" && scheme != "https" {
		return nil
	}

	// Read data from secret files
	usernameSecret, err := readSecret(baseDir, UsernameSecret)
	if err != nil {
		return err
	}
	passwordSecret, err := readSecret(baseDir, PasswordSecret)
	if err != nil {
		return err
	}
	tokenSecret, err := readSecret(baseDir, TokenSecret)
	if err != nil {
		return err
	}

	// Determine overrides
	overrideFn := longPasswordOverride(baseDir, removeCredentials)
	overrideSourceURL, gitconfigURL, err := doSetup(u.SourceURL, usernameSecret, passwordSecret, tokenSecret, overrideFn)
	if err != nil {
		return err
	}
	if overrideSourceURL != nil {
		if err := context.SetOverrideURL(overrideSourceURL); err != nil {
			return err
		}
	}

	// Write git config if needed
	if gitconfigURL != nil {
		gitcredentials, err := ioutil.TempFile("", "gitcredentials.")
		if err != nil {
			return err
		}
		defer gitcredentials.Close()
		gitconfig, err := ioutil.TempFile("", "gitcredentialscfg.")
		if err != nil {
			return err
		}
		defer gitconfig.Close()

		fmt.Fprintf(gitconfig, UserPassGitConfig, gitcredentials.Name())
		fmt.Fprintf(gitcredentials, "%s", gitconfigURL.String())

		return ensureGitConfigIncludes(gitconfig.Name(), context)
	}

	return nil
}

type overrideURLFunc func(sourceURL *url.URL, username, password string) (*url.URL, error)

func removeCredentials(sourceURL *url.URL, username, password string) (*url.URL, error) {
	overrideURL := *sourceURL
	overrideURL.User = nil
	return &overrideURL, nil
}

func longPasswordOverride(dir string, overrideFn overrideURLFunc) overrideURLFunc {
	return func(sourceURL *url.URL, username, password string) (*url.URL, error) {
		if len(password) > curlPasswordThreshold {
			return startProxy(dir, sourceURL, username, password)
		}
		return overrideFn(sourceURL, username, password)
	}
}

func doSetup(sourceURL url.URL, usernameSecret, passwordSecret, tokenSecret string, overrideURLFn overrideURLFunc) (*url.URL, *url.URL, error) {
	// Extract auth from the source URL
	urlUsername := ""
	urlPassword := ""
	if sourceURL.User != nil {
		urlUsername = sourceURL.User.Username()
		urlPassword, _ = sourceURL.User.Password()
	}

	// Determine username in this order: secret, url
	username := usernameSecret
	if username == "" {
		username = urlUsername
	}

	// Determine password in this order: token secret, password secret, url
	password := tokenSecret
	if password == "" {
		password = passwordSecret
	}
	if password == "" {
		password = urlPassword
	}

	// If we have no password, and the username matches what is already in the URL, no overrides or config are needed
	if password == "" && username == urlUsername {
		return nil, nil, nil
	}

	// If we're going to write config, ensure we have a username
	if username == "" {
		username = DefaultUsername
	}

	overrideSourceURL, err := overrideURLFn(&sourceURL, username, password)
	if err != nil {
		return nil, nil, err
	}

	// Set user/pw in the config url
	configURL := sourceURL
	configURL.User = url.UserPassword(username, password)

	return overrideSourceURL, &configURL, nil
}

// Name returns the name of this auth method.
func (_ UsernamePassword) Name() string {
	return UsernamePasswordName
}

// Handles returns true if a username, password or token secret is present
func (_ UsernamePassword) Handles(name string) bool {
	switch name {
	case UsernameSecret, PasswordSecret, TokenSecret:
		return true
	}
	return false
}

// readSecret reads the contents of a secret file
func readSecret(baseDir, fileName string) (string, error) {
	path := filepath.Join(baseDir, fileName)
	lines, err := file.ReadLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	// If the file is empty, simply return empty string
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], nil
}

type basicAuthTransport struct {
	transport http.RoundTripper
	username  string
	password  string
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.username, t.password)
	// Ensure that the Host header has the value of the real git server
	// and not the local proxy.
	req.Host = req.URL.Host
	if glog.V(8) {
		glog.Infof("Proxying request to URL %q", req.URL.String())
		if reqdump, err := httputil.DumpRequestOut(req, false); err == nil {
			glog.Infof("Request:\n%s\n", string(reqdump))
		}
	}
	resp, err := t.transport.RoundTrip(req)
	if glog.V(8) {
		if err != nil {
			glog.Infof("Proxy RoundTrip error: %#v", err)
		} else {
			if respdump, err := httputil.DumpResponse(resp, false); err == nil {
				glog.Infof("Response:\n%s\n", string(respdump))
			}
		}
	}
	return resp, err
}

func startProxy(dir string, sourceURL *url.URL, username, password string) (*url.URL, error) {
	// Setup the targetURL of the proxy to be the host of the original URL
	targetURL := &url.URL{
		Scheme: sourceURL.Scheme,
		Host:   sourceURL.Host,
	}
	proxyHandler := httputil.NewSingleHostReverseProxy(targetURL)

	// The baseTransport will either be the default transport or a
	// transport with the appropriate TLSConfig if a ca.crt is present.
	baseTransport := http.DefaultTransport

	// Check whether a CA cert is available and use it if it is.
	caCertFile := filepath.Join(dir, "ca.crt")
	_, err := os.Stat(caCertFile)
	if err == nil && sourceURL.Scheme == "https" {
		baseTransport, err = cmdutil.TransportFor(caCertFile, "", "")
		if err != nil {
			return nil, err
		}
	}

	// Build a basic auth RoundTripper for use by the proxy
	authTransport := &basicAuthTransport{
		username:  username,
		password:  password,
		transport: baseTransport,
	}
	proxyHandler.Transport = authTransport

	// Start the proxy go-routine
	go func() {
		log.Fatal(http.ListenAndServe(proxyHost, proxyHandler))
	}()

	// The new URL will use the proxy endpoint
	proxyURL := *sourceURL
	proxyURL.User = nil
	proxyURL.Host = proxyHost
	proxyURL.Scheme = "http"

	return &proxyURL, nil
}
