package scmauth

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/util/file"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
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
	SourceURL s2igit.URL
}

// Setup creates a gitconfig fragment that includes a substitution URL with the username/password
// included in the URL. Returns source URL stripped of username/password credentials.
func (u UsernamePassword) Setup(baseDir string, context SCMAuthContext) error {
	// Only apply to https and http URLs
	if !(u.SourceURL.Type == s2igit.URLTypeURL &&
		(u.SourceURL.URL.Scheme == "http" || u.SourceURL.URL.Scheme == "https") &&
		u.SourceURL.URL.Opaque == "") {
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
	overrideSourceURL, gitconfigURL, err := doSetup(u.SourceURL.URL, usernameSecret, passwordSecret, tokenSecret)
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

		configContent := fmt.Sprintf(UserPassGitConfig, gitcredentials.Name())

		glog.V(5).Infof("Adding username/password credentials to git config:\n%s\n", configContent)

		fmt.Fprintf(gitconfig, "%s", configContent)
		fmt.Fprintf(gitcredentials, "%s", gitconfigURL.String())

		return ensureGitConfigIncludes(gitconfig.Name(), context)
	}

	return nil
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
