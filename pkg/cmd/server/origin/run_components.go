package origin

import (
	"fmt"
	"io/ioutil"
	"net"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/cert"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kappsv1beta1 "k8s.io/kubernetes/pkg/apis/apps/v1beta1"
	kextensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/registry/core/service/allocator"
	etcdallocator "k8s.io/kubernetes/pkg/registry/core/service/allocator/storage"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/authorization/controller/authorizationsync"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildpodcontroller "github.com/openshift/origin/pkg/build/controller/buildpod"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	deployclient "github.com/openshift/origin/pkg/deploy/generated/internalclientset/typed/apps/internalversion"
	"github.com/openshift/origin/pkg/dns"
	imagecontroller "github.com/openshift/origin/pkg/image/controller"
	imagetriggercontroller "github.com/openshift/origin/pkg/image/controller/trigger"
	triggerannotations "github.com/openshift/origin/pkg/image/trigger/annotations"
	triggerbuildconfigs "github.com/openshift/origin/pkg/image/trigger/buildconfigs"
	triggerdeploymentconfigs "github.com/openshift/origin/pkg/image/trigger/deploymentconfigs"
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
	kclient := c.OriginNamespaceControllerClient()
	controller := projectcontroller.NewProjectFinalizerController(c.Informers.InternalKubernetesInformers().Core().InternalVersion().Namespaces(), kclient)
	go controller.Run(utilwait.NeverStop, 5)
}

// RunServiceAccountsController starts the service account controller
func (c *MasterConfig) RunServiceAccountsController() {
	if len(c.Options.ServiceAccountConfig.ManagedNames) == 0 {
		glog.Infof("Skipped starting Service Account Manager, no managed names specified")
		return
	}
	options := sacontroller.DefaultServiceAccountsControllerOptions()
	options.ServiceAccounts = []kapiv1.ServiceAccount{}

	for _, saName := range c.Options.ServiceAccountConfig.ManagedNames {
		sa := kapiv1.ServiceAccount{}
		sa.Name = saName

		options.ServiceAccounts = append(options.ServiceAccounts, sa)
	}

	//REBASE: add new args to NewServiceAccountsController
	controller := sacontroller.NewServiceAccountsController(
		c.Informers.KubernetesInformers().Core().V1().ServiceAccounts(),
		c.Informers.KubernetesInformers().Core().V1().Namespaces(),
		c.KubeClientsetExternal(),
		options,
	)
	go controller.Run(1, utilwait.NeverStop)
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
		if _, err := cert.ParseCertsPEM(rootCA); err != nil {
			glog.Fatalf("Error parsing master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
	}
	servingServingCABundle := []byte{}
	if c.Options.ControllerConfig.ServiceServingCert.Signer != nil && len(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		servingServingCA, err := ioutil.ReadFile(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile)
		if err != nil {
			glog.Fatalf("Error reading ca file for Service Serving Certificate Signer: %s: %v", c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile, err)
		}
		if _, err := crypto.CertsFromPEM(servingServingCA); err != nil {
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

	controller := sacontroller.NewTokensController(
		c.Informers.KubernetesInformers().Core().V1().ServiceAccounts(),
		c.Informers.KubernetesInformers().Core().V1().Secrets(),
		c.KubeClientsetExternal(),
		options,
	)
	go controller.Run(int(cm.ConcurrentSATokenSyncs), utilwait.NeverStop)
}

// RunServiceAccountPullSecretsControllers starts the service account pull secret controllers
func (c *MasterConfig) RunServiceAccountPullSecretsControllers() {
	go serviceaccountcontrollers.NewDockercfgDeletedController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Secrets(),
		c.KubeClientsetInternal(),
		serviceaccountcontrollers.DockercfgDeletedControllerOptions{},
	).Run(utilwait.NeverStop)
	go serviceaccountcontrollers.NewDockercfgTokenDeletedController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Secrets(),
		c.KubeClientsetInternal(),
		serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{},
	).Run(utilwait.NeverStop)

	dockerURLsInitialized := make(chan struct{})
	dockercfgController := serviceaccountcontrollers.NewDockercfgController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().ServiceAccounts(),
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Secrets(),
		c.KubeClientsetInternal(),
		serviceaccountcontrollers.DockercfgControllerOptions{DockerURLsInitialized: dockerURLsInitialized},
	)
	go dockercfgController.Run(5, utilwait.NeverStop)

	dockerRegistryControllerOptions := serviceaccountcontrollers.DockerRegistryServiceControllerOptions{
		RegistryNamespace:     "default",
		RegistryServiceName:   "docker-registry",
		DockercfgController:   dockercfgController,
		DockerURLsInitialized: dockerURLsInitialized,
	}
	go serviceaccountcontrollers.NewDockerRegistryServiceController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Secrets(),
		c.KubeClientsetInternal(),
		dockerRegistryControllerOptions,
	).Run(10, make(chan struct{}))
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

	services, err := dns.NewCachedServiceAccessor(c.Informers.InternalKubernetesInformers().Core().InternalVersion().Services())
	if err != nil {
		glog.Fatalf("Could not start DNS: failed to add ClusterIP index: %v", err)
	}

	go func() {
		s := dns.NewServer(
			config,
			services,
			c.Informers.InternalKubernetesInformers().Core().InternalVersion().Endpoints().Lister(),
			"apiserver",
		)
		err = s.ListenAndServe()
		glog.Fatalf("Could not start DNS: %v", err)
	}()

	cmdutil.WaitForSuccessfulDial(false, "tcp", c.Options.DNSConfig.BindAddress, 100*time.Millisecond, 100*time.Millisecond, 100)

	glog.Infof("DNS listening at %s", c.Options.DNSConfig.BindAddress)
}

// RunProjectCache populates project cache, used by scheduler and project admission controller.
func (c *MasterConfig) RunProjectCache() {
	glog.Infof("Using default project node label selector: %s", c.Options.ProjectConfig.DefaultNodeSelector)
	go c.ProjectCache.Run(utilwait.NeverStop)
}

// RunBuildPodController starts the build/pod status sync loop for build status
func (c *MasterConfig) RunBuildPodController() {
	buildInfomer := c.Informers.Builds().Informer()
	podInformer := c.Informers.InternalKubernetesInformers().Core().InternalVersion().Pods()
	osclient, kclientInternal, kclientExternal := c.BuildPodControllerClients()

	controller := buildpodcontroller.NewBuildPodController(buildInfomer, podInformer, kclientInternal, kclientExternal, osclient)
	go controller.Run(5, utilwait.NeverStop)
}

// RunBuildConfigChangeController starts the build config change trigger controller process.
func (c *MasterConfig) RunBuildConfigChangeController() {
	bcClient, internalKubeClientset, externalKubeClientset := c.BuildConfigChangeControllerClients()
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(bcClient)
	factory := buildcontrollerfactory.BuildConfigControllerFactory{
		Client:                  bcClient,
		KubeClient:              internalKubeClientset,
		ExternalKubeClient:      externalKubeClientset,
		BuildConfigInstantiator: bcInstantiator,
		BuildLister:             buildclient.NewOSClientBuildClient(bcClient),
		BuildConfigGetter:       buildclient.NewOSClientBuildConfigClient(bcClient),
		BuildDeleter:            buildclient.NewBuildDeleter(bcClient),
	}
	factory.Create().Run()
}

// TODO: remove when generated informers exist
type temporaryLister struct {
	*oscache.StoreToImageStreamLister
}

func (l temporaryLister) ImageStreams(namespace string) imagetriggercontroller.ImageStreamNamespaceLister {
	return l.StoreToImageStreamLister.ImageStreams(namespace)
}

type podSpecUpdater struct {
	kclient kclientsetexternal.Interface
}

func (u podSpecUpdater) Update(obj runtime.Object) error {
	switch t := obj.(type) {
	case *kextensionsv1beta1.DaemonSet:
		_, err := u.kclient.Extensions().DaemonSets(t.Namespace).Update(t)
		return err
	case *kappsv1beta1.Deployment:
		_, err := u.kclient.Apps().Deployments(t.Namespace).Update(t)
		return err
	case *kappsv1beta1.StatefulSet:
		_, err := u.kclient.Apps().StatefulSets(t.Namespace).Update(t)
		return err
	case *kapiv1.Pod:
		_, err := u.kclient.Core().Pods(t.Namespace).Update(t)
		return err
	default:
		return fmt.Errorf("unrecognized object - no trigger update possible for %T", obj)
	}
}

func (c *MasterConfig) RunImageTriggerController() {
	streamInformer := c.Informers.ImageStreams().Informer()
	lister := temporaryLister{c.Informers.ImageStreams().Lister()}

	oclient, _, kclient := c.ImageTriggerControllerClients()
	updater := podSpecUpdater{kclient}
	bcInstantiator := buildclient.NewOSClientBuildConfigInstantiatorClient(oclient)
	broadcaster := imagetriggercontroller.NewTriggerEventBroadcaster(kv1core.New(kclient.CoreV1().RESTClient()))

	sources := []imagetriggercontroller.TriggerSource{
		{
			Resource:  schema.GroupResource{Group: "apps.openshift.io", Resource: "deploymentconfigs"},
			Informer:  c.Informers.DeploymentConfigs().Informer(),
			Store:     c.Informers.DeploymentConfigs().Indexer(),
			TriggerFn: triggerdeploymentconfigs.NewDeploymentConfigTriggerIndexer,
			Reactor:   &triggerdeploymentconfigs.DeploymentConfigReactor{Client: oclient},
		},
	}
	if !c.Options.DisabledFeatures.Has(configapi.FeatureBuilder) {
		sources = append(sources, imagetriggercontroller.TriggerSource{
			Resource:  schema.GroupResource{Group: "build.openshift.io", Resource: "buildconfigs"},
			Informer:  c.Informers.BuildConfigs().Informer(),
			Store:     c.Informers.BuildConfigs().Indexer(),
			TriggerFn: triggerbuildconfigs.NewBuildConfigTriggerIndexer,
			Reactor:   &triggerbuildconfigs.BuildConfigReactor{Instantiator: bcInstantiator},
		})
	}
	if !c.Options.DisabledFeatures.Has("triggers.image.openshift.io/deployments") {
		sources = append(sources, imagetriggercontroller.TriggerSource{
			Resource:  schema.GroupResource{Group: "extensions", Resource: "deployments"},
			Informer:  c.Informers.KubernetesInformers().Apps().V1beta1().Deployments().Informer(),
			Store:     c.Informers.KubernetesInformers().Apps().V1beta1().Deployments().Informer().GetIndexer(),
			TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
			Reactor:   &triggerannotations.AnnotationReactor{Updater: updater, Copier: kapi.Scheme},
		})
	}
	if !c.Options.DisabledFeatures.Has("triggers.image.openshift.io/daemonsets") {
		sources = append(sources, imagetriggercontroller.TriggerSource{
			Resource:  schema.GroupResource{Group: "extensions", Resource: "daemonsets"},
			Informer:  c.Informers.KubernetesInformers().Extensions().V1beta1().DaemonSets().Informer(),
			Store:     c.Informers.KubernetesInformers().Extensions().V1beta1().DaemonSets().Informer().GetIndexer(),
			TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
			Reactor:   &triggerannotations.AnnotationReactor{Updater: updater, Copier: kapi.Scheme},
		})
	}
	if !c.Options.DisabledFeatures.Has("triggers.image.openshift.io/statefulsets") {
		sources = append(sources, imagetriggercontroller.TriggerSource{
			Resource:  schema.GroupResource{Group: "apps", Resource: "statefulsets"},
			Informer:  c.Informers.KubernetesInformers().Apps().V1beta1().StatefulSets().Informer(),
			Store:     c.Informers.KubernetesInformers().Apps().V1beta1().StatefulSets().Informer().GetIndexer(),
			TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
			Reactor:   &triggerannotations.AnnotationReactor{Updater: updater, Copier: kapi.Scheme},
		})
	}
	if !c.Options.DisabledFeatures.Has("triggers.image.openshift.io/cronjobs") {
		sources = append(sources, imagetriggercontroller.TriggerSource{
			Resource:  schema.GroupResource{Group: "batch", Resource: "cronjobs"},
			Informer:  c.Informers.KubernetesInformers().Batch().V2alpha1().CronJobs().Informer(),
			Store:     c.Informers.KubernetesInformers().Batch().V2alpha1().CronJobs().Informer().GetIndexer(),
			TriggerFn: triggerannotations.NewAnnotationTriggerIndexer,
			Reactor:   &triggerannotations.AnnotationReactor{Updater: updater, Copier: kapi.Scheme},
		})
	}

	trigger := imagetriggercontroller.NewTriggerController(
		broadcaster,
		streamInformer,
		lister,
		sources...,
	)
	go trigger.Run(5, utilwait.NeverStop)
}

// RunSDNController runs openshift-sdn if the said network plugin is provided
func (c *MasterConfig) RunSDNController() {
	oClient, kClient := c.SDNControllerClients()
	if err := sdnplugin.StartMaster(c.Options.NetworkConfig, oClient, kClient, c.Informers); err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
	}
}

func (c *MasterConfig) RunServiceServingCertController(client kclientsetinternal.Interface) {
	if c.Options.ControllerConfig.ServiceServingCert.Signer == nil {
		return
	}
	ca, err := crypto.GetCA(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile, c.Options.ControllerConfig.ServiceServingCert.Signer.KeyFile, "")
	if err != nil {
		glog.Fatalf("service serving cert controller failed: %v", err)
	}

	servingCertController := servingcertcontroller.NewServiceServingCertController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Services(),
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Secrets(),
		client.Core(), client.Core(),
		ca, "cluster.local", 2*time.Minute,
	)
	go servingCertController.Run(1, make(chan struct{}))

	servingCertUpdateController := servingcertcontroller.NewServiceServingCertUpdateController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Services(),
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Secrets(),
		client.Core(),
		ca, "cluster.local", 20*time.Minute,
	)
	go servingCertUpdateController.Run(5, make(chan struct{}))
}

// RunImageImportController starts the image import trigger controller process.
func (c *MasterConfig) RunImageImportController() {
	controller := imagecontroller.NewImageStreamController(c.ImageImportControllerClient(), c.Informers.ImageStreams())
	scheduledController := imagecontroller.NewScheduledImageStreamController(c.ImageImportControllerClient(), c.Informers.ImageStreams(), imagecontroller.ScheduledImageStreamControllerOptions{
		Resync: time.Duration(c.Options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds) * time.Second,

		Enabled:                  !c.Options.ImagePolicyConfig.DisableScheduledImport,
		DefaultBucketSize:        4, // TODO: Make this configurable?
		MaxImageImportsPerMinute: c.Options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
	})

	// Setup notifier on the main controller so that it informs the scheduled controller when streams are being imported
	controller.SetNotifier(scheduledController)

	// TODO align with https://github.com/openshift/origin/pull/13579 once it merges
	stopCh := make(chan struct{})
	go controller.Run(5, stopCh)
	if c.Options.ImagePolicyConfig.DisableScheduledImport {
		glog.V(2).Infof("Scheduled image import is disabled - the 'scheduled' flag on image streams will be ignored")
		return
	}

	go scheduledController.Run(stopCh)
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

	opts, err := c.RESTOptionsGetter.GetRESTOptions(schema.GroupResource{Resource: "securityuidranges"})
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

	repair := securitycontroller.NewRepair(time.Minute, kclient.Core().Namespaces(), uidRange, etcdAlloc)
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

	controller := securitycontroller.NewNamespaceSecurityDefaultsController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Namespaces(),
		kclient.Core().Namespaces(),
		uidAllocator,
		securitycontroller.DefaultMCSAllocation(uidRange, mcsRange, alloc.MCSLabelsPerProject),
	)
	// TODO: scale out
	go controller.Run(utilwait.NeverStop, 1)
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

	osClient, _, kClientExternal := c.ResourceQuotaManagerClients()
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(c.Informers, osClient, kClientExternal)
	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		KubeClient:                kClientExternal,
		ResourceQuotaInformer:     c.Informers.KubernetesInformers().Core().V1().ResourceQuotas(),
		ResyncPeriod:              controller.StaticResyncPeriodFunc(resourceQuotaSyncPeriod),
		Registry:                  resourceQuotaRegistry,
		GroupKindsToReplenish:     quota.AllEvaluatedGroupKinds,
		ControllerFactory:         quotacontroller.NewAllResourceReplenishmentControllerFactory(c.Informers, osClient),
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
	osClient, _, kClientExternal := c.ResourceQuotaManagerClients()
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(c.Informers, osClient, kClientExternal)
	groupKindsToReplenish := quota.AllEvaluatedGroupKinds

	options := clusterquotareconciliation.ClusterQuotaReconcilationControllerOptions{
		ClusterQuotaInformer: c.Informers.ClusterResourceQuotas(),
		ClusterQuotaMapper:   c.ClusterQuotaMappingController.GetClusterQuotaMapper(),
		ClusterQuotaClient:   osClient,

		Registry:                  resourceQuotaRegistry,
		ResyncPeriod:              defaultResourceQuotaSyncPeriod,
		ControllerFactory:         quotacontroller.NewAllResourceReplenishmentControllerFactory(c.Informers, osClient),
		ReplenishmentResyncPeriod: controller.StaticResyncPeriodFunc(defaultReplenishmentSyncPeriod),
		GroupKindsToReplenish:     groupKindsToReplenish,
	}
	controller := clusterquotareconciliation.NewClusterQuotaReconcilationController(options)
	c.ClusterQuotaMappingController.GetClusterQuotaMapper().AddListener(controller)
	go controller.Run(5, utilwait.NeverStop)
}

// RunIngressIPController starts the ingress ip controller if IngressIPNetworkCIDR is configured.
func (c *MasterConfig) RunIngressIPController(internalKubeClientset kclientsetinternal.Interface, externalKubeClientset kclientsetexternal.Interface) {
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
	ingressIPController := ingressip.NewIngressIPController(
		c.Informers.InternalKubernetesInformers().Core().InternalVersion().Services().Informer(),
		internalKubeClientset,
		externalKubeClientset,
		ipNet,
		defaultIngressIPSyncPeriod,
	)
	go ingressIPController.Run(utilwait.NeverStop)
}

// RunUnidlingController starts the unidling controller
func (c *MasterConfig) RunUnidlingController() {
	oc, kc, extensionsClient := c.UnidlingControllerClients()
	resyncPeriod := 2 * time.Hour
	scaleNamespacer := osclient.NewDelegatingScaleNamespacer(oc, extensionsClient)
	dcCoreClient := deployclient.New(oc.RESTClient)
	cont := unidlingcontroller.NewUnidlingController(scaleNamespacer, kc.Core(), kc.Core(), dcCoreClient, kc.Core(), resyncPeriod)

	cont.Run(utilwait.NeverStop)
}

func (c *MasterConfig) RunOriginToRBACSyncControllers() {
	clusterRoles := authorizationsync.NewOriginToRBACClusterRoleController(
		c.Informers.InternalKubernetesInformers().Rbac().InternalVersion().ClusterRoles(),
		c.AuthorizationInformers.Authorization().InternalVersion().ClusterPolicies(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go clusterRoles.Run(5, utilwait.NeverStop)
	clusterRoleBindings := authorizationsync.NewOriginToRBACClusterRoleBindingController(
		c.Informers.InternalKubernetesInformers().Rbac().InternalVersion().ClusterRoleBindings(),
		c.AuthorizationInformers.Authorization().InternalVersion().ClusterPolicyBindings(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go clusterRoleBindings.Run(5, utilwait.NeverStop)

	roles := authorizationsync.NewOriginToRBACRoleController(
		c.Informers.InternalKubernetesInformers().Rbac().InternalVersion().Roles(),
		c.AuthorizationInformers.Authorization().InternalVersion().Policies(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go roles.Run(5, utilwait.NeverStop)
	roleBindings := authorizationsync.NewOriginToRBACRoleBindingController(
		c.Informers.InternalKubernetesInformers().Rbac().InternalVersion().RoleBindings(),
		c.AuthorizationInformers.Authorization().InternalVersion().PolicyBindings(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go roleBindings.Run(5, utilwait.NeverStop)
}
