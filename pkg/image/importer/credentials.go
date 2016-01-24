package importer

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/golang/glog"

	"github.com/docker/distribution/registry/client/auth"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/util"
)

var (
	NoCredentials auth.CredentialStore = &noopCredentialStore{}

	emptyKeyring = &credentialprovider.BasicDockerKeyring{}
)

type noopCredentialStore struct{}

func (s *noopCredentialStore) Basic(url *url.URL) (string, string) {
	glog.Infof("asked to provide Basic credentials for %s", url)
	return "", ""
}

func NewBasicCredentials() *BasicCredentials {
	return &BasicCredentials{}
}

type basicForURL struct {
	url                url.URL
	username, password string
}

type BasicCredentials struct {
	creds []basicForURL
}

func (c *BasicCredentials) Add(url *url.URL, username, password string) {
	c.creds = append(c.creds, basicForURL{*url, username, password})
}

func (c *BasicCredentials) Basic(url *url.URL) (string, string) {
	for _, cred := range c.creds {
		if len(cred.url.Host) != 0 && cred.url.Host != url.Host {
			continue
		}
		if len(cred.url.Path) != 0 && cred.url.Path != url.Path {
			continue
		}
		return cred.username, cred.password
	}
	return "", ""
}

func NewLocalCredentials() auth.CredentialStore {
	return &keyringCredentialStore{credentialprovider.NewDockerKeyring()}
}

type keyringCredentialStore struct {
	credentialprovider.DockerKeyring
}

func (s *keyringCredentialStore) Basic(url *url.URL) (string, string) {
	return basicCredentialsFromKeyring(s.DockerKeyring, url)
}

func NewCredentialsForSecrets(secrets []kapi.Secret) auth.CredentialStore {
	return &secretCredentialStore{secrets: secrets}
}

type secretCredentialStore struct {
	secrets []kapi.Secret

	lock    sync.Mutex
	keyring credentialprovider.DockerKeyring
}

func (s *secretCredentialStore) Basic(url *url.URL) (string, string) {
	return basicCredentialsFromKeyring(s.init(), url)
}

func (s *secretCredentialStore) init() credentialprovider.DockerKeyring {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.keyring != nil {
		return s.keyring
	}
	// TODO: need a version of this that is best effort secret - otherwise one error blocks all secrets
	keyring, err := credentialprovider.MakeDockerKeyring(s.secrets, emptyKeyring)
	if err != nil {
		util.HandleError(fmt.Errorf("unable to create Docker registry pull credentials for secrets: %v", err))
		keyring = emptyKeyring
	}
	s.keyring = keyring
	return keyring
}

func basicCredentialsFromKeyring(keyring credentialprovider.DockerKeyring, url *url.URL) (string, string) {
	// TODO: compare this logic to Docker authConfig in v2 configuration
	value := url.Host + url.Path
	configs, found := keyring.Lookup(value)
	if !found || len(configs) == 0 {
		glog.V(5).Infof("Unable to find a secret to match %s (%s)", url, value)
		return "", ""
	}
	return configs[0].Username, configs[0].Password
}
