package pods

import (
	"context"
	"fmt"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	"strings"

	. "github.com/onsi/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = Describe("[sig-arch] Managed cluster should", func() {
	oc := exutil.NewCLIWithoutNamespace("pod")

	It("ensure platform components have system-* priority class associated", func() {
		// iterate over the references to find valid images
		pods, err := oc.KubeFramework().ClientSet.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}

		//Component name as keys and BZ's as values
		knownBugs := map[string]string{
			"community-operators":  "https://bugzilla.redhat.com/show_bug.cgi?id=1954869",
			"redhat-marketplace":   "https://bugzilla.redhat.com/show_bug.cgi?id=1954869",
			"redhat-operators":     "https://bugzilla.redhat.com/show_bug.cgi?id=1954869",
			"certified-operators":  "https://bugzilla.redhat.com/show_bug.cgi?id=1954869",
			"image-pruner":         "https://bugzilla.redhat.com/show_bug.cgi?id=1954891",
			"ingress-canary":       "https://bugzilla.redhat.com/show_bug.cgi?id=1954892",
			"network-check-source": "https://bugzilla.redhat.com/show_bug.cgi?id=1954870",
			"network-check-target": "https://bugzilla.redhat.com/show_bug.cgi?id=1954870",
			"migrator":             "https://bugzilla.redhat.com/show_bug.cgi?id=1954868",
			"downloads":            "https://bugzilla.redhat.com/show_bug.cgi?id=1954866",
			"pod-identity-webhook": "https://bugzilla.redhat.com/show_bug.cgi?id=1954865",
		}
		// list of pods that use images not in the release payload
		invalidPodPriority := sets.NewString()
		knownBugList := sets.NewString()
		// a pod in a namespace that begins with kube-* or openshift-*
		namespacePrefixes := sets.NewString("kube-", "openshift-")
		var knownBugKey string
		for _, pod := range pods.Items {
			// exclude non-openshift and non-kubernetes platform pod
			if !hasPrefixSet(pod.Namespace, namespacePrefixes) {
				continue
			}
			// OpenShift marketplace can have workloads pods that are created from Jobs which just have hashes
			// They can be safely ignored as they're not part of core platform.
			// In future, if this assumption changes, we can revisit it.
			if pod.Namespace == "openshift-marketplace" {
				continue
			}
			var componentName string
			lastHyphenIndex := strings.LastIndex(pod.Name, "-")
			if lastHyphenIndex > 0 {
				componentName = pod.Name[:lastHyphenIndex]
			}
			knownBugKey = componentName
			labels := pod.ObjectMeta.Labels
			// strip the pod-template-hash for the component again
			if _, ok := labels["pod-template-hash"]; ok {
				knownBugKey = knownBugKey[:strings.LastIndex(knownBugKey, "-")]
			} else if _, ok := labels["job-name"]; ok { // or for job, we have a image-pruner running as job.
				knownBugKey = knownBugKey[:strings.LastIndex(knownBugKey, "-")]
			}
			if bz, ok := knownBugs[knownBugKey]; ok {
				knownBugList.Insert(fmt.Sprintf("Component %v has a bug associated already: %v", knownBugKey, bz))
				continue
			}
			if !strings.HasPrefix(pod.Spec.PriorityClassName, "system-") && !strings.EqualFold(pod.Spec.PriorityClassName, "openshift-user-critical") {
				invalidPodPriority.Insert(fmt.Sprintf("%s/%s (currently %q)", pod.Namespace, pod.Name, pod.Spec.PriorityClassName))
			}
		}
		if len(knownBugList) > 0 {
			result.Flakef("Workloads with outstanding bugs:\n%s", strings.Join(knownBugList.List(), "\n"))
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
