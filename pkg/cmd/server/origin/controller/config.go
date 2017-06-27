package controller

// OpenshiftControllerConfig is the runtime (non-serializable) config object used to
// launch the set of openshift (not kube) controllers.
type OpenshiftControllerConfig struct {
	ServiceAccountTokenControllerOptions ServiceAccountTokenControllerOptions

	// TODO, this should only hold the delta on names and we run two controllers, upstream and ours
	ServiceAccountControllerOptions ServiceAccountControllerOptions

	BuildControllerConfig BuildControllerConfig

	DeployerControllerConfig          DeployerControllerConfig
	DeploymentConfigControllerConfig  DeploymentConfigControllerConfig
	DeploymentTriggerControllerConfig DeploymentTriggerControllerConfig

	ImageTriggerControllerConfig ImageTriggerControllerConfig
	ImageImportControllerConfig  ImageImportControllerConfig

	ServiceServingCertsControllerOptions ServiceServingCertsControllerOptions

	SDNControllerConfig       SDNControllerConfig
	UnidlingControllerConfig  UnidlingControllerConfig
	IngressIPControllerConfig IngressIPControllerConfig

	OriginToRBACSyncControllerConfig OriginToRBACSyncControllerConfig

	ClusterQuotaMappingControllerConfig        ClusterQuotaMappingControllerConfig
	ClusterQuotaReconciliationControllerConfig ClusterQuotaReconciliationControllerConfig
}

func (c *OpenshiftControllerConfig) GetControllerInitializers() (map[string]InitFunc, error) {
	ret := map[string]InitFunc{}

	// TODO, this should only hold the delta on names and we run two controllers, upstream and ours
	ret["serviceaccount"] = c.ServiceAccountControllerOptions.RunController

	ret["openshift.io/serviceaccount-pull-secrets"] = RunServiceAccountPullSecretsController
	ret["openshift.io/origin-namespace"] = RunOriginNamespaceController
	ret["openshift.io/service-serving-cert"] = c.ServiceServingCertsControllerOptions.RunController

	ret["openshift.io/build"] = c.BuildControllerConfig.RunController
	ret["openshift.io/build-config-change"] = RunBuildConfigChangeController

	ret["openshift.io/deployer"] = c.DeployerControllerConfig.RunController
	ret["openshift.io/deploymentconfig"] = c.DeploymentConfigControllerConfig.RunController
	ret["openshift.io/deploymenttrigger"] = c.DeploymentTriggerControllerConfig.RunController

	ret["openshift.io/image-trigger"] = c.ImageTriggerControllerConfig.RunController
	ret["openshift.io/image-import"] = c.ImageImportControllerConfig.RunController

	ret["openshift.io/templateinstance"] = RunTemplateInstanceController

	ret["openshift.io/sdn"] = c.SDNControllerConfig.RunController
	ret["openshift.io/unidling"] = c.UnidlingControllerConfig.RunController
	ret["openshift.io/ingress-ip"] = c.IngressIPControllerConfig.RunController

	// Overrides the upstream "resourcequota" controller
	ret["resourcequota"] = RunResourceQuotaManager
	ret["openshift.io/cluster-quota-reconciliation"] = c.ClusterQuotaReconciliationControllerConfig.RunController
	ret["openshift.io/cluster-quota-mapping"] = c.ClusterQuotaMappingControllerConfig.RunController

	ret["openshift.io/origin-to-rbac"] = c.OriginToRBACSyncControllerConfig.RunController
	return ret, nil
}

// NewOpenShiftControllerPreStartInitializers returns list of initializers for controllers
// that needed to be run before any other controller is started.
// Typically this has to done for the serviceaccount-token controller as it provides
// tokens to other controllers.
func (c *OpenshiftControllerConfig) ServiceAccountContentControllerInit() InitFunc {
	return c.ServiceAccountTokenControllerOptions.RunController
}
