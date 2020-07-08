package topologymanager

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
)

const (
	networkAttachmentAnnotation string = "k8s.v1.cni.cncf.io/networks"
	sriovInterfaceName          string = "sriov1"
)

var _ = g.Describe("[Serial][sig-node][Feature:TopologyManager] Configured cluster", func() {
	defer g.GinkgoRecover()

	var (
		oc                 = exutil.NewCLI("topology-manager")
		client             clientset.Interface // shortcut
		roleWorkerLabel    string
		workerNodes        []corev1.Node
		topoMgrNodes       []corev1.Node
		deviceResourceName string
		err                error
	)

	g.BeforeEach(func() {
		client = oc.KubeFramework().ClientSet
		o.Expect(client).ToNot(o.BeNil())

		roleWorkerLabel = getValueFromEnv(roleWorkerEnvVar, defaultRoleWorker, "worker role")

		workerNodes, err = getNodeByRole(client, roleWorkerLabel)
		e2e.ExpectNoError(err)
		o.Expect(workerNodes).ToNot(o.BeEmpty())

		topoMgrNodes = filterNodeWithTopologyManagerPolicy(workerNodes, client, oc, kubeletconfigv1beta1.SingleNumaNodeTopologyManager)

		deviceResourceName = getValueFromEnv(resourceNameEnvVar, defaultResourceName, "resource name")
		// we don't handle yet an uneven device amount on worker nodes. IOW, we expect the same amount of devices on each node
	})

	g.Context("with non-gu workload", func() {
		t.DescribeTable("should run with no regressions",
			func(pps PodParamsList) {
				if requestCpu, ok := enoughCoresInTheCluster(workerNodes, pps); !ok {
					g.Skip(fmt.Sprintf("not enough CPU resources in the cluster requested=%v", requestCpu))
				}

				ns := oc.KubeFramework().Namespace.Name
				testingPods := pps.MakeBusyboxPods(ns, deviceResourceName)
				// we just want to run pods and check they actually go running
				updatedPods := createPods(client, ns, testingPods...)
				defer deletePods(oc, updatedPods)
			},
			// rhbz#1813397 - k8s issue83775
			t.Entry("with single pod, single container requesting 1 core", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest: 1000,
					}},
				},
			}),
			// rhbz#1813397 - k8s issue83775
			t.Entry("with single pod, single container requesting multiple cores", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest: 2500,
					}},
				},
			}),
		)
	})

	g.Context("with gu workload", func() {
		var (
			node      *corev1.Node
			numaNodes int64
			coreCount int64
		)

		g.BeforeEach(func() {
			expectNonZeroNodes(topoMgrNodes, "topology manager not configured on all nodes")

			var nn int
			node, nn = findNodeWithMultiNuma(topoMgrNodes, client, oc)

			message := "multi-NUMA node system not found in the cluster"
			if _, ok := os.LookupEnv(strictCheckEnvVar); ok {
				o.Expect(node).ToNot(o.BeNil(), message)
			}
			if node == nil {
				g.Skip(message)
			}
			numaNodes = int64(nn)

			// This MUST be capacity (theoretical max) not Allocatable
			cpuCapacity, ok := node.Status.Capacity[corev1.ResourceCPU]
			o.Expect(ok).To(o.BeTrue())
			o.Expect(cpuCapacity.IsZero()).To(o.BeFalse())

			coreCount, ok = cpuCapacity.AsInt64()
			o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("failed to convert the CPU resource: %v", cpuCapacity))
			o.Expect(coreCount).ToNot(o.BeZero(), fmt.Sprintf("capacity cores %d equals zero", coreCount))
			o.Expect(coreCount%numaNodes).To(o.BeZero(), fmt.Sprintf("capacity cores %d not multiple of detected NUMA nodes count %d", coreCount, numaNodes))

			e2e.Logf("CPU capacity on %q: %d", node.Name, coreCount)
		})

		g.Describe("attached to SRIOV networks", func() {
			g.It("should let resource-aligned PODs have working SRIOV network interface", func() {
				if deviceResourceName == "" {
					g.Skip("no device resource configured")
				}

				// any random amount of cores > 2 (to be HT-neutral)
				pps := PodParamsList{
					{
						Containers: []ContainerParams{{
							CpuRequest:    4000,
							CpuLimit:      4000,
							DeviceRequest: 1,
							DeviceLimit:   1,
						}},
					},
					{
						Containers: []ContainerParams{{
							CpuRequest:    4000,
							CpuLimit:      4000,
							DeviceRequest: 1,
							DeviceLimit:   1,
						}},
					},
				}

				if requestCpu, ok := enoughCoresInTheCluster(topoMgrNodes, pps); !ok {
					g.Skip(fmt.Sprintf("not enough CPU resources in the cluster requested=%v", requestCpu))
				}
				if requestDevices, ok := enoughDevicesInTheCluster(topoMgrNodes, deviceResourceName, pps); !ok {
					g.Skip(fmt.Sprintf("not enough devices %q in the cluster requested=%v", deviceResourceName, requestDevices))
				}

				testFw := oc.KubeFramework()
				testNs := getValueFromEnv(sriovNetworkNamespaceEnvVar, testFw.Namespace.Name, "SRIOV network namespace")

				sriovNetwork := getValueFromEnv(sriovNetworkEnvVar, defaultSriovNetwork, "SRIOV network")
				o.Expect(sriovNetwork).ToNot(o.BeEmpty(), fmt.Sprintf("missing SRIOV network to join"))

				pods := pps.MakeBusyboxPods(testNs, deviceResourceName)
				for _, testPod := range pods {
					testPod.Annotations = map[string]string{
						networkAttachmentAnnotation: fmt.Sprintf("%s@%s", sriovNetwork, sriovInterfaceName),
					}
				}
				testingPods := createPods(client, testNs, pods...)
				defer deletePods(oc, testingPods)
				expectPodsHaveAlignedResources(testingPods, oc, deviceResourceName)

				ipAddrs, err := getPodsIPAddrs(testingPods, sriovInterfaceName)
				e2e.ExpectNoError(err)
				family := getValueFromEnv(ipFamilyEnvVar, defaultIPFamily, "IP family")

				for _, srcPod := range testingPods {
					for _, dstAddrs := range ipAddrs {
						dstAddr, err := findFirstIPForFamily(dstAddrs, family)
						e2e.ExpectNoError(err)
						err = pingAddrFromPod(oc, srcPod, &(srcPod.Spec.Containers[0]), dstAddr)
						e2e.ExpectNoError(err)
						e2e.Logf("ping from %q to %q OK", srcPod.Name, dstAddr)
					}
				}
			})
		})

		g.Describe("saturating NUMA nodes", func() {
			g.It("should reject pod requesting more cores than a single NUMA node have", func() {
				// the following assumes:
				// 1. even split of cores between NUMA nodes. Do we know when this is not the case?
				// 2. even amount of cores in the system. Gone forever are the times of the athlon II X3?
				// so for any even number of numa nodes > 1, there is no way this request can be fullfilled
				// by just a single NUMA
				cpuReq := 1 + (coreCount / int64(numaNodes))
				pp := PodParams{
					Containers: []ContainerParams{{
						CpuRequest:    cpuReq * 1000,
						CpuLimit:      cpuReq * 1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				}
				e2e.Logf("using cpuReq=%d PodParams: %#v", cpuReq, pp)
				if requestDevices, ok := enoughDevicesInTheCluster(topoMgrNodes, deviceResourceName, PodParamsList{pp}); !ok {
					g.Skip(fmt.Sprintf("not enough devices %q in the cluster requested=%v", deviceResourceName, requestDevices))
				}
				testFw := oc.KubeFramework()
				testNs := testFw.Namespace.Name

				testPod := pp.MakeBusyboxPod(testNs, deviceResourceName)
				testPod.Spec.NodeSelector = map[string]string{
					labelHostname: node.Name,
				}

				pod := testFw.PodClient().Create(testPod)
				testingPods := []*corev1.Pod{pod}
				defer deletePods(oc, testingPods)
				err := e2epod.WaitForPodCondition(testFw.ClientSet, testNs, pod.Name, "Failed", 30*time.Second, func(pod *corev1.Pod) (bool, error) {
					if pod.Status.Phase != corev1.PodPending {
						return true, nil
					}
					return false, nil
				})
				e2e.ExpectNoError(err)
				pod, err = testFw.PodClient().Get(context.Background(), pod.Name, metav1.GetOptions{})
				e2e.ExpectNoError(err)

				if pod.Status.Phase != corev1.PodFailed {
					e2e.Failf("pod %s not failed: %v", pod.Name, pod.Status)
				}
				if !isTopologyAffinityError(pod) {
					e2e.Failf("pod %s failed for wrong reason: %q", pod.Name, pod.Status.Reason)
				}
			})

			g.It("should allow a pod requesting as many cores as a full NUMA node have", func() {
				cpuReq := coreCount / int64(numaNodes)
				pps := PodParamsList{
					{
						Containers: []ContainerParams{{
							CpuRequest:    cpuReq * 1000,
							CpuLimit:      cpuReq * 1000,
							DeviceRequest: 1,
							DeviceLimit:   1,
						}},
					},
				}
				e2e.Logf("using cpuReq=%d PodParams: %#v", cpuReq, pps)
				if requestDevices, ok := enoughDevicesInTheCluster(topoMgrNodes, deviceResourceName, pps); !ok {
					g.Skip(fmt.Sprintf("not enough devices %q in the cluster requested=%v", deviceResourceName, requestDevices))
				}

				testNs := oc.KubeFramework().Namespace.Name
				pods := pps.MakeBusyboxPods(testNs, deviceResourceName)

				setNodeForPods(pods, node)

				testingPods := createPods(client, testNs, pods...)
				defer deletePods(oc, testingPods)
				expectPodsHaveAlignedResources(testingPods, oc, deviceResourceName)
			})

			// rhbz#1813567
			// a well-behaving TM must admit only *one* of the pods.
			// The reference test environment is like this:
			// 1. Two NUMA nodes
			// 2. A SRIOV Device - with 2+ VFs - on NUMA node 1. No VFs on the other node (see below)
			// 3. No other workload is running together with the tests (all resources available to the test)
			// 4. The pods request are arranged in such a way that only one pod can run in aligned environment.
			//    The simplest way to enable this is to have VFs only on one NUMA node (see above).
			g.It("should guarantee correct allocation with concurrent creation", func() {
				// the following assumes:
				// 1. even split of cores between NUMA nodes. Do we know when this is not the case?
				// 2. even amount of cores in the system. Gone forever are the times of the athlon II X3?
				// so for any even number of numa nodes > 1, there is no way this request can be fullfilled
				// by just a single NUMA

				// random fraction of a NUMA node, we try to make low enough to fit in low-end servers (16 cores per node)
				coreDelta := (coreCount / int64(numaNodes)) / 4
				if coreDelta < 1 {
					e2e.Logf("too low coreDelta with %v cores and %v numa nodes, forced to 1", coreCount, numaNodes)
					coreDelta = 1
				}

				cpuReq1 := (coreCount / int64(numaNodes)) - coreDelta
				cpuReq2 := coreDelta * 2
				// so the total we asking is ((coreCount / numaNodes) - coreDelta) + (coreDelta * 2)  = (coreCount / numaNodes) + coreDelta < coreCount
				// we know we fit on the node granted the systemReservedCpus are configured properly,
				// so no need to check for enoughCoresInTheCluster

				// invariants:
				// 1. we know we have a free NUMA node (the same one with SRIOV devices attached)
				// 2. the largest pod must fit on a NUMA node, and must leave few free cores
				// 3. the smallest pod must fit on the free cores on the other NUMA node.
				//
				// keep this invariant true if you tune these values: sum(cores) <= 0.75 * system_cores
				pps := PodParamsList{
					{
						Containers: []ContainerParams{{
							CpuRequest:    cpuReq1 * 1000,
							CpuLimit:      cpuReq1 * 1000,
							DeviceRequest: 1,
							DeviceLimit:   1,
						}},
					},
					{
						Containers: []ContainerParams{{
							CpuRequest:    cpuReq2 * 1000,
							CpuLimit:      cpuReq2 * 1000,
							DeviceRequest: 1,
							DeviceLimit:   1,
						}},
					},
				}

				e2e.Logf("using PodParams: %#v", pps)

				if requestDevices, ok := enoughDevicesInTheCluster(topoMgrNodes, deviceResourceName, pps); !ok {
					g.Skip(fmt.Sprintf("not enough devices %q in the cluster requested=%v", deviceResourceName, requestDevices))
				}

				testNs := oc.KubeFramework().Namespace.Name
				pods := pps.MakeBusyboxPods(testNs, deviceResourceName)

				setNodeForPods(pods, node)

				isNotPending := func(phase corev1.PodPhase) bool {
					return phase == corev1.PodRunning || phase == corev1.PodFailed
				}
				testingPods := createPodsAndWait(client, testNs, isNotPending, pods...)
				defer deletePods(oc, testingPods)

				// we don't know -and better not care- about the ordering here.
				// we only know that exactly one pod must fail for TopologyAffinityError
				affinityErrors := 0
				for _, testingPod := range testingPods {
					if testingPod.Status.Phase == corev1.PodFailed && isTopologyAffinityError(testingPod) {
						affinityErrors++
					}
				}
				o.Expect(affinityErrors).To(o.Equal(1), "unexpected number of pods failed for TopologyAffinityError: %v (expected 1)", affinityErrors)

				// the other pod, OTOH, must have been properly aligned
				// we do this check later because it's more expensive.
				for _, testingPod := range testingPods {
					if testingPod.Status.Phase == corev1.PodRunning {
						expectSinglePodHaveAlignedResources(testingPod, oc, deviceResourceName)
					}
				}
			})

		})

		t.DescribeTable("should guarantee NUMA-aligned cpu cores in gu pods",
			func(pps PodParamsList) {
				expectNonZeroNodes(topoMgrNodes, "topology manager not configured on all nodes")

				if requestCpu, ok := enoughCoresInTheCluster(topoMgrNodes, pps); !ok {
					g.Skip(fmt.Sprintf("not enough CPU resources in the cluster requested=%v", requestCpu))
				}
				if requestDevices, ok := enoughDevicesInTheCluster(topoMgrNodes, deviceResourceName, pps); !ok {
					g.Skip(fmt.Sprintf("not enough devices %q in the cluster requested=%v", deviceResourceName, requestDevices))
				}

				testNs := oc.KubeFramework().Namespace.Name
				pods := pps.MakeBusyboxPods(testNs, deviceResourceName)
				testingPods := createPods(client, testNs, pods...)
				defer deletePods(oc, testingPods)
				expectPodsHaveAlignedResources(testingPods, oc, deviceResourceName)
			},
			t.Entry("with single pod, single container requesting 1 core, 1 device", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
			}),
			t.Entry("with single pod, single container requesting 4 cores, 1 device", []PodParams{
				{
					// 4 cores is a random value. Anything >= 2, to make sure to request 2 physical cores also if HT is enabled, is fine
					Containers: []ContainerParams{{
						CpuRequest:    4000,
						CpuLimit:      4000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
			}),
			t.Entry("with single pod, multiple containers requesting 1 core, 1 device each", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}, {
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
			}),
			t.Entry("with multiple pods, each with a single container requesting 1 core, 1 device", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
				{
					Containers: []ContainerParams{{
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
			}),
			t.Entry("with multiple pods, each with a single container requesting 2 core, 1 device", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest:    2000,
						CpuLimit:      2000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
				{
					Containers: []ContainerParams{{
						CpuRequest:    2000,
						CpuLimit:      2000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
			}),
			t.Entry("with multiple pods, each with multiple containers requesting 1 core, 1 device", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}, {
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
				{
					Containers: []ContainerParams{{
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}, {
						CpuRequest:    1000,
						CpuLimit:      1000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}},
				},
			}),
			t.Entry("with multiple pods, each with multiple containers requesting 1 core, only one requesting 1 device", []PodParams{
				{
					Containers: []ContainerParams{{
						CpuRequest:    2000,
						CpuLimit:      2000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}, {
						CpuRequest: 2000,
						CpuLimit:   2000,
					}},
				},
				{
					Containers: []ContainerParams{{
						CpuRequest:    2000,
						CpuLimit:      2000,
						DeviceRequest: 1,
						DeviceLimit:   1,
					}, {
						CpuRequest: 2000,
						CpuLimit:   2000,
					}},
				},
			}),
		)
	})
})

type ContainerParams struct {
	CpuRequest    int64 // millicores
	CpuLimit      int64 // millicores
	DeviceRequest int64
	DeviceLimit   int64
}

type PodParams struct {
	Containers []ContainerParams
}

func (pp PodParams) TotalCpuRequest() int64 {
	var total int64
	for _, cnt := range pp.Containers {
		total += cnt.CpuRequest
	}
	return total
}

func (pp PodParams) TotalDeviceRequest() int64 {
	var total int64
	for _, cnt := range pp.Containers {
		total += cnt.DeviceRequest
	}
	return total
}

func (pp PodParams) MakeBusyboxPod(namespace, deviceName string) *corev1.Pod {
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

		if deviceName != "" {
			if cp.DeviceRequest > 0 {
				cnt.Resources.Requests[corev1.ResourceName(deviceName)] = resource.MustParse(fmt.Sprintf("%d", cp.DeviceRequest))
			}
			if cp.DeviceLimit > 0 {
				cnt.Resources.Limits[corev1.ResourceName(deviceName)] = resource.MustParse(fmt.Sprintf("%d", cp.DeviceLimit))
			}
		}
		pod.Spec.Containers = append(pod.Spec.Containers, cnt)
	}

	return &pod
}

type PodParamsList []PodParams

func (pps PodParamsList) MakeBusyboxPods(namespace, deviceName string) []*corev1.Pod {
	var pods []*corev1.Pod
	for _, pp := range pps {
		pods = append(pods, pp.MakeBusyboxPod(namespace, deviceName))
	}
	return pods
}

func (pps PodParamsList) TotalCpuRequest() int64 {
	var total int64
	for _, pp := range pps {
		total += pp.TotalCpuRequest()
	}
	return total
}

func (pps PodParamsList) TotalDeviceRequest() int64 {
	var total int64
	for _, pp := range pps {
		total += pp.TotalDeviceRequest()
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

func enoughDevicesInTheCluster(nodes []corev1.Node, deviceResourceName string, pps PodParamsList) (resource.Quantity, bool) {
	requestDevs := resource.MustParse(fmt.Sprintf("%d", pps.TotalDeviceRequest()))
	e2e.Logf("checking request %v on %d nodes", requestDevs, len(nodes))

	// bogus device, autopass check
	if deviceResourceName == "" {
		return requestDevs, true
	}

	for _, node := range nodes {
		availDevs, ok := node.Status.Allocatable[corev1.ResourceName(deviceResourceName)]
		o.Expect(ok).To(o.BeTrue())
		o.Expect(availDevs.IsZero()).To(o.BeFalse())

		e2e.Logf("node %q available devs %v requested devs %v", node.Name, availDevs, requestDevs)
		if availDevs.Cmp(requestDevs) >= 1 {
			e2e.Logf("at least node %q has enough resources, cluster OK", node.Name)
			return requestDevs, true
		}
	}

	return requestDevs, false
}

func isTopologyAffinityError(pod *corev1.Pod) bool {
	re := regexp.MustCompile(`Topology.*Affinity.*Error`)
	return re.MatchString(pod.Status.Reason)
}

func deletePods(oc *exutil.CLI, pods []*corev1.Pod) {
	client := oc.AsAdmin().KubeFramework().ClientSet
	for _, pod := range pods {
		e2epod.DeletePodWithWait(client, pod)
	}
}

func expectSinglePodHaveAlignedResources(pod *corev1.Pod, oc *exutil.CLI, deviceResourceName string) {
	for _, cnt := range pod.Spec.Containers {
		out, err := getAllowedCpuListForContainer(oc, pod, &cnt)
		e2e.ExpectNoError(err)
		envOut := makeAllowedCpuListEnv(out)

		podEnv, err := getEnvironmentVariables(oc, pod, &cnt)
		e2e.ExpectNoError(err)
		envOut += podEnv

		e2e.Logf("Full environment for pod %q container %q: %q", pod.Name, cnt.Name, envOut)

		numaNodes, err := getNumaNodeCountFromContainer(oc, pod, &cnt)
		e2e.ExpectNoError(err)
		envInfo := testEnvInfo{
			numaNodes:         numaNodes,
			sriovResourceName: deviceResourceName,
		}
		numaRes, err := checkNUMAAlignment(oc.KubeFramework(), pod, &cnt, envOut, &envInfo)
		e2e.ExpectNoError(err)
		ok := numaRes.CheckAlignment()

		o.Expect(ok).To(o.BeTrue(), "misaligned NUMA resources: %s", numaRes.String())
	}

}

func expectPodsHaveAlignedResources(updatedPods []*corev1.Pod, oc *exutil.CLI, deviceResourceName string) {
	for _, pod := range updatedPods {
		expectSinglePodHaveAlignedResources(pod, oc, deviceResourceName)
	}
}
