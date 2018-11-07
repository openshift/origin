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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	api "k8s.io/kubernetes/pkg/apis/core"

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
	allRoles := bootstrappolicy.NamespaceRoles()
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
	allRoleBindings := bootstrappolicy.NamespaceRoleBindings()
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
