package utility

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	testutils "k8s.io/kubernetes/test/utils"

	"k8s.io/utils/ptr"
)

func NewPod(namespace, name string) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To[int64](1),
			SecurityContext:               e2epodutil.GetRestrictedPodSecurityContext(),
		},
	}
	return pod
}

func NewResourceClaimTemplate(namespace, name string) *resourceapi.ResourceClaimTemplate {
	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func NewDeviceRequest(name, class string) resourceapi.DeviceRequest {
	return resourceapi.DeviceRequest{
		Name:            name,
		DeviceClassName: class,
	}
}

func NewContainer(name string) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           e2epodutil.GetDefaultTestImage(),
		Command:         e2epodutil.GenerateScriptCmd("env && sleep 100000"),
		SecurityContext: e2epodutil.GetRestrictedContainerSecurityContext(),
	}
}

func NewNvidiaSMIContainer(name string) corev1.Container {
	return corev1.Container{
		Name:    name,
		Image:   "ubuntu:22.04",
		Command: []string{"bash", "-c"},
		Args:    []string{"nvidia-smi -L; trap 'exit 0' TERM; sleep 9999 & wait"},
	}
}

func NewCUDASampleContainer(name string) corev1.Container {
	return corev1.Container{
		Name:    name,
		Image:   "nvcr.io/nvidia/k8s/cuda-sample:nbody-cuda11.6.0-ubuntu18.04",
		Command: []string{"bash", "-c"},
		Args:    []string{"trap 'exit 0' TERM; /tmp/sample --benchmark --numbodies=4226048 & wait"},
	}
}

func UsePrivilegedSCC(ctx context.Context, clientset *kubernetes.Clientset, sa, namespace string) error {
	name := fmt.Sprintf("%s-use-scc-privileged", sa)
	want := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:privileged",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa,
				Namespace: namespace,
			},
		},
	}
	_, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, &want, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func EnsureNodeLabel(ctx context.Context, clientset kubernetes.Interface, node string, key, want string) error {
	client := clientset.CoreV1().Nodes()
	current, err := client.Get(ctx, node, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if value, ok := current.Labels[key]; ok && value == want {
		return nil
	}

	if len(current.Labels) == 0 {
		current.Labels = map[string]string{}
	}
	current.Labels[key] = want
	_, err = client.Update(ctx, current, metav1.UpdateOptions{})
	return err
}

func EnsureConfigMap(ctx context.Context, clientset kubernetes.Interface, want *corev1.ConfigMap) error {
	client := clientset.CoreV1().ConfigMaps(want.Namespace)
	_, err := client.Create(context.Background(), want, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func ExtractMIGDevices(lines string) []string {
	devices := []string{}
	sc := bufio.NewScanner(strings.NewReader(lines))
	for sc.Scan() {
		after, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "MIG ")
		if !found {
			continue
		}
		split := strings.Split(after, " ")
		devices = append(devices, split[0])
	}
	return devices
}

func GetLogs(ctx context.Context, clientset kubernetes.Interface, namespace, name, ctr string) (string, error) {
	client := clientset.CoreV1().Pods(namespace)
	options := corev1.PodLogOptions{Container: ctr}
	r, err := client.GetLogs(name, &options).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs %s: %w", name, err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs %s: %w", name, err)
	}
	return string(out), nil
}

func CollectDevices(slices []resourceapi.ResourceSlice) []string {
	devices := []string{}
	for _, rs := range slices {
		for _, device := range rs.Spec.Devices {
			devices = append(devices, device.Name)
		}
	}
	return devices
}

func PodRunningReady(ctx context.Context, t testing.TB, clientset kubernetes.Interface, component, namespace string, options metav1.ListOptions) error {
	client := clientset.CoreV1().Pods(namespace)
	result, err := client.List(ctx, options)
	if err != nil || len(result.Items) == 0 {
		return fmt.Errorf("[%s] still waiting for pod to show up - %w", component, err)
	}

	for _, pod := range result.Items {
		ready, err := testutils.PodRunningReady(&pod)
		if err != nil || !ready {
			err := fmt.Errorf("[%s] still waiting for pod: %s to be ready: %v", component, pod.Name, err)
			t.Log(err.Error())
			return err
		}
		t.Logf("[%s] pod: %s ready", component, pod.Name)
	}
	return nil
}

func ExecIntoPod(ctx context.Context, t testing.TB, f *framework.Framework, name, namespace, container string, cmd []string) ([]string, error) {
	g.By(fmt.Sprintf("exec into pod: %s, command: %v", name, cmd))
	stdout, stderr, err := e2epod.ExecWithOptionsContext(ctx, f, e2epod.ExecOptions{
		Command:       cmd,
		Namespace:     namespace,
		PodName:       name,
		ContainerName: container,
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v on pod %s, stdout: %v, stderr: %v, err: %w", cmd, name, stdout, stderr, err)
	}
	t.Logf("output of pod exec: %s/%s (container=%s):\n%s\n", namespace, name, container, stdout)
	lines := []string{}
	sc := bufio.NewScanner(strings.NewReader(stdout))
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, nil
}
