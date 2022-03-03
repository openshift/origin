package util

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	kappsv1 "k8s.io/api/apps/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

func CreateDS(c kclientset.Interface, namespace string, name string, command string, readinessCommand string, vms []kapiv1.VolumeMount, volumes []kapiv1.Volume) (*kappsv1.DaemonSet, error) {
	privileged := true
	var graceTime int64 = 0
	ds, err := c.AppsV1().DaemonSets(namespace).Create(context.TODO(), &kappsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kappsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"resolve": "true"},
			},
			Template: kapiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"resolve": "true"},
				},
				Spec: kapiv1.PodSpec{
					TerminationGracePeriodSeconds: &graceTime,
					Containers: []kapiv1.Container{
						{
							Name:    name,
							Image:   imageutils.GetE2EImage(imageutils.Agnhost),
							Command: []string{"/bin/bash", "-c", command},
							ReadinessProbe: &kapiv1.Probe{
								ProbeHandler: kapiv1.ProbeHandler{Exec: &kapiv1.ExecAction{Command: []string{"/bin/bash", "-c", readinessCommand}}},
							},
							SecurityContext: &kapiv1.SecurityContext{Privileged: &privileged},
							VolumeMounts:    vms,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}, metav1.CreateOptions{})
	return ds, err
}

func WaitForDSRunning(c kclientset.Interface, namespace string, name string) error {
	return wait.Poll(200*time.Millisecond, 3*time.Minute, func() (bool, error) {
		daemonset, err := c.AppsV1().DaemonSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return daemonset.Status.NumberReady == daemonset.Status.DesiredNumberScheduled, nil
	})
}
