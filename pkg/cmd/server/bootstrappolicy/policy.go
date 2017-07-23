package bootstrappolicy

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/apps"
	kauthenticationapi "k8s.io/kubernetes/pkg/apis/authentication"
	kauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/apis/settings"
	"k8s.io/kubernetes/pkg/apis/storage"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	networkapi "github.com/openshift/origin/pkg/sdn/apis/network"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

const (
	// roleSystemOnly is an annotation key that determines if a role is system only
	roleSystemOnly = "authorization.openshift.io/system-only"
	// roleIsSystemOnly is an annotation value that denotes roleSystemOnly, and thus excludes the role from the UI
	roleIsSystemOnly = "true"
)

var (
	readWrite = []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
	read      = []string{"get", "list", "watch"}

	kapiGroup            = kapi.GroupName
	appsGroup            = apps.GroupName
	autoscalingGroup     = autoscaling.GroupName
	apiExtensionsGroup   = "apiextensions.k8s.io"
	apiRegistrationGroup = "apiregistration.k8s.io"
	batchGroup           = batch.GroupName
	certificatesGroup    = certificates.GroupName
	extensionsGroup      = extensions.GroupName
	networkingGroup      = "networking.k8s.io"
	policyGroup          = policy.GroupName
	rbacGroup            = rbac.GroupName
	securityGroup        = securityapi.GroupName
	legacySecurityGroup  = securityapi.LegacyGroupName
	storageGroup         = storage.GroupName
	settingsGroup        = settings.GroupName

	authzGroup          = authorizationapi.GroupName
	kAuthzGroup         = kauthorizationapi.GroupName
	kAuthnGroup         = kauthenticationapi.GroupName
	legacyAuthzGroup    = authorizationapi.LegacyGroupName
	buildGroup          = buildapi.GroupName
	legacyBuildGroup    = buildapi.LegacyGroupName
	deployGroup         = deployapi.GroupName
	legacyDeployGroup   = deployapi.LegacyGroupName
	imageGroup          = imageapi.GroupName
	legacyImageGroup    = imageapi.LegacyGroupName
	projectGroup        = projectapi.GroupName
	legacyProjectGroup  = projectapi.LegacyGroupName
	quotaGroup          = quotaapi.GroupName
	legacyQuotaGroup    = quotaapi.LegacyGroupName
	routeGroup          = routeapi.GroupName
	legacyRouteGroup    = routeapi.LegacyGroupName
	templateGroup       = templateapi.GroupName
	legacyTemplateGroup = templateapi.LegacyGroupName
	userGroup           = userapi.GroupName
	legacyUserGroup     = userapi.LegacyGroupName
	oauthGroup          = oauthapi.GroupName
	legacyOauthGroup    = oauthapi.LegacyGroupName
	networkGroup        = networkapi.GroupName
	legacyNetworkGroup  = networkapi.LegacyGroupName
)

func GetBootstrapOpenshiftRoles(openshiftNamespace string) []authorizationapi.Role {
	roles := []authorizationapi.Role{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(templateGroup, legacyTemplateGroup).Resources("templates").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams", "imagestreamtags", "imagestreamimages").RuleOrDie(),
				// so anyone can pull from openshift/* image streams
				authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
			},
		},
	}

	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.NormalizeResources(roles[i].Rules[j].Resources)
		}
	}

	return roles

}

func GetOpenshiftBootstrapClusterRoles() []authorizationapi.ClusterRole {
	// four resource can be a single line
	// up to ten-ish resources per line otherwise

	roles := []authorizationapi.ClusterRole{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ClusterAdminRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A super-user that can perform any action in the cluster. When granted to a user within a project, they have full control over quota and membership and can perform every action on every resource in the project.",
					roleSystemOnly:            roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("*").Groups("*").Resources("*").RuleOrDie(),
				{
					Verbs:           sets.NewString(authorizationapi.VerbAll),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SudoerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("impersonate").Groups(userGroup, legacyUserGroup).Resources(authorizationapi.SystemUserResource).Names(SystemAdminUsername).RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ClusterReaderRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("bindings", "componentstatuses", "configmaps", "endpoints", "events", "limitranges",
					"namespaces", "namespaces/status", "nodes", "nodes/status", "persistentvolumeclaims", "persistentvolumeclaims/status", "persistentvolumes",
					"persistentvolumes/status", "pods", "pods/binding", "pods/eviction", "pods/log", "pods/status", "podtemplates", "replicationcontrollers", "replicationcontrollers/scale",
					"replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "securitycontextconstraints", "serviceaccounts", "services",
					"services/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(appsGroup).Resources("statefulsets", "statefulsets/status", "deployments", "deployments/scale", "deployments/status", "controllerrevisions").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(apiExtensionsGroup).Resources("customresourcedefinitions", "customresourcedefinitions/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(apiRegistrationGroup).Resources("apiservices", "apiservices/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers", "horizontalpodautoscalers/status").RuleOrDie(),

				// TODO do we still need scheduledjobs?
				authorizationapi.NewRule(read...).Groups(batchGroup).Resources("jobs", "jobs/status", "scheduledjobs", "scheduledjobs/status", "cronjobs", "cronjobs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets", "daemonsets/status", "deployments", "deployments/scale",
					"deployments/status", "horizontalpodautoscalers", "horizontalpodautoscalers/status", "ingresses", "ingresses/status", "jobs", "jobs/status",
					"networkpolicies", "podsecuritypolicies", "replicasets", "replicasets/scale", "replicasets/status", "replicationcontrollers",
					"replicationcontrollers/scale", "storageclasses", "thirdpartyresources").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(networkingGroup).Resources("networkpolicies").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(policyGroup).Resources("poddisruptionbudgets", "poddisruptionbudgets/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(rbacGroup).Resources("roles", "rolebindings", "clusterroles", "clusterrolebindings").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(settingsGroup).Resources("podpresets").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(certificatesGroup).Resources("certificatesigningrequests", "certificatesigningrequests/approval", "certificatesigningrequests/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(authzGroup, legacyAuthzGroup).Resources("clusterpolicies", "clusterpolicybindings", "clusterroles", "clusterrolebindings",
					"policies", "policybindings", "roles", "rolebindings", "rolebindingrestrictions").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds", "builds/details", "buildconfigs", "buildconfigs/webhooks", "builds/log").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs", "deploymentconfigs/scale", "deploymentconfigs/log",
					"deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("images", "imagesignatures", "imagestreams", "imagestreamtags", "imagestreamimages",
					"imagestreams/status").RuleOrDie(),
				// pull images
				authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(oauthGroup, legacyOauthGroup).Resources("oauthclientauthorizations").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(projectGroup, legacyProjectGroup).Resources("projectrequests", "projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup, legacyQuotaGroup).Resources("appliedclusterresourcequotas", "clusterresourcequotas", "clusterresourcequotas/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(routeGroup, legacyRouteGroup).Resources("routes", "routes/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(networkGroup, legacyNetworkGroup).Resources("clusternetworks", "egressnetworkpolicies", "hostsubnets", "netnamespaces").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(securityGroup, legacySecurityGroup).Resources("securitycontextconstraints").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(templateGroup, legacyTemplateGroup).Resources("templates", "templateconfigs", "processedtemplates", "templateinstances").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(userGroup, legacyUserGroup).Resources("groups", "identities", "useridentitymappings", "users").RuleOrDie(),

				// permissions to check access.  These creates are non-mutating
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "resourceaccessreviews",
					"selfsubjectrulesreviews", "subjectrulesreviews", "subjectaccessreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("selfsubjectaccessreviews", "subjectaccessreviews", "localsubjectaccessreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthnGroup).Resources("tokenreviews").RuleOrDie(),
				// permissions to check PSP, these creates are non-mutating
				authorizationapi.NewRule("create").Groups(securityGroup, legacySecurityGroup).Resources("podsecuritypolicysubjectreviews", "podsecuritypolicyselfsubjectreviews", "podsecuritypolicyreviews").RuleOrDie(),
				// Allow read access to node metrics
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("nodes/"+authorizationapi.NodeMetricsSubresource, "nodes/"+authorizationapi.NodeSpecSubresource).RuleOrDie(),
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				authorizationapi.NewRule("get", "create").Groups(kapiGroup).Resources("nodes/" + authorizationapi.NodeStatsSubresource).RuleOrDie(),

				{
					Verbs:           sets.NewString("get"),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},

				// backwards compatibility
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ClusterDebuggerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:           sets.NewString("get"),
					NonResourceURLs: sets.NewString("/metrics", "/debug/pprof", "/debug/pprof/*"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: BuildStrategyDockerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(authorizationapi.DockerBuildResource, authorizationapi.OptimizedDockerBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: BuildStrategyCustomRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(authorizationapi.CustomBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: BuildStrategySourceRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(authorizationapi.SourceBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: BuildStrategyJenkinsPipelineRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(authorizationapi.JenkinsPipelineBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: StorageAdminRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("persistentvolumes").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("persistentvolumeclaims", "events").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: AdminRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user that has edit rights within the project and can change the project's membership.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("pods", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("replicationcontrollers", "replicationcontrollers/scale", "serviceaccounts",
					"services", "services/proxy", "endpoints", "persistentvolumeclaims", "configmaps", "secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"namespaces", "pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),
				authorizationapi.NewRule("impersonate").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(batchGroup).Resources("jobs", "scheduledjobs", "cronjobs").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(extensionsGroup).Resources("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale",
					"replicasets", "replicasets/scale", "deployments", "deployments/scale", "deployments/rollback", "networkpolicies").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(appsGroup).Resources("statefulsets", "deployments", "deployments/scale", "deployments/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(authzGroup, legacyAuthzGroup).Resources("roles", "rolebindings").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "subjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(securityGroup, legacySecurityGroup).Resources("podsecuritypolicysubjectreviews", "podsecuritypolicyselfsubjectreviews", "podsecuritypolicyreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("localsubjectaccessreviews").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(authzGroup, legacyAuthzGroup).Resources("policies", "policybindings", "rolebindingrestrictions").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds/log").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/clone").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(buildGroup, legacyBuildGroup).Resources("builds/details").RuleOrDie(),
				// access to jenkins.  multiple values to ensure that covers relationships
				authorizationapi.NewRule("admin", "edit", "view").Groups(buildapi.GroupName).Resources("jenkins").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigs/scale").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigrollbacks", "deploymentconfigs/rollback", "deploymentconfigs/instantiate").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/log", "deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages", "imagestreams/secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams/status").RuleOrDie(),
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimports").RuleOrDie(),

				authorizationapi.NewRule("get", "patch", "update", "delete").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup, legacyQuotaGroup).Resources("appliedclusterresourcequotas").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(routeGroup, legacyRouteGroup).Resources("routes").RuleOrDie(),
				// admins can create routes with custom hosts
				authorizationapi.NewRule("create").Groups(routeGroup, legacyRouteGroup).Resources("routes/custom-host").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(routeGroup, legacyRouteGroup).Resources("routes/status").RuleOrDie(),
				// an admin can run routers that write back conditions to the route
				authorizationapi.NewRule("update").Groups(routeGroup, legacyRouteGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(templateGroup, legacyTemplateGroup).Resources("templates", "templateconfigs", "processedtemplates", "templateinstances").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("resourceaccessreviews", "subjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: EditRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user that can create and edit most objects in a project, but can not update the project's membership.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("pods", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("replicationcontrollers", "replicationcontrollers/scale", "serviceaccounts",
					"services", "services/proxy", "endpoints", "persistentvolumeclaims", "configmaps", "secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"namespaces", "pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),
				authorizationapi.NewRule("impersonate").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(batchGroup).Resources("jobs", "scheduledjobs", "cronjobs").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(extensionsGroup).Resources("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale",
					"replicasets", "replicasets/scale", "deployments", "deployments/scale", "deployments/rollback").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(appsGroup).Resources("statefulsets", "deployments", "deployments/scale", "deployments/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds/log").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/clone").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(buildGroup, legacyBuildGroup).Resources("builds/details").RuleOrDie(),
				// access to jenkins.  multiple values to ensure that covers relationships
				authorizationapi.NewRule("edit", "view").Groups(buildapi.GroupName).Resources("jenkins").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigs/scale").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigrollbacks", "deploymentconfigs/rollback", "deploymentconfigs/instantiate").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/log", "deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages", "imagestreams/secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams/status").RuleOrDie(),
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimports").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup, legacyQuotaGroup).Resources("appliedclusterresourcequotas").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(routeGroup, legacyRouteGroup).Resources("routes").RuleOrDie(),
				// editors can create routes with custom hosts
				authorizationapi.NewRule("create").Groups(routeGroup, legacyRouteGroup).Resources("routes/custom-host").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(routeGroup, legacyRouteGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(templateGroup, legacyTemplateGroup).Resources("templates", "templateconfigs", "processedtemplates", "templateinstances").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ViewRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user who can view but not edit any resources within the project. They can not view secrets or membership.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// TODO add "replicationcontrollers/scale" here
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("pods", "replicationcontrollers", "serviceaccounts",
					"services", "endpoints", "persistentvolumeclaims", "configmaps").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"namespaces", "pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(batchGroup).Resources("jobs", "scheduledjobs", "cronjobs").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("jobs", "horizontalpodautoscalers", "replicasets", "replicasets/scale",
					"deployments", "deployments/scale").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(appsGroup).Resources("statefulsets", "deployments", "deployments/scale").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds/log").RuleOrDie(),
				// access to jenkins
				authorizationapi.NewRule("view").Groups(buildapi.GroupName).Resources("jenkins").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs", "deploymentconfigs/scale").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs/log", "deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams/status").RuleOrDie(),
				// TODO let them pull images?
				// pull images
				// authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup, legacyQuotaGroup).Resources("appliedclusterresourcequotas").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(routeGroup, legacyRouteGroup).Resources("routes").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(routeGroup, legacyRouteGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(templateGroup, legacyTemplateGroup).Resources("templates", "templateconfigs", "processedtemplates", "templateinstances").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: BasicUserRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user that can get basic information about projects.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get").Groups(userGroup, legacyUserGroup).Resources("users").Names("~").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(authzGroup, legacyAuthzGroup).Resources("clusterroles").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				authorizationapi.NewRule("list", "watch").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("selfsubjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SelfAccessReviewerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("selfsubjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SelfProvisionerRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user that can request projects.",
					roleSystemOnly:            roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: StatusCheckerRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user that can get basic cluster status information.",
					roleSystemOnly:            roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs: sets.NewString("get"),
					NonResourceURLs: sets.NewString(
						// Health
						"/healthz", "/healthz/*",
					),
				},
				authorizationapi.DiscoveryRule,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImageAuditorRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "patch", "update").Groups(imageGroup, legacyImageGroup).Resources("images").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImagePullerRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "Grants the right to pull images from within a project.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// pull images
				authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
			},
		},
		{
			// This role looks like a duplicate of ImageBuilderRole, but the ImageBuilder role is specifically for our builder service accounts
			// if we found another permission needed by them, we'd add it there so the intent is different if you used the ImageBuilderRole
			// you could end up accidentally granting more permissions than you intended.  This is intended to only grant enough powers to
			// push an image to our registry
			ObjectMeta: metav1.ObjectMeta{
				Name: ImagePusherRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "Grants the right to push and pull images from within a project.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImageBuilderRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "Grants the right to build, push and pull images from within a project.  Used primarily with service accounts for builds.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
				// allow auto-provisioning when pushing an image that doesn't have an imagestream yet
				authorizationapi.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreams").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(buildGroup, legacyBuildGroup).Resources("builds/details").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(buildGroup, legacyBuildGroup).Resources("builds").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImagePrunerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list").Groups(kapiGroup).Resources("pods", "replicationcontrollers").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(kapiGroup).Resources("limitranges").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs", "builds").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(deployGroup, legacyDeployGroup).Resources("deploymentconfigs").RuleOrDie(),

				authorizationapi.NewRule("delete").Groups(imageGroup, legacyImageGroup).Resources("images").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(imageGroup, legacyImageGroup).Resources("images", "imagestreams").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/status").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ImageSignerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("images", "imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule("create", "delete").Groups(imageGroup, legacyImageGroup).Resources("imagesignatures").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: DeployerRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "Grants the right to deploy within a project.  Used primarily with service accounts for automated deployments.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// "delete" is required here for compatibility with older deployer images
				// (see https://github.com/openshift/origin/pull/14322#issuecomment-303968976)
				// TODO: remove "delete" rule few releases after 3.6
				authorizationapi.NewRule("delete").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
				authorizationapi.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
				authorizationapi.NewRule("get", "list", "watch", "create").Groups(kapiGroup).Resources("pods").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("pods/log").RuleOrDie(),
				authorizationapi.NewRule("create", "list").Groups(kapiGroup).Resources("events").RuleOrDie(),

				authorizationapi.NewRule("update").Groups(imageGroup, legacyImageGroup).Resources("imagestreamtags").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: MasterRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("*").Groups("*").Resources("*").RuleOrDie(),
				{
					Verbs:           sets.NewString(authorizationapi.VerbAll),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: OAuthTokenDeleterRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("delete").Groups(oauthGroup, legacyOauthGroup).Resources("oauthaccesstokens", "oauthauthorizetokens").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: RouterRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("list", "watch").Groups(kapiGroup).Resources("endpoints").RuleOrDie(),
				authorizationapi.NewRule("list", "watch").Groups(kapiGroup).Resources("services").RuleOrDie(),

				authorizationapi.NewRule("list", "watch").Groups(routeGroup, legacyRouteGroup).Resources("routes").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(routeGroup, legacyRouteGroup).Resources("routes/status").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: RegistryRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("list").Groups(kapiGroup).Resources("limitranges", "resourcequotas").RuleOrDie(),

				authorizationapi.NewRule("get", "delete").Groups(imageGroup, legacyImageGroup).Resources("images", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimages", "imagestreams/secrets").RuleOrDie(),
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("images", "imagestreams").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreammappings").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeProxierRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// Used to build serviceLister
				authorizationapi.NewRule("list", "watch").Groups(kapiGroup).Resources("services", "endpoints").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeAdminRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				// Allow all API calls to the nodes
				authorizationapi.NewRule("proxy").Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				authorizationapi.NewRule("*").Groups(kapiGroup).Resources("nodes/proxy", "nodes/"+authorizationapi.NodeMetricsSubresource, "nodes/"+authorizationapi.NodeSpecSubresource, "nodes/"+authorizationapi.NodeStatsSubresource, "nodes/"+authorizationapi.NodeLogSubresource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeReaderRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				// Allow read access to node metrics
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("nodes/"+authorizationapi.NodeMetricsSubresource, "nodes/"+authorizationapi.NodeSpecSubresource).RuleOrDie(),
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				authorizationapi.NewRule("get", "create").Groups(kapiGroup).Resources("nodes/" + authorizationapi.NodeStatsSubresource).RuleOrDie(),
				// TODO: expose other things like /healthz on the node once we figure out non-resource URL policy across systems
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				// Needed to check API access.  These creates are non-mutating
				authorizationapi.NewRule("create").Groups(kAuthnGroup).Resources("tokenreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("subjectaccessreviews", "localsubjectaccessreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("subjectaccessreviews", "localsubjectaccessreviews").RuleOrDie(),
				// Needed to build serviceLister, to populate env vars for services
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("services").RuleOrDie(),
				// Nodes can register themselves
				// TODO: restrict to creating a node with the same name they announce
				authorizationapi.NewRule("create", "get", "list", "watch").Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				// TODO: restrict to the bound node once supported
				authorizationapi.NewRule("update", "patch").Groups(kapiGroup).Resources("nodes/status").RuleOrDie(),

				// TODO: restrict to the bound node as creator once supported
				authorizationapi.NewRule("create", "update", "patch").Groups(kapiGroup).Resources("events").RuleOrDie(),

				// TODO: restrict to pods scheduled on the bound node once supported
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("pods").RuleOrDie(),

				// TODO: remove once mirror pods are removed
				// TODO: restrict deletion to mirror pods created by the bound node once supported
				// Needed for the node to create/delete mirror pods
				authorizationapi.NewRule("get", "create", "delete").Groups(kapiGroup).Resources("pods").RuleOrDie(),
				// TODO: restrict to pods scheduled on the bound node once supported
				authorizationapi.NewRule("update").Groups(kapiGroup).Resources("pods/status").RuleOrDie(),

				// TODO: restrict to secrets and configmaps used by pods scheduled on bound node once supported
				// Needed for imagepullsecrets, rbd/ceph and secret volumes, and secrets in envs
				// Needed for configmap volume and envs
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("secrets", "configmaps").RuleOrDie(),
				// TODO: restrict to claims/volumes used by pods scheduled on bound node once supported
				// Needed for persistent volumes
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("persistentvolumeclaims", "persistentvolumes").RuleOrDie(),
				// TODO: restrict to namespaces of pods scheduled on bound node once supported
				// TODO: change glusterfs to use DNS lookup so this isn't needed?
				// Needed for glusterfs volumes
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("endpoints").RuleOrDie(),
				// Nodes are allowed to request CSRs (specifically, request serving certs)
				authorizationapi.NewRule("get", "create").Groups(certificates.GroupName).Resources("certificatesigningrequests").RuleOrDie(),
			},
		},

		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SDNReaderRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(networkGroup, legacyNetworkGroup).Resources("egressnetworkpolicies", "hostsubnets", "netnamespaces").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes", "namespaces").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("networkpolicies").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(networkGroup, legacyNetworkGroup).Resources("clusternetworks").RuleOrDie(),
			},
		},

		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SDNManagerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "create", "delete").Groups(networkGroup, legacyNetworkGroup).Resources("hostsubnets", "netnamespaces").RuleOrDie(),
				authorizationapi.NewRule("get", "create").Groups(networkGroup, legacyNetworkGroup).Resources("clusternetworks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes").RuleOrDie(),
			},
		},

		{
			ObjectMeta: metav1.ObjectMeta{
				Name: WebHooksRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/webhooks").RuleOrDie(),
			},
		},

		{
			ObjectMeta: metav1.ObjectMeta{
				Name: DiscoveryRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.DiscoveryRule,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: PersistentVolumeProvisionerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "create", "delete").Groups(kapiGroup).Resources("persistentvolumes").RuleOrDie(),
				// update is needed in addition to read access for setting lock annotations on PVCs
				authorizationapi.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("persistentvolumeclaims").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				// Needed for watching provisioning success and failure events
				authorizationapi.NewRule("create", "update", "patch", "list", "watch").Groups(kapiGroup).Resources("events").RuleOrDie(),
			},
		},

		{
			ObjectMeta: metav1.ObjectMeta{
				Name: RegistryAdminRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("serviceaccounts", "secrets").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(imageGroup, legacyImageGroup).Resources("imagestreamimages", "imagestreammappings", "imagestreams", "imagestreams/secrets", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimports").RuleOrDie(),
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(authzGroup, legacyAuthzGroup).Resources("rolebindings", "roles").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "subjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("localsubjectaccessreviews").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(authzGroup, legacyAuthzGroup).Resources("policies", "policybindings").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
				authorizationapi.NewRule("get", "delete").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("resourceaccessreviews", "subjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: RegistryEditorRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("serviceaccounts", "secrets").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(imageGroup, legacyImageGroup).Resources("imagestreamimages", "imagestreammappings", "imagestreams", "imagestreams/secrets", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup, legacyImageGroup).Resources("imagestreamimports").RuleOrDie(),
				authorizationapi.NewRule("get", "update").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: RegistryViewerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreamimages", "imagestreammappings", "imagestreams", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: TemplateServiceBrokerClientRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				{
					Verbs:           sets.NewString("get", "put", "update", "delete"),
					NonResourceURLs: sets.NewString(templateapi.ServiceBrokerRoot + "/*"),
				},
			},
		},
	}

	// TODO check if we really need to do this
	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.NormalizeResources(roles[i].Rules[j].Resources)
		}
	}

	return roles
}

func GetBootstrapClusterRoles() []authorizationapi.ClusterRole {
	openshiftClusterRoles := GetOpenshiftBootstrapClusterRoles()
	// dead cluster roles need to be checked for conflicts (in case something new comes up)
	// so add them to this list.
	openshiftClusterRoles = append(openshiftClusterRoles, GetDeadClusterRoles()...)
	kubeClusterRoles, err := GetKubeBootstrapClusterRoles()
	// coder error
	if err != nil {
		panic(err)
	}
	kubeSAClusterRoles, err := GetKubeControllerBootstrapClusterRoles()
	// coder error
	if err != nil {
		panic(err)
	}
	openshiftControllerRoles, err := GetOpenshiftControllerBootstrapClusterRoles()
	// coder error
	if err != nil {
		panic(err)
	}

	// Eventually openshift controllers and kube controllers have different prefixes
	// so we will only need to check conflicts on the "normal" cluster roles
	// for now, deconflict with all names
	openshiftClusterRoleNames := sets.NewString()
	kubeClusterRoleNames := sets.NewString()
	for _, clusterRole := range openshiftClusterRoles {
		openshiftClusterRoleNames.Insert(clusterRole.Name)
	}
	for _, clusterRole := range kubeClusterRoles {
		kubeClusterRoleNames.Insert(clusterRole.Name)
	}

	conflictingNames := kubeClusterRoleNames.Intersection(openshiftClusterRoleNames)
	extraRBACConflicts := conflictingNames.Difference(clusterRoleConflicts)
	extraWhitelistEntries := clusterRoleConflicts.Difference(conflictingNames)
	switch {
	case len(extraRBACConflicts) > 0 && len(extraWhitelistEntries) > 0:
		panic(fmt.Sprintf("kube ClusterRoles conflict with openshift ClusterRoles: %v and ClusterRole whitelist contains a extraneous entries: %v ", extraRBACConflicts.List(), extraWhitelistEntries.List()))
	case len(extraRBACConflicts) > 0:
		panic(fmt.Sprintf("kube ClusterRoles conflict with openshift ClusterRoles: %v", extraRBACConflicts.List()))
	case len(extraWhitelistEntries) > 0:
		panic(fmt.Sprintf("ClusterRole whitelist contains a extraneous entries: %v", extraWhitelistEntries.List()))
	}

	finalClusterRoles := []authorizationapi.ClusterRole{}
	finalClusterRoles = append(finalClusterRoles, openshiftClusterRoles...)
	finalClusterRoles = append(finalClusterRoles, openshiftControllerRoles...)
	finalClusterRoles = append(finalClusterRoles, kubeSAClusterRoles...)
	for i := range kubeClusterRoles {
		if !clusterRoleConflicts.Has(kubeClusterRoles[i].Name) {
			finalClusterRoles = append(finalClusterRoles, kubeClusterRoles[i])
		}
	}

	return finalClusterRoles
}

func GetBootstrapOpenshiftRoleBindings(openshiftNamespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleBindingName,
				Namespace: openshiftNamespace,
			},
			RoleRef: kapi.ObjectReference{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
	}
}

func GetOpenshiftBootstrapClusterRoleBindings() []authorizationapi.ClusterRoleBinding {
	return []authorizationapi.ClusterRoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: MasterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: MasterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: MastersGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeAdminRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeAdminRoleName,
			},
			Subjects: []kapi.ObjectReference{
				// ensure the legacy username in the master's kubelet-client certificate is allowed
				{Kind: authorizationapi.SystemUserKind, Name: LegacyMasterKubeletAdminClientUsername},
				{Kind: authorizationapi.SystemGroupKind, Name: NodeAdminsGroup},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ClusterAdminRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterAdminRoleName,
			},
			Subjects: []kapi.ObjectReference{
				{Kind: authorizationapi.SystemGroupKind, Name: ClusterAdminGroup},
				// add system:admin to this binding so that members of the sudoer group can use --as=system:admin to run a command as a cluster-admin
				{Kind: authorizationapi.SystemUserKind, Name: SystemAdminUsername},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: ClusterReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterReaderRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: ClusterReaderGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: BasicUserRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: BasicUserRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SelfAccessReviewerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SelfAccessReviewerRoleName,
			},
			Subjects: []kapi.ObjectReference{
				{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup},
				{Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SelfProvisionerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SelfProvisionerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedOAuthGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: OAuthTokenDeleterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: OAuthTokenDeleterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: StatusCheckerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: StatusCheckerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: NodeProxierRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeProxierRoleName,
			},
			// Allow node identities to run node proxies
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: SDNReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SDNReaderRoleName,
			},
			// Allow node identities to run SDN plugins
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: WebHooksRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: WebHooksRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: DiscoveryRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: DiscoveryRoleName,
			},
			Subjects: []kapi.ObjectReference{
				{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup},
				{Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup},
			},
		},

		// Allow all build strategies by default.
		// Cluster admins can remove these role bindings, and the reconcile-cluster-role-bindings command
		// run during an upgrade won't re-add the "system:authenticated" group
		{
			ObjectMeta: metav1.ObjectMeta{Name: BuildStrategyDockerRoleBindingName},
			RoleRef:    kapi.ObjectReference{Name: BuildStrategyDockerRoleName},
			Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: BuildStrategySourceRoleBindingName},
			RoleRef:    kapi.ObjectReference{Name: BuildStrategySourceRoleName},
			Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: BuildStrategyJenkinsPipelineRoleBindingName},
			RoleRef:    kapi.ObjectReference{Name: BuildStrategyJenkinsPipelineRoleName},
			Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
	}
}

func GetBootstrapClusterRoleBindings() []authorizationapi.ClusterRoleBinding {
	openshiftClusterRoleBindings := GetOpenshiftBootstrapClusterRoleBindings()
	kubeClusterRoleBindings, err := GetKubeBootstrapClusterRoleBindings()
	// coder error
	if err != nil {
		panic(err)
	}
	kubeControllerClusterRoleBindings, err := GetKubeControllerBootstrapClusterRoleBindings()
	// coder error
	if err != nil {
		panic(err)
	}
	openshiftControllerClusterRoleBindings, err := GetOpenshiftControllerBootstrapClusterRoleBindings()
	// coder error
	if err != nil {
		panic(err)
	}

	// openshift controllers and kube controllers have different prefixes
	// so we only need to check conflicts on the "normal" cluster rolebindings
	openshiftClusterRoleBindingNames := sets.NewString()
	kubeClusterRoleBindingNames := sets.NewString()
	for _, clusterRoleBinding := range openshiftClusterRoleBindings {
		openshiftClusterRoleBindingNames.Insert(clusterRoleBinding.Name)
	}
	for _, clusterRoleBinding := range kubeClusterRoleBindings {
		kubeClusterRoleBindingNames.Insert(clusterRoleBinding.Name)
	}

	conflictingNames := kubeClusterRoleBindingNames.Intersection(openshiftClusterRoleBindingNames)
	extraRBACConflicts := conflictingNames.Difference(clusterRoleBindingConflicts)
	extraWhitelistEntries := clusterRoleBindingConflicts.Difference(conflictingNames)
	switch {
	case len(extraRBACConflicts) > 0 && len(extraWhitelistEntries) > 0:
		panic(fmt.Sprintf("kube ClusterRoleBindings conflict with openshift ClusterRoleBindings: %v and ClusterRoleBinding whitelist contains a extraneous entries: %v ", extraRBACConflicts.List(), extraWhitelistEntries.List()))
	case len(extraRBACConflicts) > 0:
		panic(fmt.Sprintf("kube ClusterRoleBindings conflict with openshift ClusterRoleBindings: %v", extraRBACConflicts.List()))
	case len(extraWhitelistEntries) > 0:
		panic(fmt.Sprintf("ClusterRoleBinding whitelist contains a extraneous entries: %v", extraWhitelistEntries.List()))
	}

	finalClusterRoleBindings := []authorizationapi.ClusterRoleBinding{}
	finalClusterRoleBindings = append(finalClusterRoleBindings, openshiftClusterRoleBindings...)
	finalClusterRoleBindings = append(finalClusterRoleBindings, kubeControllerClusterRoleBindings...)
	finalClusterRoleBindings = append(finalClusterRoleBindings, openshiftControllerClusterRoleBindings...)
	for i := range kubeClusterRoleBindings {
		if !clusterRoleBindingConflicts.Has(kubeClusterRoleBindings[i].Name) {
			finalClusterRoleBindings = append(finalClusterRoleBindings, kubeClusterRoleBindings[i])
		}
	}

	return finalClusterRoleBindings
}

// clusterRoleConflicts lists the roles which are known to conflict with upstream and which we have manually
// deconflicted with our own.
var clusterRoleConflicts = sets.NewString(
	// these require special treatment to handle origin resources
	"admin",
	"edit",
	"view",

	// TODO this should probably be re-swizzled to be the delta on top of the kube role
	"system:discovery",

	// TODO these should be reconsidered
	"cluster-admin",
	"system:node",
	"system:node-proxier",
	"system:persistent-volume-provisioner",
)

// clusterRoleBindingConflicts lists the roles which are known to conflict with upstream and which we have manually
// deconflicted with our own.
var clusterRoleBindingConflicts = sets.NewString()

func GetKubeBootstrapClusterRoleBindings() ([]authorizationapi.ClusterRoleBinding, error) {
	return convertClusterRoleBindings(bootstrappolicy.ClusterRoleBindings())
}

func GetKubeControllerBootstrapClusterRoleBindings() ([]authorizationapi.ClusterRoleBinding, error) {
	return convertClusterRoleBindings(bootstrappolicy.ControllerRoleBindings())
}

func GetOpenshiftControllerBootstrapClusterRoleBindings() ([]authorizationapi.ClusterRoleBinding, error) {
	return convertClusterRoleBindings(ControllerRoleBindings())
}

func convertClusterRoleBindings(in []rbac.ClusterRoleBinding) ([]authorizationapi.ClusterRoleBinding, error) {
	out := []authorizationapi.ClusterRoleBinding{}
	errs := []error{}

	for i := range in {
		newRoleBinding := &authorizationapi.ClusterRoleBinding{}
		if err := kapi.Scheme.Convert(&in[i], newRoleBinding, nil); err != nil {
			errs = append(errs, fmt.Errorf("error converting %q: %v", in[i].Name, err))
			continue
		}
		out = append(out, *newRoleBinding)
	}

	return out, kutilerrors.NewAggregate(errs)
}

func GetKubeBootstrapClusterRoles() ([]authorizationapi.ClusterRole, error) {
	return convertClusterRoles(bootstrappolicy.ClusterRoles())
}

func GetKubeControllerBootstrapClusterRoles() ([]authorizationapi.ClusterRole, error) {
	return convertClusterRoles(bootstrappolicy.ControllerRoles())
}

func GetOpenshiftControllerBootstrapClusterRoles() ([]authorizationapi.ClusterRole, error) {
	return convertClusterRoles(ControllerRoles())
}

func convertClusterRoles(in []rbac.ClusterRole) ([]authorizationapi.ClusterRole, error) {
	out := []authorizationapi.ClusterRole{}
	errs := []error{}

	for i := range in {
		newRole := &authorizationapi.ClusterRole{}
		if err := kapi.Scheme.Convert(&in[i], newRole, nil); err != nil {
			errs = append(errs, fmt.Errorf("error converting %q: %v", in[i].Name, err))
			continue
		}
		// adding annotation to any role not explicitly in the whitelist below
		if !rolesToShow.Has(newRole.Name) {
			newRole.Annotations[roleSystemOnly] = roleIsSystemOnly
		}
		out = append(out, *newRole)
	}

	return out, kutilerrors.NewAggregate(errs)
}

// The current list of roles considered useful for normal users (non-admin)
var rolesToShow = sets.NewString(
	"admin",
	"basic-user",
	"edit",
	"system:deployer",
	"system:image-builder",
	"system:image-puller",
	"system:image-pusher",
	"view",
)
