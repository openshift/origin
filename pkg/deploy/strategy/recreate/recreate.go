package recreate

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default.
// Its behavior is to decrease the replica count of previous deployments to zero,
// and to increase the replica count of the new deployment to 1.
//
// A failure to disable any existing deployments will be considered a
// deployment failure.
type RecreateDeploymentStrategy struct {
	// client is used to interact with ReplicatonControllers.
	client replicationControllerClient
	// codec is used to decode DeploymentConfigs contained in deployments.
	codec runtime.Codec
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor

	retryTimeout time.Duration
	retryPeriod  time.Duration
}

// NewRecreateDeploymentStrategy makes a RecreateDeploymentStrategy backed by
// a real HookExecutor and client.
func NewRecreateDeploymentStrategy(client kclient.Interface, codec runtime.Codec) *RecreateDeploymentStrategy {
	return &RecreateDeploymentStrategy{
		client: &realReplicationControllerClient{client},
		codec:  codec,
		hookExecutor: &stratsupport.HookExecutor{
			PodClient: &stratsupport.HookExecutorPodClientImpl{
				CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
					return client.Pods(namespace).Create(pod)
				},
				PodWatchFunc: func(namespace, name, resourceVersion string) func() *kapi.Pod {
					return stratsupport.NewPodWatch(client, namespace, name, resourceVersion)
				},
			},
		},
		retryTimeout: 10 * time.Second,
		retryPeriod:  1 * time.Second,
	}
}

// Deploy makes deployment active and disables oldDeployments.
func (s *RecreateDeploymentStrategy) Deploy(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error {
	var err error
	var deploymentConfig *deployapi.DeploymentConfig

	if deploymentConfig, err = deployutil.DecodeDeploymentConfig(deployment, s.codec); err != nil {
		return fmt.Errorf("couldn't decode DeploymentConfig from Deployment %s: %v", deployment.Name, err)
	}

	params := deploymentConfig.Template.Strategy.RecreateParams
	// Execute any pre-hook.
	if params != nil && params.Pre != nil {
		err := s.hookExecutor.Execute(params.Pre, deployment, "prehook")
		if err != nil {
			return fmt.Errorf("Pre hook failed: %s", err)
		}
	}

	// Prefer to use an explicitly set desired replica count, falling back to
	// the value defined on the config.
	desiredReplicas := deploymentConfig.Template.ControllerTemplate.Replicas
	if desired, hasDesired := deployment.Annotations[deployapi.DesiredReplicasAnnotation]; hasDesired {
		val, err := strconv.Atoi(desired)
		if err != nil {
			glog.Errorf("Deployment has an invalid desired replica count '%s'; falling back to config value %d", desired, desiredReplicas)
		} else {
			glog.V(4).Infof("Deployment has an explicit desired replica count %d", val)
			desiredReplicas = val
		}
	} else {
		glog.V(4).Infof("Deployment has no explicit desired replica count; using the config value %d", desiredReplicas)
	}

	// Disable any old deployments.
	glog.V(4).Infof("Found %d prior deployments to disable", len(oldDeployments))
	allProcessed := true
	for _, oldDeployment := range oldDeployments {
		if err = s.updateReplicas(oldDeployment.Namespace, oldDeployment.Name, 0); err != nil {
			glog.Errorf("%v", err)
			allProcessed = false
		}
	}

	if !allProcessed {
		return fmt.Errorf("failed to disable all prior deployments for new Deployment %s", deployment.Name)
	}

	// Scale up the new deployment.
	if err = s.updateReplicas(deployment.Namespace, deployment.Name, desiredReplicas); err != nil {
		return err
	}

	// Execute any post-hook. Errors are logged and ignored.
	if params != nil && params.Post != nil {
		err := s.hookExecutor.Execute(params.Post, deployment, "posthook")
		if err != nil {
			glog.Errorf("Post hook failed: %s", err)
		} else {
			glog.Infof("Post hook finished")
		}
	}

	glog.Infof("Deployment %s successfully made active", deployment.Name)
	return nil
}

// updateReplicas attempts to set the given deployment's replicaCount using retry logic.
func (s *RecreateDeploymentStrategy) updateReplicas(namespace, name string, replicaCount int) error {
	var err error
	var deployment *kapi.ReplicationController

	timeout := time.After(s.retryTimeout)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("couldn't successfully update Deployment %s/%s replica count to %d (timeout exceeded)", namespace, name, replicaCount)
		default:
			if deployment, err = s.client.getReplicationController(namespace, name); err != nil {
				glog.Errorf("Couldn't get Deployment %s/%s: %v", namespace, name, err)
			} else {
				deployment.Spec.Replicas = replicaCount
				glog.V(4).Infof("Updating Deployment %s/%s replica count to %d", namespace, name, replicaCount)
				if _, err = s.client.updateReplicationController(namespace, deployment); err == nil {
					return nil
				}
				// For conflict errors, retry immediately
				if kerrors.IsConflict(err) {
					continue
				}
				glog.Errorf("Error updating Deployment %s/%s replica count to %d: %v", namespace, name, replicaCount, err)
			}

			time.Sleep(s.retryPeriod)
		}
	}
}

// replicationControllerClient provides access to ReplicationControllers.
type replicationControllerClient interface {
	getReplicationController(namespace, name string) (*kapi.ReplicationController, error)
	updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

// realReplicationControllerClient is a replicationControllerClient which uses
// a Kube client.
type realReplicationControllerClient struct {
	client kclient.Interface
}

func (r *realReplicationControllerClient) getReplicationController(namespace string, name string) (*kapi.ReplicationController, error) {
	return r.client.ReplicationControllers(namespace).Get(name)
}

func (r *realReplicationControllerClient) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.client.ReplicationControllers(namespace).Update(ctrl)
}

// hookExecutor knows how to execute a deployment lifecycle hook.
type hookExecutor interface {
	Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error
}

// hookExecutorImpl is a pluggable hookExecutor.
type hookExecutorImpl struct {
	executeFunc func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error
}

func (i *hookExecutorImpl) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	return i.executeFunc(hook, deployment, label)
}
