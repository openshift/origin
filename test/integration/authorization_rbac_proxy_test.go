package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var authorizationV1Encoder runtime.Encoder

func init() {
	authorizationV1Scheme := runtime.NewScheme()
	utilruntime.Must(authorizationv1.Install(authorizationV1Scheme))
	authorizationV1Codecs := serializer.NewCodecFactory(authorizationV1Scheme)
	authorizationV1Encoder = authorizationV1Codecs.LegacyCodec(authorizationv1.GroupVersion)
}

// TestLegacyLocalRoleBindingEndpoint exercises the legacy rolebinding endpoint that is proxied to rbac
func TestLegacyLocalRoleBindingEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	namespace := "testproject"
	testBindingName := "testrole"

	clusterAdminRoleBindingsClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).RoleBindings(namespace)
	clusterAdminRBACRoleBindingsClient := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig).RoleBindings(namespace)

	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, namespace, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	// create rolebinding
	roleBindingToCreate := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind: rbacv1.UserKind,
				Name: "testuser",
			},
		},
		RoleRef: corev1.ObjectReference{
			Kind:      "Role",
			Name:      "edit",
			Namespace: namespace,
		},
	}

	roleBindingCreated, err := clusterAdminRoleBindingsClient.Create(roleBindingToCreate)
	if err != nil {
		t.Fatal(err)
	}

	if roleBindingCreated.Name != roleBindingToCreate.Name {
		t.Fatalf("expected rolebinding %s, got %s", roleBindingToCreate.Name, roleBindingCreated.Name)
	}

	// list rolebindings
	roleBindingList, err := clusterAdminRoleBindingsClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	checkBindings := sets.String{}
	for _, rb := range roleBindingList.Items {
		checkBindings.Insert(rb.Name)
	}

	// check for the created rolebinding in the list
	if !checkBindings.HasAll(testBindingName) {
		t.Fatalf("rolebinding list does not have the expected bindings")
	}

	// edit rolebinding
	roleBindingToEdit := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind: rbacv1.UserKind,
				Name: "testuser",
			},
			{
				Kind: rbacv1.UserKind,
				Name: "testuser2",
			},
		},
		RoleRef: corev1.ObjectReference{
			Kind:      "Role",
			Name:      "edit",
			Namespace: namespace,
		},
	}
	roleBindingToEditBytes, err := runtime.Encode(authorizationV1Encoder, roleBindingToEdit)
	if err != nil {
		t.Fatal(err)
	}

	roleBindingEdited, err := clusterAdminRoleBindingsClient.Patch(testBindingName, types.StrategicMergePatchType, roleBindingToEditBytes)
	if err != nil {
		t.Fatal(err)
	}

	if roleBindingEdited.Name != roleBindingToEdit.Name {
		t.Fatalf("expected rolebinding %s, got %s", roleBindingToEdit.Name, roleBindingEdited.Name)
	}

	checkSubjects := sets.String{}
	for _, subj := range roleBindingEdited.Subjects {
		checkSubjects.Insert(subj.Name)
	}
	if !checkSubjects.HasAll("testuser", "testuser2") {
		t.Fatalf("rolebinding not edited")
	}

	// get rolebinding by name
	getRoleBinding, err := clusterAdminRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRoleBinding.Name != testBindingName {
		t.Fatalf("expected rolebinding %s, got %s", testBindingName, getRoleBinding.Name)
	}
	// get rolebinding by name via RBAC endpoint
	getRoleBindingRBAC, err := clusterAdminRBACRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRoleBindingRBAC.Name != testBindingName {
		t.Fatalf("expected rolebinding %s, got %s", testBindingName, getRoleBindingRBAC.Name)
	}

	// delete rolebinding
	err = clusterAdminRoleBindingsClient.Delete(testBindingName, nil)
	if err != nil {
		t.Fatal(err)
	}

	// confirm deletion
	_, err = clusterAdminRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
	// confirm deletion via RBAC endpoint
	_, err = clusterAdminRBACRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}

	// create local rolebinding for cluster role
	localClusterRoleBindingToCreate := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-crb",
			Namespace: namespace,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind: rbacv1.UserKind,
				Name: "testuser",
			},
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "edit",
		},
	}

	localClusterRoleBindingCreated, err := clusterAdminRoleBindingsClient.Create(localClusterRoleBindingToCreate)
	if err != nil {
		t.Fatal(err)
	}

	if localClusterRoleBindingCreated.Name != localClusterRoleBindingToCreate.Name {
		t.Fatalf("expected clusterrolebinding %s, got %s", localClusterRoleBindingToCreate.Name, localClusterRoleBindingCreated.Name)
	}
}

// TestLegacyClusterRoleBindingEndpoint exercises the legacy clusterrolebinding endpoint that is proxied to rbac
func TestLegacyClusterRoleBindingEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	testBindingName := "testbinding"

	clusterAdminClusterRoleBindingsClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoleBindings()
	clusterAdminRBACClusterRoleBindingsClient := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoleBindings()

	// list clusterrole bindings
	clusterRoleBindingList, err := clusterAdminClusterRoleBindingsClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	checkBindings := sets.String{}
	for _, rb := range clusterRoleBindingList.Items {
		checkBindings.Insert(rb.Name)
	}

	// ensure there are at least some of the expected bindings in the list
	if !checkBindings.HasAll("basic-users", "cluster-admin", "cluster-admins", "cluster-readers") {
		t.Fatalf("clusterrolebinding list does not have the expected bindings")
	}

	// create clusterrole binding
	clusterRoleBindingToCreate := &authorizationv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind: rbacv1.UserKind,
				Name: "testuser",
			},
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "edit",
		},
	}

	clusterRoleBindingCreated, err := clusterAdminClusterRoleBindingsClient.Create(clusterRoleBindingToCreate)
	if err != nil {
		t.Fatal(err)
	}

	if clusterRoleBindingCreated.Name != clusterRoleBindingToCreate.Name {
		t.Fatalf("expected clusterrolebinding %s, got %s", clusterRoleBindingToCreate.Name, clusterRoleBindingCreated.Name)
	}

	// edit clusterrole binding
	clusterRoleBindingToEdit := &authorizationv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: testBindingName,
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind: rbacv1.UserKind,
				Name: "testuser",
			},
			{
				Kind: rbacv1.UserKind,
				Name: "testuser2",
			},
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "edit",
		},
	}
	clusterRoleBindingToEditBytes, err := runtime.Encode(authorizationV1Encoder, clusterRoleBindingToEdit)
	if err != nil {
		t.Fatal(err)
	}

	clusterRoleBindingEdited, err := clusterAdminClusterRoleBindingsClient.Patch(testBindingName, types.StrategicMergePatchType, clusterRoleBindingToEditBytes)
	if err != nil {
		t.Fatal(err)
	}

	if clusterRoleBindingEdited.Name != clusterRoleBindingToEdit.Name {
		t.Fatalf("expected clusterrolebinding %s, got %s", clusterRoleBindingToEdit.Name, clusterRoleBindingEdited.Name)
	}

	checkSubjects := sets.String{}
	for _, subj := range clusterRoleBindingEdited.Subjects {
		checkSubjects.Insert(subj.Name)
	}
	if !checkSubjects.HasAll("testuser", "testuser2") {
		t.Fatalf("clusterrolebinding not edited")
	}

	// get clusterrolebinding by name
	getRoleBinding, err := clusterAdminClusterRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRoleBinding.Name != testBindingName {
		t.Fatalf("expected clusterrolebinding %s, got %s", testBindingName, getRoleBinding.Name)
	}
	// get clusterrolebinding by name via RBAC endpoint
	getRoleBindingRBAC, err := clusterAdminRBACClusterRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRoleBindingRBAC.Name != testBindingName {
		t.Fatalf("expected clusterrolebinding %s, got %s", testBindingName, getRoleBindingRBAC.Name)
	}

	// delete clusterrolebinding
	err = clusterAdminClusterRoleBindingsClient.Delete(testBindingName, nil)
	if err != nil {
		t.Fatal(err)
	}

	// confirm deletion
	_, err = clusterAdminClusterRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
	// confirm deletion via RBAC endpoint
	_, err = clusterAdminRBACClusterRoleBindingsClient.Get(testBindingName, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
}

// TestLegacyClusterRoleEndpoint exercises the legacy clusterrole endpoint that is proxied to rbac
func TestLegacyClusterRoleEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	testRole := "testrole"

	clusterAdminClusterRoleClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoles()
	clusterAdminRBACClusterRoleClient := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig).ClusterRoles()

	// list clusterroles
	clusterRoleList, err := clusterAdminClusterRoleClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	checkRoles := sets.String{}
	for _, role := range clusterRoleList.Items {
		checkRoles.Insert(role.Name)
	}
	// ensure there are at least some of the expected roles in the clusterrole list
	if !checkRoles.HasAll("admin", "basic-user", "cluster-admin", "edit", "sudoer") {
		t.Fatalf("clusterrole list does not have the expected roles")
	}

	// create clusterrole
	clusterRoleToCreate := &authorizationv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: testRole},
		Rules: []authorizationv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
		},
	}

	createdClusterRole, err := clusterAdminClusterRoleClient.Create(clusterRoleToCreate)
	if err != nil {
		t.Fatal(err)
	}

	if createdClusterRole.Name != clusterRoleToCreate.Name {
		t.Fatalf("expected to create %v, got %v", clusterRoleToCreate.Name, createdClusterRole.Name)
	}

	if !sets.NewString(createdClusterRole.Rules[0].Verbs...).Has("get") {
		t.Fatalf("expected clusterrole to have a get rule")
	}

	// update clusterrole
	clusterRoleUpdate := &authorizationv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: testRole},
		Rules: []authorizationv1.PolicyRule{
			{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
		},
	}

	clusterRoleUpdateBytes, err := runtime.Encode(authorizationV1Encoder, clusterRoleUpdate)
	if err != nil {
		t.Fatal(err)
	}

	updatedClusterRole, err := clusterAdminClusterRoleClient.Patch(testRole, types.StrategicMergePatchType, clusterRoleUpdateBytes)
	if err != nil {
		t.Fatal(err)
	}

	if updatedClusterRole.Name != clusterRoleUpdate.Name {
		t.Fatalf("expected to update %s, got %s", clusterRoleUpdate.Name, updatedClusterRole.Name)
	}

	if !sets.NewString(updatedClusterRole.Rules[0].Verbs...).HasAll("get", "list") {
		t.Fatalf("expected clusterrole to have a get and list rule")
	}

	// get clusterrole
	getRole, err := clusterAdminClusterRoleClient.Get(testRole, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRole.Name != testRole {
		t.Fatalf("expected %s role, got %s instead", testRole, getRole.Name)
	}
	// get clusterrole via RBAC
	getRoleRBAC, err := clusterAdminRBACClusterRoleClient.Get(testRole, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRoleRBAC.Name != testRole {
		t.Fatalf("expected %s role, got %s instead", testRole, getRoleRBAC.Name)
	}

	// delete clusterrole
	err = clusterAdminClusterRoleClient.Delete(testRole, nil)
	if err != nil {
		t.Fatal(err)
	}

	// confirm deletion
	_, err = clusterAdminClusterRoleClient.Get(testRole, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
	// confirm deletion via RBAC
	_, err = clusterAdminRBACClusterRoleClient.Get(testRole, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
}

// TestLegacyLocalRoleEndpoint exercises the legacy role endpoint that is proxied to rbac
func TestLegacyLocalRoleEndpoint(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	namespace := "testproject"
	testRole := "testrole"

	clusterAdminRoleClient := authorizationv1client.NewForConfigOrDie(clusterAdminClientConfig).Roles(namespace)
	clusterAdminRBACRoleClient := rbacv1client.NewForConfigOrDie(clusterAdminClientConfig).Roles(namespace)

	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, namespace, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	// create role
	roleToCreate := &authorizationv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRole,
			Namespace: namespace,
		},
		Rules: []authorizationv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
		},
	}

	createdRole, err := clusterAdminRoleClient.Create(roleToCreate)
	if err != nil {
		t.Fatal(err)
	}

	if createdRole.Name != roleToCreate.Name {
		t.Fatalf("expected to create %v, got %v", roleToCreate.Name, createdRole.Name)
	}

	if !sets.NewString(createdRole.Rules[0].Verbs...).Has("get") {
		t.Fatalf("expected clusterRole to have a get rule")
	}

	// list roles
	roleList, err := clusterAdminRoleClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	checkRoles := sets.String{}
	for _, role := range roleList.Items {
		checkRoles.Insert(role.Name)
	}
	// ensure the role list has the created role
	if !checkRoles.HasAll(testRole) {
		t.Fatalf("role list does not have the expected roles")
	}

	// update role
	roleUpdate := &authorizationv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRole,
			Namespace: namespace,
		},
		Rules: []authorizationv1.PolicyRule{
			{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
		},
	}

	roleUpdateBytes, err := runtime.Encode(authorizationV1Encoder, roleUpdate)
	if err != nil {
		t.Fatal(err)
	}

	updatedRole, err := clusterAdminRoleClient.Patch(testRole, types.StrategicMergePatchType, roleUpdateBytes)
	if err != nil {
		t.Fatal(err)
	}

	if updatedRole.Name != roleUpdate.Name {
		t.Fatalf("expected to update %s, got %s", roleUpdate.Name, updatedRole.Name)
	}

	if !sets.NewString(updatedRole.Rules[0].Verbs...).HasAll("get", "list") {
		t.Fatalf("expected role to have a get and list rule")
	}

	// get role
	getRole, err := clusterAdminRoleClient.Get(testRole, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRole.Name != testRole {
		t.Fatalf("expected %s role, got %s instead", testRole, getRole.Name)
	}
	// get role via RBAC
	getRoleRBAC, err := clusterAdminRBACRoleClient.Get(testRole, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if getRoleRBAC.Name != testRole {
		t.Fatalf("expected %s role, got %s instead", testRole, getRoleRBAC.Name)
	}

	// delete role
	err = clusterAdminRoleClient.Delete(testRole, nil)
	if err != nil {
		t.Fatal(err)
	}

	// confirm deletion
	_, err = clusterAdminRoleClient.Get(testRole, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
	// confirm deletion via RBAC
	_, err = clusterAdminRBACRoleClient.Get(testRole, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected error")
	} else if !kapierror.IsNotFound(err) {
		t.Fatal(err)
	}
}
