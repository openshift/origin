package recreate

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentStrategy is a simple strategy appropriate as a default. Its behavior is to increase the
// replica count of the new deployment to 1, and to decrease the replica count of previous deployments
// to zero.
//
// A failure to disable any existing deployments will be considered a deployment failure.
type DeploymentStrategy struct {
	// ReplicationController is used to interact with ReplicatonControllers.
	ReplicationController ReplicationControllerInterface
	// Codec is used to decode DeploymentConfigs contained in deployments.
	Codec runtime.Codec
}

// Deploy makes deployment active and disables oldDeployments.
func (s *DeploymentStrategy) Deploy(deployment *kapi.ReplicationController, oldDeployments []kapi.ObjectReference) error {
	var err error
	var deploymentConfig *deployapi.DeploymentConfig

	if deploymentConfig, err = deployutil.DecodeDeploymentConfig(deployment, s.Codec); err != nil {
		return fmt.Errorf("Couldn't decode DeploymentConfig from deployment %s: %v", deployment.Name, err)
	}

	deployment.Spec.Replicas = deploymentConfig.Template.ControllerTemplate.Replicas
	glog.Infof("Updating deployment %s replica count to %d", deployment.Name, deployment.Spec.Replicas)
	if _, err := s.ReplicationController.updateReplicationController(deployment.Namespace, deployment); err != nil {
		return fmt.Errorf("Error updating deployment %s replica count to %d: %v", deployment.Name, deployment.Spec.Replicas, err)
	}

	// For this simple deploy, disable previous replication controllers.
	// TODO: This isn't transactional, and we don't actually wait for the replica count to
	// become zero before deleting them.
	glog.Infof("Found %d prior deployments to disable", len(oldDeployments))
	allProcessed := true
	for _, oldDeployment := range oldDeployments {
		oldController, oldErr := s.ReplicationController.getReplicationController(oldDeployment.Namespace, oldDeployment.Name)
		if oldErr != nil {
			glog.Errorf("Error getting old deployment %s for disabling: %v", oldDeployment.Name, oldErr)
			allProcessed = false
			continue
		}

		glog.Infof("Setting replicas to zero for old deployment %s", oldController.Name)
		oldController.Spec.Replicas = 0
		if _, err := s.ReplicationController.updateReplicationController(oldController.Namespace, oldController); err != nil {
			glog.Errorf("Error updating replicas to zero for old deployment %s: %#v", oldController.Name, err)
			allProcessed = false
			continue
		}
	}

	if !allProcessed {
		return fmt.Errorf("Failed to disable all prior deployments for new deployment %s", deployment.Name)
	}

	glog.Infof("Deployment %s successfully made active", deployment.Name)
	return nil
}

type ReplicationControllerInterface interface {
	getReplicationController(namespace, name string) (*kapi.ReplicationController, error)
	updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
}

type RealReplicationController struct {
	KubeClient kclient.Interface
}

func (r RealReplicationController) getReplicationController(namespace string, name string) (*kapi.ReplicationController, error) {
	return r.KubeClient.ReplicationControllers(namespace).Get(name)
}

func (r RealReplicationController) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.KubeClient.ReplicationControllers(namespace).Update(ctrl)
}
