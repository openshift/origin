package authorization

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	kauthorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kauthorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

var _ = g.Describe("[Feature:OpenShiftAuthorization] RBAC proxy for openshift authz", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("rbac-proxy", exutil.KubeConfigPath())
	g.Context("", func() {
		g.Describe("RunLegacyLocalRoleBindingEndpoint", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				RunLegacyLocalRoleBindingEndpoint(g.GinkgoT(), oc.AdminAuthorizationClient().AuthorizationV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})

		g.Describe("RunLegacyEndpointConfirmNoEscalation", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				RunLegacyEndpointConfirmNoEscalation(g.GinkgoT(), oc.AdminAuthorizationClient().AuthorizationV1(), oc.AuthorizationClient().AuthorizationV1(), oc.AdminKubeClient(), oc.Username(), oc.Namespace())
			})
		})

		g.Describe("RunLegacyClusterRoleBindingEndpoint", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				RunLegacyClusterRoleBindingEndpoint(g.GinkgoT(), oc.AdminAuthorizationClient().AuthorizationV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})

		g.Describe("RunLegacyClusterRoleEndpoint", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				RunLegacyClusterRoleEndpoint(g.GinkgoT(), oc.AdminAuthorizationClient().AuthorizationV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})

		g.Describe("RunLegacyLocalRoleEndpoint", func() {
			g.It(fmt.Sprintf("should succeed"), func() {
				RunLegacyLocalRoleEndpoint(g.GinkgoT(), oc.AdminAuthorizationClient().AuthorizationV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})
	})
})

var authorizationV1Encoder runtime.Encoder

func init() {
	authorizationV1Scheme := runtime.NewScheme()
	utilruntime.Must(authorizationv1.Install(authorizationV1Scheme))
	authorizationV1Codecs := serializer.NewCodecFactory(authorizationV1Scheme)
	authorizationV1Encoder = authorizationV1Codecs.LegacyCodec(authorizationv1.GroupVersion)
}

// RunLegacyLocalRoleBindingEndpoint exercises the legacy rolebinding endpoint that is proxied to rbac
func RunLegacyLocalRoleBindingEndpoint(t g.GinkgoTInterface, authorizationClient authorizationv1client.AuthorizationV1Interface, kubeClient kubernetes.Interface, namespace string) {
	testBindingName := "testrole"

	clusterAdminRoleBindingsClient := authorizationClient.RoleBindings(namespace)
	clusterAdminRBACRoleBindingsClient := kubeClient.RbacV1().RoleBindings(namespace)

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

// RunLegacyClusterRoleBindingEndpoint exercises the legacy clusterrolebinding endpoint that is proxied to rbac
func RunLegacyClusterRoleBindingEndpoint(t g.GinkgoTInterface, authorizationClient authorizationv1client.AuthorizationV1Interface, kubeClient kubernetes.Interface, namespace string) {
	testBindingName := "testbinding-" + namespace

	clusterAdminClusterRoleBindingsClient := authorizationClient.ClusterRoleBindings()
	clusterAdminRBACClusterRoleBindingsClient := kubeClient.RbacV1().ClusterRoleBindings()

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
	defer func() {
		clusterAdminClusterRoleBindingsClient.Delete(testBindingName, nil)
	}()

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

// RunLegacyClusterRoleEndpoint exercises the legacy clusterrole endpoint that is proxied to rbac
func RunLegacyClusterRoleEndpoint(t g.GinkgoTInterface, authorizationClient authorizationv1client.AuthorizationV1Interface, kubeClient kubernetes.Interface, namespace string) {
	testRole := "testrole-" + namespace

	clusterAdminClusterRoleClient := authorizationClient.ClusterRoles()
	clusterAdminRBACClusterRoleClient := kubeClient.RbacV1().ClusterRoles()

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
	defer func() {
		clusterAdminClusterRoleClient.Delete(testRole, nil)
	}()

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

// RunLegacyLocalRoleEndpoint exercises the legacy role endpoint that is proxied to rbac
func RunLegacyLocalRoleEndpoint(t g.GinkgoTInterface, authorizationClient authorizationv1client.AuthorizationV1Interface, kubeClient kubernetes.Interface, namespace string) {
	testRole := "testrole"

	clusterAdminRoleClient := authorizationClient.Roles(namespace)
	clusterAdminRBACRoleClient := kubeClient.RbacV1().Roles(namespace)

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

// waitForClusterPolicyUpdate checks if the given client can perform the named verb and action.
// If PolicyCachePollTimeout is reached without the expected condition matching, an error is returned
func waitForClusterPolicyUpdate(c kauthorizationv1client.SelfSubjectAccessReviewsGetter, verb string, resource schema.GroupResource, allowed bool) error {
	review := &kauthorizationv1.SelfSubjectAccessReview{
		Spec: kauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kauthorizationv1.ResourceAttributes{
				Verb:     verb,
				Group:    resource.Group,
				Resource: resource.Resource,
			},
		},
	}
	err := wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		response, err := c.SelfSubjectAccessReviews().Create(review)
		if err != nil {
			return false, err
		}
		if response.Status.Allowed != allowed {
			return false, nil
		}
		return true, nil
	})
	return err
}

// RunLegacyEndpointConfirmNoEscalation tests that the authorization proxy endpoints cannot be used to bypass
// the RBAC escalation checks.  It also makes sure that the GR in the returned error matches authorization v1.
func RunLegacyEndpointConfirmNoEscalation(t g.GinkgoTInterface, clusterAdminAuthorizationClient, userAuthorizationClient authorizationv1client.AuthorizationV1Interface, kubeClient kubernetes.Interface, userName, namespace string) {
	resourceName := "test-resource-no-escalation"
	userSubjects := []corev1.ObjectReference{
		{
			Kind: rbacv1.UserKind,
			Name: userName,
		},
	}
	escalationFormat := `%s %q is forbidden: user %q (groups=["system:authenticated:oauth" "system:authenticated"]) is attempting to grant RBAC permissions not currently held:`
	escalatingRules := []authorizationv1.PolicyRule{
		{
			Verbs:     []string{"hug"},
			APIGroups: []string{"bear"},
			Resources: []string{"pandas"},
		},
	}
	nonEscalatingRules := []authorizationv1.PolicyRule{
		{
			Verbs:     []string{"create"},
			APIGroups: []string{kauthorizationv1.GroupName},
			Resources: []string{"selfsubjectaccessreviews"},
		},
	}

	clusterRoleName := "test-cluster-role-" + userName
	clusterRoleObj, err := clusterAdminAuthorizationClient.ClusterRoles().Create(&authorizationv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
		Rules: []authorizationv1.PolicyRule{
			{
				Verbs:     []string{"get", "create", "update"},
				APIGroups: []string{authorizationv1.GroupName, rbacv1.GroupName},
				Resources: []string{"clusterroles", "clusterrolebindings"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		clusterAdminAuthorizationClient.ClusterRoles().Delete(clusterRoleObj.Name, nil)
	}()

	if _, err := clusterAdminAuthorizationClient.ClusterRoleBindings().Create(&authorizationv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
		Subjects:   userSubjects,
		RoleRef: corev1.ObjectReference{
			Name: clusterRoleName,
		},
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		clusterAdminAuthorizationClient.ClusterRoleBindings().Delete(clusterRoleName, nil)
	}()

	for _, rule := range clusterRoleObj.Rules {
		for _, verb := range rule.Verbs {
			for _, group := range rule.APIGroups {
				for _, resource := range rule.Resources {
					if err := waitForClusterPolicyUpdate(
						kubeClient.AuthorizationV1(),
						verb,
						schema.GroupResource{Group: group, Resource: resource},
						true,
					); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}

	tests := []struct {
		name     string
		resource string
		run      func() error
	}{
		{
			name:     "role create",
			resource: "roles",
			run: func() error {
				_, err := userAuthorizationClient.Roles(namespace).Create(&authorizationv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Rules:      escalatingRules,
				})
				return err
			},
		},
		{
			name:     "role update",
			resource: "roles",
			run: func() error {
				role, err := userAuthorizationClient.Roles(namespace).Create(&authorizationv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Rules:      nonEscalatingRules,
				})
				if err != nil {
					return fmt.Errorf("failed to create role: %v", err)
				}

				role.Rules = escalatingRules
				_, err = userAuthorizationClient.Roles(namespace).Update(role)
				return err
			},
		},
		{
			name:     "role binding create",
			resource: "rolebindings",
			run: func() error {
				_, err := userAuthorizationClient.RoleBindings(namespace).Create(&authorizationv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Subjects:   userSubjects,
					RoleRef: corev1.ObjectReference{
						Name: "cluster-admin",
					},
				})
				return err
			},
		},
		{
			name:     "role binding update",
			resource: "rolebindings",
			run: func() error {
				roleBinding, err := clusterAdminAuthorizationClient.RoleBindings(namespace).Create(&authorizationv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Subjects: []corev1.ObjectReference{
						{
							Kind: rbacv1.UserKind,
							Name: "some-other-user",
						},
					},
					RoleRef: corev1.ObjectReference{
						Name: "cluster-admin",
					},
				})
				if err != nil {
					return fmt.Errorf("failed to create role binding: %v", err)
				}

				roleBinding.Subjects = userSubjects
				roleBinding.UserNames = nil // if set, this field will overwrite subjects
				_, err = userAuthorizationClient.RoleBindings(namespace).Update(roleBinding)
				return err
			},
		},
		{
			name:     "cluster role create",
			resource: "clusterroles",
			run: func() error {
				_, err := userAuthorizationClient.ClusterRoles().Create(&authorizationv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Rules:      escalatingRules,
				})
				return err
			},
		},
		{
			name:     "cluster role update",
			resource: "clusterroles",
			run: func() error {
				clusterRole, err := userAuthorizationClient.ClusterRoles().Create(&authorizationv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Rules:      nonEscalatingRules,
				})
				if err != nil {
					return fmt.Errorf("failed to create cluster role: %v", err)
				}

				clusterRole.Rules = escalatingRules
				_, err = userAuthorizationClient.ClusterRoles().Update(clusterRole)
				return err
			},
		},
		{
			name:     "cluster role binding create",
			resource: "clusterrolebindings",
			run: func() error {
				_, err := userAuthorizationClient.ClusterRoleBindings().Create(&authorizationv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Subjects:   userSubjects,
					RoleRef: corev1.ObjectReference{
						Name: "cluster-admin",
					},
				})
				return err
			},
		},
		{
			name:     "cluster role binding update",
			resource: "clusterrolebindings",
			run: func() error {
				clusterRoleBinding, err := clusterAdminAuthorizationClient.ClusterRoleBindings().Create(&authorizationv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: resourceName},
					Subjects: []corev1.ObjectReference{
						{
							Kind: rbacv1.UserKind,
							Name: "some-other-user",
						},
					},
					RoleRef: corev1.ObjectReference{
						Name: "cluster-admin",
					},
				})
				if err != nil {
					return fmt.Errorf("failed to create cluster role binding: %v", err)
				}

				clusterRoleBinding.Subjects = userSubjects
				clusterRoleBinding.UserNames = nil // if set, this field will overwrite subjects
				_, err = userAuthorizationClient.ClusterRoleBindings().Update(clusterRoleBinding)
				return err
			},
		},
	}
	for _, tt := range tests {
		g.By(tt.name, func() {
			// always clean up in between tests
			defer func() {
				_ = clusterAdminAuthorizationClient.ClusterRoles().Delete(resourceName, nil)
				_ = clusterAdminAuthorizationClient.ClusterRoleBindings().Delete(resourceName, nil)
				_ = clusterAdminAuthorizationClient.Roles(namespace).Delete(resourceName, nil)
				_ = clusterAdminAuthorizationClient.RoleBindings(namespace).Delete(resourceName, nil)
			}()

			err := tt.run()

			if err == nil {
				t.Fatal("got nil instead of escalation error")
			}

			if !kapierror.IsForbidden(err) {
				t.Fatalf("expected forbidden error, got: %v", err)
			}

			details := *err.(kapierror.APIStatus).Status().Details

			if resourceName != details.Name {
				t.Errorf("expected resource name %q got %q", resourceName, details.Name)
			}

			wantGR := authorizationv1.GroupVersion.WithResource(tt.resource).GroupResource()
			gotGR := schema.GroupResource{Group: details.Group, Resource: details.Kind}
			if wantGR != gotGR {
				t.Errorf("expected group resource %s got %s", wantGR, gotGR)
			}

			wantErr := fmt.Sprintf(escalationFormat, wantGR.String(), resourceName, userName)
			gotErr := err.Error()
			if !strings.HasPrefix(gotErr, wantErr) {
				t.Errorf("expected escalation message prefix %q got %q", wantErr, gotErr)
			}
		})
	}
}
