package scmauth

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/util/file"
)

const (
	DefaultUsername      = "builder"
	UsernamePasswordName = "password"
	UsernameSecret       = "username"
	PasswordSecret       = "password"
	TokenSecret          = "token"
	UserPassGitConfig    = `# credential git config
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
func (u UsernamePassword) Setup(baseDir string) (*url.URL, error) {
	// Only apply to https and http URLs
	if scheme := strings.ToLower(u.SourceURL.Scheme); scheme != "http" && scheme != "https" {
		return nil, nil
	}

	// Read data from secret files
	usernameSecret, err := readSecret(baseDir, UsernameSecret)
	if err != nil {
		return nil, err
	}
	passwordSecret, err := readSecret(baseDir, PasswordSecret)
	if err != nil {
		return nil, err
	}
	tokenSecret, err := readSecret(baseDir, TokenSecret)
	if err != nil {
		return nil, err
	}

	// Determine overrides
	overrideSourceURL, gitconfigURL, err := doSetup(u.SourceURL, usernameSecret, passwordSecret, tokenSecret)
	if err != nil {
		return nil, err
	}

	// Write git config if needed
	if gitconfigURL != nil {
		gitcredentials, err := ioutil.TempFile("", "gitcredentials.")
		if err != nil {
			return nil, err
		}
		defer gitcredentials.Close()
		gitconfig, err := ioutil.TempFile("", "gitcredentialscfg.")
		if err != nil {
			return nil, err
		}
		defer gitconfig.Close()

		fmt.Fprintf(gitconfig, UserPassGitConfig, gitcredentials.Name())
		fmt.Fprintf(gitcredentials, "%s", gitconfigURL.String())

		if err := ensureGitConfigIncludes(gitconfig.Name()); err != nil {
			return nil, err
		}
	}

	return overrideSourceURL, nil
}

func doSetup(sourceURL url.URL, usernameSecret, passwordSecret, tokenSecret string) (*url.URL, *url.URL, error) {
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

	// Remove user/pw from the source url
	overrideSourceURL := sourceURL
	overrideSourceURL.User = nil

	// Set user/pw in the config url
	configURL := sourceURL
	configURL.User = url.UserPassword(username, password)

	return &overrideSourceURL, &configURL, nil
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
