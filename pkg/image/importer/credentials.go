package importer

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/docker/distribution/registry/client/auth"

	"k8s.io/client-go/tools/cache"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	defaultRealmCacheTTL = time.Minute
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
	return "", ""
}

func (s *noopCredentialStore) RefreshToken(url *url.URL, service string) string {
	return ""
}

func (s *noopCredentialStore) SetRefreshToken(url *url.URL, service string, token string) {
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
	return basicCredentialsFromKeyring(s.DockerKeyring, url, nil)
}

func NewCredentialsForSecrets(secrets []kapiv1.Secret) *SecretCredentialStore {
	return &SecretCredentialStore{
		secrets:           secrets,
		refreshTokenStore: &refreshTokenStore{},
		realmStore:        cache.NewTTLStore(realmKeyFunc, defaultRealmCacheTTL),
	}
}

func NewLazyCredentialsForSecrets(secretsFn func() ([]kapiv1.Secret, error)) *SecretCredentialStore {
	return &SecretCredentialStore{
		secretsFn:         secretsFn,
		refreshTokenStore: &refreshTokenStore{},
		realmStore:        cache.NewTTLStore(realmKeyFunc, defaultRealmCacheTTL),
	}
}

type SecretCredentialStore struct {
	lock       sync.Mutex
	realmStore cache.Store
	secrets    []kapiv1.Secret
	secretsFn  func() ([]kapiv1.Secret, error)
	err        error
	keyring    credentialprovider.DockerKeyring

	*refreshTokenStore
}

func (s *SecretCredentialStore) Basic(url *url.URL) (string, string) {
	// the store holds realm entries, if the target URL matches one it means
	// we should auth against registry URL rather than realm one
	entry, exists, err := s.realmStore.GetByKey(url.String())
	if exists && err == nil {
		if ru, ok := entry.(*realmURL); ok {
			return basicCredentialsFromKeyring(s.init(), url, &ru.registries)
		}
	}

	return basicCredentialsFromKeyring(s.init(), url, nil)
}

func (s *SecretCredentialStore) Err() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.err
}

func (s *SecretCredentialStore) AddRealm(registry, realm *url.URL) {
	entry, exists, err := s.realmStore.GetByKey(realm.String())
	if !exists {
		ru := &realmURL{
			realm:    *realm,
			registry: {*registry},
		}
		s.realmStore.Add(ru)
	}
	if exists && err == nil {
		if ru, ok := entry.(*realmURL); ok {
			found := false
			for _, reg := range ru.registries {
				if reg.String() == registry.String() {
					found = true
				}
			}
			if !found {
				s.lock.Lock()
				defer s.lock.Unlock()
				ru.registries = append(ru.registries, *registry)
				s.realmStore.Update(ru)
			}
		}
	}
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

func basicCredentialsFromKeyring(keyring credentialprovider.DockerKeyring, target *url.URL, nonRealmURL *url.URL) (string, string) {
	// TODO: compare this logic to Docker authConfig in v2 configuration
	var value string
	if len(target.Scheme) == 0 || target.Scheme == "https" {
		value = target.Host + target.Path
	} else {
		// only lookup credential for http that say they are for http
		value = target.String()
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
			glog.V(5).Infof("Being asked for %s, trying %s for legacy behavior", target, "index.docker.io/v1")
			return basicCredentialsFromKeyring(keyring, &url.URL{Host: "index.docker.io", Path: "/v1"}, nil)
		}
		// docker 1.9 saves 'docker.io' in config in f23, see https://bugzilla.redhat.com/show_bug.cgi?id=1309739
		if value == "index.docker.io" {
			glog.V(5).Infof("Being asked for %s, trying %s for legacy behavior", target, "docker.io")
			return basicCredentialsFromKeyring(keyring, &url.URL{Host: "docker.io"}, nil)
		}
		if nonRealmURL != nil {
			glog.V(5).Infof("Trying non realm url %s for target %s", nonRealmURL, target)
			return basicCredentialsFromKeyring(keyring, nonRealmURL, nil)
		}

		// try removing the canonical ports for the given requests
		if (strings.HasSuffix(target.Host, ":443") && target.Scheme == "https") ||
			(strings.HasSuffix(target.Host, ":80") && target.Scheme == "http") {
			host := strings.SplitN(target.Host, ":", 2)[0]
			glog.V(5).Infof("Being asked for %s, trying %s without port", target, host)

			return basicCredentialsFromKeyring(keyring, &url.URL{Scheme: target.Scheme, Host: host, Path: target.Path}, nil)
		}

		glog.V(5).Infof("Unable to find a secret to match %s (%s)", target, value)
		return "", ""
	}
	glog.V(5).Infof("Found secret to match %s (%s): %s", target, value, configs[0].ServerAddress)
	return configs[0].Username, configs[0].Password
}

// realmURL is a container associating a realm URL with an actual registry URL
type realmURL struct {
	realm      url.URL
	registries []url.URL
}

// realmKeyFunc returns an actual registry URL for given realm URL
func realmKeyFunc(obj interface{}) (string, error) {
	if key, ok := obj.(cache.ExplicitKey); ok {
		return string(key), nil
	}
	ru, ok := obj.(*realmURL)
	if !ok {
		return "", fmt.Errorf("object %T is not a realmURL object", obj)
	}
	return ru.realm.String(), nil
}
