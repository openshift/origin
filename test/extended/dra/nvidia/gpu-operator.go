package nvidia

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

	helper "github.com/openshift/origin/test/extended/dra/helper"
)

func NewGPUOperatorInstaller(t testing.TB, clientset kubernetes.Interface, f *framework.Framework, p helper.HelmParameters) *GpuOperator {
	return &GpuOperator{
		t:         t,
		clientset: clientset,
		installer: helper.NewHelmInstaller(t, p),
		f:         f,
	}
}

func DefaultNFDHelmValues() map[string]any {
	return map[string]any{
		"worker": map[string]any{
			"config": map[string]any{
				"sources": map[string]any{
					"pci": map[string]any{
						"deviceLabelFields": []string{"vendor"},
					},
					"custom": []map[string]any{
						{
							"name": "nvidia-gpu-testing",
							"labels": map[string]any{
								"nvidia.com": true,
							},
							"matchFeatures": []map[string]any{
								{
									"feature": "pci.device",
									"matchExpressions": map[string]any{
										"class": map[string]any{
											"op":    "In",
											"value": []string{"0302"},
										},
										"vendor": map[string]any{
											"op":    "In",
											"value": []string{"10de"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func DefaultGPUOperatorHelmValues() map[string]any {
	return map[string]any{
		"devicePlugin": map[string]any{
			// we will use nvidia DRA driver, so disable the plugin
			"enabled": false,
		},
		"driver": map[string]any{
			"version": "570.148.08",
		},
		"cdi": map[string]any{
			"enabled": true,
		},
		"toolkit": map[string]any{
			"version": "v1.17.8-ubi8",
		},
		"nfd": map[string]any{
			// we hav already installed nfd with custom configuration
			"enabled": false,
		},
		"platform": map[string]any{
			"openshift": true,
		},
		"operator": map[string]any{
			"use_ocp_driver_toolkit": true,
			"logging": map[string]any{
				"level": "debug",
			},
		},
	}
}

func DefaultDRADriverHelmValues() map[string]any {
	return map[string]any{
		"nvidiaDriverRoot":            "/run/nvidia/driver",
		"gpuResourcesEnabledOverride": true,
		// the controller can run on the master node
		"controller": map[string]any{
			"tolerations": []map[string]any{
				{
					"key":      "node-role.kubernetes.io/master",
					"operator": "Exists",
					"effect":   "NoSchedule",
				},
			},
			"affinity": map[string]any{
				"nodeAffinity": map[string]any{
					"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{
						"nodeSelectorTerms": []map[string]any{
							{
								"matchExpressions": []map[string]any{
									{
										"key":      "node-role.kubernetes.io/master",
										"operator": "Exists",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

type GpuOperator struct {
	t testing.TB
	f *framework.Framework
	// we don't depend on the clientset from f, esp the clean up code
	clientset kubernetes.Interface
	installer *helper.HelmInstaller
	timeout   time.Duration
}

func (d *GpuOperator) Namespace() string { return d.installer.Namespace }

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
				return helper.PodRunningReady(ctx, d.t, d.clientset, probe.component, d.installer.Namespace, probe.options)
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
	} {
		if probe.enabled {
			g.By(fmt.Sprintf("waiting for %s to be ready", probe.component))
			o.Eventually(ctx, func(ctx context.Context) error {
				return helper.PodRunningReady(ctx, d.t, d.clientset, probe.component, d.installer.Namespace, probe.options)
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

func (d GpuOperator) RunNvidiSMI(ctx context.Context, node *corev1.Node, options ...string) ([]string, error) {
	client := d.clientset.CoreV1().Pods(d.installer.Namespace)
	result, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component" + "=" + "nvidia-driver",
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, fmt.Errorf("did not find any pod for %s on node: %s - %w", "nvidia-driver-daemonset", node.Name, err)
	}
	pod := result.Items[0].Name

	cmd := []string{"nvidia-smi"}
	cmd = append(cmd, options...)
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
	sc := bufio.NewScanner(strings.NewReader(stdout))
	lines := []string{}
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, nil
}

func (d GpuOperator) ListMIGDevicesUsingNvidiaSMI(ctx context.Context, node *corev1.Node) (NvidiaGPUs, error) {
	lines, err := d.RunNvidiSMI(ctx, node, "-L")
	if err != nil {
		return nil, err
	}
	return ExtractMIGDeviceInfoFromNvidiaSMILines(lines), nil
}

func ExtractMIGDeviceInfoFromNvidiaSMILines(lines []string) NvidiaGPUs {
	gpus := NvidiaGPUs{}
	for _, line := range lines {
		gpu := ExtractMIGDeviceInfoFromNvidiaSMI(line)
		if gpu.Type != "mig" {
			continue
		}
		gpus = append(gpus, gpu)
	}
	return gpus
}

func ExtractMIGDeviceInfoFromNvidiaSMI(line string) NvidiaGPU {
	// example line
	// GPU 0: NVIDIA A100-SXM4-40GB (UUID: GPU-fcf41002-68b6-5900-d7d1-74026173bb44)
	//   MIG 3g.20gb     Device  0: (UUID: MIG-e07a497d-1bb6-5a42-b670-1c33bf55ab6e)
	gpu := NvidiaGPU{}
	after, found := strings.CutPrefix(strings.TrimSpace(line), "MIG ")
	if !found {
		return gpu
	}
	gpu.Type = "mig"

	split := strings.Split(after, " ")
	gpu.Name = split[0]
	if len(split) > 0 {
		gpu.UUID = strings.TrimRight(split[len(split)-1], ")")
	}
	return gpu
}

func QueryGPUUsedByContainer(ctx context.Context, t testing.TB, f *framework.Framework, name, namespace, container string) (NvidiaGPUs, error) {
	cmd := []string{"nvidia-smi", "--query-gpu=index,uuid", "--format=csv"}
	t.Logf("exec into pod: %s, container: %s, command: %v", name, container, cmd)
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
	t.Logf("output of pod exec: %s/%s (container=%s):\n\n%s\n%s", namespace, name, container, stdout, stderr)
	gpus := NvidiaGPUs{}
	sc := bufio.NewScanner(strings.NewReader(stdout))
	var ignored bool
	for sc.Scan() {
		s := strings.Split(sc.Text(), ",")
		// ignore the first line, it's the header
		if !ignored {
			ignored = true
			continue
		}
		if len(s) != 2 {
			continue
		}
		gpus = append(gpus, NvidiaGPU{
			Index: strings.TrimSpace(s[0]),
			UUID:  strings.TrimSpace(s[1]),
		})
	}
	return gpus, nil
}

func (d GpuOperator) QueryCompute(ctx context.Context, node *corev1.Node, gpuIndex string) (NvidiaComputes, error) {
	options := []string{"--query-compute-apps=gpu_uuid,pid,process_name", "--format=csv"}
	lines, err := d.RunNvidiSMI(ctx, node, options...)
	if err != nil {
		return nil, err
	}

	processes := []NvidiaCompute{}
	var firsLineIngonred bool
	for _, line := range lines {
		// ignore the first line, it's the header
		if !firsLineIngonred {
			firsLineIngonred = true
			continue
		}
		s := strings.Split(line, ",")
		if len(s) != 3 {
			continue
		}
		processes = append(processes, NvidiaCompute{
			GPU:  strings.TrimSpace(s[0]),
			PID:  strings.TrimSpace(s[1]),
			Name: strings.TrimSpace(s[2]),
		})
	}
	return processes, nil
}

type NvidiaCompute struct {
	// process name, and pid
	Name string
	PID  string
	// UUID of the GPU on which this process is running
	GPU string
}

func (c NvidiaCompute) String() string {
	return fmt.Sprintf("gpu: %s, name: %s, pid: %s", c.GPU, c.Name, c.PID)
}

type NvidiaComputes []NvidiaCompute

func (s NvidiaComputes) FilterBy(f func(p NvidiaCompute) bool) NvidiaComputes {
	processes := NvidiaComputes{}
	for _, p := range s {
		if f(p) {
			processes = append(processes, p)
		}
	}
	return processes
}

func (s NvidiaComputes) Names() []string {
	names := []string{}
	for _, p := range s {
		names = append(names, p.Name)
	}
	return names
}
