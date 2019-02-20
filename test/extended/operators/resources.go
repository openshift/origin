package operators

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/v1/resource"
	v1qos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func blamePodAndContainer(pod v1.Pod, container v1.Container) string {
	return fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)
}

func blamePod(pod v1.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

var _ = Describe("[Feature:Platform][Smoke] Managed cluster should", func() {
	f := e2e.NewDefaultFramework("operators")

	It("should validate resource requirements for pods on master nodes", func() {
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

		guaranteedPods := sets.NewString()
		podsCPULimits := sets.NewString()
		podsNoCPURequests := sets.NewString()

		for i := range pods.Items {
			pod := pods.Items[i]
			if !nodeNames.Has(pod.Spec.NodeName) {
				continue
			}

			switch v1qos.GetPodQOS(&pod) {
			case v1.PodQOSGuaranteed:
				guaranteedPods.Insert(blamePod(pod))
			}

			requests, limits := resource.PodRequestsAndLimits(&pod)
			if _, found := requests["cpu"]; !found {
				podsNoCPURequests.Insert(blamePod(pod))
			}
			if _, found := limits["cpu"]; found {
				podsCPULimits.Insert(blamePod(pod))
			}
			if memory, found := limits["memory"]; found {
				e2e.Logf("Pod %s has memory limit %s, not recommended", blamePod(pod), memory.String())
			}
			if memory, found := requests["memory"]; found {
				e2e.Logf("Pod %s has no memory request %s, not recommended", blamePod(pod), memory.String())
			}
		}
		if len(guaranteedPods) > 0 {
			e2e.Failf("Invalid control plane pods found with guaranteed qos which impacts cpu latency %s", strings.Join(guaranteedPods.List(), ","))
		}
		if len(podsCPULimits) > 0 {
			e2e.Failf("Invalid control plane pods found using cpu limits impacts cpu latency %s", strings.Join(podsCPULimits.List(), ","))
		}
		// TODO when we support scheduling masters, our control plane components must assert a request
		if len(podsNoCPURequests) > 0 {
			e2e.Logf("Invalid control plane pods found missing cpu requests which may result in cpu starvation %s, recommend min 10m if operator", strings.Join(podsNoCPURequests.List(), ","))
		}
	})
})
