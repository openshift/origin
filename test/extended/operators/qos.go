package operators

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = Describe("[sig-arch] Managed cluster should", func() {
	oc := exutil.NewCLIWithoutNamespace("operators")

	It("ensure control plane containers have requests set for cpu and memory", func() {
		// iterate over the references to find valid images
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		// list of containers that do not set appropriate requests
		invalidContainers := sets.NewString()
		// a pod in a namespace that begins with kube-* or openshift-* must come from our release payload
		// TODO components in openshift-operators may not come from our payload, may want to weaken restriction
		namespacePrefixes := sets.NewString("kube-", "openshift-")
		excludeNamespaces := sets.NewString("openshift-operator-lifecycle-manager", "openshift-marketplace", "openshift-service-catalog-removed")
		excludePodPrefix := sets.NewString(
			"revision-pruner-",  // operators have retry logic built in. these are like jobs but cannot rely on jobs
			"installer-",        // operators have retry logic built in. these are like jobs but cannot rely on jobs
			"must-gather-",      // operators have retry logic built in. these are like jobs but cannot rely on jobs
			"recycler-for-nfs-", // recyclers are allowed to fail.  I guess a cluster-admin works out that he needs to take manual action.  sig-storage
		)
		for _, pod := range pods.Items {
			// exclude non-control plane namespaces
			if !hasPrefixSet(pod.Namespace, namespacePrefixes) {
				continue
			}
			if excludeNamespaces.Has(pod.Namespace) {
				continue
			}
			if hasPrefixSet(pod.Name, excludePodPrefix) {
				continue
			}
			for i, container := range pod.Spec.InitContainers {
				checkContainerRequests(invalidContainers, &pod, &container, fmt.Sprintf("initContainers[%d]", i))
			}
			for i, container := range pod.Spec.Containers {
				checkContainerRequests(invalidContainers, &pod, &container, fmt.Sprintf("containers[%d]", i))
			}
		}
		numInvalidContainers := len(invalidContainers)
		if numInvalidContainers > 0 {
			e2e.Failf("\n%d containers found without expected requests ( https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md#resources-and-limits ):\n%s", numInvalidContainers, strings.Join(invalidContainers.List(), "\n"))
		}
	})
})

func checkContainerRequests(invalidContainers sets.String, pod *v1.Pod, container *v1.Container, source string) {
	unsetResources := sets.NewString()
	for _, resource := range []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory} {
		quantity := container.Resources.Requests[resource]
		if quantity.IsZero() {
			unsetResources.Insert(string(resource))
		}
	}
	if len(unsetResources) > 0 {
		invalidContainers.Insert(fmt.Sprintf("%s/%s container %s (%s) is not requesting required resources: %s", pod.Namespace, pod.Name, source, container.Name, strings.Join(unsetResources.List(), ", ")))
	}
}
