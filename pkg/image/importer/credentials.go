package importer

import (
	"net/url"
	"strings"
	"sync"

	"github.com/golang/glog"

	"github.com/docker/distribution/registry/client/auth"

	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

var (
	NoCredentials auth.CredentialStore = &noopCredentialStore{}

	emptyKeyring = &credentialprovider.BasicDockerKeyring{}
)

type refreshTokenKey struct {
	url     string
	service string
}

type refreshTokenStore struct {
	lock  sync.Mutex
	store map[refreshTokenKey]string
}

func (s *refreshTokenStore) RefreshToken(url *url.URL, service string) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.store[refreshTokenKey{url: url.String(), service: service}]
}

func (s *refreshTokenStore) SetRefreshToken(url *url.URL, service string, token string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.store == nil {
		s.store = make(map[refreshTokenKey]string)
	}
	s.store[refreshTokenKey{url: url.String(), service: service}] = token
}

type noopCredentialStore struct{}

func (s *noopCredentialStore) Basic(url *url.URL) (string, string) {
	glog.Infof("asked to provide Basic credentials for %s", url)
	return "", ""
}

func (s *noopCredentialStore) RefreshToken(url *url.URL, service string) string {
	glog.Infof("asked to provide RefreshToken for %s", url)
	return ""
}

func (s *noopCredentialStore) SetRefreshToken(url *url.URL, service string, token string) {
	glog.Infof("asked to provide SetRefreshToken for %s", url)
}

func NewBasicCredentials() *BasicCredentials {
	return &BasicCredentials{refreshTokenStore: &refreshTokenStore{}}
}

type basicForURL struct {
	url                url.URL
	username, password string
}

type BasicCredentials struct {
	creds []basicForURL
	*refreshTokenStore
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
	return &keyringCredentialStore{
		DockerKeyring:     credentialprovider.NewDockerKeyring(),
		refreshTokenStore: &refreshTokenStore{},
	}
}

type keyringCredentialStore struct {
	credentialprovider.DockerKeyring
	*refreshTokenStore
}

func (s *keyringCredentialStore) Basic(url *url.URL) (string, string) {
	return basicCredentialsFromKeyring(s.DockerKeyring, url)
}

func NewCredentialsForSecrets(secrets []kapiv1.Secret) *SecretCredentialStore {
	return &SecretCredentialStore{
		secrets:           secrets,
		refreshTokenStore: &refreshTokenStore{},
	}
}

func NewLazyCredentialsForSecrets(secretsFn func() ([]kapiv1.Secret, error)) *SecretCredentialStore {
	return &SecretCredentialStore{
		secretsFn:         secretsFn,
		refreshTokenStore: &refreshTokenStore{},
	}
}

type SecretCredentialStore struct {
	lock      sync.Mutex
	secrets   []kapiv1.Secret
	secretsFn func() ([]kapiv1.Secret, error)
	err       error
	keyring   credentialprovider.DockerKeyring

	*refreshTokenStore
}

func (s *SecretCredentialStore) Basic(url *url.URL) (string, string) {
	return basicCredentialsFromKeyring(s.init(), url)
}

func (s *SecretCredentialStore) Err() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.err
}

func (s *SecretCredentialStore) init() credentialprovider.DockerKeyring {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.keyring != nil {
		return s.keyring
	}

	// lazily load the secrets
	if s.secrets == nil {
		if s.secretsFn != nil {
			s.secrets, s.err = s.secretsFn()
		}
	}

	// TODO: need a version of this that is best effort secret - otherwise one error blocks all secrets
	keyring, err := credentialprovider.MakeDockerKeyring(s.secrets, emptyKeyring)
	if err != nil {
		glog.V(5).Infof("Loading keyring failed for credential store: %v", err)
		s.err = err
		keyring = emptyKeyring
	}
	s.keyring = keyring
	return keyring
}

func basicCredentialsFromKeyring(keyring credentialprovider.DockerKeyring, target *url.URL) (string, string) {
	// TODO: compare this logic to Docker authConfig in v2 configuration
	var value string
	if len(target.Scheme) == 0 || target.Scheme == "https" {
		value = target.Host + target.Path
	} else {
		// always require an explicit port to look up HTTP credentials
		if !strings.Contains(target.Host, ":") {
			value = target.Host + ":80" + target.Path
		} else {
			value = target.Host + target.Path
		}
	}

	// Lookup(...) expects an image (not a URL path).
	// The keyring strips /v1/ and /v2/ version prefixes,
	// so we should also when selecting a valid auth for a URL.
	pathWithSlash := target.Path + "/"
	if strings.HasPrefix(pathWithSlash, "/v1/") || strings.HasPrefix(pathWithSlash, "/v2/") {
		value = target.Host + target.Path[3:]
	}

	configs, found := keyring.Lookup(value)

	if !found || len(configs) == 0 {
		// do a special case check for docker.io to match historical lookups when we respond to a challenge
		if value == "auth.docker.io/token" {
			glog.V(5).Infof("Being asked for %s (%s), trying %s for legacy behavior", target, value, "index.docker.io/v1")
			return basicCredentialsFromKeyring(keyring, &url.URL{Host: "index.docker.io", Path: "/v1"})
		}
		// docker 1.9 saves 'docker.io' in config in f23, see https://bugzilla.redhat.com/show_bug.cgi?id=1309739
		if value == "index.docker.io" {
			glog.V(5).Infof("Being asked for %s (%s), trying %s for legacy behavior", target, value, "docker.io")
			return basicCredentialsFromKeyring(keyring, &url.URL{Host: "docker.io"})
		}

		// try removing the canonical ports for the given requests
		if (strings.HasSuffix(target.Host, ":443") && target.Scheme == "https") ||
			(strings.HasSuffix(target.Host, ":80") && target.Scheme == "http") {
			host := strings.SplitN(target.Host, ":", 2)[0]
			glog.V(5).Infof("Being asked for %s (%s), trying %s without port", target, value, host)

			return basicCredentialsFromKeyring(keyring, &url.URL{Scheme: target.Scheme, Host: host, Path: target.Path})
		}

		glog.V(5).Infof("Unable to find a secret to match %s (%s)", target, value)
		return "", ""
	}
	glog.V(5).Infof("Found secret to match %s (%s): %s", target, value, configs[0].ServerAddress)
	return configs[0].Username, configs[0].Password
}
