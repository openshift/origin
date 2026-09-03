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

// Check IDs are the stable slugs that identify each escalation check. They key exceptions to a
// specific check, so exceptions must reference the same constant the check is registered under.
const (
	// clusterAdminCheckID must be listed first in escalationChecks so that its short-circuit (skip the
	// subsumed, more specific checks) works.
	clusterAdminCheckID      = "cluster-admin"
	escalateRBACCheckID      = "escalate-rbac"
	impersonateCheckID       = "impersonate"
	admissionWebhooksCheckID = "admission-webhooks"
	csrSigningCheckID        = "csr-signing"
)

// trackedExceptions are approved escalation grants that are known issues we intend to fix. Each is
// paired with a tracking Jira. These flake (fail + pass) rather than hard-failing, so they stay
// visible in CI and can be burned down.
//
// No new entries should be added to this list without the sign off of an OpenShift Architect.
var trackedExceptions = []bindingException{
	{
		name:     "cloud-credential-operator-rolebinding",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "cloud-credential-operator-role",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cloud-credential-operator", Name: "cloud-credential-operator"}},
		note:     "TODO",
	},
	{
		name:     "cloud-credential-operator-rolebinding",
		checkID:  escalateRBACCheckID,
		roleRef:  "cloud-credential-operator-role",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cloud-credential-operator", Name: "cloud-credential-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-autoscaler-operator",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "cluster-autoscaler-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-machine-api", Name: "cluster-autoscaler-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-baremetal-operator",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "cluster-baremetal-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-machine-api", Name: "cluster-baremetal-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-monitoring-operator",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "cluster-monitoring-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-monitoring", Name: "cluster-monitoring-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-network-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-network-operator", Name: "cluster-network-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-olm-operator-role",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "cluster-olm-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cluster-olm-operator", Name: "cluster-olm-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-olm-operator-role",
		checkID:  escalateRBACCheckID,
		roleRef:  "cluster-olm-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cluster-olm-operator", Name: "cluster-olm-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-storage-operator-role",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cluster-storage-operator", Name: "cluster-storage-operator"}},
		note:     "TODO",
	},
	{
		name:     "cluster-version-operator-1",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cluster-version", Name: "cluster-version-operator"}},
		note:     "TODO",
	},
	{
		name:     "custom-account-openshift-machine-config-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-machine-config-operator", Name: "machine-config-operator"}},
		note:     "TODO",
	},
	{
		name:     "machine-api-operator",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "machine-api-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-machine-api", Name: "machine-api-operator"}},
		note:     "TODO",
	},
	{
		name:     "olm-operator-binding-openshift-operator-lifecycle-manager",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "system:controller:operator-lifecycle-manager",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-operator-lifecycle-manager", Name: "olm-operator-serviceaccount"}},
		note:     "TODO",
	},
	{
		name:     "olm-operator-binding-openshift-operator-lifecycle-manager",
		checkID:  escalateRBACCheckID,
		roleRef:  "system:controller:operator-lifecycle-manager",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-operator-lifecycle-manager", Name: "olm-operator-serviceaccount"}},
		note:     "TODO",
	},
	{
		name:     "openshift-dns-operator",
		checkID:  impersonateCheckID,
		roleRef:  "openshift-dns-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-dns-operator", Name: "dns-operator"}},
		note:     "TODO",
	},
	{
		name:     "openshift-ingress-operator",
		checkID:  impersonateCheckID,
		roleRef:  "openshift-ingress-operator",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-ingress-operator", Name: "ingress-operator"}},
		note:     "TODO",
	},
	{
		name:     "openshift-ingress-operator-sail-library",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "openshift-ingress-operator-sail-library",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-ingress-operator", Name: "ingress-operator"}},
		note:     "TODO",
	},
	{
		name:     "openshift-ingress-operator-sail-library",
		checkID:  escalateRBACCheckID,
		roleRef:  "openshift-ingress-operator-sail-library",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-ingress-operator", Name: "ingress-operator"}},
		note:     "TODO",
	},
	{
		name:     "operator-controller-cluster-admin-rolebinding",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-operator-controller", Name: "operator-controller-controller-manager"}},
		note:     "TODO",
	},
	{
		name:     "storage-version-migration-migrator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-storage-version-migrator", Name: "kube-storage-version-migrator-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:controller:clusterrole-aggregation-controller",
		checkID:  escalateRBACCheckID,
		roleRef:  "system:controller:clusterrole-aggregation-controller",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "kube-system", Name: "clusterrole-aggregation-controller"}},
		note:     "TODO",
	},
	{
		name:     "system:controller:generic-garbage-collector",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "system:controller:generic-garbage-collector",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "kube-system", Name: "generic-garbage-collector"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:controller:service-ca",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "system:openshift:controller:service-ca",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-service-ca", Name: "service-ca"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:controller:template-instance-controller:admin",
		checkID:  impersonateCheckID,
		roleRef:  "admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-infra", Name: "template-instance-controller"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:controller:template-instance-finalizer-controller:admin",
		checkID:  impersonateCheckID,
		roleRef:  "admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-infra", Name: "template-instance-finalizer-controller"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:oauth-apiserver",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-oauth-apiserver", Name: "oauth-apiserver-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:openshift-apiserver",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-apiserver", Name: "openshift-apiserver-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:openshift-authentication",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-authentication", Name: "oauth-openshift"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:authentication",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-authentication-operator", Name: "authentication-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:cluster-kube-scheduler-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-scheduler-operator", Name: "openshift-kube-scheduler-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:etcd-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-etcd-operator", Name: "etcd-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:kube-apiserver-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-apiserver-operator", Name: "kube-apiserver-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:kube-apiserver-recovery",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-apiserver", Name: "localhost-recovery-client"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:kube-controller-manager-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-controller-manager-operator", Name: "kube-controller-manager-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:kube-controller-manager-recovery",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-controller-manager", Name: "localhost-recovery-client"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:kube-scheduler-recovery",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-scheduler", Name: "localhost-recovery-client"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:kube-storage-version-migrator-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-storage-version-migrator-operator", Name: "kube-storage-version-migrator-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-apiserver-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-apiserver-operator", Name: "openshift-apiserver-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-config-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-config-operator", Name: "openshift-config-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-controller-manager-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-controller-manager-operator", Name: "openshift-controller-manager-operator"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-etcd-installer",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-etcd", Name: "installer-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-kube-apiserver-installer",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-apiserver", Name: "installer-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-kube-controller-manager-installer",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-controller-manager", Name: "installer-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:openshift-kube-scheduler-installer",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-kube-scheduler", Name: "installer-sa"}},
		note:     "TODO",
	},
	{
		name:     "system:openshift:operator:service-ca-operator",
		checkID:  clusterAdminCheckID,
		roleRef:  "cluster-admin",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-service-ca-operator", Name: "service-ca-operator"}},
		note:     "TODO",
	},
	{
		name:     "vmware-vsphere-csi-driver-operator-clusterrolebinding",
		checkID:  admissionWebhooksCheckID,
		roleRef:  "vmware-vsphere-csi-driver-operator-clusterrole",
		subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "openshift-cluster-csi-drivers", Name: "vmware-vsphere-csi-driver-operator"}},
		note:     "TODO",
	},
}

// permanentExceptions are approved escalation grants that are legitimate and expected forever. These
// are silently accepted: they produce no JUnit result at all. Reserve this list for grants that are
// correct by design and will never be "fixed".
//
// The canonical cluster-admin -> system:masters binding is not listed here: its only subject is a
// cluster-wide Group, so bindingInScope already excludes it (and every other non-namespaced grant).
//
// No new entries should be added to this list without the sign off of an OpenShift Architect.
var permanentExceptions = []bindingException{}

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
	// flake puts the check in the temporary "discovery" mode we use while burning down findings:
	// tracked exceptions are silenced entirely (we already know about them, so we keep the signal
	// focused on what still needs triage) and every other finding flakes (a fail plus a passing
	// duplicate) instead of hard-failing, so it stays visible in Sippy without blocking CI. When
	// false, the check enforces normally: tracked exceptions flake and any unhandled finding hard-fails.
	//
	// This is deliberately per-check so a newly added check can start in flake mode (gather the real
	// set of exceptions before enforcing) while existing checks enforce, or vice versa.
	flake bool
}

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
		flake: true, // temporary: downgrade to flake while we burn down findings.
	},
	{
		// The `escalate` and `bind` verbs are the two ways to bypass Kubernetes' built-in RBAC
		// escalation prevention. Plain create/update/patch on clusterroles/clusterrolebindings does
		// NOT allow escalation on its own: the rbac policybased storage strategy runs
		// ConfirmNoEscalation and rejects writing/binding any permission the caller does not already
		// hold, unless the caller has `escalate` (for role rules) or `bind` (for a role reference).
		//
		// `bind` is flagged regardless of any resourceNames scoping. Even a bind scoped to a specific,
		// non-cluster-admin role still lets the subject grant that role's permissions to arbitrary
		// subjects (bind bypasses ConfirmNoEscalation), which is an escalation for those subjects. The
		// API server does honor resourceNames for bind, but scoping does not make it safe, so
		// roleGrantsAny strips resourceNames for the bind atom (see bindAlwaysFlaggedVerbs). Legitimate
		// scoped binds are allowlisted via the exception list rather than silently ignored.
		id:   escalateRBACCheckID,
		desc: "escalate or bind RBAC roles",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("escalate").Groups(rbacv1.GroupName).Resources("clusterroles", "roles").RuleOrDie(),
			rbacv1helpers.NewRule("bind").Groups(rbacv1.GroupName).Resources("clusterroles", "roles").RuleOrDie(),
		},
		flake: true, // temporary: downgrade to flake while we burn down findings.
	},
	{
		// Impersonation lets the subject send requests AS another user or group without needing that
		// identity's credentials. Impersonating the `system:masters` group (or any cluster-admin-bound
		// user/service account) yields cluster-admin immediately, so this bypasses RBAC entirely.
		//
		// Creating service account tokens (the `serviceaccounts/token` TokenRequest subresource) is the
		// same escalation by another route: a token IS the service account's credential, so a subject
		// that can mint a token for a cluster-admin-bound service account becomes cluster-admin.
		// Unlike a top-level create, this subresource create honors resourceNames (the SA name is known
		// at authorization time), so a grant scoped to specific service accounts is a deliberate,
		// reviewable narrow grant and is NOT flagged — only an unscoped grant (mint a token for ANY SA)
		// fires. See roleGrantsAny / isSubresourceAtom.
		id:   impersonateCheckID,
		desc: "impersonate users, groups, or service accounts, or mint service account tokens",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("impersonate").Groups("").Resources("users", "groups", "serviceaccounts").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups("").Resources("serviceaccounts/token").RuleOrDie(),
		},
		flake: true, // temporary: downgrade to flake while we burn down findings.
	},
	{
		// Webhook configurations intercept every write to the API server. Whoever can create or modify
		// them can point a webhook at their own endpoint to read the contents of (and, for mutating
		// webhooks, rewrite) any admitted object cluster-wide — including secrets and RBAC objects —
		// giving them an out-of-band path to cluster-admin.
		//
		// update/patch scoped to specific resourceNames are NOT an escalation path and are not
		// reported: a subject that can only modify a named webhook config it already owns cannot reach
		// arbitrary objects, and rbacvalidation.Covers already treats such resourceName-restricted
		// rules as not covering the unscoped atoms below (so they never fire). create, on the other
		// hand, is always reported even when scoped by resourceName: the API server ignores
		// resourceNames for create, so the restriction is ineffective and the subject can still mint an
		// arbitrary malicious webhook (see roleGrantsAny / resourceNameIneffectiveVerbs).
		id:   admissionWebhooksCheckID,
		desc: "create or modify admission webhook configurations",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create", "update", "patch").Groups("admissionregistration.k8s.io").Resources("mutatingwebhookconfigurations", "validatingwebhookconfigurations").RuleOrDie(),
		},
		flake: true, // temporary: downgrade to flake while we burn down findings.
	},
	{
		// Control over certificate issuance is control over identity. A subject that can approve a CSR
		// and sign it (via the kubernetes.io signers) can mint a client certificate for an arbitrary
		// username and group — e.g. group `system:masters` — and authenticate as cluster-admin.
		id:   csrSigningCheckID,
		desc: "approve or sign certificate signing requests",
		rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("approve").Groups("certificates.k8s.io").Resources("certificatesigningrequests").RuleOrDie(),
			rbacv1helpers.NewRule("sign").Groups("certificates.k8s.io").Resources("signers").RuleOrDie(),
		},
		flake: true, // temporary: downgrade to flake while we burn down findings.
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

// resourceNameIneffectiveVerbs are verbs for which the API server ignores a rule's resourceNames.
// The restriction is silently ineffective, so a rule that appears to "scope" one of these verbs in
// fact grants it on every object of the resource. Per the Kubernetes RBAC docs, create and
// deletecollection cannot be restricted by resourceName (for create, the object name is not known at
// authorization time).
var resourceNameIneffectiveVerbs = sets.New("create", "deletecollection")

// verbIgnoresResourceNames reports whether the atom's verb is one whose resourceNames the API server
// ignores. Atoms come from BreakdownRule, so each has exactly one verb.
func verbIgnoresResourceNames(verbs []string) bool {
	return resourceNameIneffectiveVerbs.HasAny(verbs...)
}

// bindAlwaysFlaggedVerbs are verbs the API server DOES honor resourceNames for, but which we flag
// regardless of any scoping. `bind` bypasses ConfirmNoEscalation, so any bind of a role — even one
// scoped by resourceName to a specific, non-cluster-admin role — lets the subject grant that role's
// permissions to arbitrary subjects. Scoping does not make it safe, so we treat a scoped bind the
// same as an unrestricted one; legitimate scoped binds are allowlisted via the exception list.
var bindAlwaysFlaggedVerbs = sets.New("bind")

// isSubresourceAtom reports whether the atom targets a subresource (a resource containing "/", e.g.
// "serviceaccounts/token"). This matters for create: for a top-level create the object name is not
// known at authorization time so resourceNames are ignored, but for a subresource create the parent
// object's name IS known, so the API server DOES honor resourceNames. Atoms come from BreakdownRule,
// so each has exactly one resource.
func isSubresourceAtom(resources []string) bool {
	for _, r := range resources {
		if strings.Contains(r, "/") {
			return true
		}
	}
	return false
}

// stripResourceNames returns a copy of rules with ResourceNames cleared. Used to test coverage of a
// verb whose resourceNames the API server ignores, so that an ineffective restriction cannot hide an
// escalation path from rbacvalidation.Covers (which otherwise treats a resourceName-scoped owner rule
// as not covering the unscoped atom).
func stripResourceNames(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(rules))
	for i, r := range rules {
		r.ResourceNames = nil
		out[i] = r
	}
	return out
}

// roleGrantsAny reports whether roleRules cover ANY atomic permission in servantRules, returning the
// matched atoms for reporting. Coverage is wildcard-aware (handled by rbacvalidation.Covers). The
// role's rules are compared with resourceNames stripped when the atom's verb is one whose scoping we
// disregard, so an ineffective or unsafe restriction is still flagged:
//   - create/deletecollection on a top-level resource: the API server ignores resourceNames (the
//     object name is not known at authorization time). Subresource creates (e.g. serviceaccounts/token)
//     are excluded because the parent object's name IS known, so resourceNames are honored there.
//   - bind: resourceNames are honored by the API server, but any bind is an escalation path regardless
//     of scoping (see bindAlwaysFlaggedVerbs).
func roleGrantsAny(roleRules, servantRules []rbacv1.PolicyRule) (bool, []rbacv1.PolicyRule) {
	matched := []rbacv1.PolicyRule{}
	for _, servantRule := range servantRules {
		for _, atom := range rbacvalidation.BreakdownRule(servantRule) {
			candidateRules := roleRules
			if ignoreResourceNamesForAtom(atom) {
				candidateRules = stripResourceNames(roleRules)
			}
			if covered, _ := rbacvalidation.Covers(candidateRules, []rbacv1.PolicyRule{atom}); covered {
				matched = append(matched, atom)
			}
		}
	}
	return len(matched) > 0, matched
}

// ignoreResourceNamesForAtom reports whether the role's resourceNames should be disregarded when
// checking coverage of this atom. Atoms come from BreakdownRule, so each has exactly one verb.
func ignoreResourceNamesForAtom(atom rbacv1.PolicyRule) bool {
	if bindAlwaysFlaggedVerbs.HasAny(atom.Verbs...) {
		return true
	}
	return verbIgnoresResourceNames(atom.Verbs) && !isSubresourceAtom(atom.Resources)
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

// coreNamespacePrefixes are the namespaces that hold core cluster components. We only audit bindings
// that grant to a ServiceAccount in one of these namespaces.
var coreNamespacePrefixes = []string{"kube-", "openshift-"}

// bindingInScope reports whether the binding grants to at least one ServiceAccount in a core
// namespace (prefixed kube- or openshift-). Bindings that only grant to subjects outside those
// namespaces are out of scope: transient e2e test namespaces come and go with random names (so an
// allowlist entry could never match), and cluster-wide groups/users (e.g. system:masters) are not
// namespaced. Restricting to core namespaces keeps the audit focused on the payload's own
// components.
func bindingInScope(binding rbacv1.ClusterRoleBinding) bool {
	for _, subject := range binding.Subjects {
		if subject.Kind != rbacv1.ServiceAccountKind {
			continue
		}
		for _, prefix := range coreNamespacePrefixes {
			if strings.HasPrefix(subject.Namespace, prefix) {
				return true
			}
		}
	}
	return false
}

// evaluateBinding runs every escalation check against a single ClusterRoleBinding and returns the
// resulting JUnit cases. Only checks that fire produce cases, and the outcome depends on the
// exception class of the (binding, check) pair and whether the check is in flake mode (see
// escalationCheck.flake):
//   - permanent exception: emit nothing (legitimate, by-design grant).
//   - tracked exception, check enforcing: emit a fail plus a passing duplicate (a flake) so it stays
//     visible while it is burned down.
//   - tracked exception, check in flake mode: emit nothing (we already know about it; keep the signal
//     focused on untracked findings).
//   - no exception, check enforcing: emit a hard fail.
//   - no exception, check in flake mode: emit a flake (fail + passing duplicate) so it is visible in
//     Sippy without blocking CI.
func evaluateBinding(binding rbacv1.ClusterRoleBinding, rolesByName map[string][]rbacv1.PolicyRule) []*junitapi.JUnitTestCase {
	// Only audit bindings that grant to a core-component ServiceAccount (see bindingInScope).
	if !bindingInScope(binding) {
		return nil
	}

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

			// A check in flake mode silences its tracked exceptions entirely: we already know about them,
			// so we keep the signal focused on untracked findings that still need triage.
			if check.flake {
				if isClusterAdmin {
					break
				}
				continue
			}
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:          testName,
			SystemOut:     msg,
			FailureOutput: &junitapi.FailureOutput{Output: msg},
		})

		// A finding flakes (emit a passing duplicate with the same name) rather than hard-failing when
		// either it is a tracked exception (kept visible while burned down) or the check is in flake mode
		// (temporary discovery phase). Otherwise it hard-fails.
		if tracked || check.flake {
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
