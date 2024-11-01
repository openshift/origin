package pods

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var excludedPriorityClassNamespaces = []string{
	// OpenShift marketplace can have workloads pods that are created from Jobs which just have hashes
	// They can be safely ignored as they're not part of core platform.
	// In future, if this assumption changes, we can revisit it.
	"openshift-marketplace",

	// Managed service namespaces
	"openshift-deployment-validation-operator",
	"openshift-observability-operator",
}

var excludedPriorityClassPods = map[string][]string{
	// Managed service pods
	"openshift-addon-operator": {
		"addon-operator-catalog",
		"addon-operator-manager",
		"addon-operator-webhooks",
	},
	"openshift-backplane-srep": {
		"osd-delete-ownerrefs-serviceaccounts",
	},
	"openshift-backplane": {
		"osd-delete-backplane-serviceaccounts",
	},
	"openshift-custom-domains-operator": {
		"custom-domains-operator",
		"custom-domains-operator-registry",
	},
	"openshift-managed-node-metadata-operator": {
		"managed-node-metadata-operator",
		"managed-node-metadata-operator-registry",
	},
	"openshift-managed-upgrade-operator": {
		"managed-upgrade-operator",
		"managed-upgrade-operator-catalog",
	},
	"openshift-monitoring": {
		"osd-rebalance-infra-nodes",
		"configure-alertmanager-operator",
		"osd-cluster-ready",
	},
	"openshift-must-gather-operator": {
		"must-gather-operator",
		"must-gather-operator-registry",
	},
	"openshift-ocm-agent-operator": {
		"ocm-agent",
		"ocm-agent-operator",
	},
	"openshift-osd-metrics": {
		"osd-metrics-exporter",
		"osd-metrics-exporter-registry",
	},
	"openshift-package-operator": {
		"package-operator-manager",
	},
	"openshift-rbac-permissions": {
		"rbac-permissions-operator",
		"rbac-permissions-operator-registry",
	},
	"openshift-route-monitor-operator": {
		"route-monitor-operator-controller-manager",
		"blackbox-exporter",
		"route-monitor-operator-registry",
	},
	"openshift-security": {
		"audit-exporter",
		"splunkforwarder-ds",
	},
	"openshift-splunk-forwarder-operator": {
		"splunk-forwarder-operator",
		"splunk-forwarder-operator-catalog",
	},
	"openshift-sre-pruning": {
		"deployments-pruner",
		"builds-pruner",
	},
	"openshift-validation-webhook": {
		"validation-webhook",
	},
}

var _ = Describe("[sig-arch] Managed cluster should", func() {
	oc := exutil.NewCLIWithoutNamespace("pod")

	It("ensure platform components have system-* priority class associated", func() {
		// iterate over the references to find valid images
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		//Component name as keys and BZ's as values
		// list of pods that use images not in the release payload
		invalidPodPriority := sets.NewString()
		// a pod in a namespace that begins with kube-* or openshift-*
		namespacePrefixes := sets.NewString("kube-", "openshift-")
	podLoop:
		for _, pod := range pods.Items {
			// exclude non-openshift and non-kubernetes platform pod
			if !hasPrefixSet(pod.Namespace, namespacePrefixes) {
				continue
			}

			// Exception lists from above
			for _, ns := range excludedPriorityClassNamespaces {
				if pod.Namespace == ns {
					continue podLoop
				}
			}

			if prefixes, ok := excludedPriorityClassPods[pod.Namespace]; ok {
				for _, prefix := range prefixes {
					if strings.HasPrefix(pod.Name, prefix) {
						continue podLoop
					}
				}
			}

			if !strings.HasPrefix(pod.Spec.PriorityClassName, "system-") && !strings.EqualFold(pod.Spec.PriorityClassName, "openshift-user-critical") {
				invalidPodPriority.Insert(fmt.Sprintf("%s/%s (currently %q)", pod.Namespace, pod.Name, pod.Spec.PriorityClassName))
			}
		}

		numInvalidPodPriorities := len(invalidPodPriority)
		if numInvalidPodPriorities > 0 {
			e2e.Failf("\n%d pods found with invalid priority class (should be openshift-user-critical or begin with system-):\n%s", numInvalidPodPriorities, strings.Join(invalidPodPriority.List(), "\n"))
		}
	})
})

func hasPrefixSet(name string, set sets.String) bool {
	for _, prefix := range set.List() {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
