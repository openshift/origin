package kubeletselinuxlabels

import (
	"github.com/openshift/origin/test/extended/util/image"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	busyScript = `
#!/bin/bash
while true
do
	kubelet_label=$(ps -x -Z | grep '/usr/bin/kubelet' | awk '{print $1}')
	SUB='kubelet_t'
	if [[ "$kubelet_label" != *"$SUB"* ]]; then
		echo "kubelet label does not match expected label"
		echo "failing this pod and therefore the test"
  		exit 1
	fi
	sleep 30
done
	`
)

// Generate a pod spec with the termination grace period specified, and busy work lasting a little less
// then the specified grace period
// Pod tests if kubelet has the right selinux label
// and fails the pod if it doesn't.
func selinuxPodSpec(name, namespace, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			PriorityClassName: "system-cluster-critical",
			NodeName:          nodeName,
			HostPID:           true,
			Containers: []corev1.Container{
				{

					Image:           image.ShellImage(),
					ImagePullPolicy: corev1.PullIfNotPresent,
					Name:            name,
					Command: []string{
						"/bin/bash",
						"-c",
						busyScript,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
					},
				},
			},
		},
	}
}
