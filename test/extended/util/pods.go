package util

import (
	"context"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
)

// WaitForNoPodsAvailable waits until there are no pods in the
// given namespace
func WaitForNoPodsAvailable(oc *CLI) error {
	return wait.Poll(200*time.Millisecond, 3*time.Minute, func() (bool, error) {
		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == 0, nil
	})
}

// RemovePodsWithPrefixes deletes pods whose name begins with the
// supplied prefixes
func RemovePodsWithPrefixes(oc *CLI, prefixes ...string) error {
	e2e.Logf("Removing pods from namespace %s with prefix(es): %v", oc.Namespace(), prefixes)
	pods, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	errs := []error{}
	for _, prefix := range prefixes {
		for _, pod := range pods.Items {
			if strings.HasPrefix(pod.Name, prefix) {
				if err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
					e2e.Logf("unable to remove pod %s/%s", oc.Namespace(), pod.Name)
					errs = append(errs, err)
				}
			}
		}
	}
	if len(errs) > 0 {
		return kutilerrors.NewAggregate(errs)
	}
	return nil
}

// CreateUbiExecPodOrFail creates a ubi8 pause pod used as a vessel for kubectl exec commands.
// Pod name is uniquely generated.
func CreateUbiExecPodOrFail(client kubernetes.Interface, ns, generateName string, tweak func(*v1.Pod)) *v1.Pod {
	return pod.CreateExecPodOrFail(client, ns, generateName, func(pod *v1.Pod) {
		pod.Spec.Containers[0].Image = "ubi8/ubi"
		pod.Spec.Containers[0].Command = []string{"sh", "-c", "trap exit TERM; while true; do sleep 5; done"}
		pod.Spec.Containers[0].Args = nil

		if tweak != nil {
			tweak(pod)
		}
	})
}
