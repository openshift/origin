package origin

import (
	"net"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/registry/core/service/allocator"
	etcdallocator "k8s.io/kubernetes/pkg/registry/core/service/allocator/storage"

	"github.com/openshift/origin/pkg/authorization/controller/authorizationsync"
	osclient "github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	deployclient "github.com/openshift/origin/pkg/deploy/generated/internalclientset/typed/apps/internalversion"
	"github.com/openshift/origin/pkg/dns"
	quota "github.com/openshift/origin/pkg/quota"
	quotacontroller "github.com/openshift/origin/pkg/quota/controller"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotareconciliation"
	sdnplugin "github.com/openshift/origin/pkg/sdn/plugin"
	securitycontroller "github.com/openshift/origin/pkg/security/controller"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"
	"github.com/openshift/origin/pkg/service/controller/ingressip"
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

	services, err := dns.NewCachedServiceAccessor(c.InternalKubeInformers.Core().InternalVersion().Services())
	if err != nil {
		glog.Fatalf("Could not start DNS: failed to add ClusterIP index: %v", err)
	}

	go func() {
		s := dns.NewServer(
			config,
			services,
			c.InternalKubeInformers.Core().InternalVersion().Endpoints().Lister(),
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

// RunSDNController runs openshift-sdn if the said network plugin is provided
func (c *MasterConfig) RunSDNController() {
	oClient, kClient := c.SDNControllerClients()
	if err := sdnplugin.StartMaster(c.Options.NetworkConfig, oClient, kClient, c.InternalKubeInformers); err != nil {
		glog.Fatalf("SDN initialization failed: %v", err)
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
		c.ExternalKubeInformers.Core().V1().Namespaces(),
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
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(c.ExternalKubeInformers, c.ImageInformers.Image().InternalVersion().ImageStreams(), osClient, kClientExternal)
	resourceQuotaControllerOptions := &kresourcequota.ResourceQuotaControllerOptions{
		KubeClient:                kClientExternal,
		ResourceQuotaInformer:     c.ExternalKubeInformers.Core().V1().ResourceQuotas(),
		ResyncPeriod:              controller.StaticResyncPeriodFunc(resourceQuotaSyncPeriod),
		Registry:                  resourceQuotaRegistry,
		GroupKindsToReplenish:     quota.AllEvaluatedGroupKinds,
		ControllerFactory:         quotacontroller.NewAllResourceReplenishmentControllerFactory(c.ExternalKubeInformers, c.ImageInformers.Image().InternalVersion().ImageStreams(), osClient),
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
	resourceQuotaRegistry := quota.NewAllResourceQuotaRegistry(c.ExternalKubeInformers, c.ImageInformers.Image().InternalVersion().ImageStreams(), osClient, kClientExternal)
	groupKindsToReplenish := quota.AllEvaluatedGroupKinds

	options := clusterquotareconciliation.ClusterQuotaReconcilationControllerOptions{
		ClusterQuotaInformer: c.QuotaInformers.Quota().InternalVersion().ClusterResourceQuotas(),
		ClusterQuotaMapper:   c.ClusterQuotaMappingController.GetClusterQuotaMapper(),
		ClusterQuotaClient:   osClient,

		Registry:                  resourceQuotaRegistry,
		ResyncPeriod:              defaultResourceQuotaSyncPeriod,
		ControllerFactory:         quotacontroller.NewAllResourceReplenishmentControllerFactory(c.ExternalKubeInformers, c.ImageInformers.Image().InternalVersion().ImageStreams(), osClient),
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
		c.ExternalKubeInformers.Core().V1().Services().Informer(),
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
		c.InternalKubeInformers.Rbac().InternalVersion().ClusterRoles(),
		c.AuthorizationInformers.Authorization().InternalVersion().ClusterPolicies(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go clusterRoles.Run(5, utilwait.NeverStop)
	clusterRoleBindings := authorizationsync.NewOriginToRBACClusterRoleBindingController(
		c.InternalKubeInformers.Rbac().InternalVersion().ClusterRoleBindings(),
		c.AuthorizationInformers.Authorization().InternalVersion().ClusterPolicyBindings(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go clusterRoleBindings.Run(5, utilwait.NeverStop)

	roles := authorizationsync.NewOriginToRBACRoleController(
		c.InternalKubeInformers.Rbac().InternalVersion().Roles(),
		c.AuthorizationInformers.Authorization().InternalVersion().Policies(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go roles.Run(5, utilwait.NeverStop)
	roleBindings := authorizationsync.NewOriginToRBACRoleBindingController(
		c.InternalKubeInformers.Rbac().InternalVersion().RoleBindings(),
		c.AuthorizationInformers.Authorization().InternalVersion().PolicyBindings(),
		c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	)
	go roleBindings.Run(5, utilwait.NeverStop)
}
