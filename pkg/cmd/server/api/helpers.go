package api

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

var (
	knownOpenShiftFeatureSet map[string]string
)

func init() {
	knownOpenShiftFeatureSet = make(map[string]string, len(KnownOpenShiftFeatures))
	for _, feature := range KnownOpenShiftFeatures {
		knownOpenShiftFeatureSet[strings.ToLower(feature)] = feature
	}
}

// Add extends feature list with given valid items. They are appended
// unless already present.
func (fl *FeatureList) Add(items ...string) error {
	unknown := []string{}
	toAppend := make([]string, 0, len(items))
	for _, item := range items {
		feature, exists := knownOpenShiftFeatureSet[strings.ToLower(item)]
		if !exists {
			unknown = append(unknown, item)
			continue
		}
		if fl.Has(feature) {
			continue
		}
		toAppend = append(toAppend, feature)
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown features: %s", strings.Join(unknown, ", "))
	}
	*fl = append(*fl, toAppend...)
	return nil
}

// Delete removes given items from feature list while keeping its original
// order.
func (fl *FeatureList) Delete(items ...string) {
	if len(*fl) == 0 || len(items) == 0 {
		return
	}
	toDelete := util.NewStringSet()
	for _, item := range items {
		toDelete.Insert(strings.ToLower(item))
	}
	newList := []string{}
	for _, item := range *fl {
		if !toDelete.Has(strings.ToLower(item)) {
			newList = append(newList, item)
		}
	}
	*fl = newList
}

// Has returns true if given feature exists in feature list. The check is
// case-insensitive.
func (fl FeatureList) Has(feature string) bool {
	lowerCased := strings.ToLower(feature)
	for _, item := range fl {
		if strings.ToLower(item) == lowerCased {
			return true
		}
	}
	return false
}

// ParseNamespaceAndName returns back the namespace and name (empty if something goes wrong), for a given string.
// This is useful when pointing to a particular resource inside of our config.
func ParseNamespaceAndName(in string) (string, string, error) {
	if len(in) == 0 {
		return "", "", nil
	}

	tokens := strings.Split(in, "/")
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("expected input in the form <namespace>/<resource-name>, not: %v", in)
	}

	return tokens[0], tokens[1], nil
}

func RelativizeMasterConfigPaths(config *MasterConfig, base string) error {
	return cmdutil.RelativizePathWithNoBacksteps(GetMasterFileReferences(config), base)
}

func ResolveMasterConfigPaths(config *MasterConfig, base string) error {
	return cmdutil.ResolvePaths(GetMasterFileReferences(config), base)
}

func GetMasterFileReferences(config *MasterConfig) []*string {
	refs := []*string{}

	refs = append(refs, &config.ServingInfo.ServerCert.CertFile)
	refs = append(refs, &config.ServingInfo.ServerCert.KeyFile)
	refs = append(refs, &config.ServingInfo.ClientCA)

	refs = append(refs, &config.EtcdClientInfo.ClientCert.CertFile)
	refs = append(refs, &config.EtcdClientInfo.ClientCert.KeyFile)
	refs = append(refs, &config.EtcdClientInfo.CA)

	refs = append(refs, &config.KubeletClientInfo.ClientCert.CertFile)
	refs = append(refs, &config.KubeletClientInfo.ClientCert.KeyFile)
	refs = append(refs, &config.KubeletClientInfo.CA)

	if config.EtcdConfig != nil {
		refs = append(refs, &config.EtcdConfig.ServingInfo.ServerCert.CertFile)
		refs = append(refs, &config.EtcdConfig.ServingInfo.ServerCert.KeyFile)
		refs = append(refs, &config.EtcdConfig.ServingInfo.ClientCA)

		refs = append(refs, &config.EtcdConfig.PeerServingInfo.ServerCert.CertFile)
		refs = append(refs, &config.EtcdConfig.PeerServingInfo.ServerCert.KeyFile)
		refs = append(refs, &config.EtcdConfig.PeerServingInfo.ClientCA)

		refs = append(refs, &config.EtcdConfig.StorageDir)
	}

	if config.OAuthConfig != nil {

		if config.OAuthConfig.SessionConfig != nil {
			refs = append(refs, &config.OAuthConfig.SessionConfig.SessionSecretsFile)
		}

		for _, identityProvider := range config.OAuthConfig.IdentityProviders {
			switch provider := identityProvider.Provider.Object.(type) {
			case (*RequestHeaderIdentityProvider):
				refs = append(refs, &provider.ClientCA)

			case (*HTPasswdPasswordIdentityProvider):
				refs = append(refs, &provider.File)

			case (*LDAPPasswordIdentityProvider):
				refs = append(refs, &provider.CA)

			case (*BasicAuthPasswordIdentityProvider):
				refs = append(refs, &provider.RemoteConnectionInfo.CA)
				refs = append(refs, &provider.RemoteConnectionInfo.ClientCert.CertFile)
				refs = append(refs, &provider.RemoteConnectionInfo.ClientCert.KeyFile)

			case (*OpenIDIdentityProvider):
				refs = append(refs, &provider.CA)

			}
		}
	}

	if config.AssetConfig != nil {
		refs = append(refs, &config.AssetConfig.ServingInfo.ServerCert.CertFile)
		refs = append(refs, &config.AssetConfig.ServingInfo.ServerCert.KeyFile)
		refs = append(refs, &config.AssetConfig.ServingInfo.ClientCA)
	}

	if config.KubernetesMasterConfig != nil {
		refs = append(refs, &config.KubernetesMasterConfig.SchedulerConfigFile)
	}

	refs = append(refs, &config.ServiceAccountConfig.MasterCA)
	refs = append(refs, &config.ServiceAccountConfig.PrivateKeyFile)
	for i := range config.ServiceAccountConfig.PublicKeyFiles {
		refs = append(refs, &config.ServiceAccountConfig.PublicKeyFiles[i])
	}

	refs = append(refs, &config.MasterClients.OpenShiftLoopbackKubeConfig)
	refs = append(refs, &config.MasterClients.ExternalKubernetesKubeConfig)

	refs = append(refs, &config.PolicyConfig.BootstrapPolicyFile)

	return refs
}

func RelativizeNodeConfigPaths(config *NodeConfig, base string) error {
	return cmdutil.RelativizePathWithNoBacksteps(GetNodeFileReferences(config), base)
}

func ResolveNodeConfigPaths(config *NodeConfig, base string) error {
	return cmdutil.ResolvePaths(GetNodeFileReferences(config), base)
}

func GetNodeFileReferences(config *NodeConfig) []*string {
	refs := []*string{}

	refs = append(refs, &config.ServingInfo.ServerCert.CertFile)
	refs = append(refs, &config.ServingInfo.ServerCert.KeyFile)
	refs = append(refs, &config.ServingInfo.ClientCA)

	refs = append(refs, &config.MasterKubeConfig)

	refs = append(refs, &config.VolumeDirectory)

	if config.PodManifestConfig != nil {
		refs = append(refs, &config.PodManifestConfig.Path)
	}

	return refs
}

// TODO: clients should be copied and instantiated from a common client config, tweaked, then
// given to individual controllers and other infrastructure components.
func GetKubeClient(kubeConfigFile string) (*kclient.Client, *kclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = kubeConfigFile
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	// This is an internal client which is shared by most controllers, so boost default QPS
	// TODO: this should be configured by the caller, not in this method.
	kubeConfig.QPS = 100.0
	kubeConfig.Burst = 200

	kubeConfig.WrapTransport = DefaultClientTransport
	kubeClient, err := kclient.New(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return kubeClient, kubeConfig, nil
}

// TODO: clients should be copied and instantiated from a common client config, tweaked, then
// given to individual controllers and other infrastructure components.
func GetOpenShiftClient(kubeConfigFile string) (*client.Client, *kclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = kubeConfigFile
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	// This is an internal client which is shared by most controllers, so boost default QPS
	// TODO: this should be configured by the caller, not in this method.
	kubeConfig.QPS = 150.0
	kubeConfig.Burst = 300

	kubeConfig.WrapTransport = DefaultClientTransport
	openshiftClient, err := client.New(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return openshiftClient, kubeConfig, nil
}

// DefaultClientTransport sets defaults for a client Transport that are suitable
// for use by infrastructure components.
func DefaultClientTransport(rt http.RoundTripper) http.RoundTripper {
	transport := rt.(*http.Transport)
	// TODO: this should be configured by the caller, not in this method.
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport.Dial = dialer.Dial
	// Hold open more internal idle connections
	// TODO: this should be configured by the caller, not in this method.
	transport.MaxIdleConnsPerHost = 100
	return transport
}

func UseTLS(servingInfo ServingInfo) bool {
	return len(servingInfo.ServerCert.CertFile) > 0
}

// GetAPIClientCertCAPool returns the cert pool used to validate client certificates to the API server
func GetAPIClientCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	return cmdutil.CertPoolFromFile(options.ServingInfo.ClientCA)
}

// GetClientCertCAPool returns a cert pool containing all client CAs that could be presented (union of API and OAuth)
func GetClientCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	roots := x509.NewCertPool()

	// Add CAs for OAuth
	certs, err := getOAuthClientCertCAs(options)
	if err != nil {
		return nil, err
	}
	for _, root := range certs {
		roots.AddCert(root)
	}

	// Add CAs for API
	certs, err = getAPIClientCertCAs(options)
	if err != nil {
		return nil, err
	}
	for _, root := range certs {
		roots.AddCert(root)
	}

	return roots, nil
}

// GetAPIServerCertCAPool returns the cert pool containing the roots for the API server cert
func GetAPIServerCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	if !UseTLS(options.ServingInfo.ServingInfo) {
		return x509.NewCertPool(), nil
	}

	return cmdutil.CertPoolFromFile(options.ServingInfo.ClientCA)
}

func getOAuthClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	if !UseTLS(options.ServingInfo.ServingInfo) {
		return nil, nil
	}

	allCerts := []*x509.Certificate{}

	if options.OAuthConfig != nil {
		for _, identityProvider := range options.OAuthConfig.IdentityProviders {

			switch provider := identityProvider.Provider.Object.(type) {
			case (*RequestHeaderIdentityProvider):
				caFile := provider.ClientCA
				if len(caFile) == 0 {
					continue
				}
				certs, err := cmdutil.CertificatesFromFile(caFile)
				if err != nil {
					return nil, fmt.Errorf("Error reading %s: %s", caFile, err)
				}
				allCerts = append(allCerts, certs...)
			}
		}
	}

	return allCerts, nil
}

func getAPIClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	if !UseTLS(options.ServingInfo.ServingInfo) {
		return nil, nil
	}

	return cmdutil.CertificatesFromFile(options.ServingInfo.ClientCA)
}

func GetKubeletClientConfig(options MasterConfig) *kclient.KubeletConfig {
	config := &kclient.KubeletConfig{
		Port: options.KubeletClientInfo.Port,
	}

	if len(options.KubeletClientInfo.CA) > 0 {
		config.EnableHttps = true
		config.CAFile = options.KubeletClientInfo.CA
	}

	if len(options.KubeletClientInfo.ClientCert.CertFile) > 0 {
		config.EnableHttps = true
		config.CertFile = options.KubeletClientInfo.ClientCert.CertFile
		config.KeyFile = options.KubeletClientInfo.ClientCert.KeyFile
	}

	return config
}

func IsPasswordAuthenticator(provider IdentityProvider) bool {
	switch provider.Provider.Object.(type) {
	case
		(*BasicAuthPasswordIdentityProvider),
		(*AllowAllPasswordIdentityProvider),
		(*DenyAllPasswordIdentityProvider),
		(*HTPasswdPasswordIdentityProvider),
		(*LDAPPasswordIdentityProvider):

		return true
	}

	return false
}

func IsIdentityProviderType(provider runtime.EmbeddedObject) bool {
	switch provider.Object.(type) {
	case
		(*RequestHeaderIdentityProvider),
		(*BasicAuthPasswordIdentityProvider),
		(*AllowAllPasswordIdentityProvider),
		(*DenyAllPasswordIdentityProvider),
		(*HTPasswdPasswordIdentityProvider),
		(*LDAPPasswordIdentityProvider),
		(*OpenIDIdentityProvider),
		(*GitHubIdentityProvider),
		(*GoogleIdentityProvider):

		return true
	}

	return false
}

func IsOAuthIdentityProvider(provider IdentityProvider) bool {
	switch provider.Provider.Object.(type) {
	case
		(*OpenIDIdentityProvider),
		(*GitHubIdentityProvider),
		(*GoogleIdentityProvider):

		return true
	}

	return false
}

func HasOpenShiftAPILevel(config MasterConfig, apiLevel string) bool {
	apiLevelSet := util.NewStringSet(config.APILevels...)
	return apiLevelSet.Has(apiLevel)
}

func HasKubernetesAPILevel(config KubernetesMasterConfig, apiLevel string) bool {
	apiLevelSet := util.NewStringSet(config.APILevels...)
	return apiLevelSet.Has(apiLevel)
}
