package app_create

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"

	appsv1 "github.com/openshift/api/apps/v1"
)

func (d *AppCreate) createAndCheckAppDC() bool {
	result := &d.result.App
	result.BeginTime = jsonTime(time.Now())
	defer recordTrial(result)
	if !d.createAppDC() {
		return false
	}
	result.Success = d.checkPodRunning()
	return result.Success
}

// create the DC
func (d *AppCreate) createAppDC() bool {
	defer recordTime(&d.result.App.CreatedTime)
	gracePeriod := int64(0)
	dc := &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   d.appName,
			Labels: d.label,
		},
		Spec: appsv1.DeploymentConfigSpec{
			Replicas: 1,
			Selector: d.label,
			Triggers: []appsv1.DeploymentTriggerPolicy{
				{Type: appsv1.DeploymentTriggerOnConfigChange},
			},
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: d.label},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers: []corev1.Container{
						{
							Name:  d.appName,
							Image: d.appImage,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: int32(d.appPort),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"socat", "-T", "1", "-d",
								fmt.Sprintf("%s-l:%d,reuseaddr,fork,crlf", corev1.ProtocolTCP, d.appPort),
								"system:\"echo 'HTTP/1.0 200 OK'; echo 'Content-Type: text/plain'; echo; echo 'Hello'\"",
							},
							ReadinessProbe: &corev1.Probe{
								// The action taken to determine the health of a container
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(d.appPort),
									},
								},
								InitialDelaySeconds: 0,
								TimeoutSeconds:      1,
								PeriodSeconds:       1,
							},
						},
					},
				},
			},
		},
	}

	if _, err := d.AppsClient.Apps().DeploymentConfigs(d.project).Create(dc); err != nil {
		d.out.Error("DCluAC006", err, fmt.Sprintf("%s: Creating deploymentconfig '%s' failed:\n%v", now(), d.appName, err))
		return false
	}
	return true
}

// wait for a pod to become active
func (d *AppCreate) checkPodRunning() bool {
	defer recordTime(&d.result.App.ReadyTime)
	d.out.Debug("DCluAC007", fmt.Sprintf("%s: Waiting %ds for pod to reach running state.", now(), d.deployTimeout))
	watcher, err := d.KubeClient.Core().Pods(d.project).Watch(metav1.ListOptions{LabelSelector: d.labelSelector, TimeoutSeconds: &d.deployTimeout})
	if err != nil {
		d.out.Error("DCluAC008", err, fmt.Sprintf(`
%s: Failed to establish a watch for '%s' to deploy a pod:
  %v
This may be a transient error. Check the master API logs for anomalies near this time.
		`, now(), d.appName, err))
		return false
	}
	defer stopWatcher(watcher)
	for event := range watcher.ResultChan() {
		running, err := podContainerRunning(d.appName)(event)
		if err != nil {
			d.out.Error("DCluAC009", err, fmt.Sprintf(`
%s: Error while watching for app pod to deploy:
  %v
This may be a transient error. Check the master API logs for anomalies near this time.
			`, now(), err))
			return false
		}
		if running {
			d.out.Info("DCluAC010", fmt.Sprintf("%s: App '%s' is running", now(), d.appName))
			return true
		}
	}
	d.out.Error("DCluAC011", nil, fmt.Sprintf(`
%s: App pod was not in running state before timeout (%d sec)
There are many reasons why this can occur; for example:
  * The app or deployer image may not be available (check pod status)
  * Downloading an image may have timed out (consider increasing timeout)
  * The scheduler may be unable to find an appropriate node for it to run (check deployer logs)
  * The node container runtime may be malfunctioning (check node and docker/cri-o logs)
	`, now(), d.deployTimeout))
	return false
}

// podContainerRunning returns false until the named container has ContainerStatus running (at least once),
// and will return an error if the pod is deleted, runs to completion, or the container pod is not available.
func podContainerRunning(containerName string) watch.ConditionFunc {
	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
		}
		switch t := event.Object.(type) {
		case *api.Pod:
			switch t.Status.Phase {
			case api.PodRunning, api.PodPending:
			case api.PodFailed, api.PodSucceeded:
				return false, kubectl.ErrPodCompleted
			default:
				return false, nil
			}
			for _, s := range t.Status.ContainerStatuses {
				if s.Name != containerName {
					continue
				}
				if s.State.Terminated != nil {
					return false, kubectl.ErrContainerTerminated
				}
				return s.State.Running != nil, nil
			}
			for _, s := range t.Status.InitContainerStatuses {
				if s.Name != containerName {
					continue
				}
				if s.State.Terminated != nil {
					return false, kubectl.ErrContainerTerminated
				}
				return s.State.Running != nil, nil
			}
			return false, nil
		}
		return false, nil
	}
}
