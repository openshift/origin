package integration

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	projectv1 "github.com/openshift/api/project/v1"
	userv1 "github.com/openshift/api/user/v1"
	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	userv1typedclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	groupsnewcmd "github.com/openshift/oc/pkg/cli/admin/groups/new"
	groupsuserscmd "github.com/openshift/oc/pkg/cli/admin/groups/users"
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
	clusterAdminUserClient := userv1typedclient.NewForConfigOrDie(clusterAdminClientConfig)

	valerieKubeClient, valerieConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "valerie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	valerieProjectClient := projectv1typedclient.NewForConfigOrDie(valerieConfig)

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

	theGroup := &userv1.Group{}
	theGroup.Name = "theGroup"
	theGroup.Users = append(theGroup.Users, "valerie")
	_, err = clusterAdminUserClient.Groups().Create(theGroup)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// make sure that user/~ returns system groups for backed users when it merges
	expectedValerieGroups := []string{"system:authenticated", "system:authenticated:oauth"}
	secondValerie, err := userv1typedclient.NewForConfigOrDie(valerieConfig).Users().Get("~", metav1.GetOptions{})
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

	emptyProject := &projectv1.Project{}
	emptyProject.Name = "empty"
	_, err = projectv1typedclient.NewForConfigOrDie(clusterAdminClientConfig).Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationv1.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Subjects = []corev1.ObjectReference{
		{Kind: "Group", Name: theGroup.Name},
	}
	_, err = authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).AuthorizationV1().RoleBindings("empty").Create(roleBinding)
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
	valerieProjectClient := projectv1typedclient.NewForConfigOrDie(valerieConfig)

	theGroup := &userv1.Group{}
	theGroup.Name = "thegroup"
	theGroup.Users = append(theGroup.Users, "valerie", "victor")
	_, err = userv1typedclient.NewForConfigOrDie(clusterAdminClientConfig).Groups().Create(theGroup)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = valerieProjectClient.Projects().Get("empty", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	}

	emptyProject := &projectv1.Project{}
	emptyProject.Name = "empty"
	_, err = projectv1typedclient.NewForConfigOrDie(clusterAdminClientConfig).Projects().Create(emptyProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roleBinding := &authorizationv1.RoleBinding{}
	roleBinding.Name = "admins"
	roleBinding.RoleRef.Name = "admin"
	roleBinding.Subjects = []corev1.ObjectReference{
		{Kind: "Group", Name: theGroup.Name},
	}
	_, err = authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).AuthorizationV1().RoleBindings("empty").Create(roleBinding)
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

	_, err = projectv1typedclient.NewForConfigOrDie(victorConfig).Projects().Get("empty", metav1.GetOptions{})
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
