package controller

var ControllerInitializers = map[string]InitFunc{
	"openshift.io/serviceaccount": RunServiceAccountController,

	"openshift.io/namespace-security-allocation": RunNamespaceSecurityAllocationController,

	"openshift.io/default-rolebindings": RunDefaultRoleBindingController,

	"openshift.io/serviceaccount-pull-secrets": RunServiceAccountPullSecretsController,
	"openshift.io/origin-namespace":            RunOriginNamespaceController,

	"openshift.io/build":               RunBuildController,
	"openshift.io/build-config-change": RunBuildConfigChangeController,

	"openshift.io/deployer":         RunDeployerController,
	"openshift.io/deploymentconfig": RunDeploymentConfigController,

	"openshift.io/image-trigger":          RunImageTriggerController,
	"openshift.io/image-import":           RunImageImportController,
	"openshift.io/image-signature-import": RunImageSignatureImportController,

	"openshift.io/templateinstance":          RunTemplateInstanceController,
	"openshift.io/templateinstancefinalizer": RunTemplateInstanceFinalizerController,

	"openshift.io/unidling":         RunUnidlingController,
	"openshift.io/ingress-ip":       RunIngressIPController,
	"openshift.io/ingress-to-route": RunIngressToRouteController,

	"openshift.io/resourcequota":                RunResourceQuotaManager,
	"openshift.io/cluster-quota-reconciliation": RunClusterQuotaReconciliationController,
}

const (
	infraOriginNamespaceServiceAccountName                       = "origin-namespace-controller"
	infraServiceAccountControllerServiceAccountName              = "serviceaccount-controller"
	iInfraServiceAccountPullSecretsControllerServiceAccountName  = "serviceaccount-pull-secrets-controller"
	infraBuildControllerServiceAccountName                       = "build-controller"
	infraBuildConfigChangeControllerServiceAccountName           = "build-config-change-controller"
	infraDeploymentConfigControllerServiceAccountName            = "deploymentconfig-controller"
	infraDeployerControllerServiceAccountName                    = "deployer-controller"
	infraImageTriggerControllerServiceAccountName                = "image-trigger-controller"
	infraImageImportControllerServiceAccountName                 = "image-import-controller"
	infraSDNControllerServiceAccountName                         = "sdn-controller"
	infraClusterQuotaReconciliationControllerServiceAccountName  = "cluster-quota-reconciliation-controller"
	infraUnidlingControllerServiceAccountName                    = "unidling-controller"
	infraServiceIngressIPControllerServiceAccountName            = "service-ingress-ip-controller"
	infraDefaultRoleBindingsControllerServiceAccountName         = "default-rolebindings-controller"
	infraIngressToRouteControllerServiceAccountName              = "ingress-to-route-controller"
	infraNamespaceSecurityAllocationControllerServiceAccountName = "namespace-security-allocation-controller"

	// template instance controller watches for TemplateInstance object creation
	// and instantiates templates as a result.
	infraTemplateInstanceControllerServiceAccountName          = "template-instance-controller"
	infraTemplateInstanceFinalizerControllerServiceAccountName = "template-instance-finalizer-controller"

	deployerServiceAccountName = "deployer"

	defaultOpenShiftInfraNamespace = "openshift-infra"
)
