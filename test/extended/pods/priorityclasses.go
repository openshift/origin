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

var excludedPriorityClassNamespaces = append([]string{
	// OpenShift marketplace can have workloads pods that are created from Jobs which just have hashes
	// They can be safely ignored as they're not part of core platform.
	// In the future, if this assumption changes, we can revisit it.
	"openshift-marketplace"},

	// Managed services namespaces
	exutil.ManagedServiceNamespaces.UnsortedList()...,
)

var excludedPriorityClassPods = map[string][]string{
	// Managed services pods running in platform namespaces
	"openshift-monitoring": {
		"osd-rebalance-infra-nodes",
		"configure-alertmanager-operator",
		"osd-cluster-ready",
	},

	// OLM does not provide an option to set priority class on pods created
	// by subscription.  https://issues.redhat.com/browse/OCPBUGS-54879
	// tracks removing this exclusion.
	"openshift-operators": {
		"servicemesh-operator3-",
	},

	// Istio does not provide an option to set priority class on gateway
	// pods.  https://issues.redhat.com/browse/OCPBUGS-54652 tracks setting
	// the annotation so that we can remove this exclusion.
	"openshift-ingress": {
		"gateway-",
	},
}

var _ = Describe("[sig-arch] Managed cluster should", func() {
	oc := exutil.NewCLIWithoutNamespace("pod")

	It("ensure platform components have system-* priority class associated", Label("Size:S"), func() {
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
