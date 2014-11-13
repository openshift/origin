package recreate

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default. Its behavior
// is to create new replication controllers as defined on a Deployment, and delete any previously
// existing replication controllers for the same DeploymentConfig associated with the deployment.
//
// A failure to remove any existing ReplicationController will be considered a deployment failure.
type RecreateDeploymentStrategy struct {
	ReplicationControllerClient controllerClient
}

type controllerClient interface {
	ListReplicationControllers(ctx kapi.Context, selector labels.Selector) (*kapi.ReplicationControllerList, error)
	CreateReplicationController(ctx kapi.Context, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	UpdateReplicationController(ctx kapi.Context, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	DeleteReplicationController(ctx kapi.Context, id string) error
}

func (s *RecreateDeploymentStrategy) Deploy(deployment *deployapi.Deployment) error {
	ctx := kapi.WithNamespace(kapi.NewContext(), deployment.Namespace)

	controllers := &kapi.ReplicationControllerList{}
	var err error

	configID, hasConfigID := deployment.Annotations[deployapi.DeploymentConfigAnnotation]
	if !hasConfigID {
		return fmt.Errorf("This strategy is only compatible with deployments associated with a DeploymentConfig")
	}

	selector, _ := labels.ParseSelector(deployapi.DeploymentConfigLabel + "=" + configID)
	controllers, err = s.ReplicationControllerClient.ListReplicationControllers(ctx, selector)
	if err != nil {
		return fmt.Errorf("Unable to get list of replication controllers for previous deploymentConfig %s: %v\n", configID, err)
	}

	deploymentCopy, err := kapi.Scheme.Copy(deployment)
	if err != nil {
		return fmt.Errorf("Unable to copy deployment %s: %v\n", deployment.ID, err)
	}

	controller := &kapi.ReplicationController{
		TypeMeta: kapi.TypeMeta{
			Annotations: map[string]string{deployapi.DeploymentAnnotation: deployment.ID},
		},
		DesiredState: deploymentCopy.(*deployapi.Deployment).ControllerTemplate,
		Labels:       map[string]string{deployapi.DeploymentConfigLabel: configID},
	}

	// Correlate pods created by the ReplicationController to the deployment config
	if controller.DesiredState.PodTemplate.Labels == nil {
		controller.DesiredState.PodTemplate.Labels = make(map[string]string)
	}
	controller.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigLabel] = configID

	glog.Infof("Creating replicationController for deployment %s", deployment.ID)
	if _, err := s.ReplicationControllerClient.CreateReplicationController(ctx, controller); err != nil {
		return fmt.Errorf("An error occurred creating the replication controller for deployment %s: %v", deployment.ID, err)
	}

	// For this simple deploy, remove previous replication controllers.
	// TODO: This isn't transactional, and we don't actually wait for the replica count to
	// become zero before deleting them.
	allProcessed := true
	for _, rc := range controllers.Items {
		glog.Infof("Stopping replication controller for previous deploymentConfig %s: %v", configID, rc.ID)

		controller.DesiredState.Replicas = 0
		glog.Infof("Settings Replicas=0 for replicationController %s for previous deploymentConfig %s", rc.ID, configID)
		if _, err := s.ReplicationControllerClient.UpdateReplicationController(ctx, controller); err != nil {
			glog.Errorf("Unable to stop replication controller %s for previous deploymentConfig %s: %#v\n", rc.ID, configID, err)
			allProcessed = false
			continue
		}

		glog.Infof("Deleting replication controller %s for previous deploymentConfig %s", rc.ID, configID)
		err := s.ReplicationControllerClient.DeleteReplicationController(ctx, rc.ID)
		if err != nil {
			glog.Errorf("Unable to remove replication controller %s for previous deploymentConfig %s:%#v\n", rc.ID, configID, err)
			allProcessed = false
			continue
		}
	}

	if !allProcessed {
		return fmt.Errorf("Failed to clean up all replication controllers")
	}

	return nil
}
