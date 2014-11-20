package recreate

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default. Its behavior
// is to create new replication controllers as defined on a Deployment, and delete any previously
// existing replication controllers for the same DeploymentConfig associated with the deployment.
//
// A failure to remove any existing ReplicationController will be considered a deployment failure.
type RecreateDeploymentStrategy struct {
	ReplicationController ReplicationControllerInterface
}

type ReplicationControllerInterface interface {
	listReplicationControllers(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error)
	getReplicationController(namespace, id string) (*kapi.ReplicationController, error)
	createReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
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

func (r RealReplicationController) createReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.KubeClient.ReplicationControllers(namespace).Create(ctrl)
}

func (r RealReplicationController) updateReplicationController(namespace string, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return r.KubeClient.ReplicationControllers(namespace).Update(ctrl)
}

func (r RealReplicationController) deleteReplicationController(namespace string, id string) error {
	return r.KubeClient.ReplicationControllers(namespace).Delete(id)
}

func (s *RecreateDeploymentStrategy) Deploy(deployment *deployapi.Deployment) error {
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

	deploymentCopy, err := kapi.Scheme.Copy(deployment)
	if err != nil {
		return fmt.Errorf("Unable to copy deployment %s: %v\n", deployment.Name, err)
	}

	controller := &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Annotations: map[string]string{deployapi.DeploymentAnnotation: deployment.Name},
			Labels:      map[string]string{deployapi.DeploymentConfigLabel: configID},
		},
		DesiredState: deploymentCopy.(*deployapi.Deployment).ControllerTemplate,
	}

	// Correlate pods created by the ReplicationController to the deployment config
	if controller.DesiredState.PodTemplate.Labels == nil {
		controller.DesiredState.PodTemplate.Labels = make(map[string]string)
	}
	controller.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigLabel] = configID

	glog.Infof("Creating replicationController for deployment %s", deployment.Name)
	if _, err := s.ReplicationController.createReplicationController(namespace, controller); err != nil {
		return fmt.Errorf("An error occurred creating the replication controller for deployment %s: %v", deployment.Name, err)
	}

	// For this simple deploy, remove previous replication controllers.
	// TODO: This isn't transactional, and we don't actually wait for the replica count to
	// become zero before deleting them.
	allProcessed := true
	for _, oldController := range controllers.Items {
		glog.Infof("Stopping replication controller for previous deploymentConfig %s: %v", configID, oldController.Name)

		oldController.DesiredState.Replicas = 0
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
