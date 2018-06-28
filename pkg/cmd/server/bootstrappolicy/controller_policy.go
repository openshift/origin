package bootstrappolicy

import (
	"strings"

	"github.com/golang/glog"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"

	// we need the conversions registered for our init block
	_ "github.com/openshift/origin/pkg/authorization/apis/authorization/install"
)

const saRolePrefix = "system:openshift:controller:"

const (
	InfraOriginNamespaceServiceAccountName                       = "origin-namespace-controller"
	InfraServiceAccountControllerServiceAccountName              = "serviceaccount-controller"
	InfraServiceAccountPullSecretsControllerServiceAccountName   = "serviceaccount-pull-secrets-controller"
	InfraServiceAccountTokensControllerServiceAccountName        = "serviceaccount-tokens-controller"
	InfraServiceServingCertServiceAccountName                    = "service-serving-cert-controller"
	InfraBuildControllerServiceAccountName                       = "build-controller"
	InfraBuildConfigChangeControllerServiceAccountName           = "build-config-change-controller"
	InfraDeploymentConfigControllerServiceAccountName            = "deploymentconfig-controller"
	InfraDeployerControllerServiceAccountName                    = "deployer-controller"
	InfraImageTriggerControllerServiceAccountName                = "image-trigger-controller"
	InfraImageImportControllerServiceAccountName                 = "image-import-controller"
	InfraSDNControllerServiceAccountName                         = "sdn-controller"
	InfraClusterQuotaReconciliationControllerServiceAccountName  = "cluster-quota-reconciliation-controller"
	InfraUnidlingControllerServiceAccountName                    = "unidling-controller"
	InfraServiceIngressIPControllerServiceAccountName            = "service-ingress-ip-controller"
	InfraPersistentVolumeRecyclerControllerServiceAccountName    = "pv-recycler-controller"
	InfraResourceQuotaControllerServiceAccountName               = "resourcequota-controller"
	InfraDefaultRoleBindingsControllerServiceAccountName         = "default-rolebindings-controller"
	InfraIngressToRouteControllerServiceAccountName              = "ingress-to-route-controller"
	InfraNamespaceSecurityAllocationControllerServiceAccountName = "namespace-security-allocation-controller"

	// template instance controller watches for TemplateInstance object creation
	// and instantiates templates as a result.
	InfraTemplateInstanceControllerServiceAccountName          = "template-instance-controller"
	InfraTemplateInstanceFinalizerControllerServiceAccountName = "template-instance-finalizer-controller"

	// template service broker is an open service broker-compliant API
	// implementation which serves up OpenShift templates.  It uses the
	// TemplateInstance backend for most of the heavy lifting.
	InfraTemplateServiceBrokerServiceAccountName = "template-service-broker"

	// This is a special constant which maps to the service account name used by the underlying
	// Kubernetes code, so that we can build out the extra policy required to scale OpenShift resources.
	InfraHorizontalPodAutoscalerControllerServiceAccountName = "horizontal-pod-autoscaler"
)

var (
	// controllerRoles is a slice of roles used for controllers
	controllerRoles = []rbacv1.ClusterRole{}
	// controllerRoleBindings is a slice of roles used for controllers
	controllerRoleBindings = []rbacv1.ClusterRoleBinding{}
)

func bindControllerRole(saName string, roleName string) {
	roleBinding := rbacv1helpers.NewClusterBinding(roleName).SAs(DefaultOpenShiftInfraNamespace, saName).BindingOrDie()
	addDefaultMetadata(&roleBinding)
	controllerRoleBindings = append(controllerRoleBindings, roleBinding)
}

func addControllerRole(role rbacv1.ClusterRole) {
	if !strings.HasPrefix(role.Name, saRolePrefix) {
		glog.Fatalf(`role %q must start with %q`, role.Name, saRolePrefix)
	}
	addControllerRoleToSA(DefaultOpenShiftInfraNamespace, role.Name[len(saRolePrefix):], role)
}

func addControllerRoleToSA(saNamespace, saName string, role rbacv1.ClusterRole) {
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

	roleBinding := rbacv1helpers.NewClusterBinding(role.Name).SAs(saNamespace, saName).BindingOrDie()
	addDefaultMetadata(&roleBinding)
	controllerRoleBindings = append(controllerRoleBindings, roleBinding)
}

func eventsRule() rbacv1.PolicyRule {
	return rbacv1helpers.NewRule("create", "update", "patch").Groups(kapiGroup).Resources("events").RuleOrDie()
}

func init() {
	// build-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraBuildControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch", "patch", "update", "delete").Groups(buildGroup, legacyBuildGroup).Resources("builds").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(buildGroup, legacyBuildGroup).Resources("builds/finalizers").RuleOrDie(),
			rbacv1helpers.NewRule("get").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("builds/optimizeddocker", "builds/docker", "builds/source", "builds/custom", "builds/jenkinspipeline").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "create", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbacv1helpers.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(securityGroup, legacySecurityGroup).Resources("podsecuritypolicysubjectreviews").RuleOrDie(),
			eventsRule(),
		},
	})

	// build-config-change-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraBuildConfigChangeControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate").RuleOrDie(),
			rbacv1helpers.NewRule("delete").Groups(buildGroup, legacyBuildGroup).Resources("builds").RuleOrDie(),
			eventsRule(),
		},
	})

	// deployer-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDeployerControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create", "get", "list", "watch", "patch", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),

			// "delete" is required here for compatibility with older deployer images
			// (see https://github.com/openshift/origin/pull/14322#issuecomment-303968976)
			// TODO: remove "delete" rule few releases after 3.6
			rbacv1helpers.NewRule("delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(kapiGroup).Resources("replicationcontrollers/scale").RuleOrDie(),
			eventsRule(),
		},
	})

	// deploymentconfig-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDeploymentConfigControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create", "get", "list", "watch", "update", "patch", "delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(kapiGroup).Resources("replicationcontrollers/scale").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/status").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/finalizers").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			eventsRule(),
		},
	})

	// template-instance-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraTemplateInstanceControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create").Groups(kAuthzGroup).Resources("subjectaccessreviews").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(templateGroup).Resources("templateinstances/status").RuleOrDie(),
		},
	})

	// template-instance-controller
	templateInstanceController := rbacv1helpers.NewClusterBinding(AdminRoleName).SAs(DefaultOpenShiftInfraNamespace, InfraTemplateInstanceControllerServiceAccountName).BindingOrDie()
	templateInstanceController.Name = "system:openshift:controller:" + InfraTemplateInstanceControllerServiceAccountName + ":admin"
	addDefaultMetadata(&templateInstanceController)
	controllerRoleBindings = append(controllerRoleBindings, templateInstanceController)

	// template-instance-finalizer-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraTemplateInstanceFinalizerControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("update").Groups(templateGroup).Resources("templateinstances/status").RuleOrDie(),
		},
	})

	// template-instance-finalizer-controller
	templateInstanceFinalizerController := rbacv1helpers.NewClusterBinding(AdminRoleName).SAs(DefaultOpenShiftInfraNamespace, InfraTemplateInstanceFinalizerControllerServiceAccountName).BindingOrDie()
	templateInstanceFinalizerController.Name = "system:openshift:controller:" + InfraTemplateInstanceFinalizerControllerServiceAccountName + ":admin"
	addDefaultMetadata(&templateInstanceFinalizerController)
	controllerRoleBindings = append(controllerRoleBindings, templateInstanceFinalizerController)

	// origin-namespace-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraOriginNamespaceServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(kapiGroup).Resources("namespaces/finalize", "namespaces/status").RuleOrDie(),
			eventsRule(),
		},
	})

	// serviceaccount-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceAccountControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),
			eventsRule(),
		},
	})

	// serviceaccount-pull-secrets-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceAccountPullSecretsControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("services").RuleOrDie(),
			eventsRule(),
		},
	})

	// imagetrigger-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraImageTriggerControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("list", "watch").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(extensionsGroup, appsGroup).Resources("deployments").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(appsGroup).Resources("statefulsets").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(batchGroup).Resources("cronjobs").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate").RuleOrDie(),
			// trigger controller must be able to modify these build types
			// TODO: move to a new custom binding that can be removed separately from end user access?
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(
				SourceBuildResource,
				DockerBuildResource,
				CustomBuildResource,
				OptimizedDockerBuildResource,
				JenkinsPipelineBuildResource,
			).RuleOrDie(),

			eventsRule(),
		},
	})

	// service-serving-cert-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceServingCertServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("list", "watch", "update").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			eventsRule(),
		},
	})

	// image-import-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraImageImportControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(imageGroup, legacyImageGroup).Resources("images").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimports").RuleOrDie(),
			eventsRule(),
		},
	})

	// sdn-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraSDNControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "create", "update").Groups(networkGroup, legacyNetworkGroup).Resources("clusternetworks").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "delete").Groups(networkGroup, legacyNetworkGroup).Resources("hostsubnets").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "delete").Groups(networkGroup, legacyNetworkGroup).Resources("netnamespaces").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("nodes").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(kapiGroup).Resources("nodes/status").RuleOrDie(),

			eventsRule(),
		},
	})

	// cluster-quota-reconciliation
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraClusterQuotaReconciliationControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(quotaGroup, legacyQuotaGroup).Resources("clusterresourcequotas/status").RuleOrDie(),
			eventsRule(),
		},
	})

	// unidling-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraUnidlingControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "update").Groups(kapiGroup).Resources("replicationcontrollers/scale", "endpoints").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update", "patch").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update", "patch").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(extensionsGroup, appsGroup).Resources("replicasets/scale", "deployments/scale").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/scale").RuleOrDie(),
			rbacv1helpers.NewRule("watch", "list").Groups(kapiGroup).Resources("events").RuleOrDie(),
			eventsRule(),
		},
	})

	// ingress-ip-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraServiceIngressIPControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("list", "watch", "update").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(kapiGroup).Resources("services/status").RuleOrDie(),
			eventsRule(),
		},
	})

	// ingress-to-route-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraIngressToRouteControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("secrets", "services").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(extensionsGroup).Resources("ingress").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "create", "update", "patch", "delete").Groups(routeGroup).Resources("routes").RuleOrDie(),
			rbacv1helpers.NewRule("create", "update").Groups(routeGroup).Resources("routes/custom-host").RuleOrDie(),
			eventsRule(),
		},
	})

	// pv-recycler-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraPersistentVolumeRecyclerControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "update", "create", "delete", "list", "watch").Groups(kapiGroup).Resources("persistentvolumes").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(kapiGroup).Resources("persistentvolumes/status").RuleOrDie(),
			rbacv1helpers.NewRule("get", "update", "list", "watch").Groups(kapiGroup).Resources("persistentvolumeclaims").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(kapiGroup).Resources("persistentvolumeclaims/status").RuleOrDie(),
			rbacv1helpers.NewRule("get", "create", "delete", "list", "watch").Groups(kapiGroup).Resources("pods").RuleOrDie(),
			eventsRule(),
		},
	})

	// resourcequota-controller
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraResourceQuotaControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("update").Groups(kapiGroup).Resources("resourcequotas/status").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(kapiGroup).Resources("resourcequotas").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(kapiGroup).Resources("services").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
			eventsRule(),
		},
	})

	// horizontal-pod-autoscaler-controller (the OpenShift resources only)
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraHorizontalPodAutoscalerControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "update").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/scale").RuleOrDie(),
		},
	})

	bindControllerRole(InfraHorizontalPodAutoscalerControllerServiceAccountName, "system:controller:horizontal-pod-autoscaler")

	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraTemplateServiceBrokerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create").Groups(kAuthzGroup).Resources("subjectaccessreviews").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(authzGroup).Resources("subjectaccessreviews").RuleOrDie(),
			rbacv1helpers.NewRule("get", "create", "update", "delete").Groups(templateGroup).Resources("brokertemplateinstances").RuleOrDie(),
			rbacv1helpers.NewRule("update").Groups(templateGroup).Resources("brokertemplateinstances/finalizers").RuleOrDie(),
			rbacv1helpers.NewRule("get", "create", "delete", "assign").Groups(templateGroup).Resources("templateinstances").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(templateGroup).Resources("templates").RuleOrDie(),
			rbacv1helpers.NewRule("get", "create", "delete").Groups(kapiGroup).Resources("secrets").RuleOrDie(),
			rbacv1helpers.NewRule("get").Groups(kapiGroup).Resources("services", "configmaps").RuleOrDie(),
			rbacv1helpers.NewRule("get").Groups(legacyRouteGroup).Resources("routes").RuleOrDie(),
			rbacv1helpers.NewRule("get").Groups(routeGroup).Resources("routes").RuleOrDie(),
			eventsRule(),
		},
	})

	// the controller needs to be bound to the roles it is going to try to create
	bindControllerRole(InfraDefaultRoleBindingsControllerServiceAccountName, ImagePullerRoleName)
	bindControllerRole(InfraDefaultRoleBindingsControllerServiceAccountName, ImageBuilderRoleName)
	bindControllerRole(InfraDefaultRoleBindingsControllerServiceAccountName, DeployerRoleName)
	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraDefaultRoleBindingsControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create").Groups(rbacGroup).Resources("rolebindings").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch").Groups(rbacGroup).Resources("rolebindings").RuleOrDie(),
			eventsRule(),
		},
	})

	addControllerRole(rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: saRolePrefix + InfraNamespaceSecurityAllocationControllerServiceAccountName},
		Rules: []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("get", "create", "update").Groups(securityGroup).Resources("rangeallocations").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
			eventsRule(),
		},
	})
}

// ControllerRoles returns the cluster roles used by controllers
func ControllerRoles() []rbacv1.ClusterRole {
	return controllerRoles
}

// ControllerRoleBindings returns the role bindings used by controllers
func ControllerRoleBindings() []rbacv1.ClusterRoleBinding {
	return controllerRoleBindings
}
