package recreate

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentStrategy is a simple strategy appropriate as a default. Its behavior
// is to create new replication controllers as defined on a Deployment, and delete any previously
// existing replication controllers for the same DeploymentConfig associated with the deployment.
//
// A failure to remove any existing ReplicationController will be considered a deployment failure.
type DeploymentStrategy struct {
	ReplicationController ReplicationControllerInterface
	Codec                 runtime.Codec
}

type ReplicationControllerInterface interface {
	listReplicationControllers(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error)
	getReplicationController(namespace, id string) (*kapi.ReplicationController, error)
	updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	deleteReplicationController(namespace string, id string) error
}

type RealReplicationController struct {
	KubeClient kclient.Interface
}

func (r RealReplicationController) listReplicationControllers(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
	return r.KubeClient.ReplicationControllers(namespace).List(selector)
}

func (r RealReplicationController) getReplicationController(namespace string, id string) (*kapi.ReplicationController, error) {
	return r.KubeClient.ReplicationControllers(namespace).Get(id)
}

func (r RealReplicationController) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.KubeClient.ReplicationControllers(namespace).Update(ctrl)
}

func (r RealReplicationController) deleteReplicationController(namespace string, id string) error {
	return r.KubeClient.ReplicationControllers(namespace).Delete(id)
}

func (s *DeploymentStrategy) Deploy(deployment *kapi.ReplicationController) error {
	controllers := &kapi.ReplicationControllerList{}
	namespace := deployment.Namespace
	var err error

	configID, hasConfigID := deployment.Annotations[deployapi.DeploymentConfigAnnotation]
	if !hasConfigID {
		return fmt.Errorf("This strategy is only compatible with deployments associated with a DeploymentConfig")
	}

	selector, _ := labels.ParseSelector(deployapi.DeploymentConfigLabel + "=" + configID)
	controllers, err = s.ReplicationController.listReplicationControllers(namespace, selector)
	if err != nil {
		return fmt.Errorf("Unable to get list of replication controllers for previous deploymentConfig %s: %v\n", configID, err)
	}

	var deploymentConfig *deployapi.DeploymentConfig
	var decodeError error
	if deploymentConfig, decodeError = deployutil.DecodeDeploymentConfig(deployment, s.Codec); decodeError != nil {
		return fmt.Errorf("Couldn't decode DeploymentConfig from deployment %s: %v", deployment.Name, decodeError)
	}

	deployment.Spec.Replicas = deploymentConfig.Template.ControllerTemplate.Replicas
	glog.Infof("Updating replicationController for deployment %s to replica count %d", deployment.Name, deployment.Spec.Replicas)
	if _, err := s.ReplicationController.updateReplicationController(namespace, deployment); err != nil {
		return fmt.Errorf("An error occurred updating the replication controller for deployment %s: %v", deployment.Name, err)
	}

	// For this simple deploy, remove previous replication controllers.
	// TODO: This isn't transactional, and we don't actually wait for the replica count to
	// become zero before deleting them.
	allProcessed := true
	for _, oldController := range controllers.Items {
		glog.Infof("Stopping replication controller for previous deploymentConfig %s: %v", configID, oldController.Name)

		oldController.Spec.Replicas = 0
		glog.Infof("Settings Replicas=0 for replicationController %s for previous deploymentConfig %s", oldController.Name, configID)
		if _, err := s.ReplicationController.updateReplicationController(namespace, &oldController); err != nil {
			glog.Errorf("Unable to stop replication controller %s for previous deploymentConfig %s: %#v\n", oldController.Name, configID, err)
			allProcessed = false
			continue
		}

		glog.Infof("Deleting replication controller %s for previous deploymentConfig %s", oldController.Name, configID)
		err := s.ReplicationController.deleteReplicationController(namespace, oldController.Name)
		if err != nil {
			glog.Errorf("Unable to remove replication controller %s for previous deploymentConfig %s:%#v\n", oldController.Name, configID, err)
			allProcessed = false
			continue
		}
	}

	if !allProcessed {
		return fmt.Errorf("Failed to clean up all replication controllers")
	}

	return nil
}
