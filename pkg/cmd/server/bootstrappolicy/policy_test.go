package bootstrappolicy_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/api"
	securityapiv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

var (
	fileEncodingCodecFactory serializer.CodecFactory
	scheme                   = runtime.NewScheme()
)

func init() {
	api.Install(scheme)
	api.InstallKube(scheme)
	fileEncodingCodecFactory = serializer.NewCodecFactory(scheme)
}

func TestBootstrapNamespaceRoles(t *testing.T) {
	allRoles := bootstrappolicy.NamespaceRoles()
	list := &corev1.List{}
	// enforce a strict ordering
	for _, namespace := range sets.StringKeySet(allRoles).List() {
		roles := allRoles[namespace]
		for i := range roles {
			roles[i].SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("Role"))
			list.Items = append(list.Items, runtime.RawExtension{Object: &roles[i]})
		}
	}
	testObjects(t, list, "bootstrap_namespace_roles.yaml")
}

func TestGetBootstrapNamespaceRoleBindings(t *testing.T) {
	allRoleBindings := bootstrappolicy.NamespaceRoleBindings()
	list := &corev1.List{}
	// enforce a strict ordering
	for _, namespace := range sets.StringKeySet(allRoleBindings).List() {
		roleBindings := allRoleBindings[namespace]
		for i := range roleBindings {
			roleBindings[i].SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
			list.Items = append(list.Items, runtime.RawExtension{Object: &roleBindings[i]})
		}
	}
	testObjects(t, list, "bootstrap_namespace_role_bindings.yaml")
}

func TestBootstrapProjectRoleBindings(t *testing.T) {
	roleBindings := bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings("myproject")
	list := &corev1.List{}
	for i := range roleBindings {
		roleBindings[i].SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
		list.Items = append(list.Items, runtime.RawExtension{Object: &roleBindings[i]})
	}
	testObjects(t, list, "bootstrap_service_account_project_role_bindings.yaml")
}

func TestBootstrapClusterRoleBindings(t *testing.T) {
	roleBindings := bootstrappolicy.GetBootstrapClusterRoleBindings()
	list := &corev1.List{}
	for i := range roleBindings {
		roleBindings[i].SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))
		list.Items = append(list.Items, runtime.RawExtension{Object: &roleBindings[i]})
	}
	testObjects(t, list, "bootstrap_cluster_role_bindings.yaml")
}

func TestBootstrapClusterRoles(t *testing.T) {
	roles := bootstrappolicy.GetBootstrapClusterRoles()
	list := &corev1.List{}
	for i := range roles {
		roles[i].SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"))
		list.Items = append(list.Items, runtime.RawExtension{Object: &roles[i]})
	}
	testObjects(t, list, "bootstrap_cluster_roles.yaml")
}

func TestBootstrapSCCs(t *testing.T) {
	ns := bootstrappolicy.DefaultOpenShiftInfraNamespace
	bootstrapSCCGroups, bootstrapSCCUsers := bootstrappolicy.GetBoostrapSCCAccess(ns)
	sccs := bootstrappolicy.GetBootstrapSecurityContextConstraints(bootstrapSCCGroups, bootstrapSCCUsers)
	list := &corev1.List{}
	for i := range sccs {
		sccs[i].SetGroupVersionKind(securityapiv1.SchemeGroupVersion.WithKind("SecurityContextConstraints"))
		list.Items = append(list.Items, runtime.RawExtension{Object: sccs[i]})
	}
	testObjects(t, list, "bootstrap_security_context_constraints.yaml")
}

func testObjects(t *testing.T, list *corev1.List, fixtureFilename string) {
	filename := filepath.Join("../../../../test/testdata/bootstrappolicy", fixtureFilename)
	expectedYAML, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	jsonData, err := runtime.Encode(fileEncodingCodecFactory.LegacyCodec(rbacv1.SchemeGroupVersion, securityapiv1.GroupVersion, legacy.GroupVersion), list)
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
