package integration

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"

	"github.com/openshift/oc/pkg/cli/admin/policy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestPolicyCommands(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const projectName = "hammer-project"

	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	haroldAuthorizationClient := rbacv1client.NewForConfigOrDie(haroldConfig)

	addViewer := policy.RoleModificationOptions{
		RoleBindingNamespace: projectName,
		RoleName:             "view",
		RoleKind:             "ClusterRole",
		RbacClient:           haroldAuthorizationClient,
		Users:                []string{"valerie"},
		Groups:               []string{"my-group"},
		PrintFlags:           genericclioptions.NewPrintFlags(""),
		ToPrinter:            func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },
	}

	if err := addViewer.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewers, err := haroldAuthorizationClient.RoleBindings(projectName).Get("view", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	users, groups, _, _ := rbacv1helpers.SubjectsStrings(viewers.Subjects)
	if !sets.NewString(users...).Has("valerie") {
		t.Errorf("expected valerie in users: %v", users)
	}
	if !sets.NewString(groups...).Has("my-group") {
		t.Errorf("expected my-group in groups: %v", groups)
	}

	removeValerie := policy.RemoveFromProjectOptions{
		BindingNamespace: projectName,
		Client:           haroldAuthorizationClient,
		Users:            []string{"valerie"},
		IOStreams:        genericclioptions.NewTestIOStreamsDiscard(),
	}
	if err := removeValerie.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewers, err = haroldAuthorizationClient.RoleBindings(projectName).Get("view", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	users, groups, _, _ = rbacv1helpers.SubjectsStrings(viewers.Subjects)
	if sets.NewString(users...).Has("valerie") {
		t.Errorf("unexpected valerie in users: %v", users)
	}
	if !sets.NewString(groups...).Has("my-group") {
		t.Errorf("expected my-group in groups: %v", groups)
	}

	removeMyGroup := policy.RemoveFromProjectOptions{
		BindingNamespace: projectName,
		Client:           haroldAuthorizationClient,
		Groups:           []string{"my-group"},
		IOStreams:        genericclioptions.NewTestIOStreamsDiscard(),
	}
	if err := removeMyGroup.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the removal of the last subject caused the rolebinding to be
	// removed as well
	viewers, err = haroldAuthorizationClient.RoleBindings(projectName).Get("view", metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	users, groups, _, _ = rbacv1helpers.SubjectsStrings(viewers.Subjects)
	if sets.NewString(users...).Has("valerie") {
		t.Errorf("unexpected valerie in users: %v", users)
	}
	if sets.NewString(groups...).Has("my-group") {
		t.Errorf("unexpected my-group in groups: %v", groups)
	}

}
