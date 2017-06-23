package origin

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"

	kubernetes "github.com/openshift/origin/pkg/cmd/server/kubernetes/master"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/cert"
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	kapi "k8s.io/kubernetes/pkg/api"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/serviceaccount"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/server/origin/controller"
)

func (c *MasterConfig) NewKubernetesControllerInitalizers(kc *kubernetes.MasterConfig) (map[string]kubecontroller.InitFunc, error) {
	ret := map[string]kubecontroller.InitFunc{}

	persistentVolumeController := kubernetes.PersistentVolumeControllerConfig{
		RecyclerImage: c.ImageFor("recycler"),
		// TODO: In 3.7 this is renamed to 'Cloud' and is part of kubernetes ControllerContext
		CloudProvider: kc.CloudProvider,
	}
	ret["persistentvolume-binder"] = persistentVolumeController.RunController

	persistentVolumeAttachDetachController := kubernetes.PersistentVolumeAttachDetachControllerConfig{
		// TODO: In 3.7 this is renamed to 'Cloud' and is part of kubernetes ControllerContext
		CloudProvider: kc.CloudProvider,
	}
	ret["attachdetach"] = persistentVolumeAttachDetachController.RunController

	schedulerController := kubernetes.SchedulerControllerConfig{
		PrivilegedClient:               kc.KubeClient,
		SchedulerName:                  kc.SchedulerServer.SchedulerName,
		HardPodAffinitySymmetricWeight: int(kc.SchedulerServer.HardPodAffinitySymmetricWeight),
	}
	// TODO: Move this to origin controllers and replace the privileged client with SA
	// client.
	if _, err := os.Stat(kc.Options.SchedulerConfigFile); err == nil {
		policy := schedulerapi.Policy{}
		configData, err := ioutil.ReadFile(kc.SchedulerServer.PolicyConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read scheduler config: %v", err)
		}
		if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
			return nil, fmt.Errorf("invalid scheduler configuration: %v", err)
		}
		schedulerController.SchedulerPolicy = &policy
	}
	// FIXME: Move this under openshift controller intialization once we figure out
	// deployment (options).
	ret["openshift.io/scheduler"] = schedulerController.RunController

	nodeController := kubernetes.NodeControllerConfig{
		// TODO: In 3.7 this is renamed to 'Cloud' and is part of kubernetes ControllerContext
		CloudProvider: kc.CloudProvider,
	}
	ret["node"] = nodeController.RunController

	serviceLoadBalancerController := kubernetes.ServiceLoadBalancerControllerConfig{
		// TODO: In 3.7 this is renamed to 'Cloud' and is part of kubernetes ControllerContext
		CloudProvider: kc.CloudProvider,
	}
	ret["service"] = serviceLoadBalancerController.RunController

	// Add kubernetes controller, the override above takes priority.
	for name, initFn := range kubecontroller.NewControllerInitializers() {
		if _, ok := ret[name]; ok {
			continue
		}
		// This overrides the upstream controller because we have to add origin types.
		if name == "resourcequota" {
			continue
		}
		ret[name] = initFn
	}

	return ret, nil
}

// NewOpenShiftControllerPreStartInitializers returns list of initializers for controllers
// that needed to be run before any other controller is started.
// Typically this has to done for the serviceaccount-token controller as it provides
// tokens to other controllers.
func (c *MasterConfig) NewOpenShiftControllerPreStartInitializers() (map[string]controller.InitFunc, error) {
	ret := map[string]controller.InitFunc{}

	saToken := controller.ServiceAccountTokenControllerOptions{
		RootClientBuilder: kcontroller.SimpleControllerClientBuilder{
			ClientConfig: &c.PrivilegedLoopbackClientConfig,
		},
	}

	if len(c.Options.ServiceAccountConfig.PrivateKeyFile) == 0 {
		glog.Infof("Skipped starting Service Account Token Manager, no private key specified")
		return nil, nil
	}

	var err error

	saToken.PrivateKey, err = serviceaccount.ReadPrivateKey(c.Options.ServiceAccountConfig.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("error reading signing key for Service Account Token Manager: %v", err)
	}

	if len(c.Options.ServiceAccountConfig.MasterCA) > 0 {
		saToken.RootCA, err = ioutil.ReadFile(c.Options.ServiceAccountConfig.MasterCA)
		if err != nil {
			return nil, fmt.Errorf("error reading master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
		if _, err := cert.ParseCertsPEM(saToken.RootCA); err != nil {
			return nil, fmt.Errorf("error parsing master ca file for Service Account Token Manager: %s: %v", c.Options.ServiceAccountConfig.MasterCA, err)
		}
	}

	if c.Options.ControllerConfig.ServiceServingCert.Signer != nil && len(c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		certFile := c.Options.ControllerConfig.ServiceServingCert.Signer.CertFile
		serviceServingCA, err := ioutil.ReadFile(certFile)
		if err != nil {
			return nil, fmt.Errorf("error reading ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}
		if _, err := crypto.CertsFromPEM(serviceServingCA); err != nil {
			return nil, fmt.Errorf("error parsing ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}

		// if we have a rootCA bundle add that too.  The rootCA will be used when hitting the default master service, since those are signed
		// using a different CA by default.  The rootCA's key is more closely guarded than ours and if it is compromised, that power could
		// be used to change the trusted signers for every pod anyway, so we're already effectively trusting it.
		if len(saToken.RootCA) > 0 {
			saToken.ServiceServingCA = append(saToken.ServiceServingCA, saToken.RootCA...)
			saToken.ServiceServingCA = append(saToken.ServiceServingCA, []byte("\n")...)
		}
		saToken.ServiceServingCA = append(saToken.ServiceServingCA, serviceServingCA...)
	}
	// this matches the upstream name
	ret["serviceaccount-token"] = saToken.RunController

	return ret, nil
}

func (c *MasterConfig) NewOpenshiftControllerInitializers() (map[string]controller.InitFunc, error) {
	ret := map[string]controller.InitFunc{}

	// TODO this overrides an upstream controller, so move this to where we initialize upstream controllers
	serviceAccount := controller.ServiceAccountControllerOptions{
		ManagedNames: c.Options.ServiceAccountConfig.ManagedNames,
	}
	ret["serviceaccount"] = serviceAccount.RunController

	ret["openshift.io/serviceaccount-pull-secrets"] = controller.RunServiceAccountPullSecretsController
	ret["openshift.io/origin-namespace"] = controller.RunOriginNamespaceController

	// initialize build controller
	storageVersion := c.Options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	// TODO: add codec to the controller context
	codec := kapi.Codecs.LegacyCodec(groupVersion)

	buildControllerConfig := controller.BuildControllerConfig{
		DockerImage:           c.ImageFor("docker-builder"),
		STIImage:              c.ImageFor("sti-builder"),
		AdmissionPluginConfig: c.Options.AdmissionConfig.PluginConfig,
		Codec: codec,
	}

	ret["openshift.io/build"] = buildControllerConfig.RunController
	ret["openshift.io/build-config-change"] = controller.RunBuildConfigChangeController

	// initialize apps.openshift.io controllers
	vars, err := c.GetOpenShiftClientEnvVars()
	if err != nil {
		return nil, err
	}
	deployer := controller.DeployerControllerConfig{ImageName: c.ImageFor("deployer"), Codec: codec, ClientEnvVars: vars}
	ret["openshift.io/deployer"] = deployer.RunController

	deploymentConfig := controller.DeploymentConfigControllerConfig{Codec: codec}
	ret["openshift.io/deploymentconfig"] = deploymentConfig.RunController

	deploymentTrigger := controller.DeploymentTriggerControllerConfig{Codec: codec}
	ret["openshift.io/deploymenttrigger"] = deploymentTrigger.RunController

	// initialize other controllers
	imageTrigger := controller.ImageTriggerControllerConfig{
		HasBuilderEnabled: c.Options.DisabledFeatures.Has(configapi.FeatureBuilder),
		// TODO: make these consts in configapi
		HasDeploymentsEnabled:  c.Options.DisabledFeatures.Has("triggers.image.openshift.io/deployments"),
		HasDaemonSetsEnabled:   c.Options.DisabledFeatures.Has("triggers.image.openshift.io/daemonsets"),
		HasStatefulSetsEnabled: c.Options.DisabledFeatures.Has("triggers.image.openshift.io/statefulsets"),
		HasCronJobsEnabled:     c.Options.DisabledFeatures.Has("triggers.image.openshift.io/cronjobs"),
	}
	ret["openshift.io/image-trigger"] = imageTrigger.RunController

	imageImport := controller.ImageImportControllerOptions{
		MaxScheduledImageImportsPerMinute: c.Options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
		ResyncPeriod:                      10 * time.Minute,

		DisableScheduledImport:                     c.Options.ImagePolicyConfig.DisableScheduledImport,
		ScheduledImageImportMinimumIntervalSeconds: c.Options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
	}
	ret["openshift.io/image-import"] = imageImport.RunController

	templateInstance := controller.TemplateInstanceControllerConfig{}
	ret["openshift.io/templateinstance"] = templateInstance.RunController

	serviceServingCert := controller.ServiceServingCertsControllerOptions{
		Signer: c.Options.ControllerConfig.ServiceServingCert.Signer,
	}
	ret["openshift.io/service-serving-cert"] = serviceServingCert.RunController

	sdnController := controller.SDNControllerConfig{
		NetworkConfig: c.Options.NetworkConfig,
	}
	ret["openshift.io/sdn"] = sdnController.RunController

	// Overrides the upstream "resourcequota" controller
	ret["resourcequota"] = controller.RunResourceQuotaManager

	clusterQuotaReconciliationController := controller.ClusterQuotaReconciliationControllerConfig{
		Mapper:                         c.ClusterQuotaMappingController.GetClusterQuotaMapper(),
		DefaultResyncPeriod:            5 * time.Minute,
		DefaultReplenishmentSyncPeriod: 12 * time.Hour,
	}
	ret["openshift.io/cluster-quota-reconciliation"] = clusterQuotaReconciliationController.RunController

	clusterQuotaMappingController := controller.ClusterQuotaMappingControllerConfig{
		ClusterQuotaMappingController: c.ClusterQuotaMappingController,
	}
	ret["openshift.io/cluster-quota-mapping"] = clusterQuotaMappingController.RunController

	unidlingController := controller.UnidlingControllerConfig{
		ResyncPeriod: 2 * time.Hour,
	}
	ret["openshift.io/unidling"] = unidlingController.RunController

	ingressIPController := controller.IngressIPControllerConfig{
		IngressIPSyncPeriod:  10 * time.Minute,
		IngressIPNetworkCIDR: c.Options.NetworkConfig.IngressIPNetworkCIDR,
	}
	ret["openshift.io/ingress-ip"] = ingressIPController.RunController

	originToRBACSyncController := controller.OriginToRBACSyncControllerConfig{
		PrivilegedRBACClient: c.PrivilegedLoopbackKubernetesClientsetInternal.Rbac(),
	}
	ret["openshift.io/origin-to-rbac"] = originToRBACSyncController.RunController
	return ret, nil
}
