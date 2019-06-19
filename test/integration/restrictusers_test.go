package integration

import (
	"testing"

	kapi "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestRestrictUsers(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(testutil.NonProtobufConfig(clusterAdminClientConfig))

	if err := testutil.WaitForRoleBindingRestrictionCRDAvailable(clusterAdminClientConfig); err != nil {
		t.Fatal(err)
	}

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, "namespace", "carol"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	role := &authorizationv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "role",
		},
	}
	if _, err := clusterAdminAuthorizationClient.Roles("namespace").Create(role); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingAlice := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "rolebinding1",
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind:      authorizationapi.UserKind,
				Namespace: "namespace",
				Name:      "alice",
			},
		},
		RoleRef: kapi.ObjectReference{Name: "role", Namespace: "namespace"},
	}

	// Creating a rolebinding when no restrictions exist should succeed.
	if _, err := clusterAdminAuthorizationClient.RoleBindings("namespace").Create(rolebindingAlice); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allowAlice := &authorizationv1.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "match-users-alice",
			Namespace: "namespace",
		},
		Spec: authorizationv1.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationv1.UserRestriction{
				Users: []string{"alice"},
			},
		},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindingRestrictions("namespace").Create(allowAlice); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingAliceDup := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "rolebinding2",
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind:      authorizationapi.UserKind,
				Namespace: "namespace",
				Name:      "alice",
			},
		},
		RoleRef: kapi.ObjectReference{Name: "role", Namespace: "namespace"},
	}

	// Creating a rolebinding when the subject is already bound should succeed.
	if _, err := clusterAdminAuthorizationClient.RoleBindings("namespace").Create(rolebindingAliceDup); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingBob := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "rolebinding3",
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind:      authorizationapi.UserKind,
				Namespace: "namespace",
				Name:      "bob",
			},
		},
		RoleRef: kapi.ObjectReference{Name: "role", Namespace: "namespace"},
	}

	// Creating a rolebinding when the subject is not already bound and is not
	// permitted by any RoleBindingRestrictions should fail.
	if _, err := clusterAdminAuthorizationClient.RoleBindings("namespace").Create(rolebindingBob); !kapierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// Creating a RBAC rolebinding when the subject is not already bound
	// should also fail.
	rbacRolebindingBob := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "rolebinding3",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.UserKind,
				Namespace: "namespace",
				Name:      "bob",
			},
		},
		RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "role"},
	}
	if _, err := clusterAdminKubeClient.RbacV1().RoleBindings("namespace").Create(rbacRolebindingBob); !kapierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	allowBob := &authorizationv1.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "match-users-bob",
			Namespace: "namespace",
		},
		Spec: authorizationv1.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationv1.UserRestriction{
				Users: []string{"bob"},
			},
		},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindingRestrictions("namespace").Create(allowBob); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Creating a rolebinding when the subject is permitted by some
	// RoleBindingRestrictions should succeed.
	if _, err := clusterAdminAuthorizationClient.RoleBindings("namespace").Create(rolebindingBob); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Creating rolebindings that also contains "system non existing" users should
	// not fail.
	allowWithNonExisting := &authorizationv1.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "match-users-eve-and-non-existing",
			Namespace: "namespace",
		},
		Spec: authorizationv1.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationv1.UserRestriction{
				Users: []string{"eve", "system:non-existing"},
			},
		},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindingRestrictions("namespace").Create(allowWithNonExisting); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingEve := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "rolebinding4",
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind:      authorizationapi.UserKind,
				Namespace: "namespace",
				Name:      "eve",
			},
		},
		RoleRef: kapi.ObjectReference{Name: "role", Namespace: "namespace"},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindings("namespace").Create(rolebindingEve); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingNonExisting := &authorizationv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "rolebinding5",
		},
		Subjects: []kapi.ObjectReference{
			{
				Kind:      authorizationapi.UserKind,
				Namespace: "namespace",
				Name:      "system:non-existing",
			},
		},
		RoleRef: kapi.ObjectReference{Name: "role", Namespace: "namespace"},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindings("namespace").Create(rolebindingNonExisting); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
