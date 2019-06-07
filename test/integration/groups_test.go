package integration

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	groupsnewcmd "github.com/openshift/oc/pkg/cli/admin/groups/new"
	groupsuserscmd "github.com/openshift/oc/pkg/cli/admin/groups/users"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
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
	userValerie, err := clusterAdminUserClient.Users().Get("valerie", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(userValerie.Groups) != 0 {
		t.Errorf("unexpected groups: %v", userValerie.Groups)
	}

	// make sure that user/~ returns groups for unbacked users
	expectedClusterAdminGroups := []string{"system:authenticated", "system:cluster-admins", "system:masters"}
	clusterAdminUser, err := clusterAdminUserClient.Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(clusterAdminUser.Groups, expectedClusterAdminGroups) {
		t.Errorf("expected %v, got %v", expectedClusterAdminGroups, clusterAdminUser.Groups)
	}

	theGroup := &userapi.Group{}
	theGroup.Name = "theGroup"
	theGroup.Users = append(theGroup.Users, "valerie")
	_, err = clusterAdminUserClient.Groups().Create(theGroup)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// make sure that user/~ returns system groups for backed users when it merges
	expectedValerieGroups := []string{"system:authenticated", "system:authenticated:oauth"}
	secondValerie, err := userclient.NewForConfigOrDie(valerieConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(secondValerie.Groups, expectedValerieGroups) {
		t.Errorf("expected %v, got %v", expectedValerieGroups, secondValerie.Groups)
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
	if err := testutil.WaitForPolicyUpdate(valerieKubeClient.AuthorizationV1(), "empty", "get", kapi.Resource("pods"), true); err != nil {
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
	if err := testutil.WaitForPolicyUpdate(valerieKubeClient.AuthorizationV1(), "empty", "get", kapi.Resource("pods"), true); err != nil {
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
	userClient := userv1typedclient.NewForConfigOrDie(clusterAdminClientConfig)

	newGroup := &groupsnewcmd.NewGroupOptions{
		GroupClient: userClient,
		Group:       "group1",
		Users:       []string{"first", "second", "third", "first"},
		Printer:     printers.NewDiscardingPrinter(),
	}
	if err := newGroup.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err := userClient.Groups().Get("group1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "second", "third"}, []string(group1.Users); !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

	addUsers := &groupsuserscmd.AddUsersOptions{
		GroupModificationOptions: groupsuserscmd.NewGroupModificationOptions(genericclioptions.NewTestIOStreamsDiscard()),
	}
	addUsers.GroupModificationOptions.GroupClient = userClient
	addUsers.GroupModificationOptions.Group = "group1"
	addUsers.GroupModificationOptions.Users = []string{"second", "fourth", "fifth"}
	addUsers.GroupModificationOptions.ToPrinter = func(string) (printers.ResourcePrinter, error) {
		return printers.NewDiscardingPrinter(), nil
	}
	if err := addUsers.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err = userClient.Groups().Get("group1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "second", "third", "fourth", "fifth"}, []string(group1.Users); !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}

	removeUsers := &groupsuserscmd.RemoveUsersOptions{
		GroupModificationOptions: groupsuserscmd.NewGroupModificationOptions(genericclioptions.NewTestIOStreamsDiscard()),
	}
	removeUsers.GroupModificationOptions.ToPrinter = func(string) (printers.ResourcePrinter, error) {
		return printers.NewDiscardingPrinter(), nil
	}
	removeUsers.GroupModificationOptions.GroupClient = userClient
	removeUsers.GroupModificationOptions.Group = "group1"
	removeUsers.GroupModificationOptions.Users = []string{"second", "fourth", "fifth"}
	if err := removeUsers.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	group1, err = userClient.Groups().Get("group1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, a := []string{"first", "third"}, []string(group1.Users); !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, actual %v", e, a)
	}
}
