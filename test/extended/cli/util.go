package cli

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"

	appsv1 "github.com/openshift/api/apps/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

func waitForPodModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(pod *corev1.Pod) (bool, error)) (*corev1.Pod, error) {
	watcher, err := oc.KubeClient().CoreV1().Pods(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := watchtools.UntilWithoutRetry(ctx, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (resourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for Pod modification: event: %#v", event)
		}
		return condition(event.Object.(*corev1.Pod))
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*corev1.Pod), nil
}

func waitForRCModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(rc *corev1.ReplicationController) (bool, error)) (*corev1.ReplicationController, error) {
	watcher, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := watchtools.UntilWithoutRetry(ctx, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (resourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for RC modification: event: %#v", event)
		}
		return condition(event.Object.(*corev1.ReplicationController))
	})
	if err != nil {
		return nil, err
	}
	if event.Type != watch.Modified {
		return nil, fmt.Errorf("waiting for RC modification failed: event: %v", event)
	}
	return event.Object.(*corev1.ReplicationController), nil
}

func waitForDCModification(oc *exutil.CLI, namespace string, name string, timeout time.Duration, resourceVersion string, condition func(rc *appsv1.DeploymentConfig) (bool, error)) (*appsv1.DeploymentConfig, error) {
	watcher, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion}))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := watchtools.UntilWithoutRetry(ctx, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (resourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for DC modification: event: %#v", event)
		}
		return condition(event.Object.(*appsv1.DeploymentConfig))
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*appsv1.DeploymentConfig), nil
}
