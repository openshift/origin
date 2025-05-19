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
	testutils "k8s.io/kubernetes/test/utils"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	//nvidiav1 "github.com/NVIDIA/gpu-operator/api/versioned/typed/nvidia/v1"
	nvidia "github.com/NVIDIA/gpu-operator/api/versioned"
)

func NewGPUOperatorInstaller(t testing.TB, clientset kubernetes.Interface, oc *exutil.CLI, nvidia *nvidia.Clientset, p HelmParameters) *gpuOperator {
	return &gpuOperator{
		t:         t,
		clientset: clientset,
		helm:      NewHelmInstaller(t, p),
		oc:        oc,
	}
}

type GpuOperator struct {
	t         testing.TB
	helm      *HelmInstaller
	clientset kubernetes.Interface
	nvidia    *nvidia.Clientset
	oc        *exutil.CLI
}

func (d *GpuOperator) Install() error {
	const (
		enforceKey   = "pod-security.kubernetes.io/enforce"
		enforceValue = "privileged"
	)
	client := d.clientset.CoreV1().Namespaces()
	ns, err := client.Get(context.Background(), d.helm.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		want := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: d.helm.Namespace,
				Labels: map[string]string{
					enforceKey: enforceValue,
				},
			},
		}
		_, err = client.Create(context.Background(), want, metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}

	if v, ok := ns.Labels[enforceKey]; !ok || v != enforceValue {
		want := ns.DeepCopy()
		if len(want.Labels) == 0 {
			want.Labels = map[string]string{}
		}
		want.Labels[enforceKey] = enforceValue
		_, err = client.Update(context.Background(), want, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return d.helm.Install()
}

func (d *GpuOperator) Cleanup(ctx context.Context) error { return d.helm.Remove() }

func (d GpuOperator) Ready(node *corev1.Node) error {
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
			o.Eventually(func() error {
				return d.PodRunningReady(probe.component, probe.options)
			}).WithPolling(5*time.Second).
				WithTimeout(10*time.Minute).Should(o.BeNil(), fmt.Sprintf("[%s] pod should be ready", probe.component))
		}
	}

	return nil
}

func (d GpuOperator) MIGReady(node *corev1.Node) error {
	o.Eventually(func() error {
		return d.PodRunningReady("nvidia-mig-manager-daemonset", metav1.ListOptions{
			LabelSelector: "app" + "=" + "nvidia-mig-manager",
			FieldSelector: "spec.nodeName" + "=" + node.Name,
		})
	}).WithPolling(5*time.Second).WithTimeout(10*time.Minute).Should(o.BeNil(), "nvidia-mig-manage pod should be ready")
	return nil
}

func (d GpuOperator) PodRunningReady(component string, options metav1.ListOptions) error {
	client := d.clientset.CoreV1().Pods(d.helm.Namespace)
	result, err := client.List(context.Background(), options)
	if err != nil || len(result.Items) == 0 {
		return fmt.Errorf("[%s] still waiting for pod to show up - %w", component, err)
	}

	for _, pod := range result.Items {
		ready, err := testutils.PodRunningReady(&pod)
		if err != nil || !ready {
			err := fmt.Errorf("[%s] still waiting for pod: %s to be ready: %v", component, pod.Name, err)
			d.t.Log(err.Error())
			return err
		}
		d.t.Logf("[%s] pod: %s ready", component, pod.Name)
	}
	return nil
}

func (d GpuOperator) DiscoverGPUProudct(node *corev1.Node) (string, error) {
	client := d.clientset.CoreV1().Pods(d.helm.Namespace)
	result, err := client.List(context.Background(), metav1.ListOptions{
		LabelSelector: "app" + "=" + "gpu-feature-discovery",
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return "", fmt.Errorf("did not find any pod for %s on node: %s - %w", "gpu-feature-discovery", node.Name, err)
	}

	args := []string{
		"-n", d.helm.Namespace, result.Items[0].Name, "-c", "gpu-feature-discovery", "--", "cat", "/etc/kubernetes/node-feature-discovery/features.d/gfd",
	}
	g.By(fmt.Sprintf("calling oc exec %v", args))
	out, err := d.oc.AsAdmin().Run("exec").Args(args...).Output()
	if err != nil {
		return "", err
	}
	d.t.Logf("output: \n%s\n", out)

	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		after, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "nvidia.com/gpu.product=")
		if !found {
			continue
		}
		return strings.Trim(strings.TrimSpace(after), "'"), nil
	}

	return "", fmt.Errorf("nvidia.com/gpu.product not found in output")
}

func (d GpuOperator) DiscoverMIGDevices(node *corev1.Node) ([]string, error) {
	client := d.clientset.CoreV1().Pods(d.helm.Namespace)
	result, err := client.List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component" + "=" + "nvidia-driver",
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, fmt.Errorf("did not find any pod for %s on node: %s - %w", "nvidia-driver-daemonset", node.Name, err)
	}

	args := []string{
		"-n", d.helm.Namespace, result.Items[0].Name, "--", "nvidia-smi", "-L",
	}
	g.By(fmt.Sprintf("calling oc exec %v", args))
	out, err := d.oc.AsAdmin().Run("exec").Args(args...).Output()
	if err != nil {
		return nil, err
	}
	d.t.Logf("output: \n%s\n", out)

	devices := []string{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		after, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "MIG ")
		if !found {
			continue
		}
		split := strings.Split(after, " ")
		devices = append(devices, split[0])
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no MIG devices seen")
	}
	return devices, fmt.Errorf("nvidia.com/gpu.product not found in output")
}
