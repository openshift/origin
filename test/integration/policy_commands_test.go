// +build integration,etcd

package integration

import (
	"io/ioutil"
	"testing"

	testutil "github.com/openshift/origin/test/util"

	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
	policy "github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func TestPolicyCommands(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const projectName = "hammer-project"

	haroldClient, err := testutil.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, "harold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	addViewer := policy.RoleModificationOptions{
		RoleName:            bootstrappolicy.ViewRoleName,
		RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(projectName, haroldClient),
		Users:               []string{"valerie"},
		Groups:              []string{"my-group"},
	}

	if err := addViewer.AddRole(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewers, err := haroldClient.RoleBindings(projectName).Get("view")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding := authorizationinterfaces.NewLocalRoleBindingAdapter(viewers)
	if !binding.Users().Has("valerie") {
		t.Errorf("expected valerie in users: %v", binding.Users())
	}
	if !binding.Groups().Has("my-group") {
		t.Errorf("expected my-group in groups: %v", binding.Groups())
	}

	removeValerie := policy.RemoveFromProjectOptions{
		BindingNamespace: projectName,
		Client:           haroldClient,
		Users:            []string{"valerie"},
		Out:              ioutil.Discard,
	}
	if err := removeValerie.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewers, err = haroldClient.RoleBindings(projectName).Get("view")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = authorizationinterfaces.NewLocalRoleBindingAdapter(viewers)
	if binding.Users().Has("valerie") {
		t.Errorf("unexpected valerie in users: %v", binding.Users())
	}
	if !binding.Groups().Has("my-group") {
		t.Errorf("expected my-group in groups: %v", binding.Groups())
	}

	removeMyGroup := policy.RemoveFromProjectOptions{
		BindingNamespace: projectName,
		Client:           haroldClient,
		Groups:           []string{"my-group"},
		Out:              ioutil.Discard,
	}
	if err := removeMyGroup.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewers, err = haroldClient.RoleBindings(projectName).Get("view")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	binding = authorizationinterfaces.NewLocalRoleBindingAdapter(viewers)
	if binding.Users().Has("valerie") {
		t.Errorf("unexpected valerie in users: %v", binding.Users())
	}
	if binding.Groups().Has("my-group") {
		t.Errorf("unexpected my-group in groups: %v", binding.Groups())
	}

}
