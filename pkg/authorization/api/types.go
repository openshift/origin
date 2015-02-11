package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kruntime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// Authorization is calculated against
// 1. all deny RoleBinding PolicyRules in the master namespace - short circuit on match
// 2. all allow RoleBinding PolicyRules in the master namespace - short circuit on match
// 3. all deny RoleBinding PolicyRules in the namespace - short circuit on match
// 4. all allow RoleBinding PolicyRules in the namespace - short circuit on match
// 5. deny by default

const (
	// Policy is a singleton and this is its name
	PolicyName  = "default"
	ResourceAll = "*"
	VerbAll     = "*"
)

const (
	// ResourceGroupPrefix is the prefix for indicating that a resource entry is actually a group of resources.  The groups are defined in code and indicate resources that are commonly permissioned together
	ResourceGroupPrefix = "resourcegroup"
	BuildGroupName      = ResourceGroupPrefix + ":builds"
	DeploymentGroupName = ResourceGroupPrefix + ":deployments"
	ImageGroupName      = ResourceGroupPrefix + ":images"
	OAuthGroupName      = ResourceGroupPrefix + ":oauth"
	UserGroupName       = ResourceGroupPrefix + ":users"
	// PolicyOwnerGroupName includes the physical resources behind the PermissionGrantingGroupName.  Unless these physical objects are created first, users with privileges to PermissionGrantingGroupName will
	// only be able to bind to global roles
	PolicyOwnerGroupName = ResourceGroupPrefix + ":policy"
	// PermissionGrantingGroupName includes resources that are necessary to maintain authorization roles and bindings.  By itself, this group is insufficient to create anything except for bindings
	// to master roles.  If a local Policy already exists, then privileges to this group will allow for modification of local roles.
	PermissionGrantingGroupName = ResourceGroupPrefix + ":granter"
	// OpenshiftExposedGroupName includes resources that are commonly viewed and modified by end users of the system.  It does not include any sensitive resources that control authentication or authorization
	OpenshiftExposedGroupName = ResourceGroupPrefix + ":exposedopenshift"
	OpenshiftAllGroupName     = ResourceGroupPrefix + ":allopenshift"

	QuotaGroupName = ResourceGroupPrefix + ":quota"
	// KubeInternalsGroupName includes those resources that should reasonably be viewable to end users, but that most users should probably not modify.  Kubernetes herself will maintain these resources
	KubeInternalsGroupName = ResourceGroupPrefix + ":privatekube"
	// KubeExposedGroupName includes resources that are commonly viewed and modified by end users of the system.
	KubeExposedGroupName = ResourceGroupPrefix + ":exposedkube"
	KubeAllGroupName     = ResourceGroupPrefix + ":allkube"
)

var (
	GroupsToResources = map[string][]string{
		BuildGroupName:              {"builds", "buildconfigs", "buildlogs"},
		ImageGroupName:              {"images", "imagerepositories", "imagerepositorymappings", "imagerepositorytags"},
		DeploymentGroupName:         {"deployments", "deploymentconfigs", "generatedeploymentconfigs", "deploymentconfigrollbacks"},
		UserGroupName:               {"users", "useridentitymappings"},
		OAuthGroupName:              {"oauthauthorizetokens", "oauthaccesstokens", "oauthclients", "oauthclientauthorizations"},
		PolicyOwnerGroupName:        {"policies", "policybindings"},
		PermissionGrantingGroupName: {"roles", "rolebindings"},
		OpenshiftExposedGroupName:   {BuildGroupName, ImageGroupName, DeploymentGroupName, "templateconfigs", "routes", "projects"},
		OpenshiftAllGroupName:       {OpenshiftExposedGroupName, UserGroupName, OAuthGroupName, PolicyOwnerGroupName, PermissionGrantingGroupName},

		QuotaGroupName:         {"limitranges", "resourcequotas", "resourcequotausages"},
		KubeInternalsGroupName: {"endpoints", "minions", "nodes", "bindings", "events"},
		KubeExposedGroupName:   {"pods", "replicationcontrollers", "services"},
		KubeAllGroupName:       {KubeInternalsGroupName, KubeExposedGroupName, QuotaGroupName},
	}
)

// PolicyRule holds information that describes a policy rule, but does not contain information
// about who the rule applies to or which namespace the rule applies to.
type PolicyRule struct {
	// Verbs is a list of Verbs that apply to ALL the ResourceKinds and AttributeRestrictions contained in this rule.  VerbAll represents all kinds.
	Verbs []string `json:"verbs"`
	// AttributeRestrictions will vary depending on what the Authorizer/AuthorizationAttributeBuilder pair supports.
	// If the Authorizer does not recognize how to handle the AttributeRestrictions, the Authorizer should report an error.
	AttributeRestrictions kruntime.EmbeddedObject `json:"attributeRestrictions"`
	// Resources is a list of resources this rule applies to.  ResourceAll represents all resources.
	Resources []string `json:"resources"`
}

// Role is a logical grouping of PolicyRules that can be referenced as a unit by RoleBindings.
type Role struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Rules holds all the PolicyRules for this Role
	Rules []PolicyRule `json:"rules"`
}

// RoleBinding references a Role, but not contain it.  It adds who and namespace information.
// It can reference any Role in the same namespace or in the global namespace.
type RoleBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// UserNames holds all the usernames directly bound to the role
	UserNames []string `json:"userNames"`
	// GroupNames holds all the groups directly bound to the role
	GroupNames []string `json:"groupNames"`

	// Since Policy is a singleton, this is sufficient knowledge to locate a role
	// RoleRefs can only reference the current namespace and the global namespace
	// If the RoleRef cannot be resolved, the Authorizer must return an error.
	RoleRef kapi.ObjectReference `json:"roleRef"`
}

// Policy is a object that holds all the Roles for a particular namespace.  There is at most
// one Policy document per namespace.
type Policy struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" `

	// LastModified is the last time that any part of the Policy was created, updated, or deleted
	LastModified kutil.Time `json:"lastModified"`

	// Roles holds all the Roles held by this Policy, mapped by Role.Name
	Roles map[string]Role `json:"roles"`
}

// PolicyBinding is a object that holds all the RoleBindings for a particular namespace.  There is
// one PolicyBinding document per referenced Policy namespace
type PolicyBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// LastModified is the last time that any part of the PolicyBinding was created, updated, or deleted
	LastModified kutil.Time `json:"lastModified"`

	// PolicyRef is a reference to the Policy that contains all the Roles that this PolicyBinding's RoleBindings may reference
	PolicyRef kapi.ObjectReference `json:"policyRef"`
	// RoleBindings holds all the RoleBindings held by this PolicyBinding, mapped by RoleBinding.Name
	RoleBindings map[string]RoleBinding `json:"roleBindings"`
}

// PolicyList is a collection of Policies
type PolicyList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Policy `json:"items"`
}

// PolicyBindingList is a collection of PolicyBindings
type PolicyBindingList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []PolicyBinding `json:"items"`
}
