package pods

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
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

var _ = Describe("[sig-node] Pod priority should match the default priorityClassName values", func() {
	defer GinkgoRecover()
	var (
		oc                           = exutil.NewCLI("priority-class-name")
		systemNodeCriticalPodFile    = exutil.FixturePath("testdata", "priority-class-name", "system-node-critical.yaml")
		systemClusterCriticalPodFile = exutil.FixturePath("testdata", "priority-class-name", "system-cluster-critical.yaml")
	)

	It("system-node-critical", func() {
		By("creating the pods")
		err := oc.Run("create").Args("-n", oc.Namespace(), "-f", systemNodeCriticalPodFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		By("checking the pod priority")
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), "pod-with-system-node-critical-priority-class", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect((int)(*pod.Spec.Priority)).To(o.Equal(2000001000))
		err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), "pod-with-system-node-critical-priority-class", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	It("system-cluster-critical", func() {
		By("creating the pods")
		err := oc.Run("create").Args("-n", oc.Namespace(), "-f", systemClusterCriticalPodFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		By("checking the pod priority")
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), "pod-with-system-cluster-critical-priority-class", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect((int)(*pod.Spec.Priority)).To(o.Equal(2000000000))
		err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), "pod-with-system-cluster-critical-priority-class", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
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
