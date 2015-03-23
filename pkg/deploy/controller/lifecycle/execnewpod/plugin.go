package execnewpod

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	lifecycle "github.com/openshift/origin/pkg/deploy/controller/lifecycle"
)

// Plugin is a lifecycle Plugin which knows how to work with ExecNewPod handlers.
type Plugin struct {
	// PodClient provides access to pods.
	PodClient PodClient
}

var _ = lifecycle.Plugin(&Plugin{})

func (p *Plugin) CanHandle(handler *deployapi.Handler) bool {
	return handler.ExecNewPod != nil
}

func (p *Plugin) Execute(context lifecycle.Context, handler *deployapi.Handler, deployment *kapi.ReplicationController, config *deployapi.DeploymentConfig) error {
	podSpec := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: fmt.Sprintf("deployment-%s-lifecycle-%s-", deployment.Name, strings.ToLower(string(context))),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "lifecycle",
					Image: "tianon/true",
				},
			},
			// TODO: policy handling
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}

	pod, err := p.PodClient.CreatePod(deployment.Namespace, podSpec)
	if err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return fmt.Errorf("couldn't create lifecycle pod for %s: %v", labelForDeployment(deployment), err)
		}
	} else {
		glog.V(2).Infof("Created lifecycle pod %s for deployment %s", pod.Name, labelForDeployment(deployment))
	}

	return nil
}

func (p *Plugin) Status(context lifecycle.Context, handler *deployapi.Handler, deployment *kapi.ReplicationController) lifecycle.Status {
	podAnnotation, phaseAnnotation := annotationsFor(context)
	podName := deployment.Annotations[podAnnotation]
	if len(podName) == 0 {
		return lifecycle.Pending
	}

	phase := kapi.PodPhase(deployment.Annotations[phaseAnnotation])

	var status lifecycle.Status
	switch phase {
	case kapi.PodPending, kapi.PodRunning, kapi.PodUnknown:
		status = lifecycle.Running
	case kapi.PodSucceeded:
		status = lifecycle.Complete
	case kapi.PodFailed:
		status = lifecycle.Failed
	}
	return status
}

func annotationsFor(context lifecycle.Context) (string, string) {
	var podAnnotation string
	var phaseAnnotation string
	switch context {
	case lifecycle.Pre:
		podAnnotation = deployapi.PreExecNewPodActionPodAnnotation
		phaseAnnotation = deployapi.PreExecNewPodActionPodPhaseAnnotation
	case lifecycle.Post:
		podAnnotation = deployapi.PostExecNewPodActionPodAnnotation
		phaseAnnotation = deployapi.PostExecNewPodActionPodPhaseAnnotation
	}
	return podAnnotation, phaseAnnotation
}

// labelForDeployment builds a string identifier for a DeploymentConfig.
func labelForDeployment(deployment *kapi.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// PodClient abstracts access to pods.
type PodClient interface {
	CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
}

// PodClientImpl is a pluggable PodClient.
type PodClientImpl struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
}

func (i *PodClientImpl) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.CreatePodFunc(namespace, pod)
}
