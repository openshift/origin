package pods

import (
	"context"
	//"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	//rbacv1 "k8s.io/api/rbac/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	//authv1 "github.com/openshift/api/authorization/v1"
	//projv1 "github.com/openshift/api/project/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	namespace               = "ns-cgroupversion"
	nodeLabelSelectorWorker = "node-role.kubernetes.io/worker,!node-role.kubernetes.io/edge"
	serviceAccountName      = "cgroupversion"
)
const (
	debugScript = `
#!/bin/bash

stat -f -c %T /sys/fs/cgroup
`
)

var _ = Describe("[sig-node][Serial][Feature:CgroupV1Check]", func() {

	var (
		oc           = exutil.NewCLIWithoutNamespace("node").AsAdmin()
		node         *corev1.Node
		outputString string
	)

	It("1) Should prevent cgroup v1 from being set on the cluster", func() {
		ctx := context.Background()

		By("Getting first worker node", func() {
			nodes, err := oc.KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: nodeLabelSelectorWorker})
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes.Items).NotTo(HaveLen(0))
			node = &nodes.Items[0]
		})

		By("2) Creating debug pod to verify cgroup version", func() {
			err := wait.Poll(time.Second, time.Second*60, func() (done bool, err error) {
				outputString, err = debugPod(oc, node.Name)
				if err != nil && strings.Contains(outputString, "unable to create the debug pod") {
					return false, nil
				}
				return true, err
			})
			Expect(err).NotTo(HaveOccurred(), "was not able to fetch cgroup version")
			Expect(outputString).To(ContainSubstring("cgroup2fs"), "Cgroup v2 is not enabled on the node")
		})

		Expect(outputString).To(ContainSubstring("cgroup2"), "Cluster should not allow cgroup v1 mode")

		By("3) Try Modifying node config to set cgroup mode to v1", func() {
			//command := "oc patch nodes.config.openshift.io cluster --type=merge -p '{\"spec\":{\"cgroupMode\":\"v1\"}}'"
			output, err := debugPod(oc, node.Name)
			Expect(err).NotTo(HaveOccurred(), "Failed to patch node configuration")
			Expect(output).To(ContainSubstring("error"), "Error: nodes.config.openshift.io 'cluster' is invalid")
		})

	})
})

func debugPod(oc *exutil.CLI, nodeName string) (output string, err error) {
	name := "debug"
	//cmd := "stat -c %T -f /sys/fs/cgroup"
	cmd := debugScript
	pod := adminPod(name, nodeName, cmd)
	ctx := context.Background()
	_, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return
	}

	_, err = exutil.WaitForPods(
		oc.AdminKubeClient().CoreV1().Pods(namespace),
		labels.SelectorFromSet(map[string]string{"app": name}),
		func(p corev1.Pod) bool {
			return p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodSucceeded
		}, 1, time.Minute*5)
	if err != nil {
		return
	}

	byteOutput, logError := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{}).DoRaw(ctx)
	output = string(byteOutput)
	return output, logError
}
func adminPod(podName, nodeName, script string) *corev1.Pod {
	isTrue := true
	zero := int64(0)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"app": podName},
		},
		Spec: corev1.PodSpec{
			HostPID:            true,
			RestartPolicy:      corev1.RestartPolicyNever,
			NodeName:           nodeName,
			ServiceAccountName: serviceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/", // MOUNT AT ROOT (/) INSTEAD OF /host
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: podName,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &zero,
						Privileged: &isTrue,
					},
					Image: image.ShellImage(),
					Command: []string{
						"/bin/bash",
						"-c",
						script,
					},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/", // MOUNT IT AT ROOT INSTEAD OF /host
							Name:      "root",
						},
					},
				},
			},
		},
	}
}
