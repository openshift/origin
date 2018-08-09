package bootstrappolicy_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	rulevalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	"github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

var (
	fileEncodingCodecFactory serializer.CodecFactory
)

func init() {
	scheme := runtime.NewScheme()
	install.InstallInternalOpenShift(scheme)
	install.InstallInternalKube(scheme)
	fileEncodingCodecFactory = serializer.NewCodecFactory(scheme)
}

func TestCreateBootstrapPolicyFile(t *testing.T) {
	f, err := ioutil.TempFile("", "TestCreateBootstrapPolicyFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	cmd := admin.NewCommandCreateBootstrapPolicyFile("", "", genericclioptions.NewTestIOStreamsDiscard())
	cmd.Flag("filename").Value.Set(f.Name())
	cmd.Run(cmd, nil)
	data, err := ioutil.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	list := &api.List{}
	if _, _, err := fileEncodingCodecFactory.UniversalDecoder().Decode(data, nil, list); err != nil {
		t.Fatal(err)
	}
	testObjects(t, list, "bootstrap_policy_file.yaml")
}

func TestBootstrapNamespaceRoles(t *testing.T) {
	allRoles := bootstrappolicy.GetBootstrapNamespaceRoles()
	list := &api.List{}
	// enforce a strict ordering
	for _, namespace := range sets.StringKeySet(allRoles).List() {
		roles := allRoles[namespace]
		for i := range roles {
			list.Items = append(list.Items, &roles[i])
		}
	}
	testObjects(t, list, "bootstrap_namespace_roles.yaml")
}

func TestGetBootstrapNamespaceRoleBindings(t *testing.T) {
	allRoleBindings := bootstrappolicy.GetBootstrapNamespaceRoleBindings()
	list := &api.List{}
	// enforce a strict ordering
	for _, namespace := range sets.StringKeySet(allRoleBindings).List() {
		roleBindings := allRoleBindings[namespace]
		for i := range roleBindings {
			list.Items = append(list.Items, &roleBindings[i])
		}
	}
	testObjects(t, list, "bootstrap_namespace_role_bindings.yaml")
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

	if err := runtime.EncodeList(fileEncodingCodecFactory.LegacyCodec(rbacv1.SchemeGroupVersion, legacy.GroupVersion), list.Items); err != nil {
		t.Fatal(err)
	}

	jsonData, err := runtime.Encode(fileEncodingCodecFactory.LegacyCodec(rbacv1.SchemeGroupVersion, legacy.GroupVersion), list)
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
	var admin []rbacv1.PolicyRule
	var editor []rbacv1.PolicyRule
	var viewer []rbacv1.PolicyRule
	var registryAdmin *rbacv1.ClusterRole
	var registryEditor *rbacv1.ClusterRole
	var registryViewer *rbacv1.ClusterRole
	var systemMaster *rbacv1.ClusterRole
	var systemDiscovery *rbacv1.ClusterRole
	var clusterAdmin *rbacv1.ClusterRole
	var storageAdmin *rbacv1.ClusterRole
	var imageBuilder *rbacv1.ClusterRole

	for i := range allRoles {
		role := allRoles[i]
		switch role.Name {
		case "system:openshift:aggregate-to-admin", "system:aggregate-to-admin":
			admin = append(admin, role.Rules...)
		case "system:openshift:aggregate-to-edit", "system:aggregate-to-edit":
			editor = append(editor, role.Rules...)
		case "system:openshift:aggregate-to-view", "system:aggregate-to-view":
			viewer = append(viewer, role.Rules...)
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
		case bootstrappolicy.ImageBuilderRoleName:
			imageBuilder = &role
		}
	}

	if covers, miss := rulevalidation.Covers(admin, editor); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(admin, editor); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(admin, viewer); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(admin, registryAdmin.Rules); !covers {
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

	// admin and editor should cover imagebuilder
	if covers, miss := rulevalidation.Covers(admin, imageBuilder.Rules); !covers {
		t.Errorf("failed to cover: %#v", miss)
	}
	if covers, miss := rulevalidation.Covers(editor, imageBuilder.Rules); !covers {
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
