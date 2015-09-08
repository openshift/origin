package recreate

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default.
// Its behavior is to scale down the last deployment to 0, and to scale up the
// new deployment to 1.
//
// A failure to disable any existing deployments will be considered a
// deployment failure.
type RecreateDeploymentStrategy struct {
	// getReplicationController knows how to get a replication controller.
	getReplicationController func(namespace, name string) (*kapi.ReplicationController, error)
	// scaler is used to scale replication controllers.
	scaler kubectl.Scaler
	// codec is used to decode DeploymentConfigs contained in deployments.
	codec runtime.Codec
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor
	// retryTimeout is how long to wait for the replica count update to succeed
	// before giving up.
	retryTimeout time.Duration
	// retryPeriod is how often to try updating the replica count.
	retryPeriod time.Duration
}

// NewRecreateDeploymentStrategy makes a RecreateDeploymentStrategy backed by
// a real HookExecutor and client.
func NewRecreateDeploymentStrategy(client kclient.Interface, codec runtime.Codec) *RecreateDeploymentStrategy {
	scaler, _ := kubectl.ScalerFor("ReplicationController", kubectl.NewScalerClient(client))
	return &RecreateDeploymentStrategy{
		getReplicationController: func(namespace, name string) (*kapi.ReplicationController, error) {
			return client.ReplicationControllers(namespace).Get(name)
		},
		scaler: scaler,
		codec:  codec,
		hookExecutor: &stratsupport.HookExecutor{
			PodClient: &stratsupport.HookExecutorPodClientImpl{
				CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
					return client.Pods(namespace).Create(pod)
				},
				PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
					return stratsupport.NewPodWatch(client, namespace, name, resourceVersion, stopChannel)
				},
			},
		},
		retryTimeout: 120 * time.Second,
		retryPeriod:  1 * time.Second,
	}
}

// Deploy makes deployment active and disables oldDeployments.
func (s *RecreateDeploymentStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	return s.DeployWithAcceptor(from, to, desiredReplicas, nil)
}

// DeployWithAcceptor scales down from and then scales up to. If
// updateAcceptor is provided and the desired replica count is >1, the first
// replica of to is rolled out and validated before performing the full scale
// up.
//
// This is currently only used in conjunction with the rolling update strategy
// for initial deployments.
func (s *RecreateDeploymentStrategy) DeployWithAcceptor(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int, updateAcceptor kubectl.UpdateAcceptor) error {
	config, err := deployutil.DecodeDeploymentConfig(to, s.codec)
	if err != nil {
		return fmt.Errorf("couldn't decode config from deployment %s: %v", to.Name, err)
	}

	params := config.Template.Strategy.RecreateParams
	retryParams := kubectl.NewRetryParams(s.retryPeriod, s.retryTimeout)
	waitParams := kubectl.NewRetryParams(s.retryPeriod, s.retryTimeout)

	// Execute any pre-hook.
	if params != nil && params.Pre != nil {
		if err := s.hookExecutor.Execute(params.Pre, to, "prehook"); err != nil {
			return fmt.Errorf("Pre hook failed: %s", err)
		} else {
			glog.Infof("Pre hook finished")
		}
	}

	// Scale down the from deployment.
	if from != nil {
		glog.Infof("Scaling %s down to zero", deployutil.LabelForDeployment(from))
		_, err := s.scaleAndWait(from, 0, retryParams, waitParams)
		if err != nil {
			return fmt.Errorf("couldn't scale %s to 0: %v", deployutil.LabelForDeployment(from), err)
		}
	}

	// Scale up the to deployment.
	if desiredReplicas > 0 {
		// If an UpdateAcceptor is provided, scale up to 1 and validate the replica,
		// aborting if the replica isn't acceptable.
		if updateAcceptor != nil {
			glog.Infof("Scaling %s to 1 before performing acceptance check", deployutil.LabelForDeployment(to))
			updatedTo, err := s.scaleAndWait(to, 1, retryParams, waitParams)
			if err != nil {
				return fmt.Errorf("couldn't scale %s to 1: %v", deployutil.LabelForDeployment(to), err)
			}
			glog.Infof("Performing acceptance check of %s", deployutil.LabelForDeployment(to))
			if err := updateAcceptor.Accept(updatedTo); err != nil {
				return fmt.Errorf("update acceptor rejected %s: %v", deployutil.LabelForDeployment(to), err)
			}
			to = updatedTo
		}
		// Complete the scale up.
		if to.Spec.Replicas != desiredReplicas {
			glog.Infof("Scaling %s to %d", deployutil.LabelForDeployment(to), desiredReplicas)
			updatedTo, err := s.scaleAndWait(to, desiredReplicas, retryParams, waitParams)
			if err != nil {
				return fmt.Errorf("couldn't scale %s to %d: %v", deployutil.LabelForDeployment(to), desiredReplicas, err)
			}
			to = updatedTo
		}
	}

	// Execute any post-hook. Errors are logged and ignored.
	if params != nil && params.Post != nil {
		if err := s.hookExecutor.Execute(params.Post, to, "posthook"); err != nil {
			util.HandleError(fmt.Errorf("post hook failed: %s", err))
		} else {
			glog.Infof("Post hook finished")
		}
	}

	glog.Infof("Deployment %s successfully made active", to.Name)
	return nil
}

func (s *RecreateDeploymentStrategy) scaleAndWait(deployment *kapi.ReplicationController, replicas int, retry *kubectl.RetryParams, wait *kubectl.RetryParams) (*kapi.ReplicationController, error) {
	if err := s.scaler.Scale(deployment.Namespace, deployment.Name, uint(replicas), &kubectl.ScalePrecondition{-1, ""}, retry, wait); err != nil {
		return nil, err
	}
	updatedDeployment, err := s.getReplicationController(deployment.Namespace, deployment.Name)
	if err != nil {
		return nil, err
	}
	return updatedDeployment, nil
}

// hookExecutor knows how to execute a deployment lifecycle hook.
type hookExecutor interface {
	Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error
}

// hookExecutorImpl is a pluggable hookExecutor.
type hookExecutorImpl struct {
	executeFunc func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error
}

// Execute executes the provided lifecycle hook
func (i *hookExecutorImpl) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	return i.executeFunc(hook, deployment, label)
}
