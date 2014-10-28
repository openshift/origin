package main

import (
	"net/url"
	"os"

	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	oslatest "github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/deploy/deployer/customimage"
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

	kClient, err := kclient.New(&kclient.Config{Host: masterServer, Version: klatest.Version})
	if err != nil {
		glog.Errorf("Unable to connect to kubernetes master: %v", err)
		os.Exit(1)
	}

	osClient, err := osclient.New(&kclient.Config{Host: masterServer, Version: oslatest.Version})
	if err != nil {
		glog.Errorf("Unable to connect to openshift master: %v", err)
		os.Exit(1)
	}

	deploymentID := os.Getenv("KUBERNETES_DEPLOYMENT_ID")
	if len(deploymentID) == 0 {
		glog.Fatal("No deployment id was specified. Expected KUBERNETES_DEPLOYMENT_ID variable.")
		return
	}

	d := customimage.CustomImageDeployer{kClient, osClient}
	d.Deploy(deploymentID)
}
