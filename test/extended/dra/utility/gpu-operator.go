package utility

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func NewGPUOperatorInstaller(t testing.TB, clientset kubernetes.Interface, f *framework.Framework, p HelmParameters) *GpuOperator {
	return &GpuOperator{
		t:         t,
		clientset: clientset,
		installer: NewHelmInstaller(t, p),
		f:         f,
	}
}

type GpuOperator struct {
	t testing.TB
	f *framework.Framework
	// we don't depend on the clientset from f, esp the clean up code
	clientset kubernetes.Interface
	installer *HelmInstaller
	timeout   time.Duration
}

func (d *GpuOperator) Install(ctx context.Context) error {
	const (
		enforceKey   = "pod-security.kubernetes.io/enforce"
		enforceValue = "privileged"
	)
	client := d.clientset.CoreV1().Namespaces()
	current, err := client.Get(ctx, d.installer.Namespace, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		want := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: d.installer.Namespace,
				Labels: map[string]string{
					enforceKey: enforceValue,
				},
			},
		}
		current, err = client.Create(ctx, want, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating namespace: %w", err)
		}
	case err != nil:
		return fmt.Errorf("error retrieving namespace: %w", err)
	}

	if v, ok := current.Labels[enforceKey]; !ok || v != enforceValue {
		want := current.DeepCopy()
		if len(want.Labels) == 0 {
			want.Labels = map[string]string{}
		}
		want.Labels[enforceKey] = enforceValue
		_, err = client.Update(ctx, want, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating namespace: %w", err)
		}
	}

	return d.installer.Install(ctx)
}

func (d *GpuOperator) Cleanup(ctx context.Context) error { return d.installer.Remove(ctx) }

func (d GpuOperator) Ready(ctx context.Context, node *corev1.Node) error {
	for _, probe := range []struct {
		component string
		enabled   bool
		options   metav1.ListOptions
	}{
		{
			enabled:   true,
			component: "nvidia-driver-daemonset",
			options: metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/component" + "=" + "nvidia-driver",
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
		{
			enabled:   true,
			component: "nvidia-container-toolkit-daemonset",
			options: metav1.ListOptions{
				LabelSelector: "app" + "=" + "nvidia-container-toolkit-daemonset",
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
		{
			enabled:   true,
			component: "gpu-feature-discovery-daemonset",
			options: metav1.ListOptions{
				LabelSelector: "app" + "=" + "gpu-feature-discovery",
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
	} {
		if probe.enabled {
			g.By(fmt.Sprintf("waiting for %s to be ready", probe.component))
			o.Eventually(ctx, func(ctx context.Context) error {
				return PodRunningReady(ctx, d.t, d.clientset, probe.component, d.installer.Namespace, probe.options)
			}).WithPolling(5*time.Second).Should(o.BeNil(), fmt.Sprintf("[%s] pod should be ready", probe.component))
		}
	}

	return nil
}

func (d GpuOperator) MIGManagerReady(ctx context.Context, node *corev1.Node) error {
	for _, probe := range []struct {
		component string
		enabled   bool
		options   metav1.ListOptions
	}{
		{
			enabled:   true,
			component: "nvidia-mig-manager-daemonset",
			options: metav1.ListOptions{
				LabelSelector: "app" + "=" + "nvidia-mig-manager",
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
		{
			enabled:   false, // dra driver creates mps control daemon
			component: "nvidia-device-plugin-mps-control-daemon",
			options: metav1.ListOptions{
				LabelSelector: "app" + "=" + "nvidia-device-plugin-mps-control-daemon",
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
	} {
		if probe.enabled {
			g.By(fmt.Sprintf("waiting for %s to be ready", probe.component))
			o.Eventually(ctx, func(ctx context.Context) error {
				return PodRunningReady(ctx, d.t, d.clientset, probe.component, d.installer.Namespace, probe.options)
			}).WithPolling(5*time.Second).Should(o.BeNil(), fmt.Sprintf("[%s] pod should be ready", probe.component))
		}
	}

	return nil
}

func (d GpuOperator) DiscoverGPUProudct(ctx context.Context, node *corev1.Node) (string, error) {
	client := d.clientset.CoreV1().Pods(d.installer.Namespace)
	result, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: "app" + "=" + "gpu-feature-discovery",
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return "", fmt.Errorf("did not find any pod for %s on node: %s - %w", "gpu-feature-discovery", node.Name, err)
	}
	pod := result.Items[0].Name

	cmd := []string{"cat", "/etc/kubernetes/node-feature-discovery/features.d/gfd"}
	g.By(fmt.Sprintf("exec into pod: %s command: %v", pod, cmd))
	stdout, stderr, err := e2epod.ExecWithOptionsContext(ctx, d.f, e2epod.ExecOptions{
		Command:       cmd,
		Namespace:     d.installer.Namespace,
		PodName:       pod,
		ContainerName: "gpu-feature-discovery",
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to run command %v on pod %s, stdout: %v, stderr: %v, err: %w", cmd, pod, stdout, stderr, err)
	}
	d.t.Logf("output of pod exec: %s:\n%s\n", pod, stdout)
	sc := bufio.NewScanner(strings.NewReader(stdout))
	for sc.Scan() {
		after, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "nvidia.com/gpu.product=")
		if !found {
			continue
		}
		return strings.Trim(strings.TrimSpace(after), "'"), nil
	}

	return "", fmt.Errorf("nvidia.com/gpu.product not found in output")
}

func (d GpuOperator) DiscoverMIGDevices(ctx context.Context, node *corev1.Node) ([]string, error) {
	client := d.clientset.CoreV1().Pods(d.installer.Namespace)
	result, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component" + "=" + "nvidia-driver",
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, fmt.Errorf("did not find any pod for %s on node: %s - %w", "nvidia-driver-daemonset", node.Name, err)
	}
	pod := result.Items[0].Name

	cmd := []string{"nvidia-smi", "-L"}
	g.By(fmt.Sprintf("exec into pod: %s, command: %v", pod, cmd))
	stdout, stderr, err := e2epod.ExecWithOptionsContext(ctx, d.f, e2epod.ExecOptions{
		Command:       cmd,
		Namespace:     d.installer.Namespace,
		PodName:       pod,
		ContainerName: "nvidia-driver-ctr",
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run command %v on pod %s, stdout: %v, stderr: %v, err: %w", cmd, pod, stdout, stderr, err)
	}
	d.t.Logf("output of pod exec: %s:\n%s\n", pod, stdout)
	devices := []string{}
	sc := bufio.NewScanner(strings.NewReader(stdout))
	for sc.Scan() {
		after, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "MIG ")
		if !found {
			continue
		}
		split := strings.Split(after, " ")
		devices = append(devices, split[0])
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no MIG devices found in output")
	}
	return devices, nil
}

func (d GpuOperator) LogNvidiSMIOutput(ctx context.Context, node *corev1.Node) error {
	client := d.clientset.CoreV1().Pods(d.installer.Namespace)
	result, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component" + "=" + "nvidia-driver",
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return fmt.Errorf("did not find any pod for %s on node: %s - %w", "nvidia-driver-daemonset", node.Name, err)
	}
	pod := result.Items[0].Name

	cmd := []string{"nvidia-smi"}
	g.By(fmt.Sprintf("exec into pod: %s, command: %v", pod, cmd))
	stdout, stderr, err := e2epod.ExecWithOptionsContext(ctx, d.f, e2epod.ExecOptions{
		Command:       cmd,
		Namespace:     d.installer.Namespace,
		PodName:       pod,
		ContainerName: "nvidia-driver-ctr",
		CaptureStdout: true,
		CaptureStderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to run command %v on pod %s, stdout: %v, stderr: %v, err: %w", cmd, pod, stdout, stderr, err)
	}
	d.t.Logf("output of pod exec: %s:\n%s\n", pod, stdout)
	return nil
}
