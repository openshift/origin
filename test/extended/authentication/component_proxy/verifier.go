package component_proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	ComponentProxyCAConfigMapName = "v4-0-config-system-auth-proxy-ca"

	// DefaultNoProxy is the noProxy entry set the finished enhancement mandates for the
	// authentication components. Assertions compare with superset semantics, so it must
	// stay a subset of what the operator emits. cluster-authentication-operator does not
	// emit the trailing-dot forms yet, so the oauth-apiserver assertions stay red until
	// it does; console-operator already emits them. Red is wanted here, the enhancement
	// demands this set.
	DefaultNoProxy = ".cluster.local,.cluster.local.,.svc,.svc.,127.0.0.1,localhost,localhost."

	defaultPollingInterval = 10 * time.Second
	defaultPollingTimeout  = 5 * time.Minute
)

// ProxyVerifierConfigurationOption customizes a ProxyVerifierConfiguration at
// construction time (functional-options pattern).
type ProxyVerifierConfigurationOption func(*ProxyVerifierConfiguration) error

// WithProxyVerifierPollingInterval overrides the polling interval of the verifier's
// assertions. Non-positive values fall back to the package default.
func WithProxyVerifierPollingInterval(interval time.Duration) ProxyVerifierConfigurationOption {
	return func(pvc *ProxyVerifierConfiguration) error {
		pvc.pollingInterval = interval
		return nil
	}
}

// WithProxyVerifierPollingTimeout overrides the polling timeout of the verifier's
// assertions. Non-positive values fall back to the package default.
func WithProxyVerifierPollingTimeout(timeout time.Duration) ProxyVerifierConfigurationOption {
	return func(pvc *ProxyVerifierConfiguration) error {
		pvc.pollingTimeout = timeout
		return nil
	}
}

// WithConfigFileTarget sets the config-file target the ConfigFileProxyVerifier methods
// assert against: the ConfigMap holding the generated configuration file, the file name
// within it, and the trustedCA file path the rendered auth.proxy block must reference
// when trustedCA is configured.
func WithConfigFileTarget(configMapNamespace, configMapName, configFileName, trustedCAFilePath string) ProxyVerifierConfigurationOption {
	return func(pvc *ProxyVerifierConfiguration) error {
		pvc.configMapNamespace = configMapNamespace
		pvc.configMapName = configMapName
		pvc.configFileName = configFileName
		pvc.trustedCAFilePath = trustedCAFilePath
		return nil
	}
}

// DeploymentProxyVerifier asserts proxy propagation through a Deployment: proxy env
// vars plus the trustedCA volume/mount. This is how cluster-authentication-operator
// propagates the proxy to the oauth-apiserver operand.
type DeploymentProxyVerifier interface {
	// AssertProxyConfig polls the target deployment until its HTTP_PROXY, HTTPS_PROXY,
	// and NO_PROXY env vars and trustedCA volume/mount state match the expected values.
	// NO_PROXY is compared with superset semantics because the operator appends entries
	// (e.g. the apiserver IP) beyond what is configured. Presence of trustedCA requires
	// both the volume and a mount; absence requires neither.
	AssertProxyConfig(ctx context.Context, expectedHTTPProxy, expectedHTTPSProxy, expectedNoProxy string, expectPresenceForTrustedCAVolume bool) error
	// AssertNoConfig asserts the target deployment has no proxy env vars and no
	// trustedCA volume/mount configured.
	AssertNoConfig(ctx context.Context) error
	// AssertDeploymentCAMount asserts the target deployment mounts the proxy CA
	// ConfigMap exactly when expected, without asserting anything about env vars. It
	// serves operands that carry the proxy in a configuration file but still mount the
	// CA on the Deployment.
	AssertDeploymentCAMount(ctx context.Context, expectPresent bool) error
}

// ConfigFileProxyVerifier asserts proxy propagation through a rendered configuration
// file: the auth.proxy block an operator writes into a generated config file. This is
// how console-operator propagates the proxy to the console operand.
type ConfigFileProxyVerifier interface {
	// AssertProxyConfigInFile polls the target configuration file until its auth.proxy
	// block matches the expected proxy URLs, contains the expected noProxy entries
	// (superset semantics, because the operator appends internal defaults), and
	// references the trustedCA file path exactly when expectTrustedCA is set.
	AssertProxyConfigInFile(ctx context.Context, expectedHTTPProxy, expectedHTTPSProxy, expectedNoProxy string, expectTrustedCA bool) error
	// AssertNoConfigInFile asserts the target configuration file carries no auth.proxy
	// block at all.
	AssertNoConfigInFile(ctx context.Context) error

	AssertDeploymentCAMount(ctx context.Context, expectPresent bool) error
}

// ProxyVerifierConfiguration holds the shared verifier collaborators and the two
// assertion targets. It is only populated through the constructors and their options.
type ProxyVerifierConfiguration struct {
	pollingInterval, pollingTimeout time.Duration
	cli                             *exutil.CLI

	configMapNamespace, configMapName, configFileName, trustedCAFilePath string
}

// proxyVerifier verifies how an operator propagates a component proxy to its operand.
// It implements both verification ways over two independent targets, usable together or
// separately:
//
//   - the Deployment target (namespace, name): the operand Deployment whose proxy env
//     vars and trustedCA volume/mount the DeploymentProxyVerifier methods assert.
//   - the config-file target, set with WithConfigFileTarget: the ConfigMap holding the
//     generated configuration file whose auth.proxy block the ConfigFileProxyVerifier
//     methods assert, and the file path that block must reference when trustedCA is
//     configured.
type proxyVerifier struct {
	config ProxyVerifierConfiguration

	namespace, name string
}

var (
	_ DeploymentProxyVerifier = proxyVerifier{}
	_ ConfigFileProxyVerifier = proxyVerifier{}
)

func newProxyVerifier(
	namespace,
	name string,
	cli *exutil.CLI,
	opts ...ProxyVerifierConfigurationOption,
) (*proxyVerifier, error) {
	toReturn := proxyVerifier{
		namespace: namespace,
		name:      name,
		config: ProxyVerifierConfiguration{
			cli: cli,
		},
	}

	for _, anOpt := range opts {
		err := anOpt(&toReturn.config)
		if err != nil {
			return nil, fmt.Errorf("error setting option: %w", err)
		}
	}

	if toReturn.config.pollingInterval <= 0 {
		toReturn.config.pollingInterval = defaultPollingInterval
	}
	if toReturn.config.pollingTimeout <= 0 {
		toReturn.config.pollingTimeout = defaultPollingTimeout
	}
	if toReturn.config.cli == nil {
		return nil, errors.New("no cli") //TODO use the Err defined into pr 31837
	}

	return &toReturn, nil
}

// NewDeploymentProxyVerifier builds a verifier with only the Deployment target set.
func NewDeploymentProxyVerifier(
	namespace, name string,
	cli *exutil.CLI,
	opts ...ProxyVerifierConfigurationOption,
) (DeploymentProxyVerifier, error) {
	return newProxyVerifier(namespace, name, cli, opts...)
}

// NewConfigFileProxyVerifier builds a verifier carrying both targets: the Deployment
// target for the trustedCA volume/mount, and the config-file target for the rendered
// auth.proxy block, which must be set with WithConfigFileTarget.
func NewConfigFileProxyVerifier(
	namespace, name string,
	cli *exutil.CLI,
	opts ...ProxyVerifierConfigurationOption,
) (ConfigFileProxyVerifier, error) {
	return newProxyVerifier(namespace, name, cli, opts...)
}

// AssertProxyConfig implements DeploymentProxyVerifier; see its method docs for the
// matching contract.
func (pv proxyVerifier) AssertProxyConfig(ctx context.Context, expectedHTTPProxy, expectedHTTPSProxy, expectedNoProxy string, expectPresenceForTrustedCAVolume bool) error {
	kubeClient := pv.config.cli.AdminKubeClient()

	return wait.PollUntilContextTimeout(ctx, pv.config.pollingInterval, pv.config.pollingTimeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := kubeClient.AppsV1().
			Deployments(pv.namespace).
			Get(ctx, pv.name, metav1.GetOptions{})
		if err != nil {
			g.GinkgoWriter.Printf("failed to get %s/%s deployment: %v\n", pv.namespace, pv.name, err)
			return false, nil
		}

		envVars := make(map[string]string)
		for _, container := range deployment.Spec.Template.Spec.Containers {
			for _, env := range container.Env {
				switch env.Name {
				case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY":
					envVars[env.Name] = env.Value
				}
			}
		}

		if envVars["HTTP_PROXY"] != expectedHTTPProxy || envVars["HTTPS_PROXY"] != expectedHTTPSProxy {
			g.GinkgoWriter.Printf("proxy env mismatch for %s/%s deployment: HTTP_PROXY=%q (want %q), HTTPS_PROXY=%q (want %q)\n",
				pv.namespace, pv.name,
				envVars["HTTP_PROXY"], expectedHTTPProxy, envVars["HTTPS_PROXY"], expectedHTTPSProxy)
			return false, nil
		}

		if !noProxyMatches(envVars["NO_PROXY"], expectedNoProxy) {
			g.GinkgoWriter.Printf("proxy env mismatch for %s/%s deployment: NO_PROXY=%q does not contain all of %q\n",
				pv.namespace, pv.name,
				envVars["NO_PROXY"], expectedNoProxy)
			return false, nil
		}

		foundVolume, foundMount := trustedCAVolumeState(deployment)
		if !trustedCAStateMatches(expectPresenceForTrustedCAVolume, foundVolume, foundMount) {
			g.GinkgoWriter.Printf("trustedCA for %s/%s deployment: volume=%v mount=%v (want present=%v)\n",
				pv.namespace, pv.name,
				foundVolume, foundMount, expectPresenceForTrustedCAVolume)
			return false, nil
		}
		return true, nil
	})
}

// AssertNoConfig implements DeploymentProxyVerifier; see its method docs.
func (pv proxyVerifier) AssertNoConfig(ctx context.Context) error {
	return pv.AssertProxyConfig(ctx, "", "", "", false)
}

// AssertDeploymentCAMount implements the interface method; see its docs for the
// volume/mount contract.
func (pv proxyVerifier) AssertDeploymentCAMount(ctx context.Context, expectPresent bool) error {
	kubeClient := pv.config.cli.AdminKubeClient()

	return wait.PollUntilContextTimeout(ctx, pv.config.pollingInterval, pv.config.pollingTimeout, true, func(ctx context.Context) (bool, error) {
		deployment, err := kubeClient.AppsV1().
			Deployments(pv.namespace).
			Get(ctx, pv.name, metav1.GetOptions{})
		if err != nil {
			g.GinkgoWriter.Printf("failed to get %s/%s deployment: %v\n", pv.namespace, pv.name, err)
			return false, nil
		}

		foundVolume, foundMount := trustedCAVolumeState(deployment)
		if !trustedCAStateMatches(expectPresent, foundVolume, foundMount) {
			g.GinkgoWriter.Printf("trustedCA for %s/%s deployment: volume=%v mount=%v (want present=%v)\n",
				pv.namespace, pv.name,
				foundVolume, foundMount, expectPresent)
			return false, nil
		}
		return true, nil
	})
}

// AssertProxyConfigInFile implements ConfigFileProxyVerifier; see its method docs for
// the matching contract.
func (pv proxyVerifier) AssertProxyConfigInFile(ctx context.Context, expectedHTTPProxy, expectedHTTPSProxy, expectedNoProxy string, expectTrustedCA bool) error {
	return wait.PollUntilContextTimeout(ctx, pv.config.pollingInterval, pv.config.pollingTimeout, true, func(ctx context.Context) (bool, error) {
		cfg, err := pv.readConfigFile(ctx)
		if err != nil {
			g.GinkgoWriter.Printf("failed to read the target config file: %v\n", err)
			return false, nil
		}

		if cfg.Auth.Proxy == nil {
			g.GinkgoWriter.Printf("config file %s/%s has no auth.proxy block yet\n", pv.config.configMapNamespace, pv.config.configMapName)
			return false, nil
		}
		if cfg.Auth.Proxy.HTTPProxy != expectedHTTPProxy || cfg.Auth.Proxy.HTTPSProxy != expectedHTTPSProxy {
			g.GinkgoWriter.Printf("config file proxy mismatch: httpProxy=%q (want %q), httpsProxy=%q (want %q)\n",
				cfg.Auth.Proxy.HTTPProxy, expectedHTTPProxy, cfg.Auth.Proxy.HTTPSProxy, expectedHTTPSProxy)
			return false, nil
		}

		if !noProxyEntriesMatch(cfg.Auth.Proxy.NoProxy, expectedNoProxy) {
			g.GinkgoWriter.Printf("config file noProxy mismatch: %v does not contain all of %q\n",
				cfg.Auth.Proxy.NoProxy, expectedNoProxy)
			return false, nil
		}

		expectedCAPath := ""
		if expectTrustedCA {
			expectedCAPath = pv.config.trustedCAFilePath
		}
		if cfg.Auth.Proxy.TrustedCAFilePath != expectedCAPath {
			g.GinkgoWriter.Printf("config file trustedCAFilePath mismatch: %q (want %q)\n",
				cfg.Auth.Proxy.TrustedCAFilePath, expectedCAPath)
			return false, nil
		}
		return true, nil
	})
}

// AssertNoConfigInFile implements ConfigFileProxyVerifier; see its method docs.
func (pv proxyVerifier) AssertNoConfigInFile(ctx context.Context) error {
	return wait.PollUntilContextTimeout(ctx, pv.config.pollingInterval, pv.config.pollingTimeout, true, func(ctx context.Context) (bool, error) {
		cfg, err := pv.readConfigFile(ctx)
		if err != nil {
			g.GinkgoWriter.Printf("failed to read the target config file: %v\n", err)
			return false, nil
		}
		if cfg.Auth.Proxy != nil {
			g.GinkgoWriter.Printf("config file %s/%s still has an auth.proxy block: %+v\n", pv.config.configMapNamespace, pv.config.configMapName, cfg.Auth.Proxy)
			return false, nil
		}
		return true, nil
	})
}

// readConfigFile parses the generated configuration file held by the config-file target.
func (pv proxyVerifier) readConfigFile(ctx context.Context) (authProxyConfigFile, error) {
	var cfg authProxyConfigFile
	cm, err := pv.config.cli.AdminKubeClient().CoreV1().ConfigMaps(pv.config.configMapNamespace).Get(ctx, pv.config.configMapName, metav1.GetOptions{})
	if err != nil {
		return cfg, fmt.Errorf("getting %s/%s: %w", pv.config.configMapNamespace, pv.config.configMapName, err)
	}
	data, ok := cm.Data[pv.config.configFileName]
	if !ok {
		return cfg, fmt.Errorf("configmap %s/%s has no %s key", pv.config.configMapNamespace, pv.config.configMapName, pv.config.configFileName)
	}
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", pv.config.configFileName, err)
	}
	return cfg, nil
}

// authProxyConfigFile is the subset of a generated operand configuration file that
// carries the auth proxy block, in the console-config.yaml shape: an auth.proxy block
// with the proxy URLs, the resolved noProxy list, and the trustedCA file path.
type authProxyConfigFile struct {
	Auth struct {
		AuthType string `json:"authType"`
		Proxy    *struct {
			HTTPProxy         string   `json:"httpProxy,omitempty"`
			HTTPSProxy        string   `json:"httpsProxy,omitempty"`
			NoProxy           []string `json:"noProxy,omitempty"`
			TrustedCAFilePath string   `json:"trustedCAFilePath,omitempty"`
		} `json:"proxy,omitempty"`
	} `json:"auth"`
}

// noProxyMatches reports whether the comma-separated actual list contains every entry
// of the comma-separated expected list. An empty expectation matches only an empty
// actual, so stale entries fail an AssertNoConfig check.
func noProxyMatches(actual, expected string) bool {
	if expected == "" {
		return actual == ""
	}
	return noProxyEntriesMatch(strings.Split(actual, ","), expected)
}

// noProxyEntriesMatch reports whether the actual entry list contains every entry of the
// comma-separated expected list. The operator appends internal defaults beyond what is
// configured, so the actual list is allowed to be a superset.
func noProxyEntriesMatch(actual []string, expected string) bool {
	actualEntries := sets.New(actual...)
	for _, entry := range strings.Split(expected, ",") {
		if entry == "" {
			continue
		}
		if !actualEntries.Has(entry) {
			return false
		}
	}
	return true
}

// trustedCAStateMatches reports whether the observed volume/mount state matches the
// expectation. Presence requires both the volume and a mount; absence requires neither,
// so a volume left without a mount satisfies neither and fails an AssertNoConfig check.
func trustedCAStateMatches(expectPresence, foundVolume, foundMount bool) bool {
	if expectPresence {
		return foundVolume && foundMount
	}
	return !(foundVolume || foundMount)
}

func trustedCAVolumeState(deployment *appsv1.Deployment) (foundVolume, foundMount bool) {
	var volumeName string
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == ComponentProxyCAConfigMapName {
			foundVolume = true
			volumeName = vol.Name
			break
		}
	}
	if !foundVolume {
		return false, false
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volumeName {
				return true, true
			}
		}
	}

	return true, false
}
