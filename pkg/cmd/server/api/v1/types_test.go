package v1

import (
	"testing"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"

	internal "github.com/openshift/origin/pkg/cmd/server/api"
)

const (
	// This constant lists all possible options for the node config file in v1
	// Before modifying this constant, ensure any changes have corresponding issues filed for:
	// - documentation: https://github.com/openshift/openshift-docs/
	// - install: https://github.com/openshift/openshift-ansible/
	expectedSerializedNodeConfig = `allowDisabledDocker: false
apiVersion: v1
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
volumeDirectory: ""
`

	// This constant lists all possible options for the master config file in v1.
	// It also includes the fields for all the identity provider types.
	// Before modifying this constant, ensure any changes have corresponding issues filed for:
	// - documentation: https://github.com/openshift/openshift-docs/
	// - install: https://github.com/openshift/openshift-ansible/
	expectedSerializedMasterConfig = `apiLevels: null
apiVersion: v1
assetConfig:
  extensionDevelopment: false
  extensionScripts: null
  extensionStylesheets: null
  extensions: null
  logoutURL: ""
  masterPublicURL: ""
  publicURL: ""
  servingInfo:
    bindAddress: ""
    bindNetwork: ""
    certFile: ""
    clientCA: ""
    keyFile: ""
    maxRequestsInFlight: 0
    requestTimeoutSeconds: 0
controllerLeaseTTL: 0
controllers: ""
corsAllowedOrigins: null
disabledFeatures: null
dnsConfig:
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
  servingInfo:
    bindAddress: ""
    bindNetwork: ""
    certFile: ""
    clientCA: ""
    keyFile: ""
  storageDirectory: ""
etcdStorageConfig:
  kubernetesStoragePrefix: ""
  kubernetesStorageVersion: ""
  openShiftStoragePrefix: ""
  openShiftStorageVersion: ""
imageConfig:
  format: ""
  latest: false
kind: MasterConfig
kubeletClientInfo:
  ca: ""
  certFile: ""
  keyFile: ""
  port: 0
kubernetesMasterConfig:
  apiLevels: null
  apiServerArguments: null
  controllerArguments: null
  masterCount: 0
  masterIP: ""
  podEvictionTimeout: ""
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
  assetPublicURL: ""
  grantConfig:
    method: ""
  identityProviders:
  - challenge: false
    login: false
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
    name: ""
    provider:
      apiVersion: v1
      kind: AllowAllPasswordIdentityProvider
  - challenge: false
    login: false
    name: ""
    provider:
      apiVersion: v1
      kind: DenyAllPasswordIdentityProvider
  - challenge: false
    login: false
    name: ""
    provider:
      apiVersion: v1
      file: ""
      kind: HTPasswdPasswordIdentityProvider
  - challenge: false
    login: false
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
    name: ""
    provider:
      apiVersion: v1
      clientID: ""
      clientSecret: ""
      kind: GitHubIdentityProvider
  - challenge: false
    login: false
    name: ""
    provider:
      apiVersion: v1
      clientID: ""
      clientSecret: ""
      hostedDomain: ""
      kind: GoogleIdentityProvider
  - challenge: false
    login: false
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
  masterPublicURL: ""
  masterURL: ""
  sessionConfig:
    sessionMaxAgeSeconds: 0
    sessionName: ""
    sessionSecretsFile: ""
  templates: null
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

func TestMasterConfig(t *testing.T) {
	config := &internal.MasterConfig{
		KubernetesMasterConfig: &internal.KubernetesMasterConfig{},
		EtcdConfig:             &internal.EtcdConfig{},
		OAuthConfig: &internal.OAuthConfig{
			IdentityProviders: []internal.IdentityProvider{
				{Provider: runtime.EmbeddedObject{Object: &internal.BasicAuthPasswordIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.AllowAllPasswordIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.DenyAllPasswordIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.HTPasswdPasswordIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.LDAPPasswordIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.RequestHeaderIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.GitHubIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.GoogleIdentityProvider{}}},
				{Provider: runtime.EmbeddedObject{Object: &internal.OpenIDIdentityProvider{}}},
			},
			SessionConfig: &internal.SessionConfig{},
		},
		AssetConfig: &internal.AssetConfig{},
		DNSConfig:   &internal.DNSConfig{},
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
	json, err := Codec.Encode(obj)
	if err != nil {
		return nil, err
	}

	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, err
}
