package topologymanager

import (
	"fmt"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
)

type testEntry struct {
	Description string
	PPS         PodParamsList
}

var _ = g.Describe("[Serial][sig-node][Suite:openshift/performance][Feature:TopologyManager] Configured cluster", func() {
	defer g.GinkgoRecover()

	// this MUST be called here!
	oc = exutil.NewCLI("topology-manager", exutil.KubeConfigPath())

	BeforeAll(Before)
	AfterAll(After)

	g.Context("with gu workload", func() {
		t.DescribeTable("should guarantee NUMA-aligned cpu cores in gu pods",
			func(pps PodParamsList) {
				// this is a coarse-grained check to avoid false negatives
				if requestCpu, ok := enoughCoresInTheCluster(workerNodes, pps); !ok {
					g.Skip(fmt.Sprintf("not enough CPU resources in the cluster requested=%v", requestCpu))
				}

				// shortcuts
				client := tmFixture.Client
				oc := tmFixture.OC
				ns := oc.KubeFramework().Namespace.Name

				testPods := pps.MakeBusyboxPods(ns)
				updatedPods := createPodsOnNodeSync(client, ns, nil, testPods...)
				// rely on cascade deletion when namespace is deleted

				for _, pod := range updatedPods {
					for _, cnt := range pod.Spec.Containers {
						out, err := getAllowedCpuListForContainer(oc, pod, &cnt)
						e2e.ExpectNoError(err)
						envOut := makeAllowedCpuListEnv(out)
						e2e.Logf("pod %q container %q: %s", pod.Name, cnt.Name, envOut)

						numaNodes, err := getNumaNodeCount(oc, pod, &cnt)
						e2e.ExpectNoError(err)
						envInfo := testEnvInfo{
							numaNodes: numaNodes,
						}
						numaRes, err := checkNUMAAlignment(oc.KubeFramework(), pod, &cnt, envOut, &envInfo)
						e2e.ExpectNoError(err)
						ok := numaRes.CheckAlignment()

						o.Expect(ok).To(o.BeTrue(), "misaligned NUMA resources: %s", numaRes.String())
					}

				}

			},
			t.Entry("with single pod, single container requesting one core", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
						CpuLimit:   1000,
					}},
				},
			}),
			t.Entry("with single pod, single container requesting multiple cores", []PodParams{
				{
					Containers: []ContainerParams{{
						// 4 cores is a random value. Anything > 2, to make sure to request 2 physical cores also if HT is enabled, is fine
						CpuRequest: 4000,
						CpuLimit:   4000,
					}},
				},
			}),
			t.Entry("with single pod, multiple containers requesting one core each", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
						CpuLimit:   1000,
					}, {
						CpuRequest: 1000,
						CpuLimit:   1000,
					}},
				},
			}),
			t.Entry("with multiple pods, each with a single container requesting one core", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
						CpuLimit:   1000,
					}},
				},
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
						CpuLimit:   1000,
					}},
				},
			}),
			t.Entry("with multiple pods, each with multiple containers requesting one core", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
						CpuLimit:   1000,
					}, {
						CpuRequest: 1000,
						CpuLimit:   1000,
					}},
				},
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
						CpuLimit:   1000,
					}, {
						CpuRequest: 1000,
						CpuLimit:   1000,
					}},
				},
			}),
		)
	})
})

type ContainerParams struct {
	CpuRequest int64 // millicores
	CpuLimit   int64 // millicores
}

type PodParams struct {
	Containers []ContainerParams
}

func (pp PodParams) TotalRequest() int64 {
	var total int64
	for _, cnt := range pp.Containers {
		total += cnt.CpuRequest
	}
	return total
}

func (pp PodParams) MakeBusyboxPod(namespace string) *corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "test-",
			Labels: map[string]string{
				"test": "",
			},
		},
	}

	for i, cp := range pp.Containers {
		cnt := corev1.Container{
			Name:    fmt.Sprintf("test-%d", i),
			Image:   "busybox",
			Command: []string{"sleep", "10h"},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		}
		if cp.CpuRequest > 0 {
			cnt.Resources.Requests[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%dm", cp.CpuRequest))
		}
		if cp.CpuLimit > 0 {
			cnt.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(fmt.Sprintf("%dm", cp.CpuLimit))
		}
		pod.Spec.Containers = append(pod.Spec.Containers, cnt)
	}

	return &pod
}

type PodParamsList []PodParams

func (pps PodParamsList) MakeBusyboxPods(namespace string) []*corev1.Pod {
	var pods []*corev1.Pod
	for _, pp := range pps {
		pods = append(pods, pp.MakeBusyboxPod(namespace))
	}
	return pods
}

func (pps PodParamsList) TotalCpuRequest() int64 {
	var total int64
	for _, pp := range pps {
		total += pp.TotalRequest()
	}
	return total
}

func enoughCoresInTheCluster(nodes []corev1.Node, pps PodParamsList) (resource.Quantity, bool) {
	requestCpu := resource.MustParse(fmt.Sprintf("%dm", pps.TotalCpuRequest()))
	e2e.Logf("checking request %v on %d nodes", requestCpu, len(nodes))

	for _, node := range nodes {
		availCpu, ok := node.Status.Allocatable[corev1.ResourceCPU]
		o.Expect(ok).To(o.BeTrue())
		o.Expect(availCpu.IsZero()).To(o.BeFalse())

		e2e.Logf("node %q available cpu %v requested cpu %v", node.Name, availCpu, requestCpu)
		if availCpu.Cmp(requestCpu) >= 1 {
			e2e.Logf("at least node %q has enough resources, cluster OK", node.Name)
			return requestCpu, true
		}
	}

	return requestCpu, false
}
