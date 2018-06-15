package integration

import (
	"testing"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/apis/authorization/rbacconversion"
	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestRestrictUsers(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	masterConfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
		"openshift.io/RestrictSubjectBindings": {
			Configuration: &configapi.DefaultAdmissionConfig{},
		},
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminAuthorizationClient := authorizationclient.NewForConfigOrDie(clusterAdminClientConfig).Authorization()

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, "namespace", "carol"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	role := &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "role",
		},
	}
	if _, err := clusterAdminAuthorizationClient.Roles("namespace").Create(role); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingAlice := &authorizationapi.RoleBinding{
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

	allowAlice := &authorizationapi.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "match-users-alice",
			Namespace: "namespace",
		},
		Spec: authorizationapi.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationapi.UserRestriction{
				Users: []string{"alice"},
			},
		},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindingRestrictions("namespace").Create(allowAlice); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingAliceDup := &authorizationapi.RoleBinding{
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

	rolebindingBob := &authorizationapi.RoleBinding{
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
	rbacRolebindingBob := &rbac.RoleBinding{}
	if err := rbacconversion.Convert_authorization_RoleBinding_To_rbac_RoleBinding(rolebindingBob, rbacRolebindingBob, nil); err != nil {
		t.Fatalf("failed to convert RoleBinding: %v", err)
	}
	if _, err := clusterAdminKubeClient.Rbac().RoleBindings("namespace").Create(rbacRolebindingBob); !kapierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	allowBob := &authorizationapi.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "match-users-bob",
			Namespace: "namespace",
		},
		Spec: authorizationapi.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationapi.UserRestriction{
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
	allowWithNonExisting := &authorizationapi.RoleBindingRestriction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "match-users-eve-and-non-existing",
			Namespace: "namespace",
		},
		Spec: authorizationapi.RoleBindingRestrictionSpec{
			UserRestriction: &authorizationapi.UserRestriction{
				Users: []string{"eve", "system:non-existing"},
			},
		},
	}

	if _, err := clusterAdminAuthorizationClient.RoleBindingRestrictions("namespace").Create(allowWithNonExisting); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rolebindingEve := &authorizationapi.RoleBinding{
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

	rolebindingNonExisting := &authorizationapi.RoleBinding{
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
