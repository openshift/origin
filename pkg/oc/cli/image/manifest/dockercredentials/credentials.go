package dockercredentials

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/distribution/registry/client/auth"

	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/credentialprovider"

	"github.com/openshift/library-go/pkg/image/registryclient"
)

var (
	emptyKeyring = &credentialprovider.BasicDockerKeyring{}
)

// NewLocal creates a new credential store that uses the default
// local configuration to find a valid authentication for registry
// targets.
func NewLocal() auth.CredentialStore {
	keyring := &credentialprovider.BasicDockerKeyring{}
	keyring.Add(defaultClientDockerConfig())
	return &keyringCredentialStore{
		DockerKeyring:     keyring,
		RefreshTokenStore: registryclient.NewRefreshTokenStore(),
	}
}

// NewFromFile creates a new credential store for the provided Docker config.json
// authentication file.
func NewFromFile(path string) (auth.CredentialStore, error) {
	cfg, err := credentialprovider.ReadSpecificDockerConfigJsonFile(path)
	if err != nil {
		return nil, err
	}
	keyring := &credentialprovider.BasicDockerKeyring{}
	keyring.Add(cfg)
	return &keyringCredentialStore{
		DockerKeyring:     keyring,
		RefreshTokenStore: registryclient.NewRefreshTokenStore(),
	}, nil
}

type keyringCredentialStore struct {
	credentialprovider.DockerKeyring
	registryclient.RefreshTokenStore
}

func (s *keyringCredentialStore) Basic(url *url.URL) (string, string) {
	return BasicFromKeyring(s.DockerKeyring, url)
}

// BasicFromKeyring finds Basic authorization credentials from a Docker keyring for the given URL as username and
// password. It returns empty strings if no such URL matches.
func BasicFromKeyring(keyring credentialprovider.DockerKeyring, target *url.URL) (string, string) {
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
			klog.V(5).Infof("Being asked for %s (%s), trying %s for legacy behavior", target, value, "index.docker.io/v1")
			return BasicFromKeyring(keyring, &url.URL{Host: "index.docker.io", Path: "/v1"})
		}
		// docker 1.9 saves 'docker.io' in config in f23, see https://bugzilla.redhat.com/show_bug.cgi?id=1309739
		if value == "index.docker.io" {
			klog.V(5).Infof("Being asked for %s (%s), trying %s for legacy behavior", target, value, "docker.io")
			return BasicFromKeyring(keyring, &url.URL{Host: "docker.io"})
		}

		// try removing the canonical ports for the given requests
		if (strings.HasSuffix(target.Host, ":443") && target.Scheme == "https") ||
			(strings.HasSuffix(target.Host, ":80") && target.Scheme == "http") {
			host := strings.SplitN(target.Host, ":", 2)[0]
			klog.V(5).Infof("Being asked for %s (%s), trying %s without port", target, value, host)

			return BasicFromKeyring(keyring, &url.URL{Scheme: target.Scheme, Host: host, Path: target.Path})
		}

		klog.V(5).Infof("Unable to find a secret to match %s (%s)", target, value)
		return "", ""
	}
	klog.V(5).Infof("Found secret to match %s (%s): %s", target, value, configs[0].ServerAddress)
	return configs[0].Username, configs[0].Password
}

// defaultClientDockerConfig returns the credentials that the docker command line client would
// return.
func defaultClientDockerConfig() credentialprovider.DockerConfig {
	// support the modern config file $HOME/.docker/config.json
	if cfg, err := credentialprovider.ReadDockerConfigJSONFile(defaultPathsForCredentials()); err == nil {
		return cfg
	}
	// support the legacy config file $HOME/.dockercfg
	if cfg, err := credentialprovider.ReadDockercfgFile(defaultPathsForLegacyCredentials()); err == nil {
		return cfg
	}
	return credentialprovider.DockerConfig{}
}

// defaultPathsForCredentials returns the correct search directories for a docker config
//  file
func defaultPathsForCredentials() []string {
	if runtime.GOOS == "windows" { // Windows
		return []string{filepath.Join(os.Getenv("USERPROFILE"), ".docker")}
	}
	return []string{filepath.Join(os.Getenv("HOME"), ".docker")}
}

// defaultPathsForCredentials returns the correct search directories for a docker config
//  file
func defaultPathsForLegacyCredentials() []string {
	if runtime.GOOS == "windows" { // Windows
		return []string{os.Getenv("USERPROFILE")}
	}
	return []string{os.Getenv("HOME")}
}
