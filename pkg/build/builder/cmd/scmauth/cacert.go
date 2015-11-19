package scmauth

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	CACertName   = "ca.crt"
	CACertConfig = `# SSL cert
[http]
   sslCAInfo = %[1]s
`
)

// CACert implements SCMAuth interface for using a custom certificate authority
type CACert struct {
	SourceURL url.URL
}

// Setup creates a .gitconfig fragment that points to the given ca.crt
func (s CACert) Setup(baseDir string) (*url.URL, error) {
	if strings.ToLower(s.SourceURL.Scheme) != "https" {
		return nil, nil
	}
	gitconfig, err := ioutil.TempFile("", "ca.crt.")
	if err != nil {
		return nil, err
	}
	defer gitconfig.Close()
	gitconfig.WriteString(fmt.Sprintf(CACertConfig, filepath.Join(baseDir, CACertName)))
	return nil, ensureGitConfigIncludes(gitconfig.Name())
}

// Name returns the name of this auth method.
func (_ CACert) Name() string {
	return CACertName
}

// Handles returns true if the secret is a CA certificate
func (_ CACert) Handles(name string) bool {
	return name == CACertName
}
