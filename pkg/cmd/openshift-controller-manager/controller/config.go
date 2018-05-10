package controller

var ControllerInitializers = map[string]InitFunc{
	"openshift.io/serviceaccount": RunServiceAccountController,

	"openshift.io/namespace-security-allocation": RunNamespaceSecurityAllocationController,

	"openshift.io/default-rolebindings": RunDefaultRoleBindingController,

	"openshift.io/serviceaccount-pull-secrets": RunServiceAccountPullSecretsController,
	"openshift.io/origin-namespace":            RunOriginNamespaceController,
	"openshift.io/service-serving-cert":        RunServiceServingCertsController,

	"openshift.io/build":               RunBuildController,
	"openshift.io/build-config-change": RunBuildConfigChangeController,

	"openshift.io/deployer":         RunDeployerController,
	"openshift.io/deploymentconfig": RunDeploymentConfigController,

	"openshift.io/image-trigger":          RunImageTriggerController,
	"openshift.io/image-import":           RunImageImportController,
	"openshift.io/image-signature-import": RunImageSignatureImportController,

	"openshift.io/templateinstance":          RunTemplateInstanceController,
	"openshift.io/templateinstancefinalizer": RunTemplateInstanceFinalizerController,

	"openshift.io/sdn":              RunSDNController,
	"openshift.io/unidling":         RunUnidlingController,
	"openshift.io/ingress-ip":       RunIngressIPController,
	"openshift.io/ingress-to-route": RunIngressToRouteController,

	"openshift.io/resourcequota":                RunResourceQuotaManager,
	"openshift.io/cluster-quota-reconciliation": RunClusterQuotaReconciliationController,

	// overrides the Kube HPA controller config, so that we can point it at an HTTPS Heapster
	// in openshift-infra, and pass it a scale client that knows how to scale DCs
	"openshift.io/horizontalpodautoscaling": RunHorizontalPodAutoscalerController,
}
