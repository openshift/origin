package main

import (
	"net/url"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	latest "github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"gopkg.in/v1/yaml"
)

func main() {
	util.InitLogs()
	defer util.FlushLogs()

	var masterServer string
	if len(os.Getenv("KUBERNETES_MASTER")) > 0 {
		masterServer = os.Getenv("KUBERNETES_MASTER")
	} else {
		masterServer = "http://localhost:8080"
	}
	_, err := url.Parse(masterServer)
	if err != nil {
		glog.Fatalf("Unable to parse %v as a URL\n", err)
	}

	client, err := kubeclient.New(&kubeclient.Config{Host: masterServer, Version: klatest.Version})
	if err != nil {
		glog.Errorf("Unable to connect to kubernetes master: %v", err)
		os.Exit(1)
	}

	osClient, err := osclient.New(&kubeclient.Config{Host: masterServer, Version: latest.Version})
	if err != nil {
		glog.Errorf("Unable to connect to openshift master: %v", err)
		os.Exit(1)
	}

	deployTarget(api.NewContext(), client, osClient)
}

func deployTarget(ctx api.Context, client *kubeclient.Client, osClient osclient.Interface) {
	deploymentID := os.Getenv("KUBERNETES_DEPLOYMENT_ID")
	if len(deploymentID) == 0 {
		glog.Fatal("No deployment id was specified. Expected KUBERNETES_DEPLOYMENT_ID variable.")
		return
	}
	glog.Infof("Retrieving deployment id: %v", deploymentID)

	var deployment *deployapi.Deployment
	var err error
	if deployment, err = osClient.GetDeployment(ctx, deploymentID); err != nil {
		glog.Fatalf("An error occurred retrieving the deployment object: %v", err)
		return
	}

	var replicationControllers *api.ReplicationControllerList
	configID, hasConfigID := deployment.Labels[deployapi.DeploymentConfigIDLabel]
	if hasConfigID {
		selector, _ := labels.ParseSelector(deployapi.DeploymentConfigIDLabel + "=" + configID)
		replicationControllers, err = client.ListReplicationControllers(ctx, selector)
		if err != nil {
			glog.Fatalf("Unable to get list of replication controllers: %v\n", err)
			return
		}
	}

	controller := &api.ReplicationController{
		DesiredState: deployment.ControllerTemplate,
		Labels:       map[string]string{deployapi.DeploymentConfigIDLabel: configID, "deploymentID": deploymentID},
	}
	if controller.DesiredState.PodTemplate.Labels == nil {
		controller.DesiredState.PodTemplate.Labels = make(map[string]string)
	}
	controller.DesiredState.PodTemplate.Labels[deployapi.DeploymentConfigIDLabel] = configID
	controller.DesiredState.PodTemplate.Labels["deploymentID"] = deploymentID

	glog.Info("Creating replication controller")
	obj, _ := yaml.Marshal(controller)
	glog.Info(string(obj))

	if _, err := client.CreateReplicationController(ctx, controller); err != nil {
		glog.Fatalf("An error occurred creating the replication controller: %v", err)
		return
	}

	glog.Info("Created replication controller")

	// For this simple deploy, remove previous replication controllers
	for _, rc := range replicationControllers.Items {
		glog.Infof("Stopping replication controller: %v", rc.ID)
		obj, _ := yaml.Marshal(rc)
		glog.Info(string(obj))
		rcObj, err1 := client.GetReplicationController(ctx, rc.ID)
		if err1 != nil {
			glog.Fatalf("Unable to get replication controller %s - error: %#v\n", rc.ID, err1)
		}
		rcObj.DesiredState.Replicas = 0
		_, err := client.UpdateReplicationController(ctx, rcObj)
		if err != nil {
			glog.Fatalf("Unable to stop replication controller %s - error: %#v\n", rc.ID, err)
		}
	}

	for _, rc := range replicationControllers.Items {
		glog.Infof("Deleting replication controller %s", rc.ID)
		err := client.DeleteReplicationController(ctx, rc.ID)
		if err != nil {
			glog.Fatalf("Unable to remove replication controller %s - error: %#v\n", rc.ID, err)
		}
	}
}
