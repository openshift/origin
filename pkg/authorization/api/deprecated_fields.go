package api

import (
	"k8s.io/kubernetes/pkg/util/sets"
)

// NEVER TOUCH ANYTHING IN THIS FILE!

const (
	// ResourceGroupPrefix is the prefix for indicating that a resource entry is actually a group of resources.  The groups are defined in code and indicate resources that are commonly permissioned together
	ResourceGroupPrefix = "resourcegroup:"
	BuildGroupName      = ResourceGroupPrefix + "builds"
	DeploymentGroupName = ResourceGroupPrefix + "deployments"
	ImageGroupName      = ResourceGroupPrefix + "images"
	OAuthGroupName      = ResourceGroupPrefix + "oauth"
	UserGroupName       = ResourceGroupPrefix + "users"
	TemplateGroupName   = ResourceGroupPrefix + "templates"
	SDNGroupName        = ResourceGroupPrefix + "sdn"
	// PolicyOwnerGroupName includes the physical resources behind the PermissionGrantingGroupName.  Unless these physical objects are created first, users with privileges to PermissionGrantingGroupName will
	// only be able to bind to global roles
	PolicyOwnerGroupName = ResourceGroupPrefix + "policy"
	// PermissionGrantingGroupName includes resources that are necessary to maintain authorization roles and bindings.  By itself, this group is insufficient to create anything except for bindings
	// to master roles.  If a local Policy already exists, then privileges to this group will allow for modification of local roles.
	PermissionGrantingGroupName = ResourceGroupPrefix + "granter"
	// OpenshiftExposedGroupName includes resources that are commonly viewed and modified by end users of the system.  It does not include any sensitive resources that control authentication or authorization
	OpenshiftExposedGroupName = ResourceGroupPrefix + "exposedopenshift"
	OpenshiftAllGroupName     = ResourceGroupPrefix + "allopenshift"
	OpenshiftStatusGroupName  = ResourceGroupPrefix + "allopenshift-status"

	QuotaGroupName = ResourceGroupPrefix + "quota"
	// KubeInternalsGroupName includes those resources that should reasonably be viewable to end users, but that most users should probably not modify.  Kubernetes herself will maintain these resources
	KubeInternalsGroupName = ResourceGroupPrefix + "privatekube"
	// KubeExposedGroupName includes resources that are commonly viewed and modified by end users of the system.
	KubeExposedGroupName = ResourceGroupPrefix + "exposedkube"
	KubeAllGroupName     = ResourceGroupPrefix + "allkube"
	KubeStatusGroupName  = ResourceGroupPrefix + "allkube-status"

	// NonEscalatingResourcesGroupName contains all resources that can be viewed without exposing the risk of using view rights to locate a secret to escalate privileges.  For example, view
	// rights on secrets could be used locate a secret that happened to be  serviceaccount token that has more privileges
	NonEscalatingResourcesGroupName         = ResourceGroupPrefix + "non-escalating"
	KubeNonEscalatingViewableGroupName      = ResourceGroupPrefix + "kube-non-escalating"
	OpenshiftNonEscalatingViewableGroupName = ResourceGroupPrefix + "openshift-non-escalating"

	// EscalatingResourcesGroupName contains all resources that can be used to escalate privileges when simply viewed
	EscalatingResourcesGroupName         = ResourceGroupPrefix + "escalating"
	KubeEscalatingViewableGroupName      = ResourceGroupPrefix + "kube-escalating"
	OpenshiftEscalatingViewableGroupName = ResourceGroupPrefix + "openshift-escalating"
)

var (
	GroupsToResources = map[string][]string{
		BuildGroupName:       {"builds", "buildconfigs", "buildlogs", "buildconfigs/instantiate", "buildconfigs/instantiatebinary", "builds/log", "builds/clone", "buildconfigs/webhooks"},
		ImageGroupName:       {"imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages", "imagestreamimports"},
		DeploymentGroupName:  {"deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigrollbacks", "deploymentconfigs/log", "deploymentconfigs/scale"},
		SDNGroupName:         {"clusternetworks", "hostsubnets", "netnamespaces"},
		TemplateGroupName:    {"templates", "templateconfigs", "processedtemplates"},
		UserGroupName:        {"identities", "users", "useridentitymappings", "groups"},
		OAuthGroupName:       {"oauthauthorizetokens", "oauthaccesstokens", "oauthclients", "oauthclientauthorizations"},
		PolicyOwnerGroupName: {"policies", "policybindings"},

		// RAR and SAR are in this list to support backwards compatibility with clients that expect access to those resource in a namespace scope and a cluster scope.
		// TODO remove once we have eliminated the namespace scoped resource.
		PermissionGrantingGroupName: {"roles", "rolebindings", "resourceaccessreviews" /* cluster scoped*/, "subjectaccessreviews" /* cluster scoped*/, "localresourceaccessreviews", "localsubjectaccessreviews"},
		OpenshiftExposedGroupName:   {BuildGroupName, ImageGroupName, DeploymentGroupName, TemplateGroupName, "routes"},
		OpenshiftAllGroupName: {OpenshiftExposedGroupName, UserGroupName, OAuthGroupName, PolicyOwnerGroupName, SDNGroupName, PermissionGrantingGroupName, OpenshiftStatusGroupName, "projects",
			"clusterroles", "clusterrolebindings", "clusterpolicies", "clusterpolicybindings", "images" /* cluster scoped*/, "projectrequests", "builds/details", "imagestreams/secrets",
			"selfsubjectrulesreviews"},
		OpenshiftStatusGroupName: {"imagestreams/status", "routes/status", "deploymentconfigs/status"},

		QuotaGroupName:         {"limitranges", "resourcequotas", "resourcequotausages"},
		KubeExposedGroupName:   {"pods", "replicationcontrollers", "serviceaccounts", "services", "endpoints", "persistentvolumeclaims", "pods/log", "configmaps"},
		KubeInternalsGroupName: {"minions", "nodes", "bindings", "events", "namespaces", "persistentvolumes", "securitycontextconstraints"},
		KubeAllGroupName:       {KubeInternalsGroupName, KubeExposedGroupName, QuotaGroupName},
		KubeStatusGroupName:    {"pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status"},

		OpenshiftEscalatingViewableGroupName: {"oauthauthorizetokens", "oauthaccesstokens", "imagestreams/secrets"},
		KubeEscalatingViewableGroupName:      {"secrets"},
		EscalatingResourcesGroupName:         {OpenshiftEscalatingViewableGroupName, KubeEscalatingViewableGroupName},

		NonEscalatingResourcesGroupName: {OpenshiftNonEscalatingViewableGroupName, KubeNonEscalatingViewableGroupName},
	}
)

func init() {
	// set the non-escalating groups
	GroupsToResources[OpenshiftNonEscalatingViewableGroupName] = NormalizeResources(sets.NewString(GroupsToResources[OpenshiftAllGroupName]...)).
		Difference(NormalizeResources(sets.NewString(GroupsToResources[OpenshiftEscalatingViewableGroupName]...))).List()

	GroupsToResources[KubeNonEscalatingViewableGroupName] = NormalizeResources(sets.NewString(GroupsToResources[KubeAllGroupName]...)).
		Difference(NormalizeResources(sets.NewString(GroupsToResources[KubeEscalatingViewableGroupName]...))).List()
}
