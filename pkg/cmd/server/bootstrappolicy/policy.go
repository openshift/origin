package bootstrappolicy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/apis/storage"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	securityapi "github.com/openshift/origin/pkg/security/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

var (
	readWrite = []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
	read      = []string{"get", "list", "watch"}

	kapiGroup         = kapi.GroupName
	appsGroup         = apps.GroupName
	autoscalingGroup  = autoscaling.GroupName
	batchGroup        = batch.GroupName
	certificatesGroup = certificates.GroupName
	extensionsGroup   = extensions.GroupName
	policyGroup       = policy.GroupName
	securityGroup     = securityapi.GroupName
	storageGroup      = storage.GroupName
	authzGroup        = authorizationapi.GroupName
	buildGroup        = buildapi.GroupName
	deployGroup       = deployapi.GroupName
	imageGroup        = imageapi.GroupName
	projectGroup      = projectapi.GroupName
	quotaGroup        = quotaapi.GroupName
	routeGroup        = routeapi.GroupName
	templateGroup     = templateapi.GroupName
	userGroup         = userapi.GroupName
	oauthGroup        = oauthapi.GroupName
	sdnGroup          = sdnapi.GroupName
)

func GetBootstrapOpenshiftRoles(openshiftNamespace string) []authorizationapi.Role {
	roles := []authorizationapi.Role{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(templateGroup).Resources("templates").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("imagestreams", "imagestreamtags", "imagestreamimages").RuleOrDie(),
				// so anyone can pull from openshift/* image streams
				authorizationapi.NewRule("get").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
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
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("impersonate").Groups(kapiGroup).Resources(authorizationapi.SystemUserResource).Names(SystemAdminUsername).RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ClusterReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("bindings", "componentstatuses", "configmaps", "egressnetworkpolicies", "endpoints", "events", "limitranges",
					"namespaces", "namespaces/status", "nodes", "nodes/status", "persistentvolumeclaims", "persistentvolumeclaims/status", "persistentvolumes",
					"persistentvolumes/status", "pods", "pods/binding", "pods/eviction", "pods/log", "pods/status", "podtemplates", "replicationcontrollers", "replicationcontrollers/scale",
					"replicationcontrollers/status", "resourcequotas", "resourcequotas/status", "securitycontextconstraints", "serviceaccounts", "services",
					"services/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(appsGroup).Resources("petsets", "petsets/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers", "horizontalpodautoscalers/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(batchGroup).Resources("jobs", "jobs/status", "scheduledjobs", "scheduledjobs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets", "daemonsets/status", "deployments", "deployments/scale",
					"deployments/status", "horizontalpodautoscalers", "horizontalpodautoscalers/status", "ingresses", "ingresses/status", "jobs", "jobs/status",
					"networkpolicies", "podsecuritypolicies", "replicasets", "replicasets/scale", "replicasets/status", "replicationcontrollers",
					"replicationcontrollers/scale", "storageclasses", "thirdpartyresources").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(policyGroup).Resources("poddisruptionbudgets", "poddisruptionbudgets/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(storageGroup).Resources("storageclasses").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(certificatesGroup).Resources("certificatesigningrequests", "certificatesigningrequests/approval", "certificatesigningrequests/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(authzGroup).Resources("clusterpolicies", "clusterpolicybindings", "clusterroles", "clusterrolebindings",
					"policies", "policybindings", "roles", "rolebindings").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("builds", "builds/details", "buildconfigs", "buildconfigs/webhooks", "builds/log").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(deployGroup).Resources("deploymentconfigs", "deploymentconfigs/scale", "deploymentconfigs/log",
					"deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("images", "imagesignatures", "imagestreams", "imagestreamtags", "imagestreamimages",
					"imagestreams/status").RuleOrDie(),
				// pull images
				authorizationapi.NewRule("get").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(oauthGroup).Resources("oauthclientauthorizations").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(projectGroup).Resources("projectrequests", "projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup).Resources("appliedclusterresourcequotas", "clusterresourcequotas", "clusterresourcequotas/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(routeGroup).Resources("routes", "routes/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(sdnGroup).Resources("clusternetworks", "hostsubnets", "netnamespaces").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(templateGroup).Resources("templates", "templateconfigs", "processedtemplates").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(userGroup).Resources("groups", "identities", "useridentitymappings", "users").RuleOrDie(),

				// permissions to check access.  These creates are non-mutating
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "resourceaccessreviews",
					"selfsubjectrulesreviews", "subjectrulesreviews", "subjectaccessreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups("authentication.k8s.io").Resources("tokenreviews").RuleOrDie(),
				// permissions to check PSP, these creates are non-mutating
				authorizationapi.NewRule("create").Groups(securityGroup).Resources("podsecuritypolicysubjectreviews", "podsecuritypolicyselfsubjectreviews", "podsecuritypolicyreviews").RuleOrDie(),
				// Allow read access to node metrics
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources(authorizationapi.NodeMetricsResource, authorizationapi.NodeSpecResource).RuleOrDie(),
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				authorizationapi.NewRule("get", "create").Groups(kapiGroup).Resources(authorizationapi.NodeStatsResource).RuleOrDie(),

				{
					Verbs:           sets.NewString("get"),
					NonResourceURLs: sets.NewString(authorizationapi.NonResourceAll),
				},

				// backwards compatibility
				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BuildStrategyDockerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup).Resources(authorizationapi.DockerBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BuildStrategyCustomRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup).Resources(authorizationapi.CustomBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BuildStrategySourceRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup).Resources(authorizationapi.SourceBuildResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BuildStrategyJenkinsPipelineRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(buildGroup).Resources(authorizationapi.JenkinsPipelineBuildResource).RuleOrDie(),
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
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("pods", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("replicationcontrollers", "replicationcontrollers/scale", "serviceaccounts",
					"services", "services/proxy", "endpoints", "persistentvolumeclaims", "configmaps", "secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"namespaces", "pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),
				authorizationapi.NewRule("impersonate").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(batchGroup).Resources("jobs", "scheduledjobs").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(extensionsGroup).Resources("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale",
					"replicasets", "replicasets/scale", "deployments", "deployments/scale", "deployments/rollback").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(appsGroup).Resources("petsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(authzGroup).Resources("roles", "rolebindings").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "subjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(securityGroup).Resources("podsecuritypolicysubjectreviews", "podsecuritypolicyselfsubjectreviews", "podsecuritypolicyreviews").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(authzGroup).Resources("policies", "policybindings").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(buildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("builds/log").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(buildGroup).Resources("buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/clone").RuleOrDie(),
				// access to jenkins.  multiple values to ensure that covers relationships
				authorizationapi.NewRule("admin", "edit", "view").Groups(buildapi.FutureGroupName).Resources("jenkins").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(deployGroup).Resources("deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigs/scale").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(deployGroup).Resources("deploymentconfigrollbacks", "deploymentconfigs/rollback", "deploymentconfigs/instantiate").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(deployGroup).Resources("deploymentconfigs/log", "deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(imageGroup).Resources("imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages", "imagestreams/secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("imagestreams/status").RuleOrDie(),
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup).Resources("imagestreamimports").RuleOrDie(),

				authorizationapi.NewRule("get", "patch", "update", "delete").Groups(projectGroup).Resources("projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup).Resources("appliedclusterresourcequotas").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(routeGroup).Resources("routes").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(routeGroup).Resources("routes/status").RuleOrDie(),
				// an admin can run routers that write back conditions to the route
				authorizationapi.NewRule("update").Groups(routeGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(templateGroup).Resources("templates", "templateconfigs", "processedtemplates").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(readWrite...).Groups(buildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("resourceaccessreviews", "subjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: EditRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("pods", "pods/attach", "pods/proxy", "pods/exec", "pods/portforward").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("replicationcontrollers", "replicationcontrollers/scale", "serviceaccounts",
					"services", "services/proxy", "endpoints", "persistentvolumeclaims", "configmaps", "secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"namespaces", "pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),
				authorizationapi.NewRule("impersonate").Groups(kapiGroup).Resources("serviceaccounts").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(batchGroup).Resources("jobs", "scheduledjobs").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(extensionsGroup).Resources("jobs", "horizontalpodautoscalers", "replicationcontrollers/scale",
					"replicasets", "replicasets/scale", "deployments", "deployments/scale", "deployments/rollback").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(appsGroup).Resources("petsets").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(buildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("builds/log").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(buildGroup).Resources("buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/clone").RuleOrDie(),
				// access to jenkins.  multiple values to ensure that covers relationships
				authorizationapi.NewRule("edit", "view").Groups(buildapi.FutureGroupName).Resources("jenkins").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(deployGroup).Resources("deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigs/scale").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(deployGroup).Resources("deploymentconfigrollbacks", "deploymentconfigs/rollback", "deploymentconfigs/instantiate").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(deployGroup).Resources("deploymentconfigs/log", "deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(imageGroup).Resources("imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages", "imagestreams/secrets").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("imagestreams/status").RuleOrDie(),
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup).Resources("imagestreamimports").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(projectGroup).Resources("projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup).Resources("appliedclusterresourcequotas").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(routeGroup).Resources("routes").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(routeGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(readWrite...).Groups(templateGroup).Resources("templates", "templateconfigs", "processedtemplates").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(readWrite...).Groups(buildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ViewRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// TODO add "replicationcontrollers/scale" here
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("pods", "replicationcontrollers", "serviceaccounts",
					"services", "endpoints", "persistentvolumeclaims", "configmaps").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("limitranges", "resourcequotas", "bindings", "events",
					"namespaces", "pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status", "pods/log").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(autoscalingGroup).Resources("horizontalpodautoscalers").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(batchGroup).Resources("jobs", "scheduledjobs").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("jobs", "horizontalpodautoscalers", "replicasets", "replicasets/scale",
					"deployments", "deployments/scale").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(extensionsGroup).Resources("daemonsets").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(appsGroup).Resources("petsets").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("builds", "buildconfigs", "buildconfigs/webhooks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("builds/log").RuleOrDie(),
				// access to jenkins
				authorizationapi.NewRule("view").Groups(buildapi.FutureGroupName).Resources("jenkins").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(deployGroup).Resources("deploymentconfigs", "deploymentconfigs/scale").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(deployGroup).Resources("deploymentconfigs/log", "deploymentconfigs/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("imagestreams/status").RuleOrDie(),
				// TODO let them pull images?
				// pull images
				// authorizationapi.NewRule("get").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(projectGroup).Resources("projects").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(quotaGroup).Resources("appliedclusterresourcequotas").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(routeGroup).Resources("routes").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(routeGroup).Resources("routes/status").RuleOrDie(),

				authorizationapi.NewRule(read...).Groups(templateGroup).Resources("templates", "templateconfigs", "processedtemplates").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule(read...).Groups(buildGroup).Resources("buildlogs").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("resourcequotausages").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: BasicUserRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get").Groups(userGroup).Resources("users").Names("~").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(projectGroup).Resources("projectrequests").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(authzGroup).Resources("clusterroles").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
				authorizationapi.NewRule("list", "watch").Groups(projectGroup).Resources("projects").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),
				{Verbs: sets.NewString("create"), APIGroups: []string{authzGroup}, Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfAccessReviewerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),
				{Verbs: sets.NewString("create"), APIGroups: []string{authzGroup}, Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SelfProvisionerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("create").Groups(projectGroup).Resources("projectrequests").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: StatusCheckerRoleName,
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
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "patch", "update").Groups(imageGroup).Resources("images").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePullerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// pull images
				authorizationapi.NewRule("get").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
			},
		},
		{
			// This role looks like a duplicate of ImageBuilderRole, but the ImageBuilder role is specifically for our builder service accounts
			// if we found another permission needed by them, we'd add it there so the intent is different if you used the ImageBuilderRole
			// you could end up accidentally granting more permissions than you intended.  This is intended to only grant enough powers to
			// push an image to our registry
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePusherRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImageBuilderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// push and pull images
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
				// allow auto-provisioning when pushing an image that doesn't have an imagestream yet
				authorizationapi.NewRule("create").Groups(imageGroup).Resources("imagestreams").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(buildGroup).Resources("builds/details").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImagePrunerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list").Groups(kapiGroup).Resources("pods", "replicationcontrollers").RuleOrDie(),
				authorizationapi.NewRule("list").Groups(kapiGroup).Resources("limitranges").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(buildGroup).Resources("buildconfigs", "builds").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(deployGroup).Resources("deploymentconfigs").RuleOrDie(),

				authorizationapi.NewRule("delete").Groups(imageGroup).Resources("images").RuleOrDie(),
				authorizationapi.NewRule("get", "list").Groups(imageGroup).Resources("images", "imagestreams").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(imageGroup).Resources("imagestreams/status").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: ImageSignerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get").Groups(imageGroup).Resources("images", "imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule("create", "delete").Groups(imageGroup).Resources("imagesignatures").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DeployerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "update").Groups(kapiGroup).Resources("replicationcontrollers").RuleOrDie(),
				authorizationapi.NewRule("get", "list", "watch", "create").Groups(kapiGroup).Resources("pods").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("pods/log").RuleOrDie(),
				authorizationapi.NewRule("create", "list").Groups(kapiGroup).Resources("events").RuleOrDie(),

				authorizationapi.NewRule("update").Groups(imageGroup).Resources("imagestreamtags").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: MasterRoleName,
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
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("delete").Groups(oauthGroup).Resources("oauthaccesstokens", "oauthauthorizetokens").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RouterRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("list", "watch").Groups(kapiGroup).Resources("endpoints").RuleOrDie(),
				authorizationapi.NewRule("list", "watch").Groups(kapiGroup).Resources("services").RuleOrDie(),

				authorizationapi.NewRule("list", "watch").Groups(routeGroup).Resources("routes").RuleOrDie(),
				authorizationapi.NewRule("update").Groups(routeGroup).Resources("routes/status").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("list").Groups(kapiGroup).Resources("limitranges", "resourcequotas").RuleOrDie(),

				authorizationapi.NewRule("get", "delete").Groups(imageGroup).Resources("images", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(imageGroup).Resources("imagestreamimages", "imagestreams/secrets").RuleOrDie(),
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup).Resources("imagestreammappings").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeProxierRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Used to build serviceLister
				authorizationapi.NewRule("list", "watch").Groups(kapiGroup).Resources("services", "endpoints").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				// Allow all API calls to the nodes
				authorizationapi.NewRule("proxy").Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				authorizationapi.NewRule("*").Groups(kapiGroup).Resources("nodes/proxy", authorizationapi.NodeMetricsResource, authorizationapi.NodeSpecResource, authorizationapi.NodeStatsResource, authorizationapi.NodeLogResource).RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Allow read-only access to the API objects
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes").RuleOrDie(),
				// Allow read access to node metrics
				authorizationapi.NewRule("get").Groups(kapiGroup).Resources(authorizationapi.NodeMetricsResource, authorizationapi.NodeSpecResource).RuleOrDie(),
				// Allow read access to stats
				// Node stats requests are submitted as POSTs.  These creates are non-mutating
				authorizationapi.NewRule("get", "create").Groups(kapiGroup).Resources(authorizationapi.NodeStatsResource).RuleOrDie(),
				// TODO: expose other things like /healthz on the node once we figure out non-resource URL policy across systems
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: NodeRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				// Needed to check API access.  These creates are non-mutating
				authorizationapi.NewRule("create").Groups("authentication.k8s.io").Resources("tokenreviews").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("subjectaccessreviews", "localsubjectaccessreviews").RuleOrDie(),
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
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNReaderRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(sdnGroup).Resources("hostsubnets", "netnamespaces").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes", "namespaces").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("egressnetworkpolicies").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(sdnGroup).Resources("clusternetworks").RuleOrDie(),
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: SDNManagerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "list", "watch", "create", "delete").Groups(sdnGroup).Resources("hostsubnets", "netnamespaces").RuleOrDie(),
				authorizationapi.NewRule("get", "create").Groups(sdnGroup).Resources("clusternetworks").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(kapiGroup).Resources("nodes").RuleOrDie(),
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: WebHooksRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule("get", "create").Groups(buildGroup).Resources("buildconfigs/webhooks").RuleOrDie(),
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: DiscoveryRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.DiscoveryRule,
			},
		},

		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryAdminRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("serviceaccounts", "secrets").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(imageGroup).Resources("imagestreamimages", "imagestreammappings", "imagestreams", "imagestreams/secrets", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup).Resources("imagestreamimports").RuleOrDie(),
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(authzGroup).Resources("rolebindings", "roles").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("localresourceaccessreviews", "localsubjectaccessreviews", "subjectrulesreviews").RuleOrDie(),
				authorizationapi.NewRule(read...).Groups(authzGroup).Resources("policies", "policybindings").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
				authorizationapi.NewRule("get", "delete").Groups(projectGroup).Resources("projects").RuleOrDie(),

				// backwards compatibility
				authorizationapi.NewRule("create").Groups(authzGroup).Resources("resourceaccessreviews", "subjectaccessreviews").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryEditorRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(readWrite...).Groups(kapiGroup).Resources("serviceaccounts", "secrets").RuleOrDie(),
				authorizationapi.NewRule(readWrite...).Groups(imageGroup).Resources("imagestreamimages", "imagestreammappings", "imagestreams", "imagestreams/secrets", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("create").Groups(imageGroup).Resources("imagestreamimports").RuleOrDie(),
				authorizationapi.NewRule("get", "update").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(projectGroup).Resources("projects").RuleOrDie(),
			},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryViewerRoleName,
			},
			Rules: []authorizationapi.PolicyRule{
				authorizationapi.NewRule(read...).Groups(imageGroup).Resources("imagestreamimages", "imagestreammappings", "imagestreams", "imagestreamtags").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(imageGroup).Resources("imagestreams/layers").RuleOrDie(),

				authorizationapi.NewRule("get").Groups(kapiGroup).Resources("namespaces").RuleOrDie(),
				authorizationapi.NewRule("get").Groups(projectGroup).Resources("projects").RuleOrDie(),
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
				Name: RouterRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RouterRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: RouterGroup}},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: RegistryRoleBindingName,
			},
			RoleRef: kapi.ObjectReference{
				Name: RegistryRoleName,
			},
			Subjects: []kapi.ObjectReference{{Kind: authorizationapi.SystemGroupKind, Name: RegistryGroup}},
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
