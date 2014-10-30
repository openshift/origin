package main

import (
	"flag"
	"time"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version/verflag"
	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	lbmanager "github.com/openshift/origin/pkg/router/lbmanager"
	"github.com/openshift/origin/plugins/router/haproxy"
)

var (
	master = flag.String("master", "", "The address of the Kubernetes API server")
	debug  = flag.Bool("verbose", false, "Boolean flag to turn on debug messages")
)

func main() {
	flag.Parse()
	util.InitLogs()
	defer util.FlushLogs()

	verflag.PrintAndExitIfRequested()

	if len(*master) == 0 {
		glog.Fatal("usage: openshift-router -master <master>")
	}

	config := &kclient.Config{Host: *master}
	kubeClient, err := kclient.New(config)
	if err != nil {
		glog.Fatalf("Invalid -master: %v", err)
	}

	osClient, errc := osclient.New(config)
	if errc != nil {
		glog.Fatalf("Could not reach master for routes: %v", errc)
	}
	routes := haproxy.NewRouter()
	controllerManager := lbmanager.NewLBManager(routes, kubeClient, osClient)
	controllerManager.Run(10 * time.Second)
	select {}
}
