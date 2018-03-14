package bootstrappolicy

import (
	"strings"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbac "k8s.io/kubernetes/pkg/apis/rbac"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"

	// we need the conversions registered for our init block
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
)

const saRolePrefix = "system:openshift:controller:"

const (
	InfraOriginNamespaceServiceAccountName                      = "origin-namespace-controller"
	InfraServiceAccountControllerServiceAccountName             = "serviceaccount-controller"
	InfraServiceAccountPullSecretsControllerServiceAccountName  = "serviceaccount-pull-secrets-controller"
	InfraServiceAccountTokensControllerServiceAccountName       = "serviceaccount-tokens-controller"
	InfraServiceServingCertServiceAccountName                   = "service-serving-cert-controller"
	InfraBuildControllerServiceAccountName                      = "build-controller"
	InfraBuildConfigChangeControllerServiceAccountName          = "build-config-change-controller"
	InfraDeploymentConfigControllerServiceAccountName           = "deploymentconfig-controller"
	InfraDeployerControllerServiceAccountName                   = "deployer-controller"
	InfraImageTriggerControllerServiceAccountName               = "image-trigger-controller"
	InfraImageImportControllerServiceAccountName                = "image-import-controller"
	InfraSDNControllerServiceAccountName                        = "sdn-controller"
	InfraClusterQuotaReconciliationControllerServiceAccountName = "cluster-quota-reconciliation-controller"
	InfraUnidlingControllerServiceAccountName                   = "unidling-controller"
	InfraServiceIngressIPControllerServiceAccountName           = "service-ingress-ip-controller"
	InfraPersistentVolumeRecyclerControllerServiceAccountName   = "pv-recycler-controller"
	InfraResourceQuotaControllerServiceAccountName              = "resourcequota-controller"
	InfraDefaultRoleBindingsControllerServiceAccountName        = "default-rolebindings-controller"

	// template instance controller watches for TemplateInstance object creation
	// and instantiates templates as a result.
	InfraTemplateInstanceControllerServiceAccountName = "template-instance-controller"

	// template service broker is an open service broker-compliant API
	// implementation which serves up OpenShift templates.  It uses the
	// TemplateInstance backend for most of the heavy lifting.
	InfraTemplateServiceBrokerServiceAccountName = "template-service-broker"

	// This is a special constant which maps to the service account name used by the underlying
	// Kubernetes code, so that we can build out the extra policy required to scale OpenShift resources.
	InfraHorizontalPodAutoscalerControllerServiceAccountName = "horizontal-pod-autoscaler"

	InfraNodeBootstrapServiceAccountName = "node-bootstrapper"
)

var (
	// controllerRoles is a slice of roles used for controllers
	controllerRoles = []rbac.ClusterRole{}
	// controllerRoleBindings is a slice of roles used for controllers
	controllerRoleBindings = []rbac.ClusterRoleBinding{}
)

func bindControllerRole(saName string, roleName string) {
	roleBinding := rbac.NewClusterBinding(roleName).SAs(DefaultOpenShiftInfraNamespace, saName).BindingOrDie()
	addDefaultMetadata(&roleBinding)
	controllerRoleBindings = append(controllerRoleBindings, roleBinding)
}

func addControllerRole(role rbac.ClusterRole) {
	if !strings.HasPrefix(role.Name, saRolePrefix) {
		glog.Fatalf(`role %q must start with %q`, role.Name, saRolePrefix)
	}
	addControllerRoleToSA(DefaultOpenShiftInfraNamespace, role.Name[len(saRolePrefix):], role)
}

func addControllerRoleToSA(saNamespace, saName string, role rbac.ClusterRole) {
	if !strings.HasPrefix(role.Name, saRolePrefix) {
		glog.Fatalf(`role %q must start with %q`, role.Name, saRolePrefix)
	}

	for _, existingRole := range controllerRoles {
		if role.Name == existingRole.Name {
			glog.Fatalf("role %q was already registered", role.Name)
		}
	}

	addDefaultMetadata(&role)
	controllerRoles = append(controllerRoles, role)

	roleBinding := rbac.NewClusterBinding(role.Name).SAs(saNamespace, saName).BindingOrDie()
	addDefaultMetadata(&roleBinding)
	controllerRoleBindings = append(controllerRoleBindings, roleBinding)
}

func eventsRule() rbac.PolicyRule {
	return rbac.NewRule("create", "update", "patch").Groups(kapiGroup).Resources("events").RuleOrDie()
}

func init() {
	// build-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraBuildControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch", "patch", "update", "delete").Groups(buildGroup, legacyBuildGroup).Resources("builds").RuleOrDie(),
			rbac.NewRule("update").Groups(buildGroup, legacyBuildGroup).Resources("builds/finalizers").RuleOrDie(),
			rbac.NewRule("get").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs").RuleOrDie(),
			rbac.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("builds/optimizeddocker", "builds/docker", "builds/source", "builds/custom", "builds/jenkinspipeline").RuleOrDie(),
			rbac.NewRule("get", "list").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbac.NewRule("get", "list").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbac.NewRule("get", "list").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			rbac.NewRule("get", "list", "create", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbac.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbac.NewRule("get", "list").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),
			rbac.NewRule("create").Groups(securityGroup, legacySecurityGroup).Resources("podsecuritypolicysubjectreviews").RuleOrDie(),
			eventsRule(),
		},
	})

	// build-config-change-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraBuildConfigChangeControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs").RuleOrDie(),
			rbac.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate").RuleOrDie(),
			rbac.NewRule("delete").Groups(buildGroup, legacyBuildGroup).Resources("builds").RuleOrDie(),
			eventsRule(),
		},
	})

	// deployer-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDeployerControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("create", "get", "list", "watch", "patch", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),

			// "delete" is required here for compatibility with older deployer images
			// (see https://github.com/openshift/origin/pull/14322#issuecomment-303968976)
			// TODO: remove "delete" rule few releases after 3.6
			rbac.NewRule("delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbac.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			eventsRule(),
		},
	})

	// deploymentconfig-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDeploymentConfigControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("create", "get", "list", "watch", "update", "patch", "delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbac.NewRule("update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/status").RuleOrDie(),
			rbac.NewRule("update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/finalizers").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			eventsRule(),
		},
	})

	// template-instance-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraTemplateInstanceControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("create").Groups(kAuthzGroup).Resources("subjectaccessreviews").RuleOrDie(),
			rbac.NewRule("update").Groups(templateGroup).Resources("templateinstances/status").RuleOrDie(),
			rbac.NewRule("update").Groups(templateGroup).Resources("templateinstances/finalizers").RuleOrDie(),
		},
	})

	// template-instance-controller
	templateInstanceController := rbac.NewClusterBinding(AdminRoleName).SAs(DefaultOpenShiftInfraNamespace, InfraTemplateInstanceControllerServiceAccountName).BindingOrDie()
	addDefaultMetadata(&templateInstanceController)
	controllerRoleBindings = append(controllerRoleBindings, templateInstanceController)

	// origin-namespace-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraOriginNamespaceServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbac.NewRule("update").Groups(kapiGroup).Resources("namespaces/finalize", "namespaces/status").RuleOrDie(),
			eventsRule(),
		},
	})

	// serviceaccount-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceAccountControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),
			eventsRule(),
		},
	})

	// serviceaccount-pull-secrets-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceAccountPullSecretsControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch", "create", "update").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),
			rbac.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("services").RuleOrDie(),
			eventsRule(),
		},
	})

	// imagetrigger-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraImageTriggerControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("list", "watch").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(extensionsGroup, appsGroup).Resources("deployments").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(appsGroup).Resources("statefulsets").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(batchGroup).Resources("cronjobs").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			rbac.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate").RuleOrDie(),
			// trigger controller must be able to modify these build types
			// TODO: move to a new custom binding that can be removed separately from end user access?
			rbac.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(
				authorizationapi.SourceBuildResource,
				authorizationapi.DockerBuildResource,
				authorizationapi.CustomBuildResource,
				authorizationapi.OptimizedDockerBuildResource,
				authorizationapi.JenkinsPipelineBuildResource,
			).RuleOrDie(),

			eventsRule(),
		},
	})

	// service-serving-cert-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceServingCertServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("list", "watch", "update").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbac.NewRule("get", "list", "watch", "create", "update", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			eventsRule(),
		},
	})

	// image-import-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraImageImportControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list", "watch", "create", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbac.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(imageGroup, legacyImageGroup).Resources("images").RuleOrDie(),
			rbac.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimports").RuleOrDie(),
			eventsRule(),
		},
	})

	// sdn-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraSDNControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "create", "update").Groups(networkGroup, legacyNetworkGroup).Resources("clusternetworks").RuleOrDie(),
			rbac.NewRule("get", "list", "watch", "create", "update", "delete").Groups(networkGroup, legacyNetworkGroup).Resources("hostsubnets").RuleOrDie(),
			rbac.NewRule("get", "list", "watch", "create", "update", "delete").Groups(networkGroup, legacyNetworkGroup).Resources("netnamespaces").RuleOrDie(),
			rbac.NewRule("get", "list").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("nodes").RuleOrDie(),
			rbac.NewRule("update").Groups(kapiGroup).Resources("nodes/status").RuleOrDie(),

			eventsRule(),
		},
	})

	// cluster-quota-reconciliation
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraClusterQuotaReconciliationControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "list").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			rbac.NewRule("get", "list").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbac.NewRule("update").Groups(quotaGroup, legacyQuotaGroup).Resources("clusterresourcequotas/status").RuleOrDie(),
			eventsRule(),
		},
	})

	// unidling-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraUnidlingControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "update").Groups(kapiGroup).Resources("replicationcontrollers/scale", "endpoints").RuleOrDie(),
			rbac.NewRule("get", "update", "patch").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbac.NewRule("get", "update", "patch").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(extensionsGroup, appsGroup).Resources("replicasets/scale", "deployments/scale").RuleOrDie(),
			rbac.NewRule("get", "update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/scale").RuleOrDie(),
			rbac.NewRule("watch", "list").Groups(kapiGroup).Resources("events").RuleOrDie(),
			eventsRule(),
		},
	})

	// ingress-ip-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceIngressIPControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("list", "watch", "update").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbac.NewRule("update").Groups(kapiGroup).Resources("services/status").RuleOrDie(),
			eventsRule(),
		},
	})

	// pv-recycler-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraPersistentVolumeRecyclerControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "update", "create", "delete", "list", "watch").Groups(kapiGroup).Resources("persistentvolumes").RuleOrDie(),
			rbac.NewRule("update").Groups(kapiGroup).Resources("persistentvolumes/status").RuleOrDie(),
			rbac.NewRule("get", "update", "list", "watch").Groups(kapiGroup).Resources("persistentvolumeclaims").RuleOrDie(),
			rbac.NewRule("update").Groups(kapiGroup).Resources("persistentvolumeclaims/status").RuleOrDie(),
			rbac.NewRule("get", "create", "delete", "list", "watch").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			eventsRule(),
		},
	})

	// resourcequota-controller
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraResourceQuotaControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("update").Groups(kapiGroup).Resources("resourcequotas/status").RuleOrDie(),
			rbac.NewRule("list").Groups(kapiGroup).Resources("resourcequotas").RuleOrDie(),
			rbac.NewRule("list").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbac.NewRule("list").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			rbac.NewRule("list").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbac.NewRule("list").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			eventsRule(),
		},
	})

	// horizontal-pod-autoscaler-controller (the OpenShift resources only)
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraHorizontalPodAutoscalerControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("get", "update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/scale").RuleOrDie(),
		},
	})

	bindControllerRole(InfraHorizontalPodAutoscalerControllerServiceAccountName, "system:controller:horizontal-pod-autoscaler")

	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraTemplateServiceBrokerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("create").Groups(kAuthzGroup).Resources("subjectaccessreviews").RuleOrDie(),
			rbac.NewRule("create").Groups(authzGroup).Resources("subjectaccessreviews").RuleOrDie(),
			rbac.NewRule("get", "create", "update", "delete").Groups(templateGroup).Resources("brokertemplateinstances").RuleOrDie(),
			rbac.NewRule("update").Groups(templateGroup).Resources("brokertemplateinstances/finalizers").RuleOrDie(),
			rbac.NewRule("get", "create", "delete", "assign").Groups(templateGroup).Resources("templateinstances").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(templateGroup).Resources("templates").RuleOrDie(),
			rbac.NewRule("get", "create", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbac.NewRule("get").Groups(kapiGroup).Resources("services", "configmaps").RuleOrDie(),
			rbac.NewRule("get").Groups(legacyRouteGroup).Resources("routes").RuleOrDie(),
			rbac.NewRule("get").Groups(routeGroup).Resources("routes").RuleOrDie(),
			eventsRule(),
		},
	})

	// the controller needs to be bound to the roles it is going to try to create
	bindControllerRole(InfraDefaultRoleBindingsControllerServiceAccountName, ImagePullerRoleName)
	bindControllerRole(InfraDefaultRoleBindingsControllerServiceAccountName, ImageBuilderRoleName)
	bindControllerRole(InfraDefaultRoleBindingsControllerServiceAccountName, DeployerRoleName)
	addControllerRole(rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDefaultRoleBindingsControllerServiceAccountName},
		Rules: []rbac.PolicyRule{
			rbac.NewRule("create").Groups(rbacGroup).Resources("rolebindings").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbac.NewRule("get", "list", "watch").Groups(rbacGroup).Resources("rolebindings").RuleOrDie(),
			eventsRule(),
		},
	})
}

// ControllerRoles returns the cluster roles used by controllers
func ControllerRoles() []rbac.ClusterRole {
	return controllerRoles
}

// ControllerRoleBindings returns the role bindings used by controllers
func ControllerRoleBindings() []rbac.ClusterRoleBinding {
	return controllerRoleBindings
}
