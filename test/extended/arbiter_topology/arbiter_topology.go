package arbiter_topology

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] expected Master and Arbiter node counts", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("")
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("CLuster is not in HighlyAvailableArbiterMode skipping test ")
		}
	})

	g.It("Should validate that there are Master and Arbiter nodes as specified in the cluster", func() {
		g.By("Counting nodes dynamically based on labels")
		// DONE: instead of manually comparing 2 with mcp node count we want to get the number from install config and compare it with mcp count
		// should it be like this oc get nodes with label or reach out to install config yaml?
		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Master nodes without error")
		expectedMasterCount := len(masterNodes.Items)

		arbiterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter nodes without error")
		expectedArbiterCount := len(arbiterNodes.Items)

		g.By("Counting nodes from MachineConfigPools for Masters and Arbiter")

		mcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.Background(), "master", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Master MachineConfigPool without error")
		o.Expect(int(mcp.Status.MachineCount)).To(o.Equal(expectedMasterCount), "Expected Master MachineConfigPool to match the number of Master nodes")

		arbiterMcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.Background(), "arbiter", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter MachineConfigPool without error")
		o.Expect(int(arbiterMcp.Status.MachineCount)).To(o.Equal(expectedArbiterCount), "Expected Arbiter MachineConfigPool to match the number of Arbiter nodes")
	})
})

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] required pods on the Arbiter node", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("")
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("CLuster is not in HighlyAvailableArbiterMode skipping test ")
		}
	})

	g.It("Should verify that the correct number of pods are running on the Arbiter node", func() {
		g.By("Retrieving the Arbiter node name")
		// DONE: infer the expected pod count from platfrom type, the current supported type is baremetal only
		// we expect only 17 in baremetal

		expectedPodCount := 17
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Infrastructure resource without error")

		platformType := infra.Status.PlatformStatus.Type
		// Set the expected pod count based on the platform type
		switch platformType {
		case v1.BareMetalPlatformType:
			expectedPodCount = 17
		default:
			g.Fail("Unsupported platform type: " + string(platformType))
		}
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve nodes without error")
		o.Expect(nodes.Items).To(o.Not(o.BeEmpty()), "Expected to find at least one Arbiter node")

		podCount := 0

		for _, node := range nodes.Items {
			pods, err := oc.AdminKubeClient().CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
				FieldSelector: "spec.nodeName=" + node.Name + ",status.phase=Running",
			})
			o.Expect(err).To(o.BeNil(), "Expected to retrieve pods without error")
			podCount = len(pods.Items) + podCount
		}
		o.Expect(podCount).To(o.Equal(expectedPodCount), "Expected the correct number of running pods on the Arbiter node")
	})
})

var _ = g.Describe("[sig-node] validate deployment creation on non-Arbiter nodes", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("arbiter-pod-validation").SetManagedNamespace().AsAdmin()
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("CLuster is not in HighlyAvailableArbiterMode skipping test ")
		}
	})

	g.It("Should verify the creation of deployments on arbiter and master nodes", func() {
		g.By("Retrieving Arbiter node")
		arbiterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter nodes without error")
		o.Expect(len(arbiterNodes.Items)).To(o.Equal(1), "Expected to find one Arbiter node exactly")

		var arbiterNodeName string
		for _, node := range arbiterNodes.Items {
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					arbiterNodeName = node.Name
					break
				}
			}
			if arbiterNodeName != "" {
				break
			}
		}
		o.Expect(arbiterNodeName).NotTo(o.BeEmpty(), "Expected to find a Ready Arbiter node")

		g.By("Retrieving Master nodes")
		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Master nodes without error")
		o.Expect(len(masterNodes.Items)).To(o.Equal(2), "Expected to find two Master nodes")

		// Create a map for Master nodes
		masterNodeMap := make(map[string]struct{})
		for _, node := range masterNodes.Items {
			masterNodeMap[node.Name] = struct{}{}
		}

		g.By("Creating a Normal deployment (on Master nodes)")
		_, err = createNormalDeployment(oc)
		o.Expect(err).To(o.BeNil(), "Expected Master busybox deployment creation to succeed")

		g.By("Creating an Arbiter deployment (on Arbiter node)")
		_, err = createArbiterDeployment(oc, arbiterNodeName)
		o.Expect(err).To(o.BeNil(), "Expected Arbiter busybox deployment creation to succeed")

		g.By("Validating Arbiter deployment")
		arbiterSelector, err := labels.Parse("app=busybox-arbiter")
		o.Expect(err).To(o.BeNil(), "Expected to parse Arbiter label selector without error")

		arbiterPods, err := exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()), arbiterSelector, func(pod corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning
		}, 1, time.Second*30)
		o.Expect(err).To(o.BeNil(), "Expected Arbiter pods to be running")
		o.Expect(len(arbiterPods)).To(o.Equal(1), "Expected exactly one Arbiter pod to be running on Arbiter node")

		arbiterPod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), arbiterPods[0], metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter pod without error")
		o.Expect(arbiterPod.Spec.NodeName).To(o.Equal(arbiterNodeName), "Expected Arbiter deployment to run on Arbiter node")

		g.By("Validating Normal deployment on Master nodes")
		normalSelector, err := labels.Parse("app=busybox-master")
		o.Expect(err).To(o.BeNil(), "Expected to parse Master label selector without error")

		normalPods, err := exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()), normalSelector, func(pod corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning
		}, 1, time.Second*30)
		o.Expect(err).To(o.BeNil(), "Expected Normal pods to be running on Master nodes")
		o.Expect(len(normalPods)).To(o.Equal(1), "Expected exactly two Normal pods to be running on Master nodes")

		for _, podName := range normalPods {
			pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), podName, metav1.GetOptions{})
			o.Expect(err).To(o.BeNil(), "Expected to retrieve Normal pod without error")

			_, exists := masterNodeMap[pod.Spec.NodeName]
			o.Expect(exists).To(o.BeTrue(), "Expected pod to be running on a Master node")
		}

		o.Expect(len(normalPods)).To(o.Equal(1), "Expected exactly one Normal pod to be running on a Master node")
	})
})

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] Evaluate DaemonSet placement in an Arbiter-node environment", func() {
	oc := exutil.NewCLI("daemonset-pod-validation").SetManagedNamespace().AsAdmin()
	defer g.GinkgoRecover()
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("Cluster is not in HighlyAvailableArbiterMode, skipping test")
		}
	})

	g.It("Should create a DaemonSet on the Arbiter node as expected", func() {
		g.By("Creating a DaemonSet deployment")
		_, err := createDaemonSetDeployment(oc)
		o.Expect(err).To(o.BeNil(), "Expected Arbiter busybox deployment creation to succeed")

		g.By("Parsing the DaemonSet label selector")
		daemonSetSelector, err := labels.Parse("app=busybox-daemon")
		o.Expect(err).To(o.BeNil(), "Expected to parse DaemonSet label selector without error")

		g.By("Waiting for DaemonSet pods to reach Running state")
		daemonSetPods, err := exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()), daemonSetSelector, func(pod corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning
		}, 1, time.Second*30) // Adjust timeout as needed
		o.Expect(err).To(o.BeNil(), "Expected DaemonSet pods to be running")
		o.Expect(len(daemonSetPods)).To(o.Equal(1), "Expected exactly one DaemonSet pod to be running")

		g.By("Validating that DaemonSet pods are NOT scheduled on the Arbiter node")
		// first retrive the arbiter node name
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter node without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Expected at least one Arbiter node")

		arbiterNodeName := nodes.Items[0].Name

		pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=busybox-daemon",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve DaemonSet pods without error")

		for _, pod := range pods.Items {
			o.Expect(pod.Spec.NodeName).NotTo(o.Equal(arbiterNodeName), "DaemonSet pod should NOT be scheduled on the Arbiter node")
		}
	})
})

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io] Ensure etcd health and quorum in HighlyAvailableArbiterMode", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("")
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("CLuster is not in HighlyAvailableArbiterMode skipping test ")
		}
	})

	g.It("Should have all etcd pods running and quorum met", func() {
		namespace := "openshift-etcd"
		labelSelector := "app=etcd"

		etcdPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve etcd pods without error")
		o.Expect(len(etcdPods.Items)).To(o.BeNumerically("==", 3), "Expected exactly 3 etcd pods in the 2-node + 1 arbiter cluster")

		// Ensure each etcd pod is running
		for _, pod := range etcdPods.Items {
			o.Expect(pod.Status.Phase).To(o.Equal(corev1.PodRunning), "Expected etcd pod %s to be in Running state", pod.Name)
		}

		// Check etcd quorum status by verifying endpoints leader election and member health
		etcdOperator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve etcd ClusterOperator without error")

		var isAvailable, isDegraded bool
		for _, cond := range etcdOperator.Status.Conditions {
			if cond.Type == v1.OperatorAvailable && cond.Status == v1.ConditionTrue {
				isAvailable = true
			}
			if cond.Type == v1.OperatorDegraded && cond.Status == v1.ConditionTrue {
				isDegraded = true
			}
		}

		o.Expect(isAvailable).To(o.BeTrue(), "Expected etcd operator to be available, indicating quorum is met")
		o.Expect(isDegraded).To(o.BeFalse(), "Expected etcd operator not to be degraded")
	})
})

func createNormalDeployment(oc *exutil.CLI) (*appv1.Deployment, error) {
	var replicas int32 = 1

	container := corev1.Container{
		Name:    "busybox",
		Image:   "busybox",
		Command: []string{"sleep", "3600"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
	}

	deployment := &appv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "busybox-deployment-masters",
			Namespace: oc.Namespace(),
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "busybox-master"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "busybox-master"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	return oc.KubeClient().AppsV1().
		Deployments(oc.Namespace()).
		Create(context.Background(), deployment, metav1.CreateOptions{})
}

func createArbiterDeployment(oc *exutil.CLI, arbiterNodeName string) (*appv1.Deployment, error) {
	var replicas int32 = 1

	container := corev1.Container{
		Name:    "busybox",
		Image:   "busybox",
		Command: []string{"sleep", "3600"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
	}

	deployment := &appv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "busybox-deployment-arbiter",
			Namespace: oc.Namespace(),
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "busybox-arbiter"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "busybox-arbiter"},
				},
				Spec: corev1.PodSpec{
					NodeName:   arbiterNodeName,
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	return oc.KubeClient().AppsV1().
		Deployments(oc.Namespace()).
		Create(context.Background(), deployment, metav1.CreateOptions{})
}

func createDaemonSetDeployment(oc *exutil.CLI) (*appv1.DaemonSet, error) {
	container := corev1.Container{
		Name:    "busybox",
		Image:   "busybox",
		Command: []string{"sleep", "3600"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("20m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		},
	}

	daemonSet := &appv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "busybox-daemon",
			Namespace: oc.Namespace(),
		},
		Spec: appv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "busybox-daemon"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "busybox-daemon"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	return oc.KubeClient().AppsV1().
		DaemonSets(oc.Namespace()).
		Create(context.Background(), daemonSet, metav1.CreateOptions{})
}
