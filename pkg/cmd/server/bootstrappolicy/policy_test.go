package bootstrappolicy_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"

	"github.com/openshift/origin/pkg/api/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestOpenshiftRoles(t *testing.T) {
	roles := bootstrappolicy.GetBootstrapOpenshiftRoles("openshift")
	list := &api.List{}
	for i := range roles {
		list.Items = append(list.Items, &roles[i])
	}
	testObjects(t, list, "bootstrap_openshift_roles.yaml")
}

func TestBootstrapProjectRoleBindings(t *testing.T) {
	roleBindings := bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings("myproject")
	list := &api.List{}
	for i := range roleBindings {
		list.Items = append(list.Items, &roleBindings[i])
	}
	testObjects(t, list, "bootstrap_service_account_project_role_bindings.yaml")
}

func TestBootstrapClusterRoleBindings(t *testing.T) {
	roleBindings := bootstrappolicy.GetBootstrapClusterRoleBindings()
	list := &api.List{}
	for i := range roleBindings {
		list.Items = append(list.Items, &roleBindings[i])
	}
	testObjects(t, list, "bootstrap_cluster_role_bindings.yaml")
}

func TestBootstrapClusterRoles(t *testing.T) {
	roles := bootstrappolicy.GetBootstrapClusterRoles()
	list := &api.List{}
	for i := range roles {
		list.Items = append(list.Items, &roles[i])
	}
	testObjects(t, list, "bootstrap_cluster_roles.yaml")
}

func testObjects(t *testing.T, list *api.List, fixtureFilename string) {
	filename := filepath.Join("../../../../test/testdata/bootstrappolicy", fixtureFilename)
	expectedYAML, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	if err := runtime.EncodeList(api.Codecs.LegacyCodec(v1.SchemeGroupVersion), list.Items); err != nil {
		t.Fatal(err)
	}

	jsonData, err := runtime.Encode(api.Codecs.LegacyCodec(v1.SchemeGroupVersion), list)
	if err != nil {
		t.Fatal(err)
	}
	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		t.Fatal(err)
	}
	if string(yamlData) != string(expectedYAML) {
		t.Errorf("Bootstrap policy data does not match the test fixture in %s", filename)

		const updateEnvVar = "UPDATE_BOOTSTRAP_POLICY_FIXTURE_DATA"
		if os.Getenv(updateEnvVar) == "true" {
			if err := ioutil.WriteFile(filename, []byte(yamlData), os.FileMode(0755)); err == nil {
				t.Logf("Updated data in %s", filename)
				t.Logf("Verify the diff, commit changes, and rerun the tests")
			} else {
				t.Logf("Could not update data in %s: %v", filename, err)
			}
		} else {
			t.Logf("Diff between bootstrap data and fixture data in %s:\n-------------\n%s", filename, diff.StringDiff(string(yamlData), string(expectedYAML)))
			t.Logf("If the change is expected, re-run with %s=true to update the fixtures", updateEnvVar)
		}
	}
}

// Some roles should always cover others
func TestCovers(t *testing.T) {
	allRoles := bootstrappolicy.GetBootstrapClusterRoles()
	var admin *authorizationapi.ClusterRole
	var editor *authorizationapi.ClusterRole
	var viewer *authorizationapi.ClusterRole
	var registryAdmin *authorizationapi.ClusterRole
	var registryEditor *authorizationapi.ClusterRole
	var registryViewer *authorizationapi.ClusterRole
	var systemMaster *authorizationapi.ClusterRole
	var systemDiscovery *authorizationapi.ClusterRole
	var clusterAdmin *authorizationapi.ClusterRole
	var storageAdmin *authorizationapi.ClusterRole

	for i := range allRoles {
		role := allRoles[i]
		switch role.Name {
		case bootstrappolicy.AdminRoleName:
			admin = &role
		case bootstrappolicy.EditRoleName:
			editor = &role
		case bootstrappolicy.ViewRoleName:
			viewer = &role
		case bootstrappolicy.RegistryAdminRoleName:
			registryAdmin = &role
		case bootstrappolicy.RegistryEditorRoleName:
			registryEditor = &role
		case bootstrappolicy.RegistryViewerRoleName:
			registryViewer = &role
		case bootstrappolicy.MasterRoleName:
			systemMaster = &role
		case bootstrappolicy.DiscoveryRoleName:
			systemDiscovery = &role
		case bootstrappolicy.ClusterAdminRoleName:
			clusterAdmin = &role
		case bootstrappolicy.StorageAdminRoleName:
			storageAdmin = &role
		}
	}

	if covers, miss := rulevalidation.Covers(admin.Rules, editor.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(admin.Rules, editor.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(admin.Rules, viewer.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(admin.Rules, registryAdmin.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(clusterAdmin.Rules, storageAdmin.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(registryAdmin.Rules, registryEditor.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(registryAdmin.Rules, registryViewer.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}

	// Make sure we can auto-reconcile discovery
	if covers, miss := rulevalidation.Covers(systemMaster.Rules, systemDiscovery.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	// Make sure the master has full permissions
	if covers, miss := rulevalidation.Covers(systemMaster.Rules, clusterAdmin.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
}
