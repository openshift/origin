package bootstrappolicy

import (
	"strings"
	"testing"

	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"
)

func TestOpenshiftNamespacePolicyNamespaces(t *testing.T) {
	for ns := range namespaceRoles {
		if ns == DefaultOpenShiftSharedResourcesNamespace {
			continue
		}
		if strings.HasPrefix(ns, "openshift-") {
			continue
		}
		t.Errorf("bootstrap role in %q,but must be under %q", ns, "openshift-")
	}

	for ns := range namespaceRoleBindings {
		if ns == DefaultOpenShiftSharedResourcesNamespace {
			continue
		}
		if strings.HasPrefix(ns, "openshift-") {
			continue
		}
		t.Errorf("bootstrap rolebinding in %q,but must be under %q", ns, "openshift-")
	}
}

func TestKubeNamespacePolicyNamespaces(t *testing.T) {
	for ns := range bootstrappolicy.NamespaceRoles() {
		if strings.HasPrefix(ns, "kube-") {
			continue
		}
		t.Errorf("bootstrap role in %q,but must be under %q", ns, "kube-")
	}

	for ns := range bootstrappolicy.NamespaceRoles() {
		if strings.HasPrefix(ns, "kube-") {
			continue
		}
		t.Errorf("bootstrap rolebinding in %q,but must be under %q", ns, "kube-")
	}
}
