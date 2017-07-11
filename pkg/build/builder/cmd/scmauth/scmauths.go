package scmauth

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	"github.com/golang/glog"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
)

type SCMAuths []SCMAuth

func GitAuths(sourceURL *s2igit.URL) SCMAuths {
	auths := SCMAuths{
		&SSHPrivateKey{},
		&UsernamePassword{SourceURL: *sourceURL},
		&CACert{SourceURL: *sourceURL},
		&GitConfig{},
	}
	return auths
}

func (a SCMAuths) present(files []os.FileInfo) SCMAuths {
	scmAuthsPresent := map[string]SCMAuth{}
	for _, file := range files {
		glog.V(4).Infof("Finding auth for %q", file.Name())
		for _, scmAuth := range a {
			if scmAuth.Handles(file.Name()) {
				glog.V(4).Infof("Found SCMAuth %q to handle %q", scmAuth.Name(), file.Name())
				scmAuthsPresent[scmAuth.Name()] = scmAuth
			}
		}
	}
	auths := SCMAuths{}
	for _, auth := range scmAuthsPresent {
		auths = append(auths, auth)
	}
	return auths
}

func (a SCMAuths) doSetup(secretsDir string) (*defaultSCMContext, error) {
	context := NewDefaultSCMContext()
	for _, auth := range a {
		glog.V(4).Infof("Setting up SCMAuth %q", auth.Name())
		err := auth.Setup(secretsDir, context)
		if err != nil {
			return nil, fmt.Errorf("cannot set up source authentication method %q: %v", auth.Name(), err)
		}
	}
	return context, nil

}

func (a SCMAuths) Setup(secretsDir string) (env []string, overrideURL *url.URL, err error) {
	files, err := ioutil.ReadDir(secretsDir)
	if err != nil {
		return nil, nil, err
	}
	// Filter the list of SCMAuths based on the secret files that are present
	presentAuths := a.present(files)
	if len(presentAuths) == 0 {
		return nil, nil, fmt.Errorf("no auth handler was found for secrets in %s", secretsDir)
	}

	// Setup the present SCMAuths
	context, err := presentAuths.doSetup(secretsDir)
	if err != nil {
		return nil, nil, err
	}

	return context.Env(), context.OverrideURL(), nil
}
