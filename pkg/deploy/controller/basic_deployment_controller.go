package controller

import (
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// BasicDeploymentController implements the DeploymentStrategyTypeBasic deployment strategy. Its behavior
// is to create new replication controllers as defined on a Deployment, and delete any previously existing
// replication controllers for the same DeploymentConfig associated with the deployment.
type BasicDeploymentController struct {
	DeploymentUpdater           bdcDeploymentUpdater
	ReplicationControllerClient bdcReplicationControllerClient
	NextDeployment              func() *deployapi.Deployment
}

type bdcDeploymentUpdater interface {
	UpdateDeployment(ctx kapi.Context, deployment *deployapi.Deployment) (*deployapi.Deployment, error)
}

type bdcReplicationControllerClient interface {
	ListReplicationControllers(ctx kapi.Context, selector labels.Selector) (*kapi.ReplicationControllerList, error)
	GetReplicationController(ctx kapi.Context, id string) (*kapi.ReplicationController, error)
	CreateReplicationController(ctx kapi.Context, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	UpdateReplicationController(ctx kapi.Context, ctrl *kapi.ReplicationController) (*kapi.ReplicationController, error)
	DeleteReplicationController(ctx kapi.Context, id string) error
}

func (dc *BasicDeploymentController) Run() {
	go util.Forever(func() { dc.HandleDeployment() }, 0)
}

// HandleDeployment executes a single Deployment. It's assumed that the strategy of the deployment is
// DeploymentStrategyTypeBasic.
func (dc *BasicDeploymentController) HandleDeployment() error {
	deployment := dc.NextDeployment()

	if deployment.Strategy.Type != deployapi.DeploymentStrategyTypeBasic {
		glog.V(4).Infof("Ignoring deployment %s due to incompatible strategy type %s", deployment.ID, deployment.Strategy)
		return nil
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), deployment.Namespace)

	nextStatus := deployment.Status
	switch deployment.Status {
	case deployapi.DeploymentStatusNew:
		nextStatus = dc.handleNew(ctx, deployment)
	}

	// persist any status change
	if deployment.Status != nextStatus {
		deployment.Status = nextStatus
		glog.V(4).Infof("Saving deployment %v status: %v", deployment.ID, deployment.Status)
		if _, err := dc.DeploymentUpdater.UpdateDeployment(ctx, deployment); err != nil {
			glog.V(2).Infof("Received error while saving deployment %v: %v", deployment.ID, err)
			return err
		}
	}

	return nil
}

func (dc *BasicDeploymentController) handleNew(ctx kapi.Context, deployment *deployapi.Deployment) deployapi.DeploymentStatus {
	controllers := &kapi.ReplicationControllerList{}
	var err error

	configID, hasConfigID := deployment.Labels[deployapi.DeploymentConfigLabel]
	if hasConfigID {
		selector, _ := labels.ParseSelector(deployapi.DeploymentConfigLabel + "=" + configID)
		controllers, err = dc.ReplicationControllerClient.ListReplicationControllers(ctx, selector)
		if err != nil {
			glog.V(2).Infof("Unable to get list of replication controllers for previous deploymentConfig %s: %v\n", configID, err)
			return deployapi.DeploymentStatusFailed
		}
	}

	deploymentCopy, err := kapi.Scheme.Copy(deployment)
	if err != nil {
		glog.V(2).Infof("Unable to copy deployment %s: %v\n", deployment.ID, err)
		return deployapi.DeploymentStatusFailed
	}

	controller := &kapi.ReplicationController{
		DesiredState: deploymentCopy.(*deployapi.Deployment).ControllerTemplate,
		Labels:       map[string]string{deployapi.DeploymentConfigLabel: configID, "deployment": deployment.ID},
	}

	if controller.DesiredState.PodTemplate.Labels == nil {
		controller.DesiredState.PodTemplate.Labels = make(map[string]string)
	}

	controller.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigLabel] = configID
	controller.DesiredState.PodTemplate.Labels["deployment"] = deployment.ID

	glog.V(2).Infof("Creating replicationController for deployment %s", deployment.ID)
	if _, err := dc.ReplicationControllerClient.CreateReplicationController(ctx, controller); err != nil {
		glog.V(2).Infof("An error occurred creating the replication controller for deployment %s: %v", deployment.ID, err)
		return deployapi.DeploymentStatusFailed
	}

	allProcessed := true
	// For this simple deploy, remove previous replication controllers
	for _, rc := range controllers.Items {
		configID, _ := deployment.Labels[deployapi.DeploymentConfigLabel]
		glog.V(2).Infof("Stopping replication controller for previous deploymentConfig %s: %v", configID, rc.ID)

		controller, err := dc.ReplicationControllerClient.GetReplicationController(ctx, rc.ID)
		if err != nil {
			glog.V(2).Infof("Unable to get replication controller %s for previous deploymentConfig %s: %#v\n", rc.ID, configID, err)
			allProcessed = false
			continue
		}

		controller.DesiredState.Replicas = 0
		glog.V(2).Infof("Settings Replicas=0 for replicationController %s for previous deploymentConfig %s", rc.ID, configID)
		if _, err := dc.ReplicationControllerClient.UpdateReplicationController(ctx, controller); err != nil {
			glog.V(2).Infof("Unable to stop replication controller %s for previous deploymentConfig %s: %#v\n", rc.ID, configID, err)
			allProcessed = false
			continue
		}
	}

	for _, rc := range controllers.Items {
		configID, _ := deployment.Labels[deployapi.DeploymentConfigLabel]
		glog.V(2).Infof("Deleting replication controller %s for previous deploymentConfig %s", rc.ID, configID)
		err := dc.ReplicationControllerClient.DeleteReplicationController(ctx, rc.ID)
		if err != nil {
			glog.V(2).Infof("Unable to remove replication controller %s for previous deploymentConfig %s:%#v\n", rc.ID, configID, err)
			allProcessed = false
			continue
		}
	}

	if allProcessed {
		return deployapi.DeploymentStatusComplete
	}

	return deployapi.DeploymentStatusFailed
}
