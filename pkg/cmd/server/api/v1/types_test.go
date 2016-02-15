package v1_test

import (
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/util"

	internal "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/v1"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

const (
	// This constant lists all possible options for the node config file in v1
	// Before modifying this constant, ensure any changes have corresponding issues filed for:
	// - documentation: https://github.com/openshift/openshift-docs/
	// - install: https://github.com/openshift/openshift-ansible/
	expectedSerializedNodeConfig = `allowDisabledDocker: false
apiVersion: v1
authConfig:
  authenticationCacheSize: 0
  authenticationCacheTTL: ""
  authorizationCacheSize: 0
  authorizationCacheTTL: ""
dnsDomain: ""
dnsIP: ""
dockerConfig:
  execHandlerName: ""
imageConfig:
  format: ""
  latest: false
iptablesSyncPeriod: ""
kind: NodeConfig
masterKubeConfig: ""
networkConfig:
  mtu: 0
  networkPluginName: ""
nodeIP: ""
nodeName: ""
podManifestConfig:
  fileCheckIntervalSeconds: 0
  path: ""
servingInfo:
  bindAddress: ""
  bindNetwork: ""
  certFile: ""
  clientCA: ""
  keyFile: ""
  namedCertificates: null
volumeDirectory: ""
`

	// This constant lists all possible options for the master config file in v1.
	// It also includes the fields for all the identity provider types.
	// Before modifying this constant, ensure any changes have corresponding issues filed for:
	// - documentation: https://github.com/openshift/openshift-docs/
	// - install: https://github.com/openshift/openshift-ansible/
	expectedSerializedMasterConfig = `admissionConfig:
  pluginConfig:
    plugin:
      configuration:
        apiVersion: v1
        data: ""
        kind: AdmissionPluginTestConfig
      location: ""
  pluginOrderOverride:
  - plugin
apiLevels: null
apiVersion: v1
assetConfig:
  extensionDevelopment: false
  extensionScripts: null
  extensionStylesheets: null
  extensions:
  - html5Mode: false
    name: ""
    sourceDirectory: ""
  loggingPublicURL: ""
  logoutURL: ""
  masterPublicURL: ""
  metricsPublicURL: ""
  publicURL: ""
  servingInfo:
    bindAddress: ""
    bindNetwork: ""
    certFile: ""
    clientCA: ""
    keyFile: ""
    maxRequestsInFlight: 0
    namedCertificates: null
    requestTimeoutSeconds: 0
controllerLeaseTTL: 0
controllers: ""
corsAllowedOrigins: null
disabledFeatures: null
dnsConfig:
  allowRecursiveQueries: false
  bindAddress: ""
  bindNetwork: ""
etcdClientInfo:
  ca: ""
  certFile: ""
  keyFile: ""
  urls: null
etcdConfig:
  address: ""
  peerAddress: ""
  peerServingInfo:
    bindAddress: ""
    bindNetwork: ""
    certFile: ""
    clientCA: ""
    keyFile: ""
    namedCertificates: null
  servingInfo:
    bindAddress: ""
    bindNetwork: ""
    certFile: ""
    clientCA: ""
    keyFile: ""
    namedCertificates: null
  storageDirectory: ""
etcdStorageConfig:
  kubernetesStoragePrefix: ""
  kubernetesStorageVersion: ""
  openShiftStoragePrefix: ""
  openShiftStorageVersion: ""
imageConfig:
  format: ""
  latest: false
imagePolicyConfig:
  disableScheduledImport: false
  maxImagesBulkImportedPerRepository: 0
  maxScheduledImageImportsPerMinute: 0
  scheduledImageImportMinimumIntervalSeconds: 0
kind: MasterConfig
kubeletClientInfo:
  ca: ""
  certFile: ""
  keyFile: ""
  port: 0
kubernetesMasterConfig:
  admissionConfig:
    pluginConfig:
      plugin:
        configuration:
          apiVersion: v1
          data: ""
          kind: AdmissionPluginTestConfig
        location: ""
    pluginOrderOverride:
    - plugin
  apiLevels: null
  apiServerArguments: null
  controllerArguments: null
  disabledAPIGroupVersions: null
  masterCount: 0
  masterIP: ""
  podEvictionTimeout: ""
  proxyClientInfo:
    certFile: ""
    keyFile: ""
  schedulerConfigFile: ""
  servicesNodePortRange: ""
  servicesSubnet: ""
  staticNodeNames: null
masterClients:
  externalKubernetesKubeConfig: ""
  openshiftLoopbackKubeConfig: ""
masterPublicURL: ""
networkConfig:
  clusterNetworkCIDR: ""
  hostSubnetLength: 0
  networkPluginName: ""
  serviceNetworkCIDR: ""
oauthConfig:
  alwaysShowProviderSelection: false
  assetPublicURL: ""
  grantConfig:
    method: ""
  identityProviders:
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      ca: ""
      certFile: ""
      keyFile: ""
      kind: BasicAuthPasswordIdentityProvider
      url: ""
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      kind: AllowAllPasswordIdentityProvider
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      kind: DenyAllPasswordIdentityProvider
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      file: ""
      kind: HTPasswdPasswordIdentityProvider
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      attributes:
        email: null
        id: null
        name: null
        preferredUsername: null
      bindDN: ""
      bindPassword: ""
      ca: ""
      insecure: false
      kind: LDAPPasswordIdentityProvider
      url: ""
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      challengeURL: ""
      clientCA: ""
      headers: null
      kind: RequestHeaderIdentityProvider
      loginURL: ""
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      ca: ""
      certFile: ""
      domainName: ""
      keyFile: ""
      kind: KeystonePasswordIdentityProvider
      url: ""
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      clientID: ""
      clientSecret: ""
      kind: GitHubIdentityProvider
      organizations: null
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      ca: ""
      clientID: ""
      clientSecret: ""
      kind: GitLabIdentityProvider
      url: ""
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      clientID: ""
      clientSecret: ""
      hostedDomain: ""
      kind: GoogleIdentityProvider
  - challenge: false
    login: false
    mappingMethod: ""
    name: ""
    provider:
      apiVersion: v1
      ca: ""
      claims:
        email: null
        id: null
        name: null
        preferredUsername: null
      clientID: ""
      clientSecret: ""
      extraAuthorizeParameters: null
      extraScopes: null
      kind: OpenIDIdentityProvider
      urls:
        authorize: ""
        token: ""
        userInfo: ""
  masterCA: null
  masterPublicURL: ""
  masterURL: ""
  sessionConfig:
    sessionMaxAgeSeconds: 0
    sessionName: ""
    sessionSecretsFile: ""
  templates:
    error: ""
    login: ""
    providerSelection: ""
  tokenConfig:
    accessTokenMaxAgeSeconds: 0
    authorizeTokenMaxAgeSeconds: 0
pauseControllers: false
policyConfig:
  bootstrapPolicyFile: ""
  openshiftInfrastructureNamespace: ""
  openshiftSharedResourcesNamespace: ""
projectConfig:
  defaultNodeSelector: ""
  projectRequestMessage: ""
  projectRequestTemplate: ""
  securityAllocator: null
routingConfig:
  subdomain: ""
serviceAccountConfig:
  limitSecretReferences: false
  managedNames: null
  masterCA: ""
  privateKeyFile: ""
  publicKeyFiles: null
servingInfo:
  bindAddress: ""
  bindNetwork: ""
  certFile: ""
  clientCA: ""
  keyFile: ""
  maxRequestsInFlight: 0
  namedCertificates:
  - certFile: ""
    keyFile: ""
    names: null
  requestTimeoutSeconds: 0
`
)

func TestNodeConfig(t *testing.T) {
	config := &internal.NodeConfig{
		PodManifestConfig: &internal.PodManifestConfig{},
	}
	serializedConfig, err := writeYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(serializedConfig) != expectedSerializedNodeConfig {
		t.Errorf("Diff:\n-------------\n%s", util.StringDiff(string(serializedConfig), expectedSerializedNodeConfig))
	}
}

type AdmissionPluginTestConfig struct {
	unversioned.TypeMeta
	Data string `json:"data"`
}

func (obj *AdmissionPluginTestConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

func TestMasterConfig(t *testing.T) {
	internal.Scheme.AddKnownTypes(v1.SchemeGroupVersion, &AdmissionPluginTestConfig{})
	internal.Scheme.AddKnownTypes(internal.SchemeGroupVersion, &AdmissionPluginTestConfig{})
	config := &internal.MasterConfig{
		ServingInfo: internal.HTTPServingInfo{
			ServingInfo: internal.ServingInfo{
				NamedCertificates: []internal.NamedCertificate{{}},
			},
		},
		KubernetesMasterConfig: &internal.KubernetesMasterConfig{
			AdmissionConfig: internal.AdmissionConfig{
				PluginConfig: map[string]internal.AdmissionPluginConfig{ // test config as an embedded object
					"plugin": {
						Configuration: &AdmissionPluginTestConfig{},
					},
				},
				PluginOrderOverride: []string{"plugin"}, // explicitly set this field because it's omitempty
			},
		},
		EtcdConfig: &internal.EtcdConfig{},
		OAuthConfig: &internal.OAuthConfig{
			IdentityProviders: []internal.IdentityProvider{
				{Provider: &internal.BasicAuthPasswordIdentityProvider{}},
				{Provider: &internal.AllowAllPasswordIdentityProvider{}},
				{Provider: &internal.DenyAllPasswordIdentityProvider{}},
				{Provider: &internal.HTPasswdPasswordIdentityProvider{}},
				{Provider: &internal.LDAPPasswordIdentityProvider{}},
				{Provider: &internal.RequestHeaderIdentityProvider{}},
				{Provider: &internal.KeystonePasswordIdentityProvider{}},
				{Provider: &internal.GitHubIdentityProvider{}},
				{Provider: &internal.GitLabIdentityProvider{}},
				{Provider: &internal.GoogleIdentityProvider{}},
				{Provider: &internal.OpenIDIdentityProvider{}},
			},
			SessionConfig: &internal.SessionConfig{},
			Templates:     &internal.OAuthTemplates{},
		},
		AssetConfig: &internal.AssetConfig{
			Extensions: []internal.AssetExtensionsConfig{{}},
		},
		DNSConfig: &internal.DNSConfig{},
		AdmissionConfig: internal.AdmissionConfig{
			PluginConfig: map[string]internal.AdmissionPluginConfig{ // test config as an embedded object
				"plugin": {
					Configuration: &AdmissionPluginTestConfig{},
				},
			},
			PluginOrderOverride: []string{"plugin"}, // explicitly set this field because the it's omitempty
		},
	}
	serializedConfig, err := writeYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(serializedConfig) != expectedSerializedMasterConfig {
		t.Errorf("Diff:\n-------------\n%s", util.StringDiff(string(serializedConfig), expectedSerializedMasterConfig))
	}

}

func writeYAML(obj runtime.Object) ([]byte, error) {

	json, err := runtime.Encode(serializer.NewCodecFactory(internal.Scheme).LegacyCodec(v1.SchemeGroupVersion), obj)
	if err != nil {
		return nil, err
	}

	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, err
}
