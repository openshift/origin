package registryhostname

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// RegistryHostnameRetriever represents an interface for retrieving the hostname
// of internal and external registry.
type RegistryHostnameRetriever interface {
	InternalRegistryHostname() (string, bool)
	ExternalRegistryHostname() (string, bool)
}

// DefaultRegistryHostnameRetriever is a default implementation of
// RegistryHostnameRetriever.
// The first argument is a function that lazy-loads the value of
// OPENSHIFT_DEFAULT_REGISTRY environment variable which should be deprecated in
// future.
func TestingRegistryHostnameRetriever(deprecatedDefaultRegistryEnvFn func() (string, bool), external, internal string) RegistryHostnameRetriever {
	return &defaultRegistryHostnameRetriever{
		deprecatedDefaultFn: deprecatedDefaultRegistryEnvFn,
		externalHostname:    external,
		internalHostname:    internal,
	}
}

func DefaultRegistryHostnameRetriever(clientConfig *rest.Config, external, internal string) (RegistryHostnameRetriever, error) {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	defaultRegistry := env("OPENSHIFT_DEFAULT_REGISTRY", "${DOCKER_REGISTRY_SERVICE_HOST}:${DOCKER_REGISTRY_SERVICE_PORT}")
	svcCache := newServiceResolverCache(kubeClient.CoreV1().Services(metav1.NamespaceDefault).Get)
	defaultRegistryFunc, err := svcCache.Defer(defaultRegistry)
	if err != nil {
		return nil, fmt.Errorf("OPENSHIFT_DEFAULT_REGISTRY variable is invalid %q: %v", defaultRegistry, err)
	}

	return &defaultRegistryHostnameRetriever{
		deprecatedDefaultFn: defaultRegistryFunc,
		externalHostname:    external,
		internalHostname:    internal,
	}, nil
}

// env returns an environment variable, or the defaultValue if it is not set.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

type defaultRegistryHostnameRetriever struct {
	// deprecatedDefaultFn points to a function that will lazy-load the value of
	// OPENSHIFT_DEFAULT_REGISTRY.
	deprecatedDefaultFn func() (string, bool)
	internalHostname    string
	externalHostname    string
}

// InternalRegistryHostnameFn returns a function that can be used to lazy-load
// the internal Docker Registry hostname. If the master configuration properly
// InternalRegistryHostname is set, it will prefer that over the lazy-loaded
// environment variable 'OPENSHIFT_DEFAULT_REGISTRY'.
func (r *defaultRegistryHostnameRetriever) InternalRegistryHostname() (string, bool) {
	if len(r.internalHostname) > 0 {
		return r.internalHostname, true
	}
	if r.deprecatedDefaultFn != nil {
		return r.deprecatedDefaultFn()
	}
	return "", false
}

// ExternalRegistryHostnameFn returns a function that can be used to retrieve an
// external/public hostname of Docker Registry. External location can be
// configured in master config using 'ExternalRegistryHostname' property.
func (r *defaultRegistryHostnameRetriever) ExternalRegistryHostname() (string, bool) {
	return r.externalHostname, len(r.externalHostname) > 0
}
