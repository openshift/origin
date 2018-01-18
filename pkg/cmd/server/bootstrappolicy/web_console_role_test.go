package bootstrappolicy

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

// NOTE: If this test fails, talk to the web console team to decide if your
// new role(s) should be visible to an end user in the web console.

var rolesToHide = sets.NewString(
	"cluster-admin",
	"cluster-debugger",
	"cluster-reader",
	"cluster-status",
	"registry-admin",
	"registry-editor",
	"registry-viewer",
	"self-access-reviewer",
	"self-provisioner",
	"storage-admin",
	"sudoer",
	"system:auth-delegator",
	"system:basic-user",
	"system:build-strategy-custom",
	"system:build-strategy-docker",
	"system:build-strategy-jenkinspipeline",
	"system:build-strategy-source",
	"system:discovery",
	"system:heapster",
	"system:image-auditor",
	"system:image-pruner",
	"system:image-signer",
	"system:kube-aggregator",
	"system:kube-controller-manager",
	"system:kube-dns",
	"system:kube-scheduler",
	"system:master",
	"system:node",
	"system:node-admin",
	"system:node-bootstrapper",
	"system:node-problem-detector",
	"system:node-proxier",
	"system:node-reader",
	"system:oauth-token-deleter",
	"system:openshift:templateservicebroker-client",
	"system:persistent-volume-provisioner",
	"system:registry",
	"system:router",
	"system:scope-impersonation",
	"system:sdn-manager",
	"system:sdn-reader",
	"system:webhook",
	"system:certificates.k8s.io:certificatesigningrequests:nodeclient",
	"system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
	"system:aggregate-to-admin",
	"system:aggregate-to-edit",
	"system:aggregate-to-view",
	"system:aws-cloud-provider",
	"system:openshift:aggregate-to-admin",
	"system:openshift:aggregate-to-edit",
	"system:openshift:aggregate-to-view",
)

func TestSystemOnlyRoles(t *testing.T) {
	show := sets.NewString()
	hide := sets.NewString()

	for _, role := range GetBootstrapClusterRoles() {
		if isControllerRole(&role) {
			if !isSystemOnlyRole(&role) {
				t.Errorf("Controller role %q is missing the system only annotation", role.Name)
			}
			continue // assume all controller roles can be ignored even though we require the annotation
		}
		if isSystemOnlyRole(&role) {
			hide.Insert(role.Name)
		} else {
			show.Insert(role.Name)
		}
	}

	if !show.Equal(rolesToShow) || !hide.Equal(rolesToHide) {
		t.Error("The list of expected end user roles has been changed.  Please discuss with the web console team to update role annotations.")
		t.Logf("These roles are visible but not in rolesToShow: %v", show.Difference(rolesToShow).List())
		t.Logf("These roles are hidden but not in rolesToHide: %v", hide.Difference(rolesToHide).List())
		t.Logf("These roles are in rolesToShow but are missing from the visible list: %v", rolesToShow.Difference(show).List())
		t.Logf("These roles are in rolesToHide but are missing from the hidden list: %v", rolesToHide.Difference(hide).List())
	}
}

// this logic must stay in sync w/the web console for this test to be valid/valuable
// it is the same logic that is run on the membership page
func isSystemOnlyRole(role *rbac.ClusterRole) bool {
	return role.Annotations[roleSystemOnly] == roleIsSystemOnly
}

// helper so that roles following this pattern do not need to be manaully added
// to the hide list
func isControllerRole(role *rbac.ClusterRole) bool {
	return strings.HasPrefix(role.Name, "system:controller:") ||
		strings.HasSuffix(role.Name, "-controller") ||
		strings.HasPrefix(role.Name, "system:openshift:controller:")
}
