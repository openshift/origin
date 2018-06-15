package integration

import (
	"io"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	groupscmd "github.com/openshift/origin/pkg/oc/admin/groups"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestBasicUserBasedGroupManipulation(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminUserClient := userclient.NewForConfigOrDie(clusterAdminClientConfig)

	valerieKubeClient, valerieConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	valerieProjectClient := projectclient.NewForConfigOrDie(valerieConfig).Project()

	// make sure we don't get back system groups
	firstValerie, err := clusterAdminUserClient.Users().Get("valerie", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(firstValerie.Groups) != 0 {
		t.Errorf("unexpected groups: %v", firstValerie.Groups)
	}

	// make sure that user/~ returns groups for unbacked users
	expectedClusterAdminGroups := []string{"system:cluster-admins", "system:masters"}
	clusterAdminUser, err := clusterAdminUserClient.Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(clusterAdminUser.Groups, expectedClusterAdminGroups) {
		t.Errorf("expected %v, got %v", clusterAdminUser.Groups, expectedClusterAdminGroups)
	}

	valerieGroups := []string{"theGroup"}
	firstValerie.Groups = append(firstValerie.Groups, valerieGroups...)
	_, err = clusterAdminUserClient.Users().Update(firstValerie)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// make sure that user/~ doesn't get back system groups when it merges
	secondValerie, err := userclient.NewForConfigOrDie(valerieConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(secondValerie.Groups, valerieGroups) {
		t.Errorf("expected %v, got %v", secondValerie.Groups, valerieGroups)
	}

	_, err = valerieProjectClient.Projects().Get("empty", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	}

	emptyProject := &projectapi.Project{}
	emptyProject.Name = "empty"
	_, err = projectclient.NewForConfigOrDie(clusterAdminClientConfig).Project().Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Subjects = authorizationapi.BuildSubjects([]string{}, valerieGroups)
	_, err = authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization().RoleBindings("empty").Create(roleBinding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(valerieKubeClient.Authorization(), "empty", "get", kapi.Resource("pods"), true); err != nil {
		t.Error(err)
	}

	// make sure that user groups are respected for policy
	_, err = valerieProjectClient.Projects().Get("empty", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

}

func TestBasicGroupManipulation(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	valerieKubeClient, valerieConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	valerieProjectClient := projectclient.NewForConfigOrDie(valerieConfig).Project()

	theGroup := &userapi.Group{}
	theGroup.Name = "thegroup"
	theGroup.Users = append(theGroup.Users, "valerie", "victor")
	_, err = userclient.NewForConfigOrDie(clusterAdminClientConfig).Groups().Create(theGroup)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = valerieProjectClient.Projects().Get("empty", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	}

	emptyProject := &projectapi.Project{}
	emptyProject.Name = "empty"
	_, err = projectclient.NewForConfigOrDie(clusterAdminClientConfig).Project().Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationapi.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Subjects = authorizationapi.BuildSubjects([]string{}, []string{theGroup.Name})
	_, err = authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization().RoleBindings("empty").Create(roleBinding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(valerieKubeClient.Authorization(), "empty", "get", kapi.Resource("pods"), true); err != nil {
		t.Error(err)
	}

	// make sure that user groups are respected for policy
	_, err = valerieProjectClient.Projects().Get("empty", metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, victorConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "victor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = projectclient.NewForConfigOrDie(victorConfig).Project().Projects().Get("empty", metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGroupCommands(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userClient := userclient.NewForConfigOrDie(clusterAdminClientConfig)

	newGroup := &groupscmd.NewGroupOptions{
		GroupClient: userClient.Groups(),
		Group:       "group1",
		Users:       []string{"first", "second", "third", "first"},
		Printer: func(runtime.Object, io.Writer) error {
			return nil
		},
	}
	if err := newGroup.AddGroup(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err := userClient.Groups().Get("group1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "second", "third"}, group1.Users; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

	modifyUsers := &groupscmd.GroupModificationOptions{
		GroupClient: userClient.Groups(),
		Group:       "group1",
		Users:       []string{"second", "fourth", "fifth"},
	}
	if err := modifyUsers.AddUsers(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err = userClient.Groups().Get("group1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "second", "third", "fourth", "fifth"}, group1.Users; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

	if err := modifyUsers.RemoveUsers(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err = userClient.Groups().Get("group1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "third"}, group1.Users; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

}
