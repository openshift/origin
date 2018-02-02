package v1_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"

	internal "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	testtypes "github.com/openshift/origin/pkg/cmd/server/apis/config/v1/testing"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

func TestSerializeNodeConfig(t *testing.T) {
	config := &internal.NodeConfig{
		PodManifestConfig: &internal.PodManifestConfig{},
	}
	serializedConfig, err := writeYAML(config)
	if err != nil {
		t.Fatal(err)
	}

	filename := "testdata/node-config.yaml"
	expectedSerializedNodeConfig, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(serializedConfig) != string(expectedSerializedNodeConfig) {
		const updateEnvVar = "UPDATE_NODE_CONFIG_FIXTURE_DATA"
		if os.Getenv(updateEnvVar) == "true" {
			if err := ioutil.WriteFile(filename, []byte(serializedConfig), os.FileMode(0755)); err == nil {
				t.Logf("Updated data in %s", filename)
				t.Logf("Verify the diff, commit changes, and rerun the tests")
			} else {
				t.Logf("Could not update data in %s: %v", filename, err)
			}
		} else {
			t.Logf("Diff between serialized config and fixture data in %s:\n-------------\n%s", filename, diff.StringDiff(string(serializedConfig), string(expectedSerializedNodeConfig)))
			t.Logf("If the change is expected, re-run with %s=true to update the fixtures", updateEnvVar)
			t.Logf("Be sure to open corresponding issues:")
			t.Logf("- documentation: https://github.com/openshift/openshift-docs/")
			t.Logf("- install: https://github.com/openshift/openshift-ansible/")
			t.Logf("If the new field(s) reference files, be sure to add them to GetNodeFileReferences()")
			t.Error("Mismatched config")
		}
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

func TestMasterConfig(t *testing.T) {
	internal.Scheme.AddKnownTypes(v1.SchemeGroupVersion, &testtypes.AdmissionPluginTestConfig{})
	internal.Scheme.AddKnownTypes(internal.SchemeGroupVersion, &testtypes.AdmissionPluginTestConfig{})
	config := &internal.MasterConfig{
		ServingInfo: internal.HTTPServingInfo{
			ServingInfo: internal.ServingInfo{
				NamedCertificates: []internal.NamedCertificate{{}},
			},
		},
		KubernetesMasterConfig: &internal.KubernetesMasterConfig{
			AdmissionConfig: internal.AdmissionConfig{
				PluginConfig: map[string]*internal.AdmissionPluginConfig{ // test config as an embedded object
					"plugin": {
						Configuration: &testtypes.AdmissionPluginTestConfig{},
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
		DNSConfig: &internal.DNSConfig{},
		AdmissionConfig: internal.AdmissionConfig{
			PluginConfig: map[string]*internal.AdmissionPluginConfig{ // test config as an embedded object
				"plugin": {
					Configuration: &testtypes.AdmissionPluginTestConfig{},
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

	filename := "testdata/master-config.yaml"
	expectedSerializedMasterConfig, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(serializedConfig) != string(expectedSerializedMasterConfig) {
		const updateEnvVar = "UPDATE_MASTER_CONFIG_FIXTURE_DATA"
		if os.Getenv(updateEnvVar) == "true" {
			if err := ioutil.WriteFile(filename, []byte(serializedConfig), os.FileMode(0755)); err == nil {
				t.Logf("Updated data in %s", filename)
				t.Logf("Verify the diff, commit changes, and rerun the tests")
			} else {
				t.Logf("Could not update data in %s: %v", filename, err)
			}
		} else {
			t.Logf("Diff between serialized config and fixture data in %s:\n-------------\n%s", filename, diff.StringDiff(string(serializedConfig), string(expectedSerializedMasterConfig)))
			t.Logf("If the change is expected, re-run with %s=true to update the fixtures", updateEnvVar)
			t.Logf("Be sure to open corresponding issues:")
			t.Logf("- documentation: https://github.com/openshift/openshift-docs/")
			t.Logf("- install: https://github.com/openshift/openshift-ansible/")
			t.Logf("If the new field(s) reference files, be sure to add them to GetMasterFileReferences()")
			t.Error("Mismatched config")
		}
	}
}

func writeYAML(obj runtime.Object) ([]byte, error) {
	// Round-trip to pick up defaults
	externalObj, err := internal.Scheme.ConvertToVersion(obj, v1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	internal.Scheme.Default(externalObj)
	internalObj, err := internal.Scheme.ConvertToVersion(externalObj, internal.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}

	json, err := runtime.Encode(serializer.NewCodecFactory(internal.Scheme).LegacyCodec(v1.SchemeGroupVersion), internalObj)
	if err != nil {
		return nil, err
	}

	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, err
}
