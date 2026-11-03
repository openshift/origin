package nri

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// pluginOptIn must match plugin/main.go.
	pluginOptIn = "nri.openshift.io/test"
	annEnv      = "nri.openshift.io/test-env"
	annMounts   = "nri.openshift.io/test-mounts"
	annDevices  = "nri.openshift.io/test-devices"
	annCpuset   = "nri.openshift.io/test-cpuset"
	annAnnos    = "nri.openshift.io/test-annotations"
	annFail     = "nri.openshift.io/test-fail"

	pluginName       = "nri-test-plugin"
	secondPluginName = pluginName + "-second"
	pluginImageTag   = "nri-test-plugin"
	imageEnv         = "NRI_TEST_PLUGIN_IMAGE"

	pluginDisabledNode = "nri-plugin-disabled.invalid"
)

// event must match the event type in plugin/main.go.
type event struct {
	Time        int64    `json:"time,omitempty"`
	Kind        string   `json:"kind"`
	Pod         string   `json:"pod,omitempty"`
	PodID       string   `json:"podID,omitempty"`
	Container   string   `json:"container,omitempty"`
	ContainerID string   `json:"containerID,omitempty"`
	Adjustments []string `json:"adjustments,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// createTestNamespace creates the namespace the suite runs in. The e2e
// framework deletes namespaces registered by SetupProject after every spec,
// which would tear down the plugin between the specs of this ordered container.
func createTestNamespace(ctx context.Context, oc *exutil.CLI) (string, error) {
	return createNamespace(ctx, oc, "e2e-test-nri-", map[string]string{
		"pod-security.kubernetes.io/enforce":             "privileged",
		"pod-security.kubernetes.io/audit":               "privileged",
		"pod-security.kubernetes.io/warn":                "privileged",
		"security.openshift.io/scc.podSecurityLabelSync": "false",
	})
}

func grantPrivilegedSCC(ctx context.Context, oc *exutil.CLI, namespace string) {
	out, err := oc.AsAdmin().Run("adm", "policy", "add-scc-to-user", "privileged", "-z", "default", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to grant the privileged SCC: %s", out)
}

func createNamespace(ctx context.Context, oc *exutil.CLI, generateName string, labels map[string]string) (string, error) {
	ns, err := oc.AdminKubeClient().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: generateName, Labels: labels},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return ns.Name, nil
}

func deleteNamespace(ctx context.Context, oc *exutil.CLI, name string) {
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("failed to delete namespace %s: %v", name, err)
	}
}

func pickNode(ctx context.Context, oc *exutil.CLI) (*corev1.Node, error) {
	// Tainted nodes are included: the plugin tolerates whatever the node has.
	nodes, err := e2enode.GetReadyNodesIncludingTainted(ctx, oc.AdminKubeClient())
	if err != nil {
		return nil, err
	}
	var fallback, controlPlaneFallback *corev1.Node
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if node.Labels["kubernetes.io/os"] != "linux" {
			continue
		}
		if _, isWorker := node.Labels["node-role.kubernetes.io/worker"]; isWorker {
			return node, nil
		}
		if _, isMaster := node.Labels["node-role.kubernetes.io/control-plane"]; isMaster {
			if controlPlaneFallback == nil {
				controlPlaneFallback = node
			}
		} else if fallback == nil {
			fallback = node
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	if controlPlaneFallback != nil {
		return controlPlaneFallback, nil
	}
	return nil, fmt.Errorf("no ready Linux node found")
}

func pluginImage(ctx context.Context, oc *exutil.CLI) (string, error) {
	if image := os.Getenv(imageEnv); image != "" {
		return image, nil
	}
	return exutil.DetermineImageFromRelease(ctx, oc, pluginImageTag)
}

// pluginLogs returns the logs of the plugin pods selected by label.
func pluginLogs(ctx context.Context, oc *exutil.CLI, label string) (string, error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: "app=" + label})
	if err != nil {
		return "", err
	}
	var logs strings.Builder
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			out, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container.Name}).DoRaw(ctx)
			if err != nil {
				continue
			}
			logs.Write(out)
			logs.WriteByte('\n')
		}
	}
	return logs.String(), nil
}

// readEvents parses the JSON event lines the plugin logs.
func readEvents(ctx context.Context, oc *exutil.CLI, label string) ([]event, error) {
	out, err := pluginLogs(ctx, oc, label)
	if err != nil {
		return nil, err
	}
	var events []event
	for _, line := range strings.Split(out, "\n") {
		start := strings.Index(line, "{")
		if start < 0 {
			continue
		}
		var ev event
		if err := json.Unmarshal([]byte(line[start:]), &ev); err != nil || ev.Kind == "" {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func waitForEvent(ctx context.Context, oc *exutil.CLI, label string, match func(event) bool) (event, error) {
	var found event
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		events, err := readEvents(ctx, oc, label)
		if err != nil {
			return false, nil
		}
		for _, ev := range events {
			if match(ev) {
				found = ev
				return true, nil
			}
		}
		return false, nil
	})
	return found, err
}

func nriSocketVolume() corev1.Volume {
	return corev1.Volume{
		Name: "nri-socket",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/run/nri"},
		},
	}
}

func privilegedPluginContainer(image string, args []string) corev1.Container {
	return corev1.Container{
		Name:            "plugin",
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/usr/bin/nri-test-plugin"},
		Args:            args,
		// The NRI socket directory is root-only and labeled container_var_run_t.
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(true),
			RunAsUser:  ptr.To[int64](0),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "nri-socket", MountPath: "/var/run/nri", ReadOnly: true},
		},
	}
}

type pluginOptions struct {
	name              string
	idx               string
	extraEnv          string
	ignoreAnnotations bool
}

// deployPlugin rolls out an NRI test plugin DaemonSet on the node.
func deployPlugin(ctx context.Context, oc *exutil.CLI, node *corev1.Node, image string, opts pluginOptions) {
	tolerations := []corev1.Toleration{}
	for _, taint := range node.Spec.Taints {
		tolerations = append(tolerations, corev1.Toleration{
			Key:      taint.Key,
			Operator: corev1.TolerationOpEqual,
			Value:    taint.Value,
			Effect:   taint.Effect,
		})
	}
	args := []string{
		"--namespace=" + oc.Namespace(),
		"--name=" + opts.name,
		"--idx=" + opts.idx,
	}
	if opts.extraEnv != "" {
		args = append(args, "--extra-env="+opts.extraEnv)
	}
	if opts.ignoreAnnotations {
		args = append(args, "--ignore-annotations")
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.name,
			Namespace: oc.Namespace(),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": opts.name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": opts.name}},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{"kubernetes.io/hostname": node.Name},
					Tolerations:  tolerations,
					Containers:   []corev1.Container{privilegedPluginContainer(image, args)},
					Volumes:      []corev1.Volume{nriSocketVolume()},
				},
			},
		},
	}
	_, err := oc.AdminKubeClient().AppsV1().DaemonSets(oc.Namespace()).Create(ctx, ds, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create the %s DaemonSet", opts.name)
}

func deletePlugin(ctx context.Context, oc *exutil.CLI, name string) {
	err := oc.AdminKubeClient().AppsV1().DaemonSets(oc.Namespace()).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("failed to delete DaemonSet %s: %v", name, err)
	}
}

func waitNoPluginPods(ctx context.Context, oc *exutil.CLI, name string) {
	err := wait.PollUntilContextTimeout(ctx, time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: "app=" + name})
		if err != nil {
			return false, err
		}
		return len(pods.Items) == 0, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "pods for %s are still running", name)
}

// pausePlugin removes the plugin pod by parking the DaemonSet on a node name
// that does not exist, and returns a function that restores it.
func pausePlugin(ctx context.Context, oc *exutil.CLI) func(context.Context) {
	dsClient := oc.AdminKubeClient().AppsV1().DaemonSets(oc.Namespace())
	ds, err := dsClient.Get(ctx, pluginName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	original := ds.Spec.Template.Spec.NodeSelector

	ds.Spec.Template.Spec.NodeSelector = map[string]string{"kubernetes.io/hostname": pluginDisabledNode}
	_, err = dsClient.Update(ctx, ds, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	waitNoPluginPods(ctx, oc, pluginName)

	return func(ctx context.Context) {
		ds, err := dsClient.Get(ctx, pluginName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		ds.Spec.Template.Spec.NodeSelector = original
		_, err = dsClient.Update(ctx, ds, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// waitForPlugin waits until the plugin has registered with CRI-O.
func waitForPlugin(ctx context.Context, oc *exutil.CLI, label string) error {
	_, err := waitForEvent(ctx, oc, label, func(ev event) bool { return ev.Kind == "configure" })
	return err
}

// dumpPluginState makes a plugin that never registers debuggable.
func dumpPluginState(ctx context.Context, oc *exutil.CLI, label string) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: "app=" + label})
	if err != nil {
		e2e.Logf("failed to list plugin pods: %v", err)
		return
	}
	for _, pod := range pods.Items {
		e2e.Logf("plugin pod %s phase=%s node=%s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
		for _, cs := range pod.Status.ContainerStatuses {
			e2e.Logf("  container %s ready=%v restarts=%d state=%+v", cs.Name, cs.Ready, cs.RestartCount, cs.State)
		}
		logs, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).GetLogs(pod.Name, &corev1.PodLogOptions{}).DoRaw(ctx)
		if err == nil {
			e2e.Logf("  logs:\n%s", string(logs))
		}
	}
	events, err := oc.AdminKubeClient().CoreV1().Events(oc.Namespace()).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ev := range events.Items {
			if strings.Contains(ev.InvolvedObject.Name, label) {
				e2e.Logf("  event %s %s: %s", ev.Type, ev.Reason, ev.Message)
			}
		}
	}
}

func newPod(namespace, nodeName, name, image string, annotations map[string]string, command ...string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "ctr",
				Image:   image,
				Command: command,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			}},
		},
	}
}

func hasCreateContainerEvent(events []event, pod string) bool {
	for _, ev := range events {
		if ev.Kind == "create-container" && ev.Pod == pod {
			return true
		}
	}
	return false
}

func createContainerPods(events []event) map[string]struct{} {
	pods := map[string]struct{}{}
	for _, ev := range events {
		if ev.Kind == "create-container" {
			pods[ev.Pod] = struct{}{}
		}
	}
	return pods
}

func createPod(ctx context.Context, oc *exutil.CLI, namespace, nodeName, name, image string, annotations map[string]string, command ...string) *corev1.Pod {
	return createPodWithDeadline(ctx, oc, namespace, nodeName, name, image, annotations, 0, command...)
}

func createPodWithDeadline(ctx context.Context, oc *exutil.CLI, namespace, nodeName, name, image string, annotations map[string]string, deadline int64, command ...string) *corev1.Pod {
	pod := newPod(namespace, nodeName, name, image, annotations, command...)
	if deadline > 0 {
		pod.Spec.ActiveDeadlineSeconds = ptr.To(deadline)
	}
	pod, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create test pod %s", name)
	return pod
}

func deletePod(ctx context.Context, oc *exutil.CLI, namespace, name string) {
	err := oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if !apierrors.IsNotFound(err) {
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func waitPodRunning(ctx context.Context, oc *exutil.CLI, namespace, name string, timeout time.Duration) {
	err := e2epod.WaitTimeoutForPodRunningInNamespace(ctx, oc.AdminKubeClient(), name, namespace, timeout)
	o.Expect(err).NotTo(o.HaveOccurred(), "pod %s/%s did not start", namespace, name)
}

func execInPod(ctx context.Context, oc *exutil.CLI, namespace, podName string, cmd ...string) (string, error) {
	args := append([]string{"-n", namespace, podName, "--"}, cmd...)
	return oc.AsAdmin().WithoutNamespace().Run("exec").Args(args...).Output()
}

// createPrivilegedDeployment creates a Deployment whose pods look like the
// plugin DaemonSet pod. The controller creates the pods with the namespace's
// default service account, so admission decides whether they exist.
func createPrivilegedDeployment(ctx context.Context, oc *exutil.CLI, namespace, image string) string {
	labels := map[string]string{"app": "nri-privileged-probe"}
	name := "nri-privileged-probe"
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{privilegedPluginContainer(image, nil)},
					Volumes:    []corev1.Volume{nriSocketVolume()},
				},
			},
		},
	}
	_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create the probe deployment")
	return name
}

// waitForDeploymentForbidden waits until the controller reports that it cannot
// create the deployment's pods for a reason matching message.
func waitForDeploymentForbidden(ctx context.Context, oc *exutil.CLI, namespace, name, message string) {
	pattern := regexp.MustCompile(message)
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		events, err := oc.AdminKubeClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, ev := range events.Items {
			if ev.Reason == "FailedCreate" && strings.HasPrefix(ev.InvolvedObject.Name, name) && pattern.MatchString(ev.Message) {
				return true, nil
			}
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "the privileged deployment was not rejected with %q", message)
}
