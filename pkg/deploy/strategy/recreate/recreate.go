package recreate

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default. Its behavior is to increase the
// replica count of the new deployment to 1, and to decrease the replica count of previous deployments
// to zero.
//
// A failure to disable any existing deployments will be considered a deployment failure.
type RecreateDeploymentStrategy struct {
	// client is used to interact with ReplicatonControllers.
	client replicationControllerClient
	// codec is used to decode DeploymentConfigs contained in deployments.
	codec runtime.Codec

	retryTimeout time.Duration
	retryPeriod  time.Duration
}

func NewRecreateDeploymentStrategy(client kclient.Interface, codec runtime.Codec) *RecreateDeploymentStrategy {
	return &RecreateDeploymentStrategy{
		client:       &realReplicationController{client},
		codec:        codec,
		retryTimeout: 10 * time.Second,
		retryPeriod:  1 * time.Second,
	}
}

// Deploy makes deployment active and disables oldDeployments.
func (s *RecreateDeploymentStrategy) Deploy(deployment *kapi.ReplicationController, oldDeployments []kapi.ObjectReference) error {
	var err error
	var deploymentConfig *deployapi.DeploymentConfig

	if deploymentConfig, err = deployutil.DecodeDeploymentConfig(deployment, s.codec); err != nil {
		return fmt.Errorf("Couldn't decode DeploymentConfig from deployment %s: %v", deployment.Name, err)
	}

	if err = s.updateReplicas(deployment.Namespace, deployment.Name, deploymentConfig.Template.ControllerTemplate.Replicas); err != nil {
		return err
	}

	// For this simple deploy, disable previous replication controllers.
	glog.Infof("Found %d prior deployments to disable", len(oldDeployments))
	allProcessed := true
	for _, oldDeployment := range oldDeployments {
		if err = s.updateReplicas(oldDeployment.Namespace, oldDeployment.Name, 0); err != nil {
			glog.Errorf("%v", err)
			allProcessed = false
		}
	}

	if !allProcessed {
		return fmt.Errorf("Failed to disable all prior deployments for new deployment %s", deployment.Name)
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
			return fmt.Errorf("Couldn't successfully update deployment %s replica count to %d (timeout exceeded)", deployment.Name, replicaCount)
		default:
			if deployment, err = s.client.getReplicationController(namespace, name); err != nil {
				glog.Errorf("Couldn't get deployment %s/%s: %v", namespace, name, err)
			} else {
				deployment.Spec.Replicas = replicaCount
				glog.Infof("Updating deployment %s/%s replica count to %d", namespace, name, replicaCount)
				if _, err = s.client.updateReplicationController(namespace, deployment); err == nil {
					return nil
				}
				// For conflict errors, retry immediately
				if kerrors.IsConflict(err) {
					continue
				}
				glog.Errorf("Error updating deployment %s/%s replica count to %d: %v", namespace, name, replicaCount, err)
			}

			time.Sleep(s.retryPeriod)
		}
	}
}

type replicationControllerClient interface {
	getReplicationController(namespace, name string) (*kapi.ReplicationController, error)
	updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

type realReplicationController struct {
	client kclient.Interface
}

func (r realReplicationController) getReplicationController(namespace string, name string) (*kapi.ReplicationController, error) {
	return r.client.ReplicationControllers(namespace).Get(name)
}

func (r realReplicationController) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.client.ReplicationControllers(namespace).Update(ctrl)
}
