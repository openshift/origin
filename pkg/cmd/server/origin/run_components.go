package origin

import (
	"fmt"
	"io/ioutil"
	"net"
	"path"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/registry/service/allocator"
	etcdallocator "k8s.io/kubernetes/pkg/registry/service/allocator/etcd"
	"k8s.io/kubernetes/pkg/util"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	"github.com/openshift/origin/pkg/api/latest"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configchangecontroller "github.com/openshift/origin/pkg/deploy/controller/configchange"
	deployerpodcontroller "github.com/openshift/origin/pkg/deploy/controller/deployerpod"
	deploycontroller "github.com/openshift/origin/pkg/deploy/controller/deployment"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	imagechangecontroller "github.com/openshift/origin/pkg/deploy/controller/imagechange"
	"github.com/openshift/origin/pkg/dns"
	imagecontroller "github.com/openshift/origin/pkg/image/controller"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	projectcontroller "github.com/openshift/origin/pkg/project/controller"
	securitycontroller "github.com/openshift/origin/pkg/security/controller"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"

	"github.com/openshift/openshift-sdn/plugins/osdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/flatsdn"
	"github.com/openshift/openshift-sdn/plugins/osdn/multitenant"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kubeserver "github.com/openshift/origin/pkg/cmd/server/kubernetes"
	serviceaccountcontrollers "github.com/openshift/origin/pkg/serviceaccounts/controllers"
)

// RunProjectAuthorizationCache starts the project authorization cache
func (c *MasterConfig) RunProjectAuthorizationCache() {
	// TODO: look at exposing a configuration option in future to control how often we run this loop
	period := 1 * time.Second
	c.ProjectAuthorizationCache.Run(period)
}

// RunOriginNamespaceController starts the controller that takes part in namespace termination of openshift content
func (c *MasterConfig) RunOriginNamespaceController() {
	osclient, kclient := c.OriginNamespaceControllerClients()
	factory := projectcontroller.NamespaceControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
	}
	controller := factory.Create()
	controller.Run()
}

// RunServiceAccountsController starts the service account controller
func (c *MasterConfig) RunServiceAccountsController() {
	if len(c.Options.ServiceAccountConfig.ManagedNames) == 0 {
		glog.Infof("Skipped starting Service Account Manager, no managed names specified")
		return
	}
	options := serviceaccount.DefaultServiceAccountsControllerOptions()
	options.ServiceAccounts = []kapi.ServiceAccount{}

	for _, saName := range c.Options.ServiceAccountConfig.ManagedNames {
		sa := kapi.ServiceAccount{}
		sa.Name = saName

		options.ServiceAccounts = append(options.ServiceAccounts, sa)
	}

	serviceaccount.NewServiceAccountsController(c.KubeClient(), options).Run()
}

// RunServiceAccountTokensController starts the service account token controller
func (c *MasterConfig) RunServiceAccountTokensController() {
	if len(c.Options.ServiceAccountConfig.PrivateKeyFile) == 0 {
		glog.Infof("Skipped starting Service Account Token Manager, no private key specified")
		return
	}

	privateKey, err := serviceaccount.ReadPrivateKey(c.Options.ServiceAccountConfig.PrivateKeyFile)
	if err != nil {
		glog.Fatalf("Error reading signing key for Service Account Token Manager: %v", err)
	}
	rootCA := []byte{}
	if len(c.Options.ServiceAccountConfig.MasterCA) > 0 {
		rootCA, err = ioutil.ReadFile(c.Options.ServiceAccountConfig.MasterCA)
		if err != nil {
			glog.Fatalf("Error reading master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
		if _, err := util.CertsFromPEM(rootCA); err != nil {
			glog.Fatalf("Error parsing master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
	}

	options := serviceaccount.TokensControllerOptions{
		TokenGenerator: serviceaccount.JWTTokenGenerator(privateKey),
		RootCA:         rootCA,
	}

	serviceaccount.NewTokensController(c.KubeClient(), options).Run()
}

// RunServiceAccountPullSecretsControllers starts the service account pull secret controllers
func (c *MasterConfig) RunServiceAccountPullSecretsControllers() {
	serviceaccountcontrollers.NewDockercfgDeletedController(c.KubeClient(), serviceaccountcontrollers.DockercfgDeletedControllerOptions{}).Run()
	serviceaccountcontrollers.NewDockercfgTokenDeletedController(c.KubeClient(), serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{}).Run()

	dockercfgController := serviceaccountcontrollers.NewDockercfgController(c.KubeClient(), serviceaccountcontrollers.DockercfgControllerOptions{DefaultDockerURL: serviceaccountcontrollers.DefaultOpenshiftDockerURL})
	dockercfgController.Run()

	dockerRegistryControllerOptions := serviceaccountcontrollers.DockerRegistryServiceControllerOptions{
		RegistryNamespace:   "default",
		RegistryServiceName: "docker-registry",
		DockercfgController: dockercfgController,
		DefaultDockerURL:    serviceaccountcontrollers.DefaultOpenshiftDockerURL,
	}
	serviceaccountcontrollers.NewDockerRegistryServiceController(c.KubeClient(), dockerRegistryControllerOptions).Run()
}

// RunPolicyCache starts the policy cache
func (c *MasterConfig) RunPolicyCache() {
	c.PolicyCache.Run()
}

// RunAssetServer starts the asset server for the OpenShift UI.
func (c *MasterConfig) RunAssetServer() {

}

// RunDNSServer starts the DNS server
func (c *MasterConfig) RunDNSServer() {
	config, err := dns.NewServerDefaults()
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	config.Domain = c.Options.DNSConfig.Domain + "."
	switch c.Options.DNSConfig.BindNetwork {
	case "tcp":
		config.BindNetwork = "ip"
	case "tcp4":
		config.BindNetwork = "ipv4"
	case "tcp6":
		config.BindNetwork = "ipv6"
	}
	config.DnsAddr = c.Options.DNSConfig.BindAddress
	config.NoRec = true // do not want to deploy an open resolver

	_, port, err := net.SplitHostPort(c.Options.DNSConfig.BindAddress)
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	if port != "53" {
		glog.Warningf("Binding DNS on port %v instead of 53 (you may need to run as root and update your config), using %s which will not resolve from all locations", port, c.Options.DNSConfig.BindAddress)
	}

	if ok, err := cmdutil.TryListen(c.Options.DNSConfig.BindNetwork, c.Options.DNSConfig.BindAddress); !ok {
		glog.Warningf("Could not start DNS: %v", err)
		return
	}

	go func() {
		err := dns.ListenAndServe(config, c.DNSServerClient(), c.EtcdClient)
		glog.Fatalf("Could not start DNS: %v", err)
	}()

	cmdutil.WaitForSuccessfulDial(false, "tcp", c.Options.DNSConfig.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("DNS listening at %s", c.Options.DNSConfig.BindAddress)
}

// RunProjectCache populates project cache, used by scheduler and project admission controller.
func (c *MasterConfig) RunProjectCache() {
	glog.Infof("Using default project node label selector: %s", c.Options.ProjectConfig.DefaultNodeSelector)
	projectcache.RunProjectCache(c.PrivilegedLoopbackKubernetesClient, c.Options.ProjectConfig.DefaultNodeSelector)
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController() {
	// initialize build controller
	dockerImage := c.ImageFor("docker-builder")
	stiImage := c.ImageFor("sti-builder")

	storageVersion := c.Options.EtcdStorageConfig.OpenShiftStorageVersion
	interfaces, err := latest.InterfacesFor(storageVersion)
	if err != nil {
		glog.Fatalf("Unable to load storage version %s: %v", storageVersion, err)
	}

	admissionControl := admission.NewFromPlugins(c.PrivilegedLoopbackKubernetesClient, []string{"SecurityContextConstraint"}, "")

	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildControllerFactory{
		OSClient:     osclient,
		KubeClient:   kclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: dockerImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: interfaces.Codec,
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image:                stiImage,
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec:            interfaces.Codec,
			AdmissionControl: admissionControl,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: interfaces.Codec,
		},
	}

	controller := factory.Create()
	controller.Run()
	deleteController := factory.CreateDeleteController()
	deleteController.Run()
}

// RunBuildPodController starts the build/pod status sync loop for build status
func (c *MasterConfig) RunBuildPodController() {
	osclient, kclient := c.BuildPodControllerClients()
	factory := buildcontrollerfactory.BuildPodControllerFactory{
		OSClient:     osclient,
		KubeClient:   kclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
	}
	controller := factory.Create()
	controller.Run()
	deletecontroller := factory.CreateDeleteController()
	deletecontroller.Run()
}

// RunBuildImageChangeTriggerController starts the build image change trigger controller process.
func (c *MasterConfig) RunBuildImageChangeTriggerController() {
	bcClient, _ := c.BuildImageChangeTriggerControllerClients()
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient)
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: bcClient, BuildConfigInstantiator: bcInstantiator}
	factory.Create().Run()
}

// RunBuildConfigChangeController starts the build config change trigger controller process.
func (c *MasterConfig) RunBuildConfigChangeController() {
	bcClient, _ := c.BuildConfigChangeControllerClients()
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient)
	factory := buildcontrollerfactory.BuildConfigControllerFactory{Client: bcClient, BuildConfigInstantiator: bcInstantiator}
	factory.Create().Run()
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunDeploymentController() {
	_, kclient := c.DeploymentControllerClients()

	_, kclientConfig, err := configapi.GetKubeClient(c.Options.MasterClients.OpenShiftLoopbackKubeConfig)
	if err != nil {
		glog.Fatalf("Unable to initialize deployment controller: %v", err)
	}

	port, err := kubeserver.GetKubernetesMasterPort(c.Options)
	if err != nil {
		glog.Fatal(err)
	}
	master := fmt.Sprintf("https://kubernetes.default.svc:%d", port)
	// TODO eliminate these environment variables once service accounts provide a kubeconfig that includes all of this info
	env := clientcmd.EnvVars(
		master,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	)

	factory := deploycontroller.DeploymentControllerFactory{
		KubeClient:     kclient,
		Codec:          c.EtcdHelper.Codec(),
		Environment:    env,
		DeployerImage:  c.ImageFor("deployer"),
		ServiceAccount: bootstrappolicy.DeployerServiceAccountName,
	}

	controller := factory.Create()
	controller.Run()
}

// RunDeployerPodController starts the deployer pod controller process.
func (c *MasterConfig) RunDeployerPodController() {
	_, kclient := c.DeployerPodControllerClients()
	factory := deployerpodcontroller.DeployerPodControllerFactory{
		KubeClient: kclient,
	}

	controller := factory.Create()
	controller.Run()
}

// RunDeploymentConfigController starts the deployment config controller process.
func (c *MasterConfig) RunDeploymentConfigController() {
	osclient, kclient := c.DeploymentConfigControllerClients()
	factory := deployconfigcontroller.DeploymentConfigControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      c.EtcdHelper.Codec(),
	}
	controller := factory.Create()
	controller.Run()
}

// RunDeploymentConfigChangeController starts the deployment config change controller process.
func (c *MasterConfig) RunDeploymentConfigChangeController() {
	osclient, kclient := c.DeploymentConfigChangeControllerClients()
	factory := configchangecontroller.DeploymentConfigChangeControllerFactory{
		Client:     osclient,
		KubeClient: kclient,
		Codec:      c.EtcdHelper.Codec(),
	}
	controller := factory.Create()
	controller.Run()
}

// RunDeploymentImageChangeTriggerController starts the image change trigger controller process.
func (c *MasterConfig) RunDeploymentImageChangeTriggerController() {
	osclient := c.DeploymentImageChangeTriggerControllerClient()
	factory := imagechangecontroller.ImageChangeControllerFactory{Client: osclient}
	controller := factory.Create()
	controller.Run()
}

// RunSDNController runs openshift-sdn if the said network plugin is provided
func (c *MasterConfig) RunSDNController() {
	registry := osdn.NewOsdnRegistryInterface(c.SDNControllerClients())
	switch c.Options.NetworkConfig.NetworkPluginName {
	case flatsdn.NetworkPluginName():
		flatsdn.Master(registry, c.Options.NetworkConfig.ClusterNetworkCIDR, c.Options.NetworkConfig.HostSubnetLength, c.Options.NetworkConfig.ServiceNetworkCIDR)
	case multitenant.NetworkPluginName():
		multitenant.Master(registry, c.Options.NetworkConfig.ClusterNetworkCIDR, c.Options.NetworkConfig.HostSubnetLength, c.Options.NetworkConfig.ServiceNetworkCIDR)
	}
}

// RunImageImportController starts the image import trigger controller process.
func (c *MasterConfig) RunImageImportController() {
	osclient := c.ImageImportControllerClient()
	factory := imagecontroller.ImportControllerFactory{
		Client: osclient,
	}
	controller := factory.Create()
	controller.Run()
}

// RunSecurityAllocationController starts the security allocation controller process.
func (c *MasterConfig) RunSecurityAllocationController() {
	alloc := c.Options.ProjectConfig.SecurityAllocator
	if alloc == nil {
		glog.V(3).Infof("Security allocator is disabled - no UIDs assigned to projects")
		return
	}

	// TODO: move range initialization to run_config
	uidRange, err := uid.ParseRange(alloc.UIDAllocatorRange)

	if err != nil {
		glog.Fatalf("Unable to describe UID range: %v", err)
	}
	var etcdAlloc *etcdallocator.Etcd
	uidAllocator := uidallocator.New(uidRange, func(max int, rangeSpec string) allocator.Interface {
		mem := allocator.NewContiguousAllocationMap(max, rangeSpec)
		etcdAlloc = etcdallocator.NewEtcd(mem, "/ranges/uids", "uidallocation", c.EtcdHelper)
		return etcdAlloc
	})
	mcsRange, err := mcs.ParseRange(alloc.MCSAllocatorRange)
	if err != nil {
		glog.Fatalf("Unable to describe MCS category range: %v", err)
	}

	kclient := c.SecurityAllocationControllerClient()

	repair := securitycontroller.NewRepair(time.Minute, kclient.Namespaces(), uidRange, etcdAlloc)
	if err := repair.RunOnce(); err != nil {
		// TODO: v scary, may need to use direct etcd calls?
		// If the security controller fails during RunOnce it could mean a
		// couple of things:
		// 1. an unexpected etcd error occurred getting an allocator or the namespaces
		// 2. the allocation blocks were full - would result in an admission controller that is unable
		//	  to create the strategies correctly which would likely mean that the cluster
		//	  would not admit pods the the majority of users.
		// 3. an unexpected error persisting an allocation for a namespace has occurred - same as above
		// In all cases we do not want to continue normal operations, this should be fatal.
		glog.Fatalf("Unable to initialize namespaces: %v", err)
	}

	factory := securitycontroller.AllocationFactory{
		UIDAllocator: uidAllocator,
		MCSAllocator: securitycontroller.DefaultMCSAllocation(uidRange, mcsRange, alloc.MCSLabelsPerProject),
		Client:       kclient.Namespaces(),
		// TODO: reuse namespace cache
	}
	controller := factory.Create()
	controller.Run()
}

// RunGroupCache starts the group cache
func (c *MasterConfig) RunGroupCache() {
	c.GroupCache.Run()
}
