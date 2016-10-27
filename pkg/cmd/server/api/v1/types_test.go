package v1_test

import (
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/util/diff"

	internal "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
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
enableUnidling: false
imageConfig:
  format: ""
  latest: false
iptablesSyncPeriod: ""
kind: NodeConfig
masterClientConnectionOverrides: null
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
volumeConfig:
  localQuota:
    perFSGroup: null
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
  extensionProperties: null
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
auditConfig:
  auditFilePath: ""
  enabled: false
  maximumFileRetentionDays: 0
  maximumFileSizeMegabytes: 0
  maximumRetainedFiles: 0
controllerConfig:
  serviceServingCert:
    signer: null
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
jenkinsPipelineConfig:
  autoProvisionEnabled: null
  parameters: null
  serviceName: ""
  templateName: ""
  templateNamespace: ""
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
  schedulerArguments: null
  schedulerConfigFile: ""
  servicesNodePortRange: ""
  servicesSubnet: ""
  staticNodeNames: null
masterClients:
  externalKubernetesClientConnectionOverrides: null
  externalKubernetesKubeConfig: ""
  openshiftLoopbackClientConnectionOverrides: null
  openshiftLoopbackKubeConfig: ""
masterPublicURL: ""
networkConfig:
  clusterNetworkCIDR: ""
  externalIPNetworkCIDRs: null
  hostSubnetLength: 0
  ingressIPNetworkCIDR: ""
  networkPluginName: ""
  serviceNetworkCIDR: ""
oauthConfig:
  alwaysShowProviderSelection: false
  assetPublicURL: ""
  grantConfig:
    method: ""
    serviceAccountMethod: ""
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
      attributes:
        email: null
        id: null
        name: null
        preferredUsername: null
      bindDN: ""
      bindPassword:
        env: ""
        file: filename
        keyFile: ""
        value: ""
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
      clientCommonNames: null
      emailHeaders: null
      headers: null
      kind: RequestHeaderIdentityProvider
      loginURL: ""
      nameHeaders: null
      preferredUsernameHeaders: null
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
      clientID: ""
      clientSecret:
        env: ""
        file: filename
        keyFile: ""
        value: ""
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
      ca: ""
      clientID: ""
      clientSecret:
        env: ""
        file: filename
        keyFile: ""
        value: ""
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
      clientID: ""
      clientSecret:
        env: ""
        file: filename
        keyFile: ""
        value: ""
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
      clientSecret:
        env: ""
        file: filename
        keyFile: ""
        value: ""
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
  userAgentMatchingConfig:
    defaultRejectionMessage: ""
    deniedClients: null
    requiredClients: null
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
volumeConfig:
  dynamicProvisioningEnabled: false
`
)

func TestSerializeNodeConfig(t *testing.T) {
	config := &internal.NodeConfig{
		PodManifestConfig: &internal.PodManifestConfig{},
	}
	serializedConfig, err := writeYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(serializedConfig) != expectedSerializedNodeConfig {
		t.Errorf("Diff:\n-------------\n%s", diff.StringDiff(string(serializedConfig), expectedSerializedNodeConfig))
	}
}

func TestReadNodeConfigLocalVolumeDirQuota(t *testing.T) {

	tests := map[string]struct {
		config   string
		expected string
	}{
		"null quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: null
`,
			expected: "",
		},
		"missing quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
`,
			expected: "",
		},
		"missing localQuota": {
			config: `
apiVersion: v1
volumeConfig:
`,
			expected: "",
		},
		"missing volumeConfig": {
			config: `
apiVersion: v1
`,
			expected: "",
		},
		"no unit (bytes) quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: 200000
`,
			expected: "200k",
		},
		"Kb quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: 200Ki
`,
			expected: "200Ki",
		},
		"Mb quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: 512Mi
`,
			expected: "512Mi",
		},
		"Gb quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: 2Gi
`,
			expected: "2Gi",
		},
		"Tb quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: 2Ti
`,
			expected: "2Ti",
		},
		// This is invalid config, would be caught by validation but just
		// testing it parses ok:
		"negative quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: -512Mi
`,
			expected: "-512Mi",
		},
		"zero quota": {
			config: `
apiVersion: v1
volumeConfig:
  localQuota:
    perFSGroup: 0
`,
			expected: "0",
		},
	}

	for name, test := range tests {
		t.Logf("Running test: %s", name)
		nodeConfig := &internal.NodeConfig{}
		if err := latest.ReadYAMLInto([]byte(test.config), nodeConfig); err != nil {
			t.Errorf("Error reading yaml: %s", err.Error())
		}
		if test.expected == "" && nodeConfig.VolumeConfig.LocalQuota.PerFSGroup != nil {
			t.Errorf("Expected empty quota but got: %v", nodeConfig.VolumeConfig.LocalQuota.PerFSGroup)
		}
		if test.expected != "" {
			if nodeConfig.VolumeConfig.LocalQuota.PerFSGroup == nil {
				t.Errorf("Expected quota: %s, got: nil", test.expected)
			} else {
				amount := nodeConfig.VolumeConfig.LocalQuota.PerFSGroup
				t.Logf("%s", amount.String())
				if test.expected != amount.String() {
					t.Errorf("Expected quota: %s, got: %s", test.expected, amount.String())
				}
			}
		}
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
				{Provider: &internal.LDAPPasswordIdentityProvider{BindPassword: internal.StringSource{StringSourceSpec: internal.StringSourceSpec{File: "filename"}}}},
				{Provider: &internal.RequestHeaderIdentityProvider{}},
				{Provider: &internal.KeystonePasswordIdentityProvider{}},
				{Provider: &internal.GitHubIdentityProvider{}},
				{Provider: &internal.GitHubIdentityProvider{ClientSecret: internal.StringSource{StringSourceSpec: internal.StringSourceSpec{File: "filename"}}}},
				{Provider: &internal.GitLabIdentityProvider{}},
				{Provider: &internal.GitLabIdentityProvider{ClientSecret: internal.StringSource{StringSourceSpec: internal.StringSourceSpec{File: "filename"}}}},
				{Provider: &internal.GoogleIdentityProvider{}},
				{Provider: &internal.GoogleIdentityProvider{ClientSecret: internal.StringSource{StringSourceSpec: internal.StringSourceSpec{File: "filename"}}}},
				{Provider: &internal.OpenIDIdentityProvider{}},
				{Provider: &internal.OpenIDIdentityProvider{ClientSecret: internal.StringSource{StringSourceSpec: internal.StringSourceSpec{File: "filename"}}}},
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
			PluginOrderOverride: []string{"plugin"}, // explicitly set this field because it's omitempty
		},
		VolumeConfig: internal.MasterVolumeConfig{
			DynamicProvisioningEnabled: false,
		},
	}
	serializedConfig, err := writeYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	if string(serializedConfig) != expectedSerializedMasterConfig {
		t.Errorf("Diff:\n-------------\n%s", diff.StringDiff(string(serializedConfig), expectedSerializedMasterConfig))
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
