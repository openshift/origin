package rbacadminescalationtests

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	rbacvalidation "k8s.io/component-helpers/auth/rbac/validation"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/pkg/registry/rbac/validation"
)

type escalationChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &escalationChecker{}
}

// bindingException describes an approved escalation grant. An exception applies only when the actual
// binding matches the approved grant exactly: same name, same referenced role, and the same set of
// subjects. Pinning roleRef and subjects means that repointing an allowlisted binding at a different
// role, or granting it to a new subject, revokes the exemption and re-triggers the finding for
// re-review — the exemption covers the grant we reviewed, not just the name.
type bindingException struct {
	name     string
	checkID  string
	roleRef  string
	subjects []rbacv1.Subject
	// note is a tracking Jira for a tracked exception, or a rationale for a permanent one.
	note string
}

// matches reports whether the given (binding, check) pair is exactly the approved grant.
func (e bindingException) matches(binding rbacv1.ClusterRoleBinding, checkID string) bool {
	return e.checkID == checkID &&
		e.name == binding.Name &&
		e.roleRef == binding.RoleRef.Name &&
		subjectSet(e.subjects).Equal(subjectSet(binding.Subjects))
}

// trackedExceptions are approved escalation grants that are known issues we intend to fix. Each is
// paired with a tracking Jira. These flake (fail + pass) rather than hard-failing, so they stay
// visible in CI and can be burned down.
//
// No new entries should be added to this list without the sign off of an OpenShift Architect.
var trackedExceptions = []bindingException{}

// permanentExceptions are approved escalation grants that are legitimate and expected forever. These
// are silently accepted: they produce no JUnit result at all. Reserve this list for grants that are
// correct by design and will never be "fixed".
//
// No new entries should be added to this list without the sign off of an OpenShift Architect.
var permanentExceptions = []bindingException{
	{
		// The cluster-admin binding grants cluster-admin to system:masters by design.
		name:     "cluster-admin",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "Group", APIGroup: rbacv1.GroupName, Name: "system:masters"}},
		note:     "by-design: binds cluster-admin to system:masters",
	},
}

// escalationCheck is a single way in which a ClusterRoleBinding can hand a subject a path to
// cluster-admin. Each check becomes its own JUnit test (and its own independently flakeable
// exception) so that allowing one binding to hold one dangerous permission does not silently waive
// every other escalation check for that binding.
type escalationCheck struct {
	// id is the stable slug used to key exceptions, e.g. "escalate-rbac".
	id string
	// desc is the human phrase embedded in the test name, e.g. "escalate or bind RBAC roles".
	desc string
	// rules are the escalation-enabling permissions. The check fires when the bound role grants ANY
	// atom of these rules.
	rules []rbacv1.PolicyRule
}

// clusterAdminCheckID is the id of the full cluster-admin check. It must be listed first in
// escalationChecks so that its short-circuit (skip the subsumed, more specific checks) works.
const clusterAdminCheckID = "cluster-admin"

// escalationChecks is the curated, tunable set of escalation paths we audit. Broadening this set
// (e.g. cluster-wide secrets read, pod/exec, node proxy) will expand findings and the allowlist, so
// it is deliberately conservative.
var escalationChecks = []escalationCheck{
	{
		// All verbs on all resources in all API groups IS cluster-admin: the subject can read every
		// secret, mutate any object, and grant itself anything. This is the direct, definitional case
		// rather than a path to escalation.
		id:   clusterAdminCheckID,
		desc: "cluster-admin equivalent access (all verbs on all resources)",
		rules: []rbacv1.PolicyRule{
			{Verbs: []string{rbacv1.VerbAll}, APIGroups: []string{rbacv1.APIGroupAll}, Resources: []string{rbacv1.ResourceAll}},
		},
	},
	{
		// The `escalate` and `bind` verbs are the two ways to bypass Kubernetes' built-in RBAC
		// escalation prevention. Plain create/update/patch on clusterroles/clusterrolebindings does
		// NOT allow escalation on its own: the rbac policybased storage strategy runs
		// ConfirmNoEscalation and rejects writing/binding any permission the caller does not already
		// hold, unless the caller has `escalate` (for role rules) or `bind` (for a role reference).
		id:   "escalate-rbac",
		desc: "escalate or bind RBAC roles",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("escalate", "bind").Groups(rbacv1.GroupName).Resources("clusterroles", "roles").RuleOrDie(),
		},
	},
	{
		// Impersonation lets the subject send requests AS another user or group without needing that
		// identity's credentials. Impersonating the `system:masters` group (or any cluster-admin-bound
		// user/service account) yields cluster-admin immediately, so this bypasses RBAC entirely.
		id:   "impersonate",
		desc: "impersonate users, groups, or service accounts",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("impersonate").Groups("").Resources("users", "groups", "serviceaccounts").RuleOrDie(),
		},
	},
	{
		// Webhook configurations intercept every write to the API server. Whoever can create or modify
		// them can point a webhook at their own endpoint to read the contents of (and, for mutating
		// webhooks, rewrite) any admitted object cluster-wide — including secrets and RBAC objects —
		// giving them an out-of-band path to cluster-admin.
		id:   "admission-webhooks",
		desc: "create or modify admission webhook configurations",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create", "update", "patch").Groups("admissionregistration.k8s.io").Resources("mutatingwebhookconfigurations", "validatingwebhookconfigurations").RuleOrDie(),
		},
	},
	{
		// Control over certificate issuance is control over identity. A subject that can approve a CSR
		// and sign it (via the kubernetes.io signers) can mint a client certificate for an arbitrary
		// username and group — e.g. group `system:masters` — and authenticate as cluster-admin.
		id:   "csr-signing",
		desc: "approve or sign certificate signing requests",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("approve").Groups("certificates.k8s.io").Resources("certificatesigningrequests").RuleOrDie(),
			rbacv1helpers.NewRule("sign").Groups("certificates.k8s.io").Resources("signers").RuleOrDie(),
		},
	},
	// Intentionally omitted for noise: `create` on pods/pods/exec (token theft via any mounted SA).
	// Add here if the escalation surface should be widened.
}

// matchException reports whether the given (binding, check) pair matches any approved grant in the
// provided list, returning the associated note.
func matchException(list []bindingException, binding rbacv1.ClusterRoleBinding, checkID string) (string, bool) {
	for _, exception := range list {
		if exception.matches(binding, checkID) {
			return exception.note, true
		}
	}
	return "", false
}

// roleGrantsAny reports whether roleRules cover ANY atomic permission in servantRules, returning the
// matched atoms for reporting. Coverage is wildcard-aware (handled by rbacvalidation.Covers).
func roleGrantsAny(roleRules, servantRules []rbacv1.PolicyRule) (bool, []rbacv1.PolicyRule) {
	matched := []rbacv1.PolicyRule{}
	for _, servantRule := range servantRules {
		for _, atom := range rbacvalidation.BreakdownRule(servantRule) {
			if covered, _ := rbacvalidation.Covers(roleRules, []rbacv1.PolicyRule{atom}); covered {
				matched = append(matched, atom)
			}
		}
	}
	return len(matched) > 0, matched
}

// subjectSet returns a canonical, order-insensitive set of a binding's subjects for exact matching.
// APIGroup is included so that, e.g., a User and a ServiceAccount of the same name are distinct.
func subjectSet(subjects []rbacv1.Subject) sets.Set[string] {
	out := sets.New[string]()
	for _, s := range subjects {
		out.Insert(fmt.Sprintf("%s/%s/%s/%s", s.APIGroup, s.Kind, s.Namespace, s.Name))
	}
	return out
}

// subjectString renders a binding's subjects for inclusion in a failure message.
func subjectString(subjects []rbacv1.Subject) string {
	rendered := sets.NewString()
	for _, s := range subjects {
		if s.Namespace != "" {
			rendered.Insert(fmt.Sprintf("%s/%s/%s", s.Kind, s.Namespace, s.Name))
			continue
		}
		rendered.Insert(fmt.Sprintf("%s/%s", s.Kind, s.Name))
	}
	return strings.Join(rendered.List(), ", ")
}

// rulesToString renders matched permissions in a compact, human-readable form.
func rulesToString(rules []rbacv1.PolicyRule) string {
	compactRules := rules
	if compact, err := validation.CompactRules(rules); err == nil {
		compactRules = compact
	}
	descriptions := sets.NewString()
	for _, rule := range compactRules {
		descriptions.Insert(rbacv1helpers.CompactString(rule))
	}
	return strings.Join(descriptions.List(), "\n")
}

// evaluateBinding runs every escalation check against a single ClusterRoleBinding and returns the
// resulting JUnit cases. Only checks that fire produce cases, and the outcome depends on the
// exception class of the (binding, check) pair:
//   - permanent exception: emit nothing (legitimate, by-design grant).
//   - tracked exception: emit a fail plus a passing duplicate (a flake) so it stays visible.
//   - no exception: emit a hard fail.
func evaluateBinding(binding rbacv1.ClusterRoleBinding, rolesByName map[string][]rbacv1.PolicyRule) []*junitapi.JUnitTestCase {
	// A ClusterRoleBinding's RoleRef always references a ClusterRole. A dangling reference grants
	// nothing, so there is nothing to evaluate.
	roleRules, ok := rolesByName[binding.RoleRef.Name]
	if !ok {
		return nil
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, check := range escalationChecks {
		hit, matched := roleGrantsAny(roleRules, check.rules)
		if !hit {
			continue
		}

		// A full cluster-admin grant covers every more specific escalation check, so evaluating those
		// as well would be redundant noise. Whatever we decide for the cluster-admin check, we must
		// stop after it — including when it is silently excepted, otherwise the subsumed checks would
		// fire and hard-fail. Track that decision here and honor it at the end of the iteration.
		isClusterAdmin := check.id == clusterAdminCheckID

		// Permanent exceptions are silently accepted: emit nothing at all.
		if _, ok := matchException(permanentExceptions, binding, check.id); ok {
			if isClusterAdmin {
				break
			}
			continue
		}

		testName := fmt.Sprintf("[sig-auth] clusterrolebinding %q must not grant permission to %s", binding.Name, check.desc)
		msg := fmt.Sprintf("clusterrolebinding %q (clusterrole %q) grants permission to %s to subjects [%s] via:\n%s",
			binding.Name, binding.RoleRef.Name, check.desc, subjectString(binding.Subjects), rulesToString(matched))

		jira, tracked := matchException(trackedExceptions, binding, check.id)
		if tracked {
			msg += fmt.Sprintf("\n(tracked exception: %s)", jira)
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:          testName,
			SystemOut:     msg,
			FailureOutput: &junitapi.FailureOutput{Output: msg},
		})

		// Tracked exceptions flake rather than hard-fail: emit a passing duplicate with the same name.
		if tracked {
			junits = append(junits, &junitapi.JUnitTestCase{Name: testName})
		}

		if isClusterAdmin {
			break
		}
	}
	return junits
}

// CollectData implements monitortestframework.MonitorTest.
func (e *escalationChecker) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if e.kubeClient == nil {
		return nil, nil, nil
	}

	clusterRoles, err := e.kubeClient.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	rolesByName := make(map[string][]rbacv1.PolicyRule, len(clusterRoles.Items))
	for _, role := range clusterRoles.Items {
		rolesByName[role.Name] = role.Rules
	}

	bindings, err := e.kubeClient.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	// Sort for deterministic ordering of emitted test cases.
	sort.Slice(bindings.Items, func(i, j int) bool {
		return bindings.Items[i].Name < bindings.Items[j].Name
	})

	junits := []*junitapi.JUnitTestCase{}
	for _, binding := range bindings.Items {
		// Dynamic (generateName) binding names would break static test naming; skip them.
		if binding.GenerateName != "" {
			continue
		}
		junits = append(junits, evaluateBinding(binding, rolesByName)...)
	}
	return nil, junits, nil
}

// StartCollection implements monitortestframework.MonitorTest.
func (e *escalationChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	e.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	return nil
}

// PrepareCollection implements monitortestframework.MonitorTest.
func (e *escalationChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// ConstructComputedIntervals implements monitortestframework.MonitorTest.
func (e *escalationChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning time.Time, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals implements monitortestframework.MonitorTest.
func (e *escalationChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

// WriteContentToStorage implements monitortestframework.MonitorTest.
func (e *escalationChecker) WriteContentToStorage(ctx context.Context, storageDir string, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

// Cleanup implements monitortestframework.MonitorTest.
func (e *escalationChecker) Cleanup(ctx context.Context) error {
	return nil
}
