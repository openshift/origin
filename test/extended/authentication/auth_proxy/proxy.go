package auth_proxy

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	squidImage       = "registry.redhat.io/rhel10/squid:10.2-1784702318"
	squidHTTPPort    = int32(3128)
	squidHTTPSPort   = int32(3129)
	squidServiceName = "auth-proxy"
)

// DefaultNoProxy returns a fresh set so callers can mutate it without affecting
// other tests.
func DefaultNoProxy() sets.Set[string] {
	return sets.New(
		".cluster.local",
		".svc",
		"127.0.0.1",
		"localhost",
	)
}

// ComponentProxyTestLabels returns a fresh map so callers can mutate it without
// affecting other tests.
func ComponentProxyTestLabels() map[string]string {
	return map[string]string{
		"e2e-test": "openshift-authentication-operator",
	}
}

// ProxyConfig separates the transport to the proxy (the URL scheme) from the protocol
// it serves (the field name). Copies share URLs, NoProxy, and CA bytes.
type ProxyConfig struct {
	HttpProxy, HttpsProxy *url.URL
	NoProxy               sets.Set[string]
	CACertPEM             []byte
}

// EmptyProxyConfig returns an initialized NoProxy set for callers to fill in; a zero
// ProxyConfig is also safe to read.
func EmptyProxyConfig() ProxyConfig {
	return ProxyConfig{
		HttpProxy:  nil,
		HttpsProxy: nil,
		NoProxy:    sets.New[string](),
		CACertPEM:  []byte{},
	}
}

// DeploymentProxyConfig reads literal values only: ValueFrom and EnvFrom are not
// resolved, so a deployment that injects proxies indirectly reads as empty. The CA is
// read only when a volume references the ConfigMap, since a container mount alone does
// not prove the deployment consumes it. Optional is ignored, so a missing ConfigMap or
// key is always an error. On error the configuration is zero.
func DeploymentProxyConfig(
	ctx context.Context,
	namespace, name, containerName string,
	caConfigMap v1.ConfigMapKeySelector,
	deployments appsclientv1.DeploymentsGetter,
	configMaps coreclientv1.ConfigMapsGetter,
) (ProxyConfig, error) {
	var zero ProxyConfig
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if deployments == nil {
		return zero, fmt.Errorf("deployments client must not be nil")
	}
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(containerName) == "" {
		return zero, fmt.Errorf("namespace, deployment name, and container name must be non-empty")
	}
	if caConfigMap.Name != "" || caConfigMap.Key != "" {
		if strings.TrimSpace(caConfigMap.Name) == "" || strings.TrimSpace(caConfigMap.Key) == "" {
			return zero, fmt.Errorf("CA ConfigMap selector must provide both name and key, or neither")
		}
	}

	deployment, err := deployments.
		Deployments(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return zero, err
	}

	// A cluster without a global proxy omits these variables, so the zero configuration
	// is the correct baseline and only values the deployment sets replace it.
	config := EmptyProxyConfig()
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if caConfigMap.Name != "" && volume.ConfigMap != nil && volume.ConfigMap.Name == caConfigMap.Name {
			if configMaps == nil {
				return zero, fmt.Errorf("configMaps client is required to read the deployment proxy CA")
			}
			cm, err := configMaps.ConfigMaps(namespace).
				Get(ctx, caConfigMap.Name, metav1.GetOptions{})
			if err != nil {
				return zero, fmt.Errorf("error getting %s/%s proxy CA certificate: %w", namespace, name, err)
			}

			bundle := cm.Data[caConfigMap.Key]
			if strings.TrimSpace(bundle) == "" {
				return zero, fmt.Errorf("ConfigMap %s/%s must contain non-empty %q", cm.Namespace, cm.Name, caConfigMap.Key)
			}
			config.CACertPEM = []byte(bundle)
			break
		}
	}

	var proxyContainer *v1.Container
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			proxyContainer = &container
			break
		}
	}

	if proxyContainer == nil {
		return zero, fmt.Errorf("container %q not found in deployment %s/%s", containerName, namespace, name)
	}

	for _, env := range proxyContainer.Env {
		switch env.Name {
		case "HTTP_PROXY":
			httpProxy, err := parseProxyField(env.Value)
			if err != nil {
				return zero, fmt.Errorf("error parsing http proxy field for container %q: %w", containerName, err)
			}
			config.HttpProxy = httpProxy
		case "HTTPS_PROXY":
			httpsProxy, err := parseProxyField(env.Value)
			if err != nil {
				return zero, fmt.Errorf("error parsing https proxy field for container %q: %w", containerName, err)
			}
			config.HttpsProxy = httpsProxy
		case "NO_PROXY":
			for v := range strings.SplitSeq(env.Value, ",") {
				trimmed := strings.TrimSpace(v)
				if trimmed != "" {
					config.NoProxy.Insert(trimmed)
				}
			}
		}
	}
	return config, nil
}

// String omits credentials and CA contents, so the value is safe to log.
func (pC ProxyConfig) String() string {
	noProxy := strings.Join(sets.List(pC.NoProxy), ",")

	return fmt.Sprintf("ProxyConfig[HttpProxy=%q HttpsProxy=%q NoProxy=%q]",
		stripCredentials(pC.HttpProxy),
		stripCredentials(pC.HttpsProxy),
		noProxy,
	)
}

// HasProxy does not validate URLs, so a malformed proxy still reports true.
func (pC ProxyConfig) HasProxy() bool {
	return pC.HttpProxy != nil || pC.HttpsProxy != nil
}

// Matches ignores CA bundles and requires expected bypass entries to be a subset of
// the actual ones, so the comparison is asymmetric.
func (actual ProxyConfig) Matches(expected ProxyConfig) bool {
	return proxyURLsEqual(expected.HttpProxy, actual.HttpProxy) &&
		proxyURLsEqual(expected.HttpsProxy, actual.HttpsProxy) &&
		actual.NoProxy.IsSuperset(expected.NoProxy) &&
		(expected.NoProxy.Len() > 0 || actual.NoProxy.Len() == 0)
}

// stripCredentials strips the User from the URL.
// A nil proxy yields an empty string.
func stripCredentials(proxy *url.URL) string {
	if proxy == nil {
		return ""
	}
	clean := *proxy
	clean.User = nil
	return clean.String()
}

// parseProxyField rejects a port outside 1..65535 and yields nil for an empty field,
// which disables that proxy.
func parseProxyField(field string) (*url.URL, error) {
	if field == "" {
		return nil, nil
	}

	proxyURL, err := url.Parse(field)
	if err != nil {
		reason := "invalid syntax"
		if _, ok := errors.AsType[url.EscapeError](err); ok {
			reason = "invalid percent-encoding"
		}

		if _, ok := errors.AsType[url.InvalidHostError](err); ok {
			reason = "invalid hostname"
		}
		return nil, fmt.Errorf("cannot parse proxy URL: %s", reason)
	}

	if (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") || proxyURL.Hostname() == "" {
		return nil, fmt.Errorf("proxy URL must use HTTP or HTTPS and include a hostname")
	}
	port := proxyURL.Port()
	if port != "" {
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portNumber == 0 {
			return nil, fmt.Errorf("proxy URL port must be in the range 1..65535")
		}
	} else if strings.HasSuffix(proxyURL.Host, ":") {
		return nil, fmt.Errorf("proxy URL has an empty port")
	}
	return proxyURL, nil
}

// proxyURLsEqual compares the two URLs.
// Only the byte comparison for equal-length userinfo is constant-time, and absent and
// empty userinfo stay distinct so a missing password cannot match an empty one.
func proxyURLsEqual(a, b *url.URL) bool {
	if a == nil || b == nil {
		return a == b
	}
	publicA, publicB := stripCredentials(a), stripCredentials(b)
	if publicA != publicB {
		return false
	}
	if a.User == nil || b.User == nil {
		return a.User == b.User
	}
	return subtle.ConstantTimeCompare([]byte(a.User.String()), []byte(b.User.String())) == 1
}
