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

	It("ensure control plane pods do not run in best-effort QoS", func() {
		// iterate over the references to find valid images
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		// list of pods that use images not in the release payload
		invalidPodQoS := sets.NewString()
		// a pod in a namespace that begins with kube-* or openshift-* must come from our release payload
		// TODO components in openshift-operators may not come from our payload, may want to weaken restriction
		namespacePrefixes := sets.NewString("kube-", "openshift-")
		excludeNamespaces := sets.NewString("openshift-operator-lifecycle-manager", "openshift-marketplace")
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
			if pod.Status.QOSClass == v1.PodQOSBestEffort {
				invalidPodQoS.Insert(fmt.Sprintf("%s/%s is running in best-effort QoS", pod.Namespace, pod.Name))
			}
		}
		numInvalidPodQoS := len(invalidPodQoS)
		if numInvalidPodQoS > 0 {
			e2e.Failf("\n%d pods found in best-effort QoS:\n%s", numInvalidPodQoS, strings.Join(invalidPodQoS.List(), "\n"))
		}
	})
})
