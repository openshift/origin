// +build integration,!no-etcd

package integration

import (
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	groupscmd "github.com/openshift/origin/pkg/cmd/admin/groups"
	projectapi "github.com/openshift/origin/pkg/project/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	testutil "github.com/openshift/origin/test/util"
)

func TestBasicUserBasedGroupManipulation(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieOpenshiftClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// make sure we don't get back system groups
	firstValerie, err := clusterAdminClient.Users().Get("valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firstValerie.Groups) != 0 {
		t.Errorf("unexpected groups: %v", firstValerie.Groups)
	}

	// make sure that user/~ returns groups for unbacked users
	expectedClusterAdminGroups := []string{"system:cluster-admins"}
	clusterAdminUser, err := clusterAdminClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(clusterAdminUser.Groups, expectedClusterAdminGroups) {
		t.Errorf("expected %v, got %v", clusterAdminUser.Groups, expectedClusterAdminGroups)
	}

	valerieGroups := []string{"theGroup"}
	firstValerie.Groups = append(firstValerie.Groups, valerieGroups...)
	_, err = clusterAdminClient.Users().Update(firstValerie)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// make sure that user/~ doesn't get back system groups when it merges
	secondValerie, err := valerieOpenshiftClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(secondValerie.Groups, valerieGroups) {
		t.Errorf("expected %v, got %v", secondValerie.Groups, valerieGroups)
	}

	_, err = valerieOpenshiftClient.Projects().Get("empty")
	if err == nil {
		t.Fatalf("expected error")
	}

	emptyProject := &projectapi.Project{}
	emptyProject.Name = "empty"
	_, err = clusterAdminClient.Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Groups = util.NewStringSet(valerieGroups...)
	_, err = clusterAdminClient.RoleBindings("empty").Create(roleBinding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(valerieOpenshiftClient, "empty", "get", "pods", true); err != nil {
		t.Error(err)
	}

	// make sure that user groups are respected for policy
	_, err = valerieOpenshiftClient.Projects().Get("empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

}

func TestBasicGroupManipulation(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieOpenshiftClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	theGroup := &userapi.Group{}
	theGroup.Name = "thegroup"
	theGroup.Users = append(theGroup.Users, "valerie", "victor")
	_, err = clusterAdminClient.Groups().Create(theGroup)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = valerieOpenshiftClient.Projects().Get("empty")
	if err == nil {
		t.Fatalf("expected error")
	}

	emptyProject := &projectapi.Project{}
	emptyProject.Name = "empty"
	_, err = clusterAdminClient.Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Groups = util.NewStringSet(theGroup.Name)
	_, err = clusterAdminClient.RoleBindings("empty").Create(roleBinding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(valerieOpenshiftClient, "empty", "get", "pods", true); err != nil {
		t.Error(err)
	}

	// make sure that user groups are respected for policy
	_, err = valerieOpenshiftClient.Projects().Get("empty")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	victorOpenshiftClient, _, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "victor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = victorOpenshiftClient.Projects().Get("empty")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGroupCommands(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newGroup := &groupscmd.NewGroupOptions{clusterAdminClient.Groups(), "group1", []string{"first", "second", "third", "first"}}
	if err := newGroup.AddGroup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err := clusterAdminClient.Groups().Get("group1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "second", "third"}, group1.Users; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

	modifyUsers := &groupscmd.GroupModificationOptions{clusterAdminClient.Groups(), "group1", []string{"second", "fourth", "fifth"}}
	if err := modifyUsers.AddUsers(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err = clusterAdminClient.Groups().Get("group1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "second", "third", "fourth", "fifth"}, group1.Users; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

	if err := modifyUsers.RemoveUsers(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err = clusterAdminClient.Groups().Get("group1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "third"}, group1.Users; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

}
