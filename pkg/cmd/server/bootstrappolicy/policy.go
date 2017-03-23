package bootstrappolicy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/apps"
	kauthenticationapi "k8s.io/kubernetes/pkg/apis/authentication"
	kauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/apis/storage"
	"k8s.io/kubernetes/pkg/util/sets"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	networkapi "github.com/openshift/origin/pkg/sdn/api"
	securityapi "github.com/openshift/origin/pkg/security/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
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

	kapiGroup           = kapi.GroupName
	appsGroup           = apps.GroupName
	autoscalingGroup    = autoscaling.GroupName
	batchGroup          = batch.GroupName
	certificatesGroup   = certificates.GroupName
	extensionsGroup     = extensions.GroupName
	policyGroup         = policy.GroupName
	securityGroup       = securityapi.GroupName
	legacySecurityGroup = securityapi.LegacyGroupName
	storageGroup        = storage.GroupName

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
			ObjectMeta: kapi.ObjectMeta{
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

func GetBootstrapClusterRoles() []authorizationapi.ClusterRole {

	// four resource can be a single line
	// up to ten-ish resources per line otherwise

	roles := []authorizationapi.ClusterRole{
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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

				authorizationapi.NewRule(read...).Groups(appsGroup).Resources("statefulsets", "statefulsets/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers", "horizontalpodautoscalers/status").RuleOrDie(),

				// TODO do we still need scheduledjobs?
				authorizationapi.NewRule(read...).Groups(batchGroup).Resources("jobs", "jobs/status", "scheduledjobs", "scheduledjobs/status", "cronjobs", "cronjobs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets", "daemonsets/status", "deployments", "deployments/scale",
					"deployments/status", "horizontalpodautoscalers", "horizontalpodautoscalers/status", "ingresses", "ingresses/status", "jobs", "jobs/status",
					"networkpolicies", "podsecuritypolicies", "replicasets", "replicasets/scale", "replicasets/status", "replicationcontrollers",
					"replicationcontrollers/scale", "storageclasses", "thirdpartyresources").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(policyGroup).Resources("poddisruptionbudgets", "poddisruptionbudgets/status").RuleOrDie(),

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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
				Name: BuildStrategyDockerRoleName,
				Annotations: map[string]string{
					roleSystemOnly: roleIsSystemOnly,
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(authorizationapi.DockerBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
				Name: StorageAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("persistentvolumes").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("persistentvolumeclaims", "events").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
					"replicasets", "replicasets/scale", "deployments", "deployments/scale", "deployments/rollback").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(appsGroup).Resources("statefulsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(authzGroup, legacyAuthzGroup).Resources("roles", "rolebindings").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "subjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(securityGroup, legacySecurityGroup).Resources("podsecuritypolicysubjectreviews", "podsecuritypolicyselfsubjectreviews", "podsecuritypolicyreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("localsubjectaccessreviews").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(authzGroup, legacyAuthzGroup).Resources("policies", "policybindings", "rolebindingrestrictions").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds/log").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/clone").RuleOrDie(),
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
			ObjectMeta: kapi.ObjectMeta{
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

				authorizationapi.NewRule(readWrite...).Groups(appsGroup).Resources("statefulsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup, legacyBuildGroup).Resources("builds/log").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/clone").RuleOrDie(),
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
				authorizationapi.NewRule(read...).Groups(routeGroup, legacyRouteGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(templateGroup, legacyTemplateGroup).Resources("templates", "templateconfigs", "processedtemplates", "templateinstances").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(readWrite...).Groups(buildGroup, legacyBuildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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

				authorizationapi.NewRule(read...).Groups(appsGroup).Resources("statefulsets").RuleOrDie(),

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
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "A user that can get basic information about projects.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get").Groups(userGroup, legacyUserGroup).Resources("users").Names("~").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(authzGroup, legacyAuthzGroup).Resources("clusterroles").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				authorizationapi.NewRule("list", "watch").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(kAuthzGroup).Resources("selfsubjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
				Name: DeployerRoleName,
				Annotations: map[string]string{
					oapi.OpenShiftDescription: "Grants the right to deploy within a project.  Used primarily with service accounts for automated deployments.",
				},
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
				authorizationapi.NewRule("get", "list", "watch", "create").Groups(kapiGroup).Resources("pods").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("pods/log").RuleOrDie(),
				authorizationapi.NewRule("create", "list").Groups(kapiGroup).Resources("events").RuleOrDie(),

				authorizationapi.NewRule("update").Groups(imageGroup, legacyImageGroup).Resources("imagestreamtags").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
				authorizationapi.NewRule("update").Groups(kapiGroup).Resources("nodes/status").RuleOrDie(),

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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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

	saRoles := InfraSAs.AllRoles()
	for _, saRole := range saRoles {
		for _, existingRole := range roles {
			if existingRole.Name == saRole.Name {
				panic(fmt.Sprintf("clusterrole/%s is already registered", existingRole.Name))
			}
		}
	}

	// TODO roundtrip roles to pick up defaulting for API groups.  Without this, the covers check in reconcile-cluster-roles will fail.
	// we can remove this again once everything gets group qualified and we have unit tests enforcing that.  other pulls are in
	// progress to do that.
	// we only want to roundtrip the sa roles now.  We'll remove this once we convert the SA roles
	versionedRoles := []authorizationapiv1.ClusterRole{}
	for i := range saRoles {
		newRole := &authorizationapiv1.ClusterRole{}
		if err := kapi.Scheme.Convert(&saRoles[i], newRole, nil); err != nil {
			panic(err)
		}
		versionedRoles = append(versionedRoles, *newRole)
	}
	roundtrippedRoles := []authorizationapi.ClusterRole{}
	for i := range versionedRoles {
		newRole := &authorizationapi.ClusterRole{}
		if err := kapi.Scheme.Convert(&versionedRoles[i], newRole, nil); err != nil {
			panic(err)
		}
		roundtrippedRoles = append(roundtrippedRoles, *newRole)
	}

	roles = append(roles, roundtrippedRoles...)

	// we don't want to expose the resourcegroups externally because it makes it very difficult for customers to learn from
	// our default roles and hard for them to reason about what power they are granting their users
	for i := range roles {
		for j := range roles[i].Rules {
			roles[i].Rules[j].Resources = authorizationapi.NormalizeResources(roles[i].Rules[j].Resources)
		}
	}

	return roles
}

func GetBootstrapOpenshiftRoleBindings(openshiftNamespace string) []authorizationapi.RoleBinding {
	return []authorizationapi.RoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
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

func GetBootstrapClusterRoleBindings() []authorizationapi.ClusterRoleBinding {
	return []authorizationapi.ClusterRoleBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: MasterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: MasterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: MastersGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: ClusterReaderRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: ClusterReaderGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: BasicUserRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfProvisionerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SelfProvisionerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedOAuthGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OAuthTokenDeleterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: OAuthTokenDeleterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: StatusCheckerRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: StatusCheckerRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: NodeProxierRoleName,
			},
			// Allow node identities to run node proxies
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: SDNReaderRoleName,
			},
			// Allow node identities to run SDN plugins
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: NodesGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: WebHooksRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}, {Kind: authorizationapi.SystemGroupKind, Name: UnauthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
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
			ObjectMeta: kapi.ObjectMeta{Name: BuildStrategyDockerRoleBindingName},
			RoleRef:    kapi.ObjectReference{Name: BuildStrategyDockerRoleName},
			Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{Name: BuildStrategySourceRoleBindingName},
			RoleRef:    kapi.ObjectReference{Name: BuildStrategySourceRoleName},
			Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{Name: BuildStrategyJenkinsPipelineRoleBindingName},
			RoleRef:    kapi.ObjectReference{Name: BuildStrategyJenkinsPipelineRoleName},
			Subjects:   []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: AuthenticatedGroup}},
		},
	}
}
