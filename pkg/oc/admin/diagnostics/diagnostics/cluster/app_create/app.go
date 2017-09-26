package app_create

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	conditions "k8s.io/kubernetes/pkg/client/unversioned"

	apps "github.com/openshift/origin/pkg/apps/apis/apps"
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
	dc := &apps.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   d.appName,
			Labels: d.label,
		},
		Spec: apps.DeploymentConfigSpec{
			Replicas: 1,
			Selector: d.label,
			Triggers: []apps.DeploymentTriggerPolicy{
				{Type: apps.DeploymentTriggerOnConfigChange},
			},
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: d.label},
				Spec: kapi.PodSpec{
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers: []kapi.Container{
						{
							Name:  d.appName,
							Image: d.appImage,
							Ports: []kapi.ContainerPort{
								{
									Name:          "http",
									ContainerPort: int32(d.appPort),
									Protocol:      kapi.ProtocolTCP,
								},
							},
							ImagePullPolicy: kapi.PullIfNotPresent,
							Command: []string{
								"socat", "-T", "1", "-d",
								fmt.Sprintf("%s-l:%d,reuseaddr,fork,crlf", kapi.ProtocolTCP, d.appPort),
								"system:\"echo 'HTTP/1.0 200 OK'; echo 'Content-Type: text/plain'; echo; echo 'Hello'\"",
							},
							ReadinessProbe: &kapi.Probe{
								// The action taken to determine the health of a container
								Handler: kapi.Handler{
									HTTPGet: &kapi.HTTPGetAction{
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
		running, err := conditions.PodContainerRunning(d.appName)(event)
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
