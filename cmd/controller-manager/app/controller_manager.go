/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package app implements a server that runs the service catalog controllers.
package app

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/leaderelection/resourcelock"
	"k8s.io/kubernetes/pkg/util/configz"

	// The API groups for our API must be installed before we can use the
	// client to work with them.  This needs to be done once per process; this
	// is the point at which we handle this for the controller-manager
	// process.  Please do not remove.
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
	// The core API has to be installed in order for the client to understand
	// error messages from the API server.  Please do not remove.
	_ "k8s.io/client-go/pkg/api/install"

	"github.com/kubernetes-incubator/service-catalog/cmd/controller-manager/app/options"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/openservicebroker"
	servicecataloginformers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions"
	"github.com/kubernetes-incubator/service-catalog/pkg/controller"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewControllerManagerCommand creates a *cobra.Command object with default
// parameters.
func NewControllerManagerCommand() *cobra.Command {
	s := options.NewControllerManagerServer()
	s.AddFlags(pflag.CommandLine)
	cmd := &cobra.Command{
		Use: "controller-manager",
		Long: `The service-catalog controller manager is a daemon that embeds
the core control loops shipped with the service catalog.`,
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return cmd
}

const controllerManagerAgentName = "service-catalog-controller-manager"
const controllerDiscoveryAgentName = "service-catalog-controller-discovery"

// Run runs the service-catalog controller-manager; should never exit.
func Run(controllerManagerOptions *options.ControllerManagerServer) error {
	// TODO: what does this do

	// if c, err := configz.New("componentconfig"); err == nil {
	// 	c.Set(controllerManagerOptions.KubeControllerManagerConfiguration)
	// } else {
	// 	glog.Errorf("unable to register configz: %s", err)
	// }

	// Build the K8s kubeconfig / client / clientBuilder
	glog.V(4).Info("Building k8s kubeconfig")

	var err error
	var k8sKubeconfig *rest.Config
	if controllerManagerOptions.K8sAPIServerURL == "" && controllerManagerOptions.K8sKubeconfigPath == "" {
		k8sKubeconfig, err = rest.InClusterConfig()
	} else {
		k8sKubeconfig, err = clientcmd.BuildConfigFromFlags(
			controllerManagerOptions.K8sAPIServerURL,
			controllerManagerOptions.K8sKubeconfigPath)
	}
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client config: %v", err)
	}
	k8sKubeconfig.GroupVersion = &schema.GroupVersion{}

	k8sKubeconfig.ContentConfig.ContentType = controllerManagerOptions.ContentType
	// Override kubeconfig qps/burst settings from flags
	k8sKubeconfig.QPS = controllerManagerOptions.KubeAPIQPS
	k8sKubeconfig.Burst = int(controllerManagerOptions.KubeAPIBurst)
	k8sKubeClient, err := kubernetes.NewForConfig(
		rest.AddUserAgent(k8sKubeconfig, controllerManagerAgentName),
	)
	if err != nil {
		return fmt.Errorf("invalid Kubernetes API configuration: %v", err)
	}
	leaderElectionClient := clientset.NewForConfigOrDie(rest.AddUserAgent(k8sKubeconfig, "leader-election"))

	glog.V(4).Infof("Building service-catalog kubeconfig for url: %v\n", controllerManagerOptions.ServiceCatalogAPIServerURL)

	var serviceCatalogKubeconfig *rest.Config
	// Build the service-catalog kubeconfig / clientBuilder
	if controllerManagerOptions.ServiceCatalogAPIServerURL == "" && controllerManagerOptions.ServiceCatalogKubeconfigPath == "" {
		// explicitly fall back to InClusterConfig, assuming we're talking to an API server which does aggregation
		// (BuildConfigFromFlags does this, but gives a more generic warning message than we do here)
		glog.V(4).Infof("Using inClusterConfig to talk to service catalog API server -- make sure your API server is registered with the aggregator")
		serviceCatalogKubeconfig, err = rest.InClusterConfig()
	} else {
		serviceCatalogKubeconfig, err = clientcmd.BuildConfigFromFlags(
			controllerManagerOptions.ServiceCatalogAPIServerURL,
			controllerManagerOptions.ServiceCatalogKubeconfigPath)
	}
	if err != nil {
		// TODO: disambiguate API errors
		return fmt.Errorf("failed to get Service Catalog client configuration: %v", err)
	}

	// due to using both k8s.io/kubernetes and k8s.io/client-go, we need to convert this object over

	glog.V(4).Info("Starting http server and mux")
	// Start http server and handlers
	go func() {
		mux := http.NewServeMux()
		healthz.InstallHandler(mux)
		configz.InstallHandler(mux)

		server := &http.Server{
			Addr:    net.JoinHostPort(controllerManagerOptions.Address, strconv.Itoa(int(controllerManagerOptions.Port))),
			Handler: mux,
		}
		glog.Fatal(server.ListenAndServe())
	}()

	// Create event broadcaster
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: k8sKubeClient.Core().Events("")})
	recorder := eventBroadcaster.NewRecorder(api.Scheme, v1.EventSource{Component: controllerManagerAgentName})

	// 'run' is the logic to run the controllers for the controller manager
	run := func(stop <-chan struct{}) {
		serviceCatalogClientBuilder := controller.SimpleClientBuilder{
			ClientConfig: serviceCatalogKubeconfig,
		}

		// TODO: understand service account story for this controller-manager

		// if len(s.ServiceAccountKeyFile) > 0 && controllerManagerOptions.UseServiceAccountCredentials {
		// 	k8sClientBuilder = controller.SAControllerClientBuilder{
		// 		ClientConfig: restclient.AnonymousClientConfig(k8sKubeconfig),
		// 		CoreClient:   k8sKubeClient.Core(),
		// 		Namespace:    "kube-system",
		// 	}
		// } else {
		// 	k8sClientBuilder = rootClientBuilder
		// }

		err := StartControllers(controllerManagerOptions, k8sKubeconfig, serviceCatalogClientBuilder, recorder, stop)
		glog.Fatalf("error running controllers: %v", err)
		panic("unreachable")
	}

	if !controllerManagerOptions.LeaderElection.LeaderElect {
		run(make(<-chan (struct{})))
		panic("unreachable")
	}

	// Identity used to distinguish between multiple cloud controller manager instances
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// Lock required for leader election
	rl := resourcelock.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{
			Namespace: "kube-system",
			Name:      "service-catalog-controller-manager",
		},
		Client: leaderElectionClient,
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      id + "-external-service-catalog-controller",
			EventRecorder: recorder,
		},
	}

	// Try and become the leader and start cloud controller manager loops
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          &rl,
		LeaseDuration: controllerManagerOptions.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: controllerManagerOptions.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   controllerManagerOptions.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})
	panic("unreachable")
}

// getAvailableResources uses the discovery client to determine which API
// groups are available in the endpoint reachable from the given client and
// returns a map of them.
func getAvailableResources(clientBuilder controller.ClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	var discoveryClient discovery.DiscoveryInterface

	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 10*time.Second, func() (bool, error) {
		client, err := clientBuilder.Client(controllerDiscoveryAgentName)
		if err != nil {
			glog.Errorf("Failed to get api versions from server: %v", err)
			return false, nil
		}

		glog.V(4).Info("Created client for discovery")

		discoveryClient = client.Discovery()
		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get api versions from server: %v", err)
	}

	resourceMap, err := discoveryClient.ServerResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get supported resources from server: %v", err)
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resourceMap {
		glog.V(4).Infof("Resource: %#v", apiResourceList)
		version, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, apiResource := range apiResourceList.APIResources {
			allResources[version.WithResource(apiResource.Name)] = true
		}
	}

	return allResources, nil
}

// StartControllers starts all the controllers in the service-catalog
// controller manager.
func StartControllers(s *options.ControllerManagerServer,
	coreKubeconfig *rest.Config,
	serviceCatalogClientBuilder controller.ClientBuilder,
	recorder record.EventRecorder,
	stop <-chan struct{}) error {

	// Get available service-catalog resources
	glog.V(5).Info("Getting available resources")
	availableResources, err := getAvailableResources(serviceCatalogClientBuilder)
	if err != nil {
		return err
	}

	coreKubeconfig = rest.AddUserAgent(coreKubeconfig, controllerManagerAgentName)
	coreClient, err := kubernetes.NewForConfig(coreKubeconfig)
	if err != nil {
		glog.Fatal(err)
	}

	// Launch service-catalog controller
	if availableResources[schema.GroupVersionResource{Group: "servicecatalog.k8s.io", Version: "v1alpha1", Resource: "brokers"}] {
		glog.V(5).Info("Creating shared informers; resync interval: %v", s.ResyncInterval)
		// Build the informer factory for service-catalog resources
		informerFactory := servicecataloginformers.NewSharedInformerFactory(
			serviceCatalogClientBuilder.ClientOrDie("shared-informers"),
			s.ResyncInterval,
		)
		// All shared informers are v1alpha1 API level
		serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1alpha1()

		glog.V(5).Info("Creating controller; broker relist interval: %v", s.BrokerRelistInterval)
		serviceCatalogController, err := controller.NewController(
			coreClient,
			serviceCatalogClientBuilder.ClientOrDie(controllerManagerAgentName).ServicecatalogV1alpha1(),
			serviceCatalogSharedInformers.Brokers(),
			serviceCatalogSharedInformers.ServiceClasses(),
			serviceCatalogSharedInformers.Instances(),
			serviceCatalogSharedInformers.Bindings(),
			openservicebroker.NewClient,
			s.BrokerRelistInterval,
			s.OSBAPIContextProfile,
			recorder,
		)
		if err != nil {
			return err
		}

		glog.V(5).Info("Running controller")
		go serviceCatalogController.Run(s.ConcurrentSyncs, stop)

		glog.V(1).Info("Starting shared informers")
		informerFactory.Start(stop)
	} else {
		return fmt.Errorf("unable to start service-catalog controller: servicecatalog/v1alpha1 is not available")
	}

	select {}
}
