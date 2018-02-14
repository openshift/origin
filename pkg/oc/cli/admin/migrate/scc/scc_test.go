package scc

import (
	"reflect"
	"strings"
	"testing"

	fuzz "github.com/google/gofuzz"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	securityv1 "github.com/openshift/api/security/v1"
)

func TestConvertSccToClusterRole(t *testing.T) {
	expectedRoleName := "psp:some-name"
	expectedPSPName := "some-name"

	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: expectedPSPName,
		},
		Users: []string{"user1"},
	}

	role, err := convertSccToClusterRole(scc)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role == nil {
		t.Fatalf("unexpected nil role")
	}

	if role.Name != expectedRoleName {
		t.Errorf("expected cluster role name to be %q but got %q", expectedRoleName, role.Name)
	}
	if len(role.Rules) != 1 {
		t.Errorf("expected cluster role with 1 rule but got %d (%#v)", len(role.Rules), role.Rules)
	} else if role.Rules[0].ResourceNames[0] != expectedPSPName {
		t.Errorf("expected cluster role that allows resource %q but it allows only %q (%#v)",
			expectedPSPName, role.Rules[0].ResourceNames[0], role.Rules)
	}
}

func TestConvertSccToClusterRoleWithEmptyUsersAndGroups(t *testing.T) {
	scc := &securityv1.SecurityContextConstraints{}
	role, err := convertSccToClusterRole(scc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != nil {
		t.Errorf("expected nil but got %#v", role)
	}
}

func TestConvertSccToClusterRoleBinding(t *testing.T) {
	expectedName := "psp:some-name"
	expectedUsers := []string{"user1", "user2"}
	expectedGroups := []string{"group1", "group2"}
	expectedServiceAccounts := []string{"registry", "kube-dns"}

	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-name",
		},
		Users:  []string{"user1", "user2", "system:serviceaccount:default:registry", "system:serviceaccount:kube-dns:kube-dns"},
		Groups: expectedGroups,
	}

	binding, err := convertSccToClusterRoleBinding(scc)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if binding == nil {
		t.Fatalf("unexpected nil binding")
	}

	if binding.Name != expectedName {
		t.Errorf("expected cluster role binding name to be %q but got %q", expectedName, binding.Name)
	}
	if binding.RoleRef.Name != expectedName {
		t.Errorf("expected cluster role binding that references a role with name %q but got %q",
			expectedName, binding.RoleRef.Name)
	}

	actualUsers := collectSubjectNames(binding.Subjects, func(subject rbacv1.Subject) bool {
		return subject.Kind == "User"
	})
	if !reflect.DeepEqual(actualUsers, expectedUsers) {
		t.Errorf("expected that cluster role binding (%#v) will have the following users: %v but it has %v",
			binding, expectedUsers, actualUsers)
	}

	actualGroups := collectSubjectNames(binding.Subjects, func(subject rbacv1.Subject) bool {
		return subject.Kind == "Group"
	})
	if !reflect.DeepEqual(actualGroups, expectedGroups) {
		t.Errorf("expected that cluster role binding (%#v) will have the following groups: %v but it has %v",
			binding, expectedGroups, actualGroups)
	}

	actualServiceAccounts := collectSubjectNames(binding.Subjects, func(subject rbacv1.Subject) bool {
		return subject.Kind == "ServiceAccount"
	})
	if !reflect.DeepEqual(actualServiceAccounts, expectedServiceAccounts) {
		t.Errorf("expected that cluster role binding (%#v) will have the following Service Accounts: %v but it has %v",
			binding, expectedServiceAccounts, actualServiceAccounts)
	}
}

func TestConvertSccToClusterRoleBindingWithEmptyUsersAndGroups(t *testing.T) {
	scc := &securityv1.SecurityContextConstraints{}
	binding, err := convertSccToClusterRoleBinding(scc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if binding != nil {
		t.Fatalf("expected nil but got %#v", binding)
	}
}

func TestEverythingWithFuzzing(t *testing.T) {
	fuzzer := createFuzzerForSCC()
	scc := securityv1.SecurityContextConstraints{}

	for i := 0; i < 100; i++ {
		fuzzer.Fuzz(&scc)

		_, err := convertSccToPsp(&scc)
		if err != nil && !knownPossibleError(err) {
			t.Errorf("unexpected error while creating PSP for SCC: %v\nSCC:\n%#v\n", err, scc)
		}

		_, err = convertSccToClusterRole(&scc)
		if err != nil {
			t.Errorf("unexpected error while creating cluster role for SCC: %v\nSCC:\n%#v\n", err, scc)
		}

		_, err = convertSccToClusterRoleBinding(&scc)
		if err != nil {
			t.Errorf("unexpected error while creating cluster role binding for SCC: %v\nSCC:\n%#v\n", err, scc)
		}
	}
}

func knownPossibleError(err error) bool {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "found RunAsUser with both uid"), strings.HasPrefix(msg, "found RunAsUser with half-filled range"):
		return true
	default:
		return false
	}
}

func createFuzzerForSCC() *fuzz.Fuzzer {
	return fuzz.New().Funcs(func(scc *securityv1.SecurityContextConstraints, c fuzz.Continue) {
		c.FuzzNoCustom(scc)

		seLinuxTypes := []securityv1.SELinuxContextStrategyType{
			securityv1.SELinuxStrategyMustRunAs,
			securityv1.SELinuxStrategyRunAsAny,
		}
		scc.SELinuxContext.Type = seLinuxTypes[c.Rand.Intn(len(seLinuxTypes))]

		runAsUserTypes := []securityv1.RunAsUserStrategyType{
			securityv1.RunAsUserStrategyMustRunAs,
			securityv1.RunAsUserStrategyMustRunAsNonRoot,
			securityv1.RunAsUserStrategyMustRunAsRange,
			securityv1.RunAsUserStrategyRunAsAny,
		}
		scc.RunAsUser.Type = runAsUserTypes[c.Rand.Intn(len(runAsUserTypes))]

		supplementalGroupsTypes := []securityv1.SupplementalGroupsStrategyType{
			securityv1.SupplementalGroupsStrategyMustRunAs,
			securityv1.SupplementalGroupsStrategyRunAsAny,
		}
		scc.SupplementalGroups.Type = supplementalGroupsTypes[c.Rand.Intn(len(supplementalGroupsTypes))]

		fsGroupTypes := []securityv1.FSGroupStrategyType{
			securityv1.FSGroupStrategyMustRunAs,
			securityv1.FSGroupStrategyRunAsAny,
		}
		scc.FSGroup.Type = fsGroupTypes[c.Rand.Intn(len(fsGroupTypes))]
	})
}
