package customimage

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"gopkg.in/v1/yaml"
)

type CustomImageDeployer struct {
	KClient  *kclient.Client
	OSClient osclient.Interface
}

func (d *CustomImageDeployer) Deploy(deploymentID string) {
	ctx := kapi.NewContext()
	glog.V(2).Infof("Retrieving deployment id: %v", deploymentID)

	var deployment *deployapi.Deployment
	var err error
	if deployment, err = d.OSClient.GetDeployment(ctx, deploymentID); err != nil {
		glog.Fatalf("An error occurred retrieving the deployment object: %v", err)
		return
	}

	var replicationControllers *kapi.ReplicationControllerList
	configID, hasConfigID := deployment.Labels[deployapi.DeploymentConfigLabel]
	if hasConfigID {
		selector, _ := labels.ParseSelector(deployapi.DeploymentConfigLabel + "=" + configID)
		replicationControllers, err = d.KClient.ListReplicationControllers(ctx, selector)
		if err != nil {
			glog.Fatalf("Unable to get list of replication controllers: %v\n", err)
			return
		}
	}

	controller := &kapi.ReplicationController{
		DesiredState: deployment.ControllerTemplate,
		Labels:       map[string]string{deployapi.DeploymentConfigLabel: configID, "deploymentID": deploymentID},
	}
	if controller.DesiredState.PodTemplate.Labels == nil {
		controller.DesiredState.PodTemplate.Labels = make(map[string]string)
	}
	if hasConfigID {
		controller.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigLabel] = configID
	}
	controller.DesiredState.PodTemplate.Labels["deploymentID"] = deploymentID

	glog.V(2).Info("Creating replication controller")
	obj, _ := yaml.Marshal(controller)
	glog.V(4).Info(string(obj))

	if _, err := d.KClient.CreateReplicationController(ctx, controller); err != nil {
		glog.Fatalf("An error occurred creating the replication controller: %v", err)
		return
	}

	glog.Info("Created replication controller")

	// For this simple deploy, remove previous replication controllers
	for _, rc := range replicationControllers.Items {
		glog.Infof("Stopping replication controller: %v", rc.ID)
		obj, _ := yaml.Marshal(rc)
		glog.Info(string(obj))
		rcObj, err1 := d.KClient.GetReplicationController(ctx, rc.ID)
		if err1 != nil {
			glog.Fatalf("Unable to get replication controller %s - error: %#v\n", rc.ID, err1)
		}
		rcObj.DesiredState.Replicas = 0
		_, err := d.KClient.UpdateReplicationController(ctx, rcObj)
		if err != nil {
			glog.Fatalf("Unable to stop replication controller %s - error: %#v\n", rc.ID, err)
		}
	}

	for _, rc := range replicationControllers.Items {
		glog.Infof("Deleting replication controller %s", rc.ID)
		err := d.KClient.DeleteReplicationController(ctx, rc.ID)
		if err != nil {
			glog.Fatalf("Unable to remove replication controller %s - error: %#v\n", rc.ID, err)
		}
	}
}
