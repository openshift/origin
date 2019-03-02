package operators

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = Describe("[Feature:Platform][Smoke] Managed cluster should", func() {
	f := e2e.NewDefaultFramework("operators")

	It("should ensure control plane pods specify a priority", func() {
		nodes, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		if err != nil {
			e2e.Failf("unable to list nodes: %v", err)
		}
		pods, err := f.ClientSet.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			e2e.Failf("unable to list pods: %v", err)
		}
		nodeNames := sets.NewString()
		for i := range nodes.Items {
			nodeNames.Insert(nodes.Items[i].Name)
		}

		invalidPriority := sets.NewString()
		for i := range pods.Items {
			pod := pods.Items[i]
			if !nodeNames.Has(pod.Spec.NodeName) {
				continue
			}

			if pod.Spec.PriorityClassName != "system-cluster-critical" && pod.Spec.PriorityClassName != "system-node-critical" {
				invalidPriority.Insert(fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			}
		}
		if len(invalidPriority) > 0 {
			e2e.Failf("Control plane pods found with invalid priority that will impact scheduling: %s", strings.Join(invalidPriority.List(), ","))
		}
	})
})
