package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kruntime "k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"
)

// Authorization is calculated against
// 1. all deny RoleBinding PolicyRules in the master namespace - short circuit on match
// 2. all allow RoleBinding PolicyRules in the master namespace - short circuit on match
// 3. all deny RoleBinding PolicyRules in the namespace - short circuit on match
// 4. all allow RoleBinding PolicyRules in the namespace - short circuit on match
// 5. deny by default

const (
	// PolicyName is the name of Policy
	PolicyName     = "default"
	APIGroupAll    = "*"
	ResourceAll    = "*"
	VerbAll        = "*"
	NonResourceAll = "*"

	UserKind           = "User"
	GroupKind          = "Group"
	ServiceAccountKind = "ServiceAccount"
	SystemUserKind     = "SystemUser"
	SystemGroupKind    = "SystemGroup"
)

const (
	// ResourceGroupPrefix is the prefix for indicating that a resource entry is actually a group of resources.  The groups are defined in code and indicate resources that are commonly permissioned together
	ResourceGroupPrefix = "resourcegroup"
	BuildGroupName      = ResourceGroupPrefix + ":builds"
	DeploymentGroupName = ResourceGroupPrefix + ":deployments"
	ImageGroupName      = ResourceGroupPrefix + ":images"
	OAuthGroupName      = ResourceGroupPrefix + ":oauth"
	UserGroupName       = ResourceGroupPrefix + ":users"
	TemplateGroupName   = ResourceGroupPrefix + ":templates"
	SDNGroupName        = ResourceGroupPrefix + ":sdn"
	// PolicyOwnerGroupName includes the physical resources behind the PermissionGrantingGroupName.  Unless these physical objects are created first, users with privileges to PermissionGrantingGroupName will
	// only be able to bind to global roles
	PolicyOwnerGroupName = ResourceGroupPrefix + ":policy"
	// PermissionGrantingGroupName includes resources that are necessary to maintain authorization roles and bindings.  By itself, this group is insufficient to create anything except for bindings
	// to master roles.  If a local Policy already exists, then privileges to this group will allow for modification of local roles.
	PermissionGrantingGroupName = ResourceGroupPrefix + ":granter"
	// OpenshiftExposedGroupName includes resources that are commonly viewed and modified by end users of the system.  It does not include any sensitive resources that control authentication or authorization
	OpenshiftExposedGroupName = ResourceGroupPrefix + ":exposedopenshift"
	OpenshiftAllGroupName     = ResourceGroupPrefix + ":allopenshift"
	OpenshiftStatusGroupName  = ResourceGroupPrefix + ":allopenshift-status"

	QuotaGroupName = ResourceGroupPrefix + ":quota"
	// KubeInternalsGroupName includes those resources that should reasonably be viewable to end users, but that most users should probably not modify.  Kubernetes herself will maintain these resources
	KubeInternalsGroupName = ResourceGroupPrefix + ":privatekube"
	// KubeExposedGroupName includes resources that are commonly viewed and modified by end users of the system.
	KubeExposedGroupName = ResourceGroupPrefix + ":exposedkube"
	KubeAllGroupName     = ResourceGroupPrefix + ":allkube"
	KubeStatusGroupName  = ResourceGroupPrefix + ":allkube-status"

	// NonEscalatingResourcesGroupName contains all resources that can be viewed without exposing the risk of using view rights to locate a secret to escalate privileges.  For example, view
	// rights on secrets could be used locate a secret that happened to be  serviceaccount token that has more privileges
	NonEscalatingResourcesGroupName         = ResourceGroupPrefix + ":non-escalating"
	KubeNonEscalatingViewableGroupName      = ResourceGroupPrefix + ":kube-non-escalating"
	OpenshiftNonEscalatingViewableGroupName = ResourceGroupPrefix + ":openshift-non-escalating"

	// EscalatingResourcesGroupName contains all resources that can be used to escalate privileges when simply viewed
	EscalatingResourcesGroupName         = ResourceGroupPrefix + ":escalating"
	KubeEscalatingViewableGroupName      = ResourceGroupPrefix + ":kube-escalating"
	OpenshiftEscalatingViewableGroupName = ResourceGroupPrefix + ":openshift-escalating"
)

var (
	GroupsToResources = map[string][]string{
		BuildGroupName:       {"builds", "buildconfigs", "buildlogs", "buildconfigs/instantiate", "builds/log", "builds/clone", "buildconfigs/webhooks"},
		ImageGroupName:       {"imagestreams", "imagestreammappings", "imagestreamtags", "imagestreamimages"},
		DeploymentGroupName:  {"deployments", "deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigrollbacks"},
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
			"clusterroles", "clusterrolebindings", "clusterpolicies", "clusterpolicybindings", "images" /* cluster scoped*/, "projectrequests"},
		OpenshiftStatusGroupName: {"imagestreams/status", "routes/status"},

		QuotaGroupName:         {"limitranges", "resourcequotas", "resourcequotausages"},
		KubeInternalsGroupName: {"minions", "nodes", "bindings", "events", "namespaces"},
		KubeExposedGroupName:   {"pods", "replicationcontrollers", "serviceaccounts", "services", "endpoints", "persistentvolumeclaims", "pods/log"},
		KubeAllGroupName:       {KubeInternalsGroupName, KubeExposedGroupName, QuotaGroupName},
		KubeStatusGroupName:    {"pods/status", "resourcequotas/status", "namespaces/status", "replicationcontrollers/status"},

		OpenshiftEscalatingViewableGroupName: {"oauthauthorizetokens", "oauthaccesstokens"},
		KubeEscalatingViewableGroupName:      {"secrets"},
		EscalatingResourcesGroupName:         {OpenshiftEscalatingViewableGroupName, KubeEscalatingViewableGroupName},

		NonEscalatingResourcesGroupName: {OpenshiftNonEscalatingViewableGroupName, KubeNonEscalatingViewableGroupName},
	}
)

func init() {
	// set the non-escalating groups
	GroupsToResources[OpenshiftNonEscalatingViewableGroupName] = ExpandResources(sets.NewString(GroupsToResources[OpenshiftAllGroupName]...)).
		Difference(ExpandResources(sets.NewString(GroupsToResources[OpenshiftEscalatingViewableGroupName]...))).List()

	GroupsToResources[KubeNonEscalatingViewableGroupName] = ExpandResources(sets.NewString(GroupsToResources[KubeAllGroupName]...)).
		Difference(ExpandResources(sets.NewString(GroupsToResources[KubeEscalatingViewableGroupName]...))).List()
}

// PolicyRule holds information that describes a policy rule, but does not contain information
// about who the rule applies to or which namespace the rule applies to.
type PolicyRule struct {
	// Verbs is a list of Verbs that apply to ALL the ResourceKinds and AttributeRestrictions contained in this rule.  VerbAll represents all kinds.
	Verbs sets.String
	// AttributeRestrictions will vary depending on what the Authorizer/AuthorizationAttributeBuilder pair supports.
	// If the Authorizer does not recognize how to handle the AttributeRestrictions, the Authorizer should report an error.
	AttributeRestrictions kruntime.EmbeddedObject
	// APIGroups is the name of the APIGroup that contains the resources.  If this field is empty, then both kubernetes and origin API groups are assumed.
	// That means that if an action is requested against one of the enumerated resources in either the kubernetes or the origin API group, the request
	// will be allowed
	APIGroups []string
	// Resources is a list of resources this rule applies to.  ResourceAll represents all resources.
	Resources sets.String
	// ResourceNames is an optional white list of names that the rule applies to.  An empty set means that everything is allowed.
	ResourceNames sets.String
	// NonResourceURLs is a set of partial urls that a user should have access to.  *s are allowed, but only as the full, final step in the path
	// If an action is not a resource API request, then the URL is split on '/' and is checked against the NonResourceURLs to look for a match.
	NonResourceURLs sets.String
}

// IsPersonalSubjectAccessReview is a marker for PolicyRule.AttributeRestrictions that denotes that subjectaccessreviews on self should be allowed
type IsPersonalSubjectAccessReview struct {
	kapi.TypeMeta
}

// Role is a logical grouping of PolicyRules that can be referenced as a unit by RoleBindings.
type Role struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Rules holds all the PolicyRules for this Role
	Rules []PolicyRule
}

// RoleBinding references a Role, but not contain it.  It can reference any Role in the same namespace or in the global namespace.
// It adds who information via Users and Groups and namespace information by which namespace it exists in.  RoleBindings in a given
// namespace only have effect in that namespace (excepting the master namespace which has power in all namespaces).
type RoleBinding struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Subjects hold object references of to authorize with this rule
	Subjects []kapi.ObjectReference

	// RoleRef can only reference the current namespace and the global namespace
	// If the RoleRef cannot be resolved, the Authorizer must return an error.
	// Since Policy is a singleton, this is sufficient knowledge to locate a role
	RoleRef kapi.ObjectReference
}

// Policy is a object that holds all the Roles for a particular namespace.  There is at most
// one Policy document per namespace.
type Policy struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// LastModified is the last time that any part of the Policy was created, updated, or deleted
	LastModified util.Time

	// Roles holds all the Roles held by this Policy, mapped by Role.Name
	Roles map[string]*Role
}

// PolicyBinding is a object that holds all the RoleBindings for a particular namespace.  There is
// one PolicyBinding document per referenced Policy namespace
type PolicyBinding struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// LastModified is the last time that any part of the PolicyBinding was created, updated, or deleted
	LastModified util.Time

	// PolicyRef is a reference to the Policy that contains all the Roles that this PolicyBinding's RoleBindings may reference
	PolicyRef kapi.ObjectReference
	// RoleBindings holds all the RoleBindings held by this PolicyBinding, mapped by RoleBinding.Name
	RoleBindings map[string]*RoleBinding
}

// ResourceAccessReviewResponse describes who can perform the action
type ResourceAccessReviewResponse struct {
	kapi.TypeMeta

	// Namespace is the namespace used for the access review
	Namespace string
	// Users is the list of users who can perform the action
	Users sets.String
	// Groups is the list of groups who can perform the action
	Groups sets.String
}

// ResourceAccessReview is a means to request a list of which users and groups are authorized to perform the
// action specified by spec
type ResourceAccessReview struct {
	kapi.TypeMeta

	// Action describes the action being tested
	Action AuthorizationAttributes
}

// SubjectAccessReviewResponse describes whether or not a user or group can perform an action
type SubjectAccessReviewResponse struct {
	kapi.TypeMeta

	// Namespace is the namespace used for the access review
	Namespace string
	// Allowed is required.  True if the action would be allowed, false otherwise.
	Allowed bool
	// Reason is optional.  It indicates why a request was allowed or denied.
	Reason string
}

// SubjectAccessReview is an object for requesting information about whether a user or group can perform an action
type SubjectAccessReview struct {
	kapi.TypeMeta

	// Action describes the action being tested
	Action AuthorizationAttributes
	// User is optional.  If both User and Groups are empty, the current authenticated user is used.
	User string
	// Groups is optional.  Groups is the list of groups to which the User belongs.
	Groups sets.String
}

// LocalResourceAccessReview is a means to request a list of which users and groups are authorized to perform the action specified by spec in a particular namespace
type LocalResourceAccessReview struct {
	kapi.TypeMeta

	// Action describes the action being tested
	Action AuthorizationAttributes
}

// LocalSubjectAccessReview is an object for requesting information about whether a user or group can perform an action in a particular namespace
type LocalSubjectAccessReview struct {
	kapi.TypeMeta

	// Action describes the action being tested.  The Namespace element is FORCED to the current namespace.
	Action AuthorizationAttributes
	// User is optional.  If both User and Groups are empty, the current authenticated user is used.
	User string
	// Groups is optional.  Groups is the list of groups to which the User belongs.
	Groups sets.String
}

type AuthorizationAttributes struct {
	// Namespace is the namespace of the action being requested.  Currently, there is no distinction between no namespace and all namespaces
	Namespace string
	// Verb is one of: get, list, watch, create, update, delete
	Verb string
	// Resource is one of the existing resource types
	Resource string
	// ResourceName is the name of the resource being requested for a "get" or deleted for a "delete"
	ResourceName string
	// Content is the actual content of the request for create and update
	Content kruntime.EmbeddedObject
}

// PolicyList is a collection of Policies
type PolicyList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of policies
	Items []Policy
}

// PolicyBindingList is a collection of PolicyBindings
type PolicyBindingList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of policyBindings
	Items []PolicyBinding
}

// RoleBindingList is a collection of RoleBindings
type RoleBindingList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of roleBindings
	Items []RoleBinding
}

// RoleList is a collection of Roles
type RoleList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of roles
	Items []Role
}

// ClusterRole is a logical grouping of PolicyRules that can be referenced as a unit by ClusterRoleBindings.
type ClusterRole struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Rules holds all the PolicyRules for this ClusterRole
	Rules []PolicyRule
}

// ClusterRoleBinding references a ClusterRole, but not contain it.  It can reference any ClusterRole in the same namespace or in the global namespace.
// It adds who information via Users and Groups and namespace information by which namespace it exists in.  ClusterRoleBindings in a given
// namespace only have effect in that namespace (excepting the master namespace which has power in all namespaces).
type ClusterRoleBinding struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Subjects hold object references of to authorize with this rule
	Subjects []kapi.ObjectReference

	// RoleRef can only reference the current namespace and the global namespace
	// If the ClusterRoleRef cannot be resolved, the Authorizer must return an error.
	// Since Policy is a singleton, this is sufficient knowledge to locate a role
	RoleRef kapi.ObjectReference
}

// ClusterPolicy is a object that holds all the ClusterRoles for a particular namespace.  There is at most
// one ClusterPolicy document per namespace.
type ClusterPolicy struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// LastModified is the last time that any part of the ClusterPolicy was created, updated, or deleted
	LastModified util.Time

	// Roles holds all the ClusterRoles held by this ClusterPolicy, mapped by Role.Name
	Roles map[string]*ClusterRole
}

// ClusterPolicyBinding is a object that holds all the ClusterRoleBindings for a particular namespace.  There is
// one ClusterPolicyBinding document per referenced ClusterPolicy namespace
type ClusterPolicyBinding struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// LastModified is the last time that any part of the ClusterPolicyBinding was created, updated, or deleted
	LastModified util.Time

	// ClusterPolicyRef is a reference to the ClusterPolicy that contains all the ClusterRoles that this ClusterPolicyBinding's RoleBindings may reference
	PolicyRef kapi.ObjectReference
	// RoleBindings holds all the RoleBindings held by this ClusterPolicyBinding, mapped by RoleBinding.Name
	RoleBindings map[string]*ClusterRoleBinding
}

// ClusterPolicyList is a collection of ClusterPolicies
type ClusterPolicyList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of ClusterPolicies
	Items []ClusterPolicy
}

// ClusterPolicyBindingList is a collection of ClusterPolicyBindings
type ClusterPolicyBindingList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of ClusterPolicyBindings
	Items []ClusterPolicyBinding
}

// ClusterRoleBindingList is a collection of ClusterRoleBindings
type ClusterRoleBindingList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of ClusterRoleBindings
	Items []ClusterRoleBinding
}

// ClusterRoleList is a collection of ClusterRoles
type ClusterRoleList struct {
	kapi.TypeMeta
	kapi.ListMeta

	// Items is a list of ClusterRoles
	Items []ClusterRole
}
