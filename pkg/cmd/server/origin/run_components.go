package origin

import (
	"io/ioutil"
	"net"
	"path"
	"sync"
	"time"

	"github.com/golang/glog"

	deployclient "github.com/openshift/origin/pkg/deploy/client/clientset_generated/internalclientset/typed/core/unversioned"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/registry/service/allocator"
	etcdallocator "k8s.io/kubernetes/pkg/registry/service/allocator/etcd"
	"k8s.io/kubernetes/pkg/serviceaccount"
	kcrypto "k8s.io/kubernetes/pkg/util/crypto"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	builddefaults "github.com/openshift/origin/pkg/build/admission/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/admission/overrides"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	cmdadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/controller/shared"
	deploycontroller "github.com/openshift/origin/pkg/deploy/controller/deployment"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	triggercontroller "github.com/openshift/origin/pkg/deploy/controller/generictrigger"
	"github.com/openshift/origin/pkg/dns"
	imagecontroller "github.com/openshift/origin/pkg/image/controller"
	projectcontroller "github.com/openshift/origin/pkg/project/controller"
	quota "github.com/openshift/origin/pkg/quota"
	quotacontroller "github.com/openshift/origin/pkg/quota/controller"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotareconciliation"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
	securitycontroller "github.com/openshift/origin/pkg/security/controller"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"
	"github.com/openshift/origin/pkg/service/controller/ingressip"
	servingcertcontroller "github.com/openshift/origin/pkg/service/controller/servingcert"
	serviceaccountcontrollers "github.com/openshift/origin/pkg/serviceaccounts/controllers"
	unidlingcontroller "github.com/openshift/origin/pkg/unidling/controller"
)

const (
	defaultConcurrentResourceQuotaSyncs int           = 5
	defaultResourceQuotaSyncPeriod      time.Duration = 5 * time.Minute

	// from CMServer MinResyncPeriod
	defaultReplenishmentSyncPeriod time.Duration = 12 * time.Hour

	defaultIngressIPSyncPeriod time.Duration = 10 * time.Minute
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
	options := sacontroller.DefaultServiceAccountsControllerOptions()
	options.ServiceAccounts = []kapi.ServiceAccount{}

	for _, saName := range c.Options.ServiceAccountConfig.ManagedNames {
		sa := kapi.ServiceAccount{}
		sa.Name = saName

		options.ServiceAccounts = append(options.ServiceAccounts, sa)
	}

	sacontroller.NewServiceAccountsController(clientadapter.FromUnversionedClient(c.KubeClient()), options).Run()
}

// RunServiceAccountTokensController starts the service account token controller
func (c *MasterConfig) RunServiceAccountTokensController(cm *cmapp.CMServer) {
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
		if _, err := kcrypto.CertsFromPEM(rootCA); err != nil {
			glog.Fatalf("Error parsing master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
	}
	servingServingCABundle := []byte{}
	if c.Options.ControllerConfig.ServiceServingCert.Signer != nil && len(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		servingServingCA, err := ioutil.ReadFile(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile)
		if err != nil {
			glog.Fatalf("Error reading ca file for Service Serving Certificate Signer: %s: %v", c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile, err)
		}
		if _, err := kcrypto.CertsFromPEM(servingServingCA); err != nil {
			glog.Fatalf("Error parsing ca file for Service Serving Certificate Signer: %s: %v", c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile, err)
		}

		// if we have a rootCA bundle add that too.  The rootCA will be used when hitting the default master service, since those are signed
		// using a different CA by default.  The rootCA's key is more closely guarded than ours and if it is compromised, that power could
		// be used to change the trusted signers for every pod anyway, so we're already effectively trusting it.
		if len(rootCA) > 0 {
			servingServingCABundle = append(servingServingCABundle, rootCA...)
			servingServingCABundle = append(servingServingCABundle, []byte("\n")...)
		}
		servingServingCABundle = append(servingServingCABundle, servingServingCA...)
	}

	options := sacontroller.TokensControllerOptions{
		TokenGenerator:   serviceaccount.JWTTokenGenerator(privateKey),
		RootCA:           rootCA,
		ServiceServingCA: servingServingCABundle,
	}

	go sacontroller.NewTokensController(clientadapter.FromUnversionedClient(c.KubeClient()), options).Run(int(cm.ConcurrentSATokenSyncs), utilwait.NeverStop)
}

// RunServiceAccountPullSecretsControllers starts the service account pull secret controllers
func (c *MasterConfig) RunServiceAccountPullSecretsControllers() {
	serviceaccountcontrollers.NewDockercfgDeletedController(c.KubeClient(), serviceaccountcontrollers.DockercfgDeletedControllerOptions{}).Run()
	serviceaccountcontrollers.NewDockercfgTokenDeletedController(c.KubeClient(), serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{}).Run()

	dockerURLsIntialized := make(chan struct{})
	dockercfgController := serviceaccountcontrollers.NewDockercfgController(c.KubeClient(), serviceaccountcontrollers.DockercfgControllerOptions{DockerURLsIntialized: dockerURLsIntialized})
	go dockercfgController.Run(5, utilwait.NeverStop)

	dockerRegistryControllerOptions := serviceaccountcontrollers.DockerRegistryServiceControllerOptions{
		RegistryNamespace:    "default",
		RegistryServiceName:  "docker-registry",
		DockercfgController:  dockercfgController,
		DockerURLsIntialized: dockerURLsIntialized,
	}
	go serviceaccountcontrollers.NewDockerRegistryServiceController(c.KubeClient(), dockerRegistryControllerOptions).Run(10, make(chan struct{}))
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
	switch c.Options.DNSConfig.BindNetwork {
	case "tcp":
		config.BindNetwork = "ip"
	case "tcp4":
		config.BindNetwork = "ipv4"
	case "tcp6":
		config.BindNetwork = "ipv6"
	}
	config.DnsAddr = c.Options.DNSConfig.BindAddress
	config.NoRec = !c.Options.DNSConfig.AllowRecursiveQueries

	_, port, err := net.SplitHostPort(c.Options.DNSConfig.BindAddress)
	if err != nil {
		glog.Fatalf("Could not start DNS: %v", err)
	}
	if port != "53" {
		glog.Warningf("Binding DNS on port %v instead of 53, which may not be resolvable from all clients", port)
	}

	if ok, err := cmdutil.TryListen(c.Options.DNSConfig.BindNetwork, c.Options.DNSConfig.BindAddress); !ok {
		glog.Warningf("Could not start DNS: %v", err)
		return
	}

	go func() {
		s := dns.NewServer(config, c.DNSServerClient())
		s.MetricsName = "apiserver"
		err := s.ListenAndServe()
		glog.Fatalf("Could not start DNS: %v", err)
	}()

	cmdutil.WaitForSuccessfulDial(false, "tcp", c.Options.DNSConfig.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("DNS listening at %s", c.Options.DNSConfig.BindAddress)
}

// RunProjectCache populates project cache, used by scheduler and project admission controller.
func (c *MasterConfig) RunProjectCache() {
	glog.Infof("Using default project node label selector: %s", c.Options.ProjectConfig.DefaultNodeSelector)
	c.ProjectCache.Run()
}

// RunBuildController starts the build sync loop for builds and buildConfig processing.
func (c *MasterConfig) RunBuildController(informers shared.InformerFactory) error {
	// initialize build controller
	dockerImage := c.ImageFor("docker-builder")
	stiImage := c.ImageFor("sti-builder")

	storageVersion := c.Options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := unversioned.GroupVersion{Group: "", Version: storageVersion}
	codec := kapi.Codecs.LegacyCodec(groupVersion)

	admissionControl := admission.InitPlugin("SecurityContextConstraint", clientadapter.FromUnversionedClient(c.PrivilegedLoopbackKubernetesClient), "")
	if wantsInformers, ok := admissionControl.(cmdadmission.WantsInformers); ok {
		wantsInformers.SetInformers(informers)
	}

	buildDefaults, err := builddefaults.NewBuildDefaults(c.Options.AdmissionConfig.PluginConfig)
	if err != nil {
		return err
	}
	buildOverrides, err := buildoverrides.NewBuildOverrides(c.Options.AdmissionConfig.PluginConfig)
	if err != nil {
		return err
	}

	osclient, kclient := c.BuildControllerClients()
	factory := buildcontrollerfactory.BuildControllerFactory{
		KubeClient:   kclient,
		OSClient:     osclient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osclient),
		BuildLister:  buildclient.NewOSClientBuildClient(osclient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: dockerImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: codec,
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image: stiImage,
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec:            codec,
			AdmissionControl: admissionControl,
		},
		CustomBuildStrategy: &buildstrategy.CustomBuildStrategy{
			// TODO: this will be set to --storage-version (the internal schema we use)
			Codec: codec,
		},
		BuildDefaults:  buildDefaults,
		BuildOverrides: buildOverrides,
	}

	controller := factory.Create()
	controller.Run()
	deleteController := factory.CreateDeleteController()
	deleteController.Run()
	return nil
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
	bcIndex := &oscache.StoreToBuildConfigListerImpl{Indexer: c.Informers.BuildConfigs().Indexer()}
	bcIndexSynced := c.Informers.BuildConfigs().Informer().HasSynced
	factory := buildcontrollerfactory.ImageChangeControllerFactory{Client: bcClient, BuildConfigInstantiator: bcInstantiator, BuildConfigIndex: bcIndex, BuildConfigIndexSynced: bcIndexSynced}
	go func() {
		factory.Create().Run()
	}()
}

// RunBuildConfigChangeController starts the build config change trigger controller process.
func (c *MasterConfig) RunBuildConfigChangeController() {
	bcClient, kClient := c.BuildConfigChangeControllerClients()
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient)
	factory := buildcontrollerfactory.BuildConfigControllerFactory{
		Client:                  bcClient,
		KubeClient:              kClient,
		BuildConfigInstantiator: bcInstantiator,
	}
	factory.Create().Run()
}

// RunDeploymentController starts the deployment controller process.
func (c *MasterConfig) RunDeploymentController() {
	rcInformer := c.Informers.ReplicationControllers().Informer()
	podInformer := c.Informers.Pods().Informer()
	_, kclient := c.DeploymentControllerClients()

	_, kclientConfig, err := configapi.GetKubeClient(c.Options.MasterClients.OpenShiftLoopbackKubeConfig, c.Options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		glog.Fatalf("Unable to initialize deployment controller: %v", err)
	}
	// TODO eliminate these environment variables once service accounts provide a kubeconfig that includes all of this info
	env := clientcmd.EnvVars(
		kclientConfig.Host,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	)

	controller := deploycontroller.NewDeploymentController(
		rcInformer,
		podInformer,
		kclient,
		bootstrappolicy.DeployerServiceAccountName,
		c.ImageFor("deployer"),
		env,
		c.ExternalVersionCodec,
	)
	go controller.Run(5, utilwait.NeverStop)
}

// RunDeploymentConfigController starts the deployment config controller process.
func (c *MasterConfig) RunDeploymentConfigController() {
	dcInfomer := c.Informers.DeploymentConfigs().Informer()
	rcInformer := c.Informers.ReplicationControllers().Informer()
	podInformer := c.Informers.Pods().Informer()
	osclient, kclient := c.DeploymentConfigControllerClients()

	controller := deployconfigcontroller.NewDeploymentConfigController(dcInfomer, rcInformer, podInformer, osclient, kclient, c.ExternalVersionCodec)
	go controller.Run(5, utilwait.NeverStop)
}

// RunDeploymentTriggerController starts the deployment trigger controller process.
func (c *MasterConfig) RunDeploymentTriggerController() {
	dcInfomer := c.Informers.DeploymentConfigs().Informer()
	rcInformer := c.Informers.ReplicationControllers().Informer()
	streamInformer := c.Informers.ImageStreams().Informer()
	osclient := c.DeploymentTriggerControllerClient()

	controller := triggercontroller.NewDeploymentTriggerController(dcInfomer, rcInformer, streamInformer, osclient, c.ExternalVersionCodec)
	go controller.Run(5, utilwait.NeverStop)
}

// RunSDNController runs openshift-sdn if the said network plugin is provided
func (c *MasterConfig) RunSDNController() {
	oClient, kClient := c.SDNControllerClients()
	if err := sdnplugin.StartMaster(c.Options.NetworkConfig, oClient, kClient); err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func (c *MasterConfig) RunServiceServingCertController(client *kclient.Client) {
	if c.Options.ControllerConfig.ServiceServingCert.Signer == nil {
		return
	}
	ca, err := crypto.GetCA(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile, c.Options.ControllerConfig.ServiceServingCert.Signer.KeyFile, "")
	if err != nil {
		glog.Fatalf("service serving cert controller failed: %v", err)
	}

	servingCertController := servingcertcontroller.NewServiceServingCertController(client, client, ca, "cluster.local", 2*time.Minute)
	go servingCertController.Run(1, make(chan struct{}))
}

// RunImageImportController starts the image import trigger controller process.
func (c *MasterConfig) RunImageImportController() {
	osclient := c.ImageImportControllerClient()
	importRate := float32(c.Options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute) / float32(time.Minute/time.Second)
	importBurst := c.Options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute * 2
	factory := imagecontroller.ImportControllerFactory{
		Client:               osclient,
		ResyncInterval:       10 * time.Minute,
		MinimumCheckInterval: time.Duration(c.Options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds) * time.Second,
		ImportRateLimiter:    flowcontrol.NewTokenBucketRateLimiter(importRate, importBurst),
		ScheduleEnabled:      !c.Options.ImagePolicyConfig.DisableScheduledImport,
	}
	controller, scheduledController := factory.Create()
	controller.Run()
	if c.Options.ImagePolicyConfig.DisableScheduledImport {
		glog.V(2).Infof("Scheduled image import is disabled - the 'scheduled' flag on image streams will be ignored")
	} else {
		scheduledController.RunUntil(utilwait.NeverStop)
	}
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

	opts, err := c.RESTOptionsGetter.GetRESTOptions(unversioned.GroupResource{Resource: "securityuidranges"})
	if err != nil {
		glog.Fatalf("Unable to load storage options for security UID ranges")
	}

	var etcdAlloc *etcdallocator.Etcd
	uidAllocator := uidallocator.New(uidRange, func(max int, rangeSpec string) allocator.Interface {
		mem := allocator.NewContiguousAllocationMap(max, rangeSpec)
		etcdAlloc = etcdallocator.NewEtcd(mem, "/ranges/uids", kapi.Resource("uidallocation"), opts.StorageConfig)
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

// RunResourceQuotaManager starts resource quota controller for OpenShift resources
func (c *MasterConfig) RunResourceQuotaManager(cm *cmapp.CMServer) {
	concurrentResourceQuotaSyncs := defaultConcurrentResourceQuotaSyncs
	resourceQuotaSyncPeriod := defaultResourceQuotaSyncPeriod
	replenishmentSyncPeriodFunc := controller.StaticResyncPeriodFunc(defaultReplenishmentSyncPeriod)
	if cm != nil {
		// TODO: should these be part of os master config?
		concurrentResourceQuotaSyncs = int(cm.ConcurrentResourceQuotaSyncs)
		resourceQuotaSyncPeriod = cm.ResourceQuotaSyncPeriod.Duration
		replenishmentSyncPeriodFunc = kctrlmgr.ResyncPeriod(cm)
	}

	osClient, kClient := c.ResourceQuotaManagerClients()
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(osClient, kClient)
	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		KubeClient:                kClient,
		ResyncPeriod:              controller.StaticResyncPeriodFunc(resourceQuotaSyncPeriod),
		Registry:                  resourceQuotaRegistry,
		GroupKindsToReplenish:     quota.AllEvaluatedGroupKinds,
		ControllerFactory:         quotacontroller.NewAllResourceReplenishmentControllerFactory(c.Informers, osClient, kClient),
		ReplenishmentResyncPeriod: replenishmentSyncPeriodFunc,
	}
	go kresourcequota.NewResourceQuotaController(resourceQuotaControllerOptions).Run(concurrentResourceQuotaSyncs, utilwait.NeverStop)
}

var initClusterQuotaMapping sync.Once

func (c *MasterConfig) RunClusterQuotaMappingController() {
	initClusterQuotaMapping.Do(func() {
		go c.ClusterQuotaMappingController.Run(5, utilwait.NeverStop)
	})
}

func (c *MasterConfig) RunClusterQuotaReconciliationController() {
	osClient, kClient := c.ResourceQuotaManagerClients()
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(osClient, kClient)
	groupKindsToReplenish := quota.AllEvaluatedGroupKinds

	options := clusterquotareconciliation.ClusterQuotaReconcilationControllerOptions{
		ClusterQuotaInformer: c.Informers.ClusterResourceQuotas(),
		ClusterQuotaMapper:   c.ClusterQuotaMappingController.GetClusterQuotaMapper(),
		ClusterQuotaClient:   osClient,

		Registry:                  resourceQuotaRegistry,
		ResyncPeriod:              defaultResourceQuotaSyncPeriod,
		ControllerFactory:         quotacontroller.NewAllResourceReplenishmentControllerFactory(c.Informers, osClient, kClient),
		ReplenishmentResyncPeriod: controller.StaticResyncPeriodFunc(defaultReplenishmentSyncPeriod),
		GroupKindsToReplenish:     groupKindsToReplenish,
	}
	controller := clusterquotareconciliation.NewClusterQuotaReconcilationController(options)
	c.ClusterQuotaMappingController.GetClusterQuotaMapper().AddListener(controller)
	go controller.Run(5, utilwait.NeverStop)
}

// RunIngressIPController starts the ingress ip controller if IngressIPNetworkCIDR is configured.
func (c *MasterConfig) RunIngressIPController(client *kclient.Client) {
	if len(c.Options.NetworkConfig.IngressIPNetworkCIDR) == 0 {
		return
	}

	_, ipNet, err := net.ParseCIDR(c.Options.NetworkConfig.IngressIPNetworkCIDR)
	if err != nil {
		// should have been caught with validation
		glog.Fatalf("Unable to start ingress ip controller: %v", err)
	}
	if ipNet.IP.IsUnspecified() {
		return
	}
	ingressIPController := ingressip.NewIngressIPController(client, ipNet, defaultIngressIPSyncPeriod)
	go ingressIPController.Run(utilwait.NeverStop)
}

// RunUnidlingController starts the unidling controller
func (c *MasterConfig) RunUnidlingController() {
	oc, kc := c.UnidlingControllerClients()
	resyncPeriod := 2 * time.Hour
	scaleNamespacer := osclient.NewDelegatingScaleNamespacer(oc, kc)
	coreClient := clientadapter.FromUnversionedClient(kc).Core()
	dcCoreClient := deployclient.New(oc.RESTClient)
	cont := unidlingcontroller.NewUnidlingController(scaleNamespacer, coreClient, coreClient, dcCoreClient, coreClient, resyncPeriod)

	cont.Run(utilwait.NeverStop)
}
