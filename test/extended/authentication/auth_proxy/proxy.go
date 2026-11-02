package auth_proxy

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"

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

	upstreamCAFile       = "upstream-ca-bundle.crt"
	upstreamCACertFolder = "/etc/squid/upstream-ca/"
	upstreamCAConfigMap  = "upstream-ca-config"

	caCertConfigMapKey = "ca-bundle.crt"
)

var (
	upstreamCACertPath = path.Join(upstreamCACertFolder, upstreamCAFile)
)

// DefaultNoProxy returns a new set of the default OAuth component proxy exclusions.
// The caller can modify the set without affecting subsequent calls.
func DefaultNoProxy() sets.Set[string] {
	return sets.New(
		".cluster.local",
		".svc",
		"127.0.0.1",
		"localhost",
	)
}

// ComponentProxyTestLabels returns a new map of labels for authentication proxy
// test resources. The caller can add or remove labels.
func ComponentProxyTestLabels() map[string]string {
	return map[string]string{
		"e2e-test": "openshift-authentication-operator",
	}
}

// ProxyConfig holds proxy endpoints, bypass entries, and an optional upstream CA
// bundle. Nil URLs mean no proxy for the corresponding destination protocol.
// The URL scheme specifies the transport to the proxy, independently of that protocol.
// Copying a ProxyConfig shares its URLs, NoProxy set, and CA bytes.
type ProxyConfig struct {
	HttpProxy, HttpsProxy *url.URL
	NoProxy               sets.Set[string]
	UpstreamCACertPEM     []byte
}

// EmptyProxyConfig returns settings without proxies or a CA, with an initialized
// NoProxy set that the caller can insert into. A zero ProxyConfig is safe to read.
func EmptyProxyConfig() ProxyConfig {
	return ProxyConfig{
		HttpProxy:         nil,
		HttpsProxy:        nil,
		NoProxy:           sets.New[string](),
		UpstreamCACertPEM: []byte{},
	}
}

// ClusterProxy reads endpoints and bypass entries from proxy/cluster status.
// If spec.trustedCA is set, it reads ca-bundle.crt from the named ConfigMap in
// openshift-config. The bundle must be non-empty; PEM contents are not validated.
// proxies is required; configMaps is required only when a CA must be read.
// Discard the returned configuration on error; it may contain partial results.
func ClusterProxy(
	ctx context.Context,
	proxies configclientv1.ProxyInterface,
	configMaps coreclientv1.ConfigMapsGetter,
) (ProxyConfig, error) {
	var toReturn ProxyConfig
	if err := ctx.Err(); err != nil {
		return toReturn, err
	}
	if proxies == nil {
		return toReturn, fmt.Errorf("proxies client must not be nil")
	}
	proxy, err := proxies.Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return toReturn, fmt.Errorf("fetching cluster proxy configuration: %w", err)
	}

	toReturn.HttpProxy, err = parseProxyField(proxy.Status.HTTPProxy)
	if err != nil {
		return toReturn, fmt.Errorf("error parsing http proxy field from cluster wide proxy configuration: %w", err)
	}
	toReturn.HttpsProxy, err = parseProxyField(proxy.Status.HTTPSProxy)
	if err != nil {
		return toReturn, fmt.Errorf("error parsing https proxy field from cluster wide proxy configuration: %w", err)
	}
	toReturn.NoProxy = sets.New[string]()
	for v := range strings.SplitSeq(proxy.Status.NoProxy, ",") {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {

			toReturn.NoProxy.Insert(trimmed)
		}
	}

	if proxy.Spec.TrustedCA.Name != "" {
		if configMaps == nil {
			return toReturn, fmt.Errorf("configMaps client is required to read the cluster proxy CA")
		}
		cm, err := configMaps.ConfigMaps("openshift-config").
			Get(ctx, proxy.Spec.TrustedCA.Name, metav1.GetOptions{})
		if err != nil {
			return toReturn, fmt.Errorf("fetching cluster proxy CA certificate config map: %w", err)
		}

		bundle := cm.Data[caCertConfigMapKey]
		if strings.TrimSpace(bundle) == "" {
			return toReturn, fmt.Errorf("ConfigMap %s/%s must contain non-empty %q", cm.Namespace, cm.Name, caCertConfigMapKey)
		}
		toReturn.UpstreamCACertPEM = []byte(bundle)
	}
	return toReturn, nil
}

// DeploymentProxyConfig reads literal HTTP_PROXY, HTTPS_PROXY, and NO_PROXY values
// from the named container in a deployment. ValueFrom and EnvFrom are not resolved.
// namespace, name, containerName, and deployments are required.
//
// An empty CA selector disables CA lookup; otherwise both Name and Key are required.
// The CA is read only when a deployment volume references the selected ConfigMap;
// a mount in the selected container is not checked. configMaps is required only for
// that lookup. The selected key must contain a non-empty bundle; PEM is not validated.
// Optional is ignored: a missing selected ConfigMap or key is always an error.
// On error, the returned configuration is zero.
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

	// A cluster without a global proxy does not add these environment variables.
	// Start with that baseline, then replace only the values present on the
	// existing deployment.
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
			config.UpstreamCACertPEM = []byte(bundle)
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

// String describes proxy endpoints and sorted bypass entries without URL
// credentials or CA contents. It does not modify the configuration.
func (pC ProxyConfig) String() string {
	noProxy := strings.Join(sets.List(pC.NoProxy), ",")

	return fmt.Sprintf("ProxyConfig[HttpProxy=%q HttpsProxy=%q NoProxy=%q]",
		stripCredentials(pC.HttpProxy),
		stripCredentials(pC.HttpsProxy),
		noProxy,
	)
}

// HasProxy reports whether either proxy URL is non-nil. It does not validate URLs.
func (pC ProxyConfig) HasProxy() bool {
	return pC.HttpProxy != nil || pC.HttpsProxy != nil
}

// Matches compares URL strings, including credentials, and ignores CA bundles.
// Non-empty expected bypass entries must be a subset of actual entries; an empty
// expected set requires an empty actual set. The comparison can be asymmetric.
func (actual ProxyConfig) Matches(expected ProxyConfig) bool {
	return proxyURLsEqual(expected.HttpProxy, actual.HttpProxy) &&
		proxyURLsEqual(expected.HttpsProxy, actual.HttpsProxy) &&
		actual.NoProxy.IsSuperset(expected.NoProxy) &&
		(expected.NoProxy.Len() > 0 || actual.NoProxy.Len() == 0)
}

// stripCredentials formats a copy of proxy without user information; nil yields "".
func stripCredentials(proxy *url.URL) string {
	if proxy == nil {
		return ""
	}
	clean := *proxy
	clean.User = nil
	return clean.String()
}

// parseProxyField accepts an absolute HTTP or HTTPS proxy URL with a hostname and
// an optional port in the range 1..65535. An empty field yields nil without error.
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

// proxyURLsEqual compares URL strings without userinfo, then compares serialized
// userinfo with ConstantTimeCompare. Only the byte comparison is constant-time
// for equal lengths. Absent and empty userinfo or passwords remain distinct.
// Two nil URLs are equal. Neither input is modified.
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
