package operators

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-arch] Managed cluster should", func() {
	oc := exutil.NewCLIWithoutNamespace("operators")

	It("ensure control plane pods do not run in best-effort QoS", Label("Size:M"), func() {
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

		excludeNamespaces := sets.NewString(
			"openshift-operator-lifecycle-manager",
			"openshift-marketplace",

			// Managed services namespaces OSD-26069
			"openshift-backplane-srep",
			"openshift-backplane",
			"openshift-custom-domains-operator",
			"openshift-osd-metrics",
			"openshift-rbac-permissions",
			"openshift-route-monitor-operator",
			"openshift-splunk-forwarder-operator",
			"openshift-sre-pruning",
			"openshift-validation-webhook",
			"openshift-velero",
		)
		excludePodPrefix := sets.NewString(
			"revision-pruner-",  // operators have retry logic built in. these are like jobs but cannot rely on jobs
			"installer-",        // operators have retry logic built in. these are like jobs but cannot rely on jobs
			"must-gather-",      // operators have retry logic built in. these are like jobs but cannot rely on jobs
			"recycler-for-nfs-", // recyclers are allowed to fail.  I guess a cluster-admin works out that he needs to take manual action.  sig-storage

			// Managed services pods OSD-26069
			"configure-alertmanager-operator-",
			"osd-",
			"splunkforwarder-",
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
