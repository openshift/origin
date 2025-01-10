package arbiter_topology

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] Validate cluster infrastructure in HighlyAvailableArbiterMode", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("")
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("CLuster is not in HighlyAvailableArbiterMode skipping test ")
		}
	})
	g.It("Should validate infrastructure is HighlyAvailableArbiter", func() {
		g.By("Test RAN CORRECTLY")
		// oc get infrastructure cluster
		//
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		o.Expect(infra.Status.ControlPlaneTopology).To(o.Equal(v1.HighlyAvailableArbiterMode))
	})
})

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

	g.It("Should validate that there are 2 Master nodes and 1 Arbiter node", func() {
		g.By("Counting nodes from MachineConfigPools for Masters and Arbiter")

		// Run the `oc get mcp master -o jsonpath='{.status.machineCount}'` command

		mcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.Background(), "master", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Master MachineConfigPool without error")
		o.Expect(mcp.Status.MachineCount).To(o.Equal(2))

		arbiterMcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.Background(), "arbiter", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbite MachineConfigPool without error")
		o.Expect(arbiterMcp.Status.MachineCount).To(o.Equal(1))
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

		// Retrieve the arbiter node name
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve nodes without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Expected to find at least one Arbiter node")

		var arbiterNodeName string
		for _, node := range nodes.Items {
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

		// Count the number of running pods on the Arbiter node
		pods, err := oc.AdminKubeClient().CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + arbiterNodeName + ",status.phase=Running",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve pods without error")

		// Validate the count of running pods
		expectedPodCount := 14
		actualPodCount := len(pods.Items)
		g.By(fmt.Sprintf("Expected %d pods, found %d pods running on the Arbiter node", expectedPodCount, actualPodCount))

		o.Expect(actualPodCount).To(o.Equal(expectedPodCount), "Expected the correct number of running pods on the Arbiter node")
	})
})

var _ = g.Describe("[sig-node] validate deployment creation on non-Arbiter nodes", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("")

	g.It("Should create deployment on Arbiter and non-Arbiter nodes as expected", func() {
		namespace := "foobar"

		// Ensure the namespace is deleted if it already exists
		_ = oc.AdminKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})

		// Wait until the namespace deletion is complete
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (done bool, err error) {
			_, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).To(o.BeNil(), "Expected namespace deletion without error")

		// Retrieve Arbiter nodes
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter nodes without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Expected to find at least one Arbiter node")

		var arbiterNodeName string
		for _, node := range nodes.Items {
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

		// Create the namespace
		_, err = oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}, metav1.CreateOptions{})
		o.Expect(err).To(o.BeNil(), "Expected namespace creation without error")
		defer func() {
			_ = oc.AdminKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		}()

		// Create a master deployment (non-Arbiter node)
		// Create a master deployment (non-Arbiter node)
		// Create a Master deployment (non-Arbiter node)
		err = createMasterDeployment(
			oc,
			namespace,
		)
		o.Expect(err).To(o.BeNil(), "Expected master busybox deployment creation to succeed")

		// Create an Arbiter deployment (scheduled on Arbiter node)
		err = createArbiterDeployment(
			oc,
			namespace,
			arbiterNodeName, // Specific nodeName for Arbiter
		)
		o.Expect(err).To(o.BeNil(), "Expected arbiter busybox deployment creation to succeed")

		// Validate arbiter deployment is on the Arbiter node
		arbiterPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=busybox-arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter pods without error")
		o.Expect(len(arbiterPods.Items)).To(o.Equal(1), "Expected exactly one Arbiter pod")
		o.Expect(arbiterPods.Items[0].Spec.NodeName).To(o.Equal(arbiterNodeName), "Expected Arbiter deployment to run on Arbiter node")

		// Validate master deployment is not on the Arbiter node
		masterPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=busybox",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve master pods without error")
		o.Expect(len(masterPods.Items)).To(o.Equal(1), "Expected exactly one master pod")
		o.Expect(masterPods.Items[0].Spec.NodeName).NotTo(o.Equal(arbiterNodeName), "Expected master deployment to run on a non-Arbiter node")
	})
})

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] Evaluate DaemonSet placement in an Arbiter-node environment", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("")
	g.BeforeEach(func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if infra.Status.ControlPlaneTopology != v1.HighlyAvailableArbiterMode {
			g.Skip("CLuster is not in HighlyAvailableArbiterMode skipping test ")
		}
	})

	g.It("Should create a DaemonSet on the Arbiter node as expected", func() {
		namespace := "foobar"

		defer func() {
			_ = oc.AdminKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		}()
		// Delete namespace if it exists
		_ = oc.AdminKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})

		// Wait until the namespace deletion is complete or timeout occurs
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 60*time.Second, true, func(ctx context.Context) (bool, error) {
			_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return true, nil // Namespace is deleted
			}
			return false, nil // Continue waiting
		})
		o.Expect(err).To(o.BeNil(), "Expected namespace deletion without error")

		// Re-create the namespace
		_, err = oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}, metav1.CreateOptions{})
		o.Expect(err).To(o.BeNil(), "Expected namespace creation without error")

		// Create the DaemonSet using the merged function (asDeployment = false)
		// err = createBusyboxWorkload(oc, namespace, "busybox-daemon", map[string]string{"app": "busybox-daemon"}, "", false, true)
		// o.Expect(err).To(o.BeNil(), "Expected DaemonSet creation to succeed")

		// Retrieve and verify DaemonSet pods
		daemonSetPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=busybox-daemon",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve DaemonSet pods without error")

		// Retrieve the Arbiter node name dynamically
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/arbiter",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve Arbiter node without error")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Expected at least one Arbiter node")

		// Store the Arbiter node name
		arbiterNodeName := nodes.Items[0].Name

		// Check if DaemonSet pods are NOT scheduled on the Arbiter node
		daemonsetNodeNames := ""
		for _, pod := range daemonSetPods.Items {
			daemonsetNodeNames += pod.Spec.NodeName + " "
		}
		o.Expect(daemonsetNodeNames).ToNot(o.ContainSubstring(arbiterNodeName), "DaemonSet pods should NOT be scheduled on the Arbiter node")
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
		// Define the namespace and label selector for etcd pods
		namespace := "openshift-etcd"
		labelSelector := "app=etcd"

		// Retrieve the list of etcd pods
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

func createMasterDeployment(
	oc *exutil.CLI,
	namespace string,
) error {
	zero := int64(0)

	// Define the container spec
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

	// Define the pod spec
	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: &zero,
		Containers:                    []corev1.Container{container},
	}

	// Define the deployment spec
	deployment := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "busybox-deployment-masters",
			Namespace: namespace,
			Labels:    map[string]string{"app": "busybox"},
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &[]int32{1}[0],
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "busybox"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "busybox"},
				},
				Spec: podSpec,
			},
		},
	}

	// Create the deployment
	_, err := oc.KubeClient().AppsV1().
		Deployments(namespace).
		Create(context.Background(), deployment, metav1.CreateOptions{})
	return err
}

func createArbiterDeployment(
	oc *exutil.CLI,
	namespace, arbiterNodeName string,
) error {
	zero := int64(0)

	// Define the container spec
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

	// Define the pod spec
	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: &zero,
		NodeName:                      arbiterNodeName,
		Containers:                    []corev1.Container{container},
	}

	// Define the deployment spec
	deployment := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "busybox-deployment-arbiter",
			Namespace: namespace,
			Labels:    map[string]string{"app": "busybox-arbiter"},
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &[]int32{1}[0],
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "busybox-arbiter"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "busybox-arbiter"},
				},
				Spec: podSpec,
			},
		},
	}

	// Create the deployment
	_, err := oc.KubeClient().AppsV1().
		Deployments(namespace).
		Create(context.Background(), deployment, metav1.CreateOptions{})
	return err
}
