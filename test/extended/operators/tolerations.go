package operators

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = Describe("[sig-arch] Managed cluster should", func() {
	oc := exutil.NewCLIWithoutNamespace("operators")

	It("ensure control plane operators do not make themselves unevictable", Label("Size:M"), func() {
		// iterate over the references to find valid images
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		// list of pods that use images not in the release payload
		invalidPodTolerations := sets.NewString()
		// a pod in a namespace that begins with kube-* or openshift-* must come from our release payload
		// TODO components in openshift-operators may not come from our payload, may want to weaken restriction
		namespacePrefixes := sets.NewString("kube-", "openshift-")
		excludedNamespaces := sets.NewString("openshift-kube-apiserver", "openshift-kube-controller-manager", "openshift-kube-scheduler", "openshift-etcd", "openshift-openstack-infra", "openshift-ovirt-infra")
		// exclude these pods from checks
		whitelistPods := sets.NewString("network-operator", "dns-operator", "olm-operators", "gcp-routes-controller", "ovnkube-master", "must-gather")
		// The kube-apiserver proxy exists only in Hypershift. It is used to proxy the kubelet->kube-apiserver connection, hence it must be a static pod
		// which makes the kubelet add a keyless noExecution toleration, so we have to exclude it here.
		whitelistPods.Insert("kube-apiserver-proxy")
		// The kube-rbac-proxy-crio pod exists in the machine config operator
		// namespace. It is a static pod which makes the kubelet add a keyless
		// noExecution toleration, so we have to exclude it here.
		whitelistPods.Insert("kube-rbac-proxy-crio")
		for _, pod := range pods.Items {
			// exclude non-control plane namespaces
			if !hasPrefixSet(pod.Namespace, namespacePrefixes) {
				continue
			}
			// exclude static pod managed namespaces
			if excludedNamespaces.Has(pod.Namespace) {
				continue
			}
			if hasPrefixSet(pod.Name, whitelistPods) {
				continue
			}
			// exclude pods started by DaemonSets
			if ownedByDaemonSet(pod) {
				continue
			}
			for _, toleration := range pod.Spec.Tolerations {
				if toleration.Operator == v1.TolerationOpExists && toleration.Effect == v1.TaintEffectNoExecute {
					if toleration.Key == "" {
						invalidPodTolerations.Insert(fmt.Sprintf("%s/%s tolerates all taints", pod.Namespace, pod.Name))
					}
					if toleration.Key == v1.TaintNodeUnreachable || toleration.Key == v1.TaintNodeNotReady {
						if toleration.TolerationSeconds == nil {
							invalidPodTolerations.Insert(fmt.Sprintf("%s/%s tolerates %s with no tolerationSeconds", pod.Namespace, pod.Name, toleration.Key))
						}
						/* TODO enable this once we can get tolerationSeconds explicity defined in every component */
						/*if *toleration.TolerationSeconds == 300 {
							invalidPodTolerations.Insert(fmt.Sprintf("%s/%s tolerates %s with default tolerationSeconds", pod.Namespace, pod.Name, toleration.Key))
						}*/
					}
				}
			}
		}
		// log for debugging output before we ultimately fail
		//e2e.Logf("Pods found with invalid tolerations: %s", strings.Join(invalidPodTolerations.List(), "\n"))
		numInvalidPodTolerations := len(invalidPodTolerations)
		if numInvalidPodTolerations > 0 {
			e2e.Failf("\n%d pods found with invalid tolerations:\n%s", numInvalidPodTolerations, strings.Join(invalidPodTolerations.List(), "\n"))
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

func ownedByDaemonSet(pod v1.Pod) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}
