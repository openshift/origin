package scmauth

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
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
	SourceURL s2igit.URL
}

// Setup creates a .gitconfig fragment that points to the given ca.crt
func (s CACert) Setup(baseDir string, context SCMAuthContext) error {
	if !(s.SourceURL.Type == s2igit.URLTypeURL && s.SourceURL.URL.Scheme == "https" && s.SourceURL.URL.Opaque == "") {
		return nil
	}
	gitconfig, err := ioutil.TempFile("", "ca.crt.")
	if err != nil {
		return err
	}
	defer gitconfig.Close()
	gitconfig.WriteString(fmt.Sprintf(CACertConfig, filepath.Join(baseDir, CACertName)))
	return ensureGitConfigIncludes(gitconfig.Name(), context)
}

// Name returns the name of this auth method.
func (_ CACert) Name() string {
	return CACertName
}

// Handles returns true if the secret is a CA certificate
func (_ CACert) Handles(name string) bool {
	return name == CACertName
}
