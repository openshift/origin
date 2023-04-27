package operators

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	authv1 "github.com/openshift/api/authorization/v1"
	configv1 "github.com/openshift/api/config/v1"
	projv1 "github.com/openshift/api/project/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/utils/strings/slices"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	"github.com/openshift/origin/test/extended/util/prometheus"
)

const (
	namespace          = "e2e-machines-testbed"
	serviceAccountName = "e2e-machines"
)

var _ = g.Describe("[sig-cluster-lifecycle][Feature:Machines][Early] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have same number of Machines and Nodes [apigroup:machine.openshift.io]", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for the openshift machine api operator")
		// TODO: skip if platform != aws
		skipUnlessMachineAPIOperator(dc, c.CoreV1().Namespaces())

		g.By("getting MachineSet list")
		machineSetClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machinesets", Version: "v1beta1"})
		msList, err := machineSetClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		machineSetList := objx.Map(msList.UnstructuredContent())
		machineSetItems := objects(machineSetList.Get("items"))

		if len(machineSetItems) == 0 {
			e2eskipper.Skipf("cluster does not have machineset resources")
		}

		g.By("getting Node list")
		nodeList, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeItems := nodeList.Items

		g.By("getting Machine list")
		machineClient := dc.Resource(schema.GroupVersionResource{Group: "machine.openshift.io", Resource: "machines", Version: "v1beta1"})
		obj, err := machineClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		machineList := objx.Map(obj.UnstructuredContent())
		machineItems := objects(machineList.Get("items"))

		g.By("ensure number of Machines and Nodes are equal")
		o.Expect(len(nodeItems)).To(o.Equal(len(machineItems)))
	})
})

var _ = g.Describe("[sig-node] Managed cluster", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("managed-cluster-node").AsAdmin()
	)

	var staticNodeNames []string
	g.It("record the number of nodes at the beginning of the tests [Early]", func() {
		nodeList, err := oc.KubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, node := range nodeList.Items {
			staticNodeNames = append(staticNodeNames, node.Name)
		}
	})

	// This test makes use of Prometheus metrics, which are not present in the absence of cluster-monitoring-operator, the owner for
	// the api groups tagged here.
	g.It("should report ready nodes the entire duration of the test run [Late][apigroup:monitoring.coreos.com]", func() {
		// we only consider samples since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		tests := map[string]bool{
			// static (nodes we collected before starting the tests) nodes should be reporting ready throughout the entire run, as long as they are older than 6m, and they still
			// exist in 1m (because prometheus doesn't support negative offsets, we have to shift the entire query left). Since
			// the late test might not catch a node not ready at the very end of the run anyway, we don't do anything special
			// to shift the test execution later, we just note that there's a scrape_interval+wait_interval gap here of up to
			// 1m30s and we can live with ith
			//
			// note:
			// we are only interested in examining the health of nodes collected at the beginning of a test suite
			// because some tests might add and remove nodes as part of their testing logic
			// nodes added dynamically naturally initially are not ready causing this query to fail
			fmt.Sprintf(`(min_over_time((max by (node) (kube_node_status_condition{condition="Ready",status="true",node=~"%s"} offset 1m) and (((max by (node) (kube_node_status_condition offset 1m))) and (0*max by (node) (kube_node_status_condition offset 7m)) and (0*max by (node) (kube_node_status_condition))))[%s:1s])) < 1`, strings.Join(staticNodeNames, "|"), testDuration): false,
		}
		err := prometheus.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should verify that nodes have no unexpected reboots [Late]", func() {
		ctx := context.Background()

		// This test is applicable for SNO installations only
		configClient, err := configv1client.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		infrastructure, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infrastructure.Status.ControlPlaneTopology != configv1.SingleReplicaTopologyMode {
			return
		}

		createTestBed(ctx, oc)
		defer deleteTestBed(ctx, oc)

		// List all nodes
		nodes, err := oc.KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes.Items).NotTo(o.HaveLen(0))

		expected := make(map[string]int)
		actual := make(map[string]int)
		errs := make([]error, 0)

		// Find a number of rendered-* machineconfigs for masters and workers
		// Each rendered config adds expected reboot
		expectedReboots := map[string]int{
			"worker": 0,
			"master": 0,
		}
		dynamicClient, err := dynamic.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		machineConfigGroupVersionResource := schema.GroupVersionResource{
			Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigs",
		}
		mcList, err := dynamicClient.Resource(machineConfigGroupVersionResource).List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for i := range mcList.Items {
			mcName := mcList.Items[i].GetName()
			for prefix := range expectedReboots {
				if strings.HasPrefix(mcName, fmt.Sprintf("rendered-%s-", prefix)) {
					expectedReboots[prefix] += 1
				}
			}
		}

		// List nodes, set actual and expected number of reboots
		for _, node := range nodes.Items {
			// Examine only the nodes which are available at the start of the test
			if !slices.Contains(staticNodeNames, node.Name) {
				continue
			}
			expectedRebootsForNodeRole := expectedReboots[getNodeRole(&node)]
			o.Expect(expectedReboots).To(o.HaveKey(expectedRebootsForNodeRole))
			expected[node.Name] = expectedRebootsForNodeRole
			nodeReboots, err := getNumberOfBootsForNode(oc.KubeClient(), node.Name, 0)
			if err != nil {
				errs = append(errs, err)
			}
			actual[node.Name] = nodeReboots
		}
		// Use gomega's WithTransform to compare actual to expected - and check that errs is empty
		var emptyErrors = func(a interface{}) (interface{}, error) {
			if len(errs) > 0 {
				return a, fmt.Errorf("errors found: %v", errs)
			}
			return a, nil
		}
		o.Expect(actual).To(o.WithTransform(emptyErrors, o.Equal(expected)))
	})
})

// getNodeRole reads node labels and returns either "worker" or "master"
func getNodeRole(node *corev1.Node) string {
	if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
		return "worker"
	}
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return "master"
	}
	return "worker"
}

func getNumberOfBootsForNode(kubeClient kubernetes.Interface, nodeName string, attempt int) (int, error) {

	// Run up to 10 attempts
	if attempt > 9 {
		return 0, fmt.Errorf("giving up after 10 attempts")
	}

	// Run journalctl to collect a list of boots
	command := "exec chroot /host journalctl --list-boots"
	isTrue := true
	zero := int64(0)
	ctx := context.Background()
	name := fmt.Sprintf("list-boots-%s-%d", nodeName, attempt)
	pod, err := kubeClient.CoreV1().Pods(namespace).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Tolerations: []corev1.Toleration{
				{
					Effect:   "NoSchedule",
					Key:      "node-role.kubernetes.io/master",
					Operator: corev1.TolerationOpExists,
				},
			},
			HostPID:            true,
			RestartPolicy:      corev1.RestartPolicyNever,
			NodeName:           nodeName,
			ServiceAccountName: serviceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "list-boots",
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &zero,
						Privileged: &isTrue,
					},
					Image: image.ShellImage(),
					Command: []string{
						command,
					},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/host",
							Name:      "host",
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return getNumberOfBootsForNode(kubeClient, nodeName, attempt+1)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to create pod %s: %v", name, err)
	}

	podName := pod.Name

	// Wait for pod to run
	err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
		podGet, getErr := kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if getErr != nil {
			return false, getErr
		}
		switch podGet.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			return true, fmt.Errorf("journalctl command in pod %s failed. Pod status: %#v", name, podGet.Status)
		default:
			return false, nil
		}
	})
	if err != nil {
		return 0, fmt.Errorf("pod %s failed to complete: %v", name, err)
	}

	// Fetch pod logs
	linesInPodLogs := -1
	podLogOpts := corev1.PodLogOptions{}
	req := kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return linesInPodLogs, fmt.Errorf("failed to open stream to read %s pod logs: %v", name, err)
	}
	defer podLogs.Close()

	// Count number of boots
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return 0, fmt.Errorf("failed to copy information from podLogs to buf: %v", err)
	}
	linesInPodLogs = strings.Count(buf.String(), "\n") - 1
	if linesInPodLogs < 1 {
		return 0, fmt.Errorf("failed to fetch boot list from %s pod logs: %v", name, buf)
	}
	return linesInPodLogs, nil
}

func createTestBed(ctx context.Context, oc *exutil.CLI) {
	err := callProject(ctx, oc, true)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callServiceAccount(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = callRBAC(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = exutil.WaitForServiceAccountWithSecret(
		oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace),
		serviceAccountName)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteTestBed(ctx context.Context, oc *exutil.CLI) {
	err := callProject(ctx, oc, false)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func callRBAC(ctx context.Context, oc *exutil.CLI) error {
	obj := &authv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
		RoleRef: corev1.ObjectReference{
			Kind: "ClusterRole",
			Name: "system:openshift:scc:privileged",
		},
		Subjects: []corev1.ObjectReference{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}

	client := oc.AdminAuthorizationClient().AuthorizationV1().RoleBindings(namespace)
	_, err := client.Create(ctx, obj, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callServiceAccount(ctx context.Context, oc *exutil.CLI) error {
	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}

	client := oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace)
	_, err := client.Create(ctx, obj, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func callProject(ctx context.Context, oc *exutil.CLI, create bool) error {
	obj := &projv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/audit":   "privileged",
				"pod-security.kubernetes.io/enforce": "privileged",
				"pod-security.kubernetes.io/warn":    "privileged",
			},
		},
	}

	client := oc.AsAdmin().ProjectClient().ProjectV1().Projects()
	var err error
	if create {
		_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	} else {
		err = client.Delete(ctx, obj.Name, metav1.DeleteOptions{})
	}

	if apierrors.IsAlreadyExists(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
