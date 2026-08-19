package rbacadminescalationtests

import (
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func binding(name, roleName string, subjects ...rbacv1.Subject) rbacv1.ClusterRoleBinding {
	return rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: roleName},
		Subjects:   subjects,
	}
}

func rule(verbs, groups, resources []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{Verbs: verbs, APIGroups: groups, Resources: resources}
}

// checkID extracts the escalation check that a test case name corresponds to by matching against the
// known checks' descriptions.
func checkIDForName(name string) string {
	for _, c := range escalationChecks {
		if strings.Contains(name, c.desc) {
			return c.id
		}
	}
	return ""
}

func TestEvaluateBinding(t *testing.T) {
	// Subjects must be ServiceAccounts in a core (kube-/openshift-) namespace to be in scope.
	saSubject := rbacv1.Subject{Kind: "ServiceAccount", Namespace: "openshift-ns", Name: "sa"}
	permSubject := rbacv1.Subject{Kind: "ServiceAccount", Namespace: "openshift-perm", Name: "perm-sa"}

	// Seed a tracked exception for the duration of this test so we can exercise the flake path. It
	// pins the exact grant: name, check, roleRef, and subjects.
	originalTracked := trackedExceptions
	trackedExceptions = append(append([]bindingException{}, originalTracked...), bindingException{
		name:     "tracked-esc",
		checkID:  "escalate-rbac",
		roleRef:  "escalate-role",
		subjects: []rbacv1.Subject{saSubject},
		note:     "https://issues.redhat.com/browse/EXAMPLE-1",
	})
	defer func() { trackedExceptions = originalTracked }()

	// Seed a permanent exception (in-scope, so it is reached) to exercise the silent-accept path.
	originalPermanent := permanentExceptions
	permanentExceptions = append(append([]bindingException{}, originalPermanent...), bindingException{
		name:     "perm-admin",
		checkID:  "cluster-admin",
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{permSubject},
		note:     "by-design",
	})
	defer func() { permanentExceptions = originalPermanent }()

	clusterAdminRule := rule([]string{"*"}, []string{"*"}, []string{"*"})
	escalateRule := rbacv1.PolicyRule{Verbs: []string{"escalate"}, APIGroups: []string{rbacv1.GroupName}, Resources: []string{"clusterroles"}}
	impersonateRule := rbacv1.PolicyRule{Verbs: []string{"impersonate"}, APIGroups: []string{""}, Resources: []string{"users"}}
	readOnlyRule := rule([]string{"get", "list", "watch"}, []string{""}, []string{"pods"})

	tests := []struct {
		name            string
		binding         rbacv1.ClusterRoleBinding
		rolesByName     map[string][]rbacv1.PolicyRule
		wantCheckIDs    []string // expected failing check ids, in order
		wantFlakeChecks map[string]bool
	}{
		{
			name:         "direct cluster-admin",
			binding:      binding("some-admin", "admin-role", saSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{"admin-role": {clusterAdminRule}},
			wantCheckIDs: []string{"cluster-admin"},
		},
		{
			name:         "escalate rbac",
			binding:      binding("escalator", "escalate-role", saSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{"escalate-role": {escalateRule}},
			wantCheckIDs: []string{"escalate-rbac"},
		},
		{
			name:         "impersonate",
			binding:      binding("imp", "imp-role", saSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{"imp-role": {impersonateRule}},
			wantCheckIDs: []string{"impersonate"},
		},
		{
			name:        "two checks tripped",
			binding:     binding("multi", "multi-role", saSubject),
			rolesByName: map[string][]rbacv1.PolicyRule{"multi-role": {escalateRule, impersonateRule}},
			// escalate-rbac precedes impersonate in escalationChecks ordering.
			wantCheckIDs: []string{"escalate-rbac", "impersonate"},
		},
		{
			name:         "benign role emits nothing",
			binding:      binding("benign", "read-role", saSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{"read-role": {readOnlyRule}},
			wantCheckIDs: nil,
		},
		{
			name:         "missing role reference emits nothing",
			binding:      binding("dangling", "does-not-exist", saSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{},
			wantCheckIDs: nil,
		},
		{
			// A binding that only grants to a ServiceAccount outside the core namespaces (e.g. a
			// transient e2e test namespace) is out of scope and emits nothing.
			name:         "out-of-scope namespace emits nothing",
			binding:      binding("e2e-test-thing", "admin-role", rbacv1.Subject{Kind: "ServiceAccount", Namespace: "e2e-test-thing", Name: "sa"}),
			rolesByName:  map[string][]rbacv1.PolicyRule{"admin-role": {clusterAdminRule}},
			wantCheckIDs: nil,
		},
		{
			// A binding whose only subject is a cluster-wide group (no namespace) is out of scope.
			name:         "group-only subject emits nothing",
			binding:      binding("group-admin", "admin-role", rbacv1.Subject{Kind: "Group", APIGroup: rbacv1.GroupName, Name: "system:masters"}),
			rolesByName:  map[string][]rbacv1.PolicyRule{"admin-role": {clusterAdminRule}},
			wantCheckIDs: nil,
		},
		{
			// A permanent exception (in scope) is silently accepted: no JUnit result at all, and the
			// cluster-admin short-circuit still suppresses the subsumed checks.
			name:         "permanent exception emits nothing",
			binding:      binding("perm-admin", "cluster-admin", permSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{"cluster-admin": {clusterAdminRule}},
			wantCheckIDs: nil,
		},
		{
			// A new subject on the permanently-excepted binding no longer matches the approved grant,
			// so the exemption is revoked and the finding hard-fails.
			name: "permanent exception revoked by new subject",
			binding: binding("perm-admin", "cluster-admin",
				permSubject,
				rbacv1.Subject{Kind: "ServiceAccount", Namespace: "openshift-ns", Name: "sneaky"}),
			rolesByName:  map[string][]rbacv1.PolicyRule{"cluster-admin": {clusterAdminRule}},
			wantCheckIDs: []string{"cluster-admin"},
		},
		{
			// A tracked exception flakes: one fail + one pass for that check.
			name:            "tracked exception flakes",
			binding:         binding("tracked-esc", "escalate-role", saSubject),
			rolesByName:     map[string][]rbacv1.PolicyRule{"escalate-role": {escalateRule}},
			wantCheckIDs:    []string{"escalate-rbac"},
			wantFlakeChecks: map[string]bool{"escalate-rbac": true},
		},
		{
			// Same tracked binding+check but repointed at a different escalating role: the roleRef no
			// longer matches the approved grant, so it hard-fails instead of flaking.
			name:         "tracked exception revoked by roleref change",
			binding:      binding("tracked-esc", "other-escalate-role", saSubject),
			rolesByName:  map[string][]rbacv1.PolicyRule{"other-escalate-role": {escalateRule}},
			wantCheckIDs: []string{"escalate-rbac"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			junits := evaluateBinding(tc.binding, tc.rolesByName)

			// Collect the failing cases (non-nil FailureOutput) and passing cases (nil) per name.
			failsByName := map[string]int{}
			passesByName := map[string]int{}
			failOrder := []string{}
			for _, j := range junits {
				if j.FailureOutput != nil {
					failsByName[j.Name]++
					failOrder = append(failOrder, checkIDForName(j.Name))
				} else {
					passesByName[j.Name]++
				}
			}

			if len(failOrder) != len(tc.wantCheckIDs) {
				t.Fatalf("expected %d failing checks %v, got %d: %v", len(tc.wantCheckIDs), tc.wantCheckIDs, len(failOrder), failOrder)
			}
			for i, want := range tc.wantCheckIDs {
				if failOrder[i] != want {
					t.Errorf("failing check %d: expected %q, got %q", i, want, failOrder[i])
				}
			}

			// A flaked check has exactly one fail and one pass for the same name; a hard-fail has no
			// matching pass.
			for _, c := range escalationChecks {
				name := "[sig-auth] clusterrolebinding \"" + tc.binding.Name + "\" must not grant permission to " + c.desc
				wantFlake := tc.wantFlakeChecks[c.id]
				if wantFlake {
					if failsByName[name] != 1 || passesByName[name] != 1 {
						t.Errorf("check %q expected flake (1 fail + 1 pass), got fail=%d pass=%d", c.id, failsByName[name], passesByName[name])
					}
					continue
				}
				if passesByName[name] != 0 {
					t.Errorf("check %q expected no passing (green) case, got %d", c.id, passesByName[name])
				}
			}
		})
	}
}

func TestRoleGrantsAny(t *testing.T) {
	// A role with resource wildcard should be detected as covering escalate on clusterroles.
	wildcard := []rbacv1.PolicyRule{rule([]string{"*"}, []string{rbacv1.GroupName}, []string{"*"})}
	servant := []rbacv1.PolicyRule{{Verbs: []string{"escalate"}, APIGroups: []string{rbacv1.GroupName}, Resources: []string{"clusterroles"}}}
	if hit, matched := roleGrantsAny(wildcard, servant); !hit || len(matched) == 0 {
		t.Errorf("expected wildcard role to grant escalate, got hit=%v matched=%v", hit, matched)
	}

	// An unrelated role should not match.
	unrelated := []rbacv1.PolicyRule{rule([]string{"get"}, []string{""}, []string{"configmaps"})}
	if hit, _ := roleGrantsAny(unrelated, servant); hit {
		t.Errorf("expected unrelated role not to grant escalate")
	}
}
