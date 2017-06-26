package integration

import (
	"testing"
	//"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	//"k8s.io/kubernetes/pkg/api/unversioned"
	//utilwait "k8s.io/apimachinery/pkg/util/wait"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestRestrictUsers(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}

	masterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
		"openshift.io/RestrictSubjectBindings": {
			Configuration: &configapi.DefaultAdmissionConfig{},
		},
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
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

	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, "namespace", "carol"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	role := &authorizationapi.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "role",
		},
	}
	if _, err := clusterAdminClient.Roles("namespace").Create(role); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	policyBinding := &authorizationapi.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      "policybinding",
		},
		PolicyRef: kapi.ObjectReference{
			Namespace: "namespace",
			Name:      authorizationapi.GetPolicyBindingName("policy"),
		},
	}
	if _, err := clusterAdminClient.PolicyBindings("namespace").Create(policyBinding); err != nil {
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
	if _, err := clusterAdminClient.RoleBindings("namespace").Create(rolebindingAlice); err != nil {
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

	if _, err := clusterAdminClient.RoleBindingRestrictions("namespace").Create(allowAlice); err != nil {
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
	if _, err := clusterAdminClient.RoleBindings("namespace").Create(rolebindingAliceDup); err != nil {
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
	if _, err := clusterAdminClient.RoleBindings("namespace").Create(rolebindingBob); !kapierrors.IsForbidden(err) {
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

	if _, err := clusterAdminClient.RoleBindingRestrictions("namespace").Create(allowBob); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Creating a rolebinding when the subject is permitted by some
	// RoleBindingRestrictions should succeed.
	if _, err := clusterAdminClient.RoleBindings("namespace").Create(rolebindingBob); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
