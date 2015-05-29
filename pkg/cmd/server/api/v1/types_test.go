package v1

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/ghodss/yaml"

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
kind: NodeConfig
masterKubeConfig: ""
networkPluginName: ""
nodeName: ""
podManifestConfig:
  fileCheckIntervalSeconds: 0
  path: ""
servingInfo:
  bindAddress: ""
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
	expectedSerializedMasterConfig = `apiVersion: v1
assetConfig:
  logoutURL: ""
  masterPublicURL: ""
  publicURL: ""
  servingInfo:
    bindAddress: ""
    certFile: ""
    clientCA: ""
    keyFile: ""
corsAllowedOrigins: null
dnsConfig:
  bindAddress: ""
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
    certFile: ""
    clientCA: ""
    keyFile: ""
  servingInfo:
    bindAddress: ""
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
  masterCount: 0
  masterIP: ""
  schedulerConfigFile: ""
  servicesSubnet: ""
  staticNodeNames: null
masterClients:
  deployerKubeConfig: ""
  externalKubernetesKubeConfig: ""
  openshiftLoopbackKubeConfig: ""
masterPublicURL: ""
networkConfig:
  clusterNetworkCIDR: ""
  hostSubnetLength: 0
  networkPluginName: ""
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
      clientCA: ""
      headers: null
      kind: RequestHeaderIdentityProvider
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
  tokenConfig:
    accessTokenMaxAgeSeconds: 0
    authorizeTokenMaxAgeSeconds: 0
policyConfig:
  bootstrapPolicyFile: ""
  openshiftSharedResourcesNamespace: ""
projectConfig:
  defaultNodeSelector: ""
  projectRequestMessage: ""
  projectRequestTemplate: ""
serviceAccountConfig:
  managedNames: null
  privateKeyFile: ""
  publicKeyFiles: null
servingInfo:
  bindAddress: ""
  certFile: ""
  clientCA: ""
  keyFile: ""
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
