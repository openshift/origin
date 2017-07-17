package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

var (
	// Maps lower-cased feature flag names and aliases to their canonical names.
	knownOpenShiftFeatureSet map[string]string
)

func init() {
	knownOpenShiftFeatureSet = make(map[string]string, len(KnownOpenShiftFeatures))
	for _, feature := range KnownOpenShiftFeatures {
		knownOpenShiftFeatureSet[strings.ToLower(feature)] = feature
	}
	for alias, feature := range FeatureAliases {
		knownOpenShiftFeatureSet[strings.ToLower(alias)] = feature
	}
}

// NormalizeOpenShiftFeature returns canonical name for given OpenShift feature
// flag or an alias if known. Otherwise lower-cased name is returned.
func NormalizeOpenShiftFeature(name string) (string, bool) {
	name = strings.ToLower(name)
	if feature, ok := knownOpenShiftFeatureSet[name]; ok {
		return feature, true
	}
	return name, false
}

// Add extends feature list with given valid items. They are appended
// unless already present.
func (fl *FeatureList) Add(items ...string) error {
	unknown := []string{}
	toAppend := make([]string, 0, len(items))
	for _, item := range items {
		feature, known := NormalizeOpenShiftFeature(item)
		if !known {
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
	toDelete := FeatureList(items)
	newList := []string{}
	for _, item := range *fl {
		if !toDelete.Has(item) {
			newList = append(newList, item)
		}
	}
	*fl = newList
}

// Has returns true if given feature exists in feature list. The check is
// case-insensitive.
func (fl FeatureList) Has(feature string) bool {
	normalized, _ := NormalizeOpenShiftFeature(feature)
	for _, item := range fl {
		itemNormalized, _ := NormalizeOpenShiftFeature(item)
		if normalized == itemNormalized {
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
	for i := range config.ServingInfo.NamedCertificates {
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].CertFile)
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].KeyFile)
	}

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
		for i := range config.EtcdConfig.ServingInfo.NamedCertificates {
			refs = append(refs, &config.EtcdConfig.ServingInfo.NamedCertificates[i].CertFile)
			refs = append(refs, &config.EtcdConfig.ServingInfo.NamedCertificates[i].KeyFile)
		}

		refs = append(refs, &config.EtcdConfig.PeerServingInfo.ServerCert.CertFile)
		refs = append(refs, &config.EtcdConfig.PeerServingInfo.ServerCert.KeyFile)
		refs = append(refs, &config.EtcdConfig.PeerServingInfo.ClientCA)
		for i := range config.EtcdConfig.PeerServingInfo.NamedCertificates {
			refs = append(refs, &config.EtcdConfig.PeerServingInfo.NamedCertificates[i].CertFile)
			refs = append(refs, &config.EtcdConfig.PeerServingInfo.NamedCertificates[i].KeyFile)
		}

		refs = append(refs, &config.EtcdConfig.StorageDir)
	}

	if config.OAuthConfig != nil {

		if config.OAuthConfig.MasterCA != nil {
			refs = append(refs, config.OAuthConfig.MasterCA)
		}

		if config.OAuthConfig.SessionConfig != nil {
			refs = append(refs, &config.OAuthConfig.SessionConfig.SessionSecretsFile)
		}

		for _, identityProvider := range config.OAuthConfig.IdentityProviders {
			switch provider := identityProvider.Provider.(type) {
			case (*RequestHeaderIdentityProvider):
				refs = append(refs, &provider.ClientCA)

			case (*HTPasswdPasswordIdentityProvider):
				refs = append(refs, &provider.File)

			case (*LDAPPasswordIdentityProvider):
				refs = append(refs, &provider.CA)
				refs = append(refs, GetStringSourceFileReferences(&provider.BindPassword)...)

			case (*BasicAuthPasswordIdentityProvider):
				refs = append(refs, &provider.RemoteConnectionInfo.CA)
				refs = append(refs, &provider.RemoteConnectionInfo.ClientCert.CertFile)
				refs = append(refs, &provider.RemoteConnectionInfo.ClientCert.KeyFile)

			case (*KeystonePasswordIdentityProvider):
				refs = append(refs, &provider.RemoteConnectionInfo.CA)
				refs = append(refs, &provider.RemoteConnectionInfo.ClientCert.CertFile)
				refs = append(refs, &provider.RemoteConnectionInfo.ClientCert.KeyFile)

			case (*GitLabIdentityProvider):
				refs = append(refs, &provider.CA)
				refs = append(refs, GetStringSourceFileReferences(&provider.ClientSecret)...)

			case (*OpenIDIdentityProvider):
				refs = append(refs, &provider.CA)
				refs = append(refs, GetStringSourceFileReferences(&provider.ClientSecret)...)

			case (*GoogleIdentityProvider):
				refs = append(refs, GetStringSourceFileReferences(&provider.ClientSecret)...)

			case (*GitHubIdentityProvider):
				refs = append(refs, GetStringSourceFileReferences(&provider.ClientSecret)...)

			}
		}

		if config.OAuthConfig.Templates != nil {
			refs = append(refs, &config.OAuthConfig.Templates.Login)
			refs = append(refs, &config.OAuthConfig.Templates.ProviderSelection)
			refs = append(refs, &config.OAuthConfig.Templates.Error)
		}
	}

	if config.AssetConfig != nil {
		refs = append(refs, &config.AssetConfig.ServingInfo.ServerCert.CertFile)
		refs = append(refs, &config.AssetConfig.ServingInfo.ServerCert.KeyFile)
		refs = append(refs, &config.AssetConfig.ServingInfo.ClientCA)
		for i := range config.AssetConfig.ServingInfo.NamedCertificates {
			refs = append(refs, &config.AssetConfig.ServingInfo.NamedCertificates[i].CertFile)
			refs = append(refs, &config.AssetConfig.ServingInfo.NamedCertificates[i].KeyFile)
		}

		for i := range config.AssetConfig.ExtensionScripts {
			refs = append(refs, &config.AssetConfig.ExtensionScripts[i])
		}
		for i := range config.AssetConfig.ExtensionStylesheets {
			refs = append(refs, &config.AssetConfig.ExtensionStylesheets[i])
		}
		for i := range config.AssetConfig.Extensions {
			refs = append(refs, &config.AssetConfig.Extensions[i].SourceDirectory)
		}
	}

	if config.KubernetesMasterConfig != nil {
		refs = append(refs, &config.KubernetesMasterConfig.SchedulerConfigFile)

		refs = append(refs, &config.KubernetesMasterConfig.ProxyClientInfo.CertFile)
		refs = append(refs, &config.KubernetesMasterConfig.ProxyClientInfo.KeyFile)
	}

	if config.AuthConfig.RequestHeader != nil {
		refs = append(refs, &config.AuthConfig.RequestHeader.ClientCA)
	}

	refs = append(refs, &config.AggregatorConfig.ProxyClientInfo.CertFile)
	refs = append(refs, &config.AggregatorConfig.ProxyClientInfo.KeyFile)

	refs = append(refs, &config.ServiceAccountConfig.MasterCA)
	refs = append(refs, &config.ServiceAccountConfig.PrivateKeyFile)
	for i := range config.ServiceAccountConfig.PublicKeyFiles {
		refs = append(refs, &config.ServiceAccountConfig.PublicKeyFiles[i])
	}

	refs = append(refs, &config.MasterClients.OpenShiftLoopbackKubeConfig)
	refs = append(refs, &config.MasterClients.ExternalKubernetesKubeConfig)

	refs = append(refs, &config.PolicyConfig.BootstrapPolicyFile)

	if config.ControllerConfig.ServiceServingCert.Signer != nil {
		refs = append(refs, &config.ControllerConfig.ServiceServingCert.Signer.CertFile)
		refs = append(refs, &config.ControllerConfig.ServiceServingCert.Signer.KeyFile)
	}

	refs = append(refs, &config.AuditConfig.AuditFilePath)

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
	for i := range config.ServingInfo.NamedCertificates {
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].CertFile)
		refs = append(refs, &config.ServingInfo.NamedCertificates[i].KeyFile)
	}

	refs = append(refs, &config.DNSRecursiveResolvConf)

	refs = append(refs, &config.MasterKubeConfig)

	refs = append(refs, &config.VolumeDirectory)

	if config.PodManifestConfig != nil {
		refs = append(refs, &config.PodManifestConfig.Path)
	}

	return refs
}

// SetProtobufClientDefaults sets the appropriate content types for defaulting to protobuf
// client communications and increases the default QPS and burst. This is used to override
// defaulted config supporting versions older than 1.3 for new configurations generated in 1.3+.
func SetProtobufClientDefaults(overrides *ClientConnectionOverrides) {
	overrides.AcceptContentTypes = "application/vnd.kubernetes.protobuf,application/json"
	overrides.ContentType = "application/vnd.kubernetes.protobuf"
	overrides.QPS *= 2
	overrides.Burst *= 2
}

// TODO: clients should be copied and instantiated from a common client config, tweaked, then
// given to individual controllers and other infrastructure components.
func GetInternalKubeClient(kubeConfigFile string, overrides *ClientConnectionOverrides) (kclientsetinternal.Interface, *restclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = kubeConfigFile
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	applyClientConnectionOverrides(overrides, kubeConfig)

	kubeConfig.WrapTransport = DefaultClientTransport
	clientset, err := kclientsetinternal.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientset, kubeConfig, nil
}

// TODO: clients should be copied and instantiated from a common client config, tweaked, then
// given to individual controllers and other infrastructure components.
func GetExternalKubeClient(kubeConfigFile string, overrides *ClientConnectionOverrides) (kclientsetexternal.Interface, *restclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = kubeConfigFile
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	applyClientConnectionOverrides(overrides, kubeConfig)

	kubeConfig.WrapTransport = DefaultClientTransport
	clientset, err := kclientsetexternal.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientset, kubeConfig, nil
}

// TODO: clients should be copied and instantiated from a common client config, tweaked, then
// given to individual controllers and other infrastructure components. Overrides are optional
// and may alter the default configuration.
func GetOpenShiftClient(kubeConfigFile string, overrides *ClientConnectionOverrides) (*client.Client, *restclient.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = kubeConfigFile
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	kubeConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	applyClientConnectionOverrides(overrides, kubeConfig)

	kubeConfig.WrapTransport = DefaultClientTransport
	openshiftClient, err := client.New(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return openshiftClient, kubeConfig, nil
}

// applyClientConnectionOverrides updates a kubeConfig with the overrides from the config.
func applyClientConnectionOverrides(overrides *ClientConnectionOverrides, kubeConfig *restclient.Config) {
	if overrides == nil {
		return
	}
	kubeConfig.QPS = overrides.QPS
	kubeConfig.Burst = int(overrides.Burst)
	kubeConfig.ContentConfig.AcceptContentTypes = overrides.AcceptContentTypes
	kubeConfig.ContentConfig.ContentType = overrides.ContentType
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

// GetNamedCertificateMap returns a map of strings to *tls.Certificate, suitable for use in tls.Config#NamedCertificates
// Returns an error if any of the certs cannot be loaded, or do not match the configured name
// Returns nil if len(namedCertificates) == 0
func GetNamedCertificateMap(namedCertificates []NamedCertificate) (map[string]*tls.Certificate, error) {
	if len(namedCertificates) == 0 {
		return nil, nil
	}
	namedCerts := map[string]*tls.Certificate{}
	for _, namedCertificate := range namedCertificates {
		cert, err := tls.LoadX509KeyPair(namedCertificate.CertFile, namedCertificate.KeyFile)
		if err != nil {
			return nil, err
		}
		for _, name := range namedCertificate.Names {
			namedCerts[name] = &cert
		}
	}
	return namedCerts, nil
}

// GetClientCertCAPool returns a cert pool containing all client CAs that could be presented (union of API and OAuth)
func GetClientCertCAPool(options MasterConfig) (*x509.CertPool, error) {
	roots := x509.NewCertPool()

	// Add CAs for OAuth
	certs, err := GetOAuthClientCertCAs(options)
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

func GetOAuthClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	if !UseTLS(options.ServingInfo.ServingInfo) {
		return nil, nil
	}

	allCerts := []*x509.Certificate{}

	if options.OAuthConfig != nil {
		for _, identityProvider := range options.OAuthConfig.IdentityProviders {

			switch provider := identityProvider.Provider.(type) {
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

func GetRequestHeaderClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	if !UseTLS(options.ServingInfo.ServingInfo) {
		return nil, nil
	}
	if options.AuthConfig.RequestHeader == nil {
		return nil, nil
	}

	certs, err := cmdutil.CertificatesFromFile(options.AuthConfig.RequestHeader.ClientCA)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s", options.AuthConfig.RequestHeader.ClientCA, err)
	}
	return certs, nil
}

func getAPIClientCertCAs(options MasterConfig) ([]*x509.Certificate, error) {
	if !UseTLS(options.ServingInfo.ServingInfo) {
		return nil, nil
	}

	return cmdutil.CertificatesFromFile(options.ServingInfo.ClientCA)
}

func GetKubeletClientConfig(options MasterConfig) *kubeletclient.KubeletClientConfig {
	config := &kubeletclient.KubeletClientConfig{
		Port: options.KubeletClientInfo.Port,
		PreferredAddressTypes: []string{
			string(api.NodeHostName),
			string(api.NodeInternalIP),
			string(api.NodeExternalIP),
		},
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
	switch provider.Provider.(type) {
	case
		(*BasicAuthPasswordIdentityProvider),
		(*AllowAllPasswordIdentityProvider),
		(*DenyAllPasswordIdentityProvider),
		(*HTPasswdPasswordIdentityProvider),
		(*LDAPPasswordIdentityProvider),
		(*KeystonePasswordIdentityProvider):

		return true
	}

	return false
}

func IsIdentityProviderType(provider runtime.Object) bool {
	switch provider.(type) {
	case
		(*RequestHeaderIdentityProvider),
		(*BasicAuthPasswordIdentityProvider),
		(*AllowAllPasswordIdentityProvider),
		(*DenyAllPasswordIdentityProvider),
		(*HTPasswdPasswordIdentityProvider),
		(*LDAPPasswordIdentityProvider),
		(*KeystonePasswordIdentityProvider),
		(*OpenIDIdentityProvider),
		(*GitHubIdentityProvider),
		(*GitLabIdentityProvider),
		(*GoogleIdentityProvider):

		return true
	}

	return false
}

func IsOAuthIdentityProvider(provider IdentityProvider) bool {
	switch provider.Provider.(type) {
	case
		(*OpenIDIdentityProvider),
		(*GitHubIdentityProvider),
		(*GitLabIdentityProvider),
		(*GoogleIdentityProvider):

		return true
	}

	return false
}

func HasOpenShiftAPILevel(config MasterConfig, apiLevel string) bool {
	apiLevelSet := sets.NewString(config.APILevels...)
	return apiLevelSet.Has(apiLevel)
}

const kubeAPIEnablementFlag = "runtime-config"

// GetKubeAPIServerFlagAPIEnablement parses the available flag at the groupVersion level
// with no support for individual resources and no support for the legacy API.
func GetKubeAPIServerFlagAPIEnablement(flagValue []string) map[schema.GroupVersion]bool {
	versions := map[schema.GroupVersion]bool{}
	for _, val := range flagValue {
		// skip bad flags
		if !strings.HasPrefix(val, "apis/") {
			continue
		}
		tokens := strings.Split(val[len("apis/"):], "=")
		if len(tokens) != 2 {
			continue
		}
		gv, err := schema.ParseGroupVersion(tokens[0])
		if err != nil {
			continue
		}
		enabled, _ := strconv.ParseBool(tokens[1])
		versions[gv] = enabled
	}

	return versions
}

// GetEnabledAPIVersionsForGroup returns the list of API Versions that are enabled for that group.
// It respects the extended args which are used to enable and disable versions in kube too.
func GetEnabledAPIVersionsForGroup(config KubernetesMasterConfig, apiGroup string) []string {
	allowedVersions := KubeAPIGroupsToAllowedVersions[apiGroup]
	blacklist := sets.NewString(config.DisabledAPIGroupVersions[apiGroup]...)

	if blacklist.Has(AllVersions) {
		return []string{}
	}

	flagVersions := GetKubeAPIServerFlagAPIEnablement(config.APIServerArguments[kubeAPIEnablementFlag])

	enabledVersions := sets.String{}
	for _, currVersion := range allowedVersions {
		if blacklist.Has(currVersion) {
			continue
		}
		gv := schema.GroupVersion{Group: apiGroup, Version: currVersion}
		// if this was explicitly disabled via flag, skip it
		if enabled, ok := flagVersions[gv]; ok && !enabled {
			continue
		}

		enabledVersions.Insert(currVersion)
	}

	for currVersion, enabled := range flagVersions {
		if !enabled {
			continue
		}
		if blacklist.Has(currVersion.Version) {
			continue
		}
		if currVersion.Group != apiGroup {
			continue
		}
		enabledVersions.Insert(currVersion.Version)
	}

	return enabledVersions.List()
}

// It respects the extended args which are used to enable and disable versions in kube too.
// GetDisabledAPIVersionsForGroup returns the list of API Versions that are disabled for that group.
func GetDisabledAPIVersionsForGroup(config KubernetesMasterConfig, apiGroup string) []string {
	allowedVersions := sets.NewString(KubeAPIGroupsToAllowedVersions[apiGroup]...)
	enabledVersions := sets.NewString(GetEnabledAPIVersionsForGroup(config, apiGroup)...)
	disabledVersions := allowedVersions.Difference(enabledVersions)
	disabledVersions.Insert(config.DisabledAPIGroupVersions[apiGroup]...)

	flagVersions := GetKubeAPIServerFlagAPIEnablement(config.APIServerArguments[kubeAPIEnablementFlag])
	for currVersion, enabled := range flagVersions {
		if enabled {
			continue
		}
		if disabledVersions.Has(currVersion.Version) {
			continue
		}
		if currVersion.Group != apiGroup {
			continue
		}
		disabledVersions.Insert(currVersion.Version)
	}

	return disabledVersions.List()
}

func CIDRsOverlap(cidr1, cidr2 string) bool {
	_, ipNet1, err := net.ParseCIDR(cidr1)
	if err != nil {
		return false
	}
	_, ipNet2, err := net.ParseCIDR(cidr2)
	if err != nil {
		return false
	}
	return ipNet1.Contains(ipNet2.IP) || ipNet2.Contains(ipNet1.IP)
}
