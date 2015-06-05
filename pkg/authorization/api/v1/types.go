package v1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	kruntime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// Authorization is calculated against
// 1. all deny RoleBinding PolicyRules in the master namespace - short circuit on match
// 2. all allow RoleBinding PolicyRules in the master namespace - short circuit on match
// 3. all deny RoleBinding PolicyRules in the namespace - short circuit on match
// 4. all allow RoleBinding PolicyRules in the namespace - short circuit on match
// 5. deny by default

// PolicyRule holds information that describes a policy rule, but does not contain information
// about who the rule applies to or which namespace the rule applies to.
type PolicyRule struct {
	// Verbs is a list of Verbs that apply to ALL the ResourceKinds and AttributeRestrictions contained in this rule.  VerbAll represents all kinds.
	Verbs []string `json:"verbs" description:"list of verbs that apply to ALL the resourceKinds and attributeRestrictions contained in this rule.  The verb * represents all kinds."`
	// AttributeRestrictions will vary depending on what the Authorizer/AuthorizationAttributeBuilder pair supports.
	// If the Authorizer does not recognize how to handle the AttributeRestrictions, the Authorizer should report an error.
	AttributeRestrictions kruntime.RawExtension `json:"attributeRestrictions,omitempty" description:"vary depending on what the authorizer supports.  If the authorizer does not recognize how to handle the specified value, it should report an error."`
	// Resources is a list of resources this rule applies to.  ResourceAll represents all resources.
	Resources []string `json:"resources" description:"list of resources this rule applies to.  * represents all resources."`
	// ResourceNames is an optional white list of names that the rule applies to.  An empty set means that everything is allowed.
	ResourceNames []string `json:"resourceNames,omitempty" description:"optional white list of names that the rule applies to.  An empty set means that everything is allowed."`
	// NonResourceURLsSlice is a set of partial urls that a user should have access to.  *s are allowed, but only as the full, final step in the path
	// This name is intentionally different than the internal type so that the DefaultConvert works nicely and because the ordering may be different.
	NonResourceURLsSlice []string `json:"nonResourceURLs,omitempty" description:"set of partial urls that a user should have access to. *s are allowed, but only as the full, final step in the path."`
}

// IsPersonalSubjectAccessReview is a marker for PolicyRule.AttributeRestrictions that denotes that subjectaccessreviews on self should be allowed
type IsPersonalSubjectAccessReview struct {
	kapi.TypeMeta `json:",inline"`
}

// Role is a logical grouping of PolicyRules that can be referenced as a unit by RoleBindings.
type Role struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Rules holds all the PolicyRules for this Role
	Rules []PolicyRule `json:"rules" description:"all the rules for this role"`
}

// RoleBinding references a Role, but not contain it.  It can reference any Role in the same namespace or in the global namespace.
// It adds who information via Users and Groups and namespace information by which namespace it exists in.  RoleBindings in a given
// namespace only have effect in that namespace (excepting the master namespace which has power in all namespaces).
type RoleBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// UserNames holds all the usernames directly bound to the role
	UserNames []string `json:"userNames" description:"all the usernames directly bound to the role"`
	// GroupNames holds all the groups directly bound to the role
	GroupNames []string `json:"groupNames" description:"all the groups directly bound to the role"`

	// RoleRef can only reference the current namespace and the global namespace
	// If the RoleRef cannot be resolved, the Authorizer must return an error.
	// Since Policy is a singleton, this is sufficient knowledge to locate a role
	RoleRef kapi.ObjectReference `json:"roleRef" description:"a reference to a role"`
}

// Policy is a object that holds all the Roles for a particular namespace.  There is at most
// one Policy document per namespace.
type Policy struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// LastModified is the last time that any part of the Policy was created, updated, or deleted
	LastModified kutil.Time `json:"lastModified" description:"last time that any part of the policy was created, updated, or deleted"`

	// Roles holds all the Roles held by this Policy, mapped by Role.Name
	Roles []NamedRole `json:"roles" description:"roles held by this policy"`
}

// PolicyBinding is a object that holds all the RoleBindings for a particular namespace.  There is
// one PolicyBinding document per referenced Policy namespace
type PolicyBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// LastModified is the last time that any part of the PolicyBinding was created, updated, or deleted
	LastModified kutil.Time `json:"lastModified" description:"last time that any part of the object was created, updated, or deleted"`

	// PolicyRef is a reference to the Policy that contains all the Roles that this PolicyBinding's RoleBindings may reference
	PolicyRef kapi.ObjectReference `json:"policyRef" description:"reference to the policy that contains all the Roles that this object's roleBindings may reference"`
	// RoleBindings holds all the RoleBindings held by this PolicyBinding, mapped by RoleBinding.Name
	RoleBindings []NamedRoleBinding `json:"roleBindings" description:"all roleBindings held by this policyBinding"`
}

// ResourceAccessReviewResponse describes who can perform the action
type ResourceAccessReviewResponse struct {
	kapi.TypeMeta `json:",inline"`

	// Namespace is the namespace used for the access review
	Namespace string `json:"namespace,omitempty" description:"namespace used for the access review"`
	// UsersSlice is the list of users who can perform the action
	UsersSlice []string `json:"users" description:"list of users who can perform the action"`
	// GroupsSlice is the list of groups who can perform the action
	GroupsSlice []string `json:"groups" description:"list of groups who can perform the action"`
}

// ResourceAccessReview is a means to request a list of which users and groups are authorized to perform the
// action specified by spec
type ResourceAccessReview struct {
	kapi.TypeMeta `json:",inline"`

	// Verb is one of: get, list, watch, create, update, delete
	Verb string `json:"verb" description:"one of get, list, watch, create, update, delete"`
	// Resource is one of the existing resource types
	Resource string `json:"resource" description:"one of the existing resource types"`
	// Content is the actual content of the request for create and update
	Content kruntime.RawExtension `json:"content,omitempty" description:"actual content of the request for a create or update"`
	// ResourceName is the name of the resource being requested for a "get" or deleted for a "delete"
	ResourceName string `json:"resourceName,omitempty" description:"name of the resource being requested for a get or delete operation"`
}

type NamedRole struct {
	Name string `json:"name" description:"name of the role"`
	Role Role   `json:"role" description:"the role"`
}

type NamedRoleBinding struct {
	Name        string      `json:"name" description:"name of the roleBinding"`
	RoleBinding RoleBinding `json:"roleBinding" description:"the roleBinding"`
}

// SubjectAccessReviewResponse describes whether or not a user or group can perform an action
type SubjectAccessReviewResponse struct {
	kapi.TypeMeta `json:",inline"`

	// Namespace is the namespace used for the access review
	Namespace string `json:"namespace,omitempty" description:"the namespace used for the access review"`
	// Allowed is required.  True if the action would be allowed, false otherwise.
	Allowed bool `json:"allowed" description:"true if the action would be allowed, false otherwise"`
	// Reason is optional.  It indicates why a request was allowed or denied.
	Reason string `json:"reason,omitempty" description:"reason is optional, it indicates why a request was allowed or denied"`
}

// SubjectAccessReview is an object for requesting information about whether a user or group can perform an action
type SubjectAccessReview struct {
	kapi.TypeMeta `json:",inline"`

	// Verb is one of: get, list, watch, create, update, delete
	Verb string `json:"verb" description:"one of get, list, watch, create, update, delete"`
	// Resource is one of the existing resource types
	Resource string `json:"resource" description:"one of the existing resource types"`
	// User is optional. If both User and Groups are empty, the current authenticated user is used.
	User string `json:"user" description:"optional, if both user and groups are empty, the current authenticated user is used"`
	// GroupsSlice is optional. Groups is the list of groups to which the User belongs.
	GroupsSlice []string `json:"groups" description:"optional, list of groups to which the user belongs"`
	// Content is the actual content of the request for create and update
	Content kruntime.RawExtension `json:"content,omitempty" description:"actual content of the request for create and update"`
	// ResourceName is the name of the resource being requested for a "get" or deleted for a "delete"
	ResourceName string `json:"resourceName" description:"name of the resource being requested for a get or delete"`
}

// PolicyList is a collection of Policies
type PolicyList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of Policies
	Items []Policy `json:"items" description:"list of policies"`
}

// PolicyBindingList is a collection of PolicyBindings
type PolicyBindingList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of PolicyBindings
	Items []PolicyBinding `json:"items" description:"list of policy bindings"`
}

// RoleBindingList is a collection of RoleBindings
type RoleBindingList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of RoleBindings
	Items []RoleBinding `json:"items" description:"list of role bindings"`
}

// RoleList is a collection of Roles
type RoleList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of Roles
	Items []Role `json:"items" description:"list of roles"`
}

// ClusterRole is a logical grouping of PolicyRules that can be referenced as a unit by ClusterRoleBindings.
type ClusterRole struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Rules holds all the PolicyRules for this ClusterRole
	Rules []PolicyRule `json:"rules" description:"list of policy rules"`
}

// ClusterRoleBinding references a ClusterRole, but not contain it.  It can reference any ClusterRole in the same namespace or in the global namespace.
// It adds who information via Users and Groups and namespace information by which namespace it exists in.  ClusterRoleBindings in a given
// namespace only have effect in that namespace (excepting the master namespace which has power in all namespaces).
type ClusterRoleBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// UserNames holds all the usernames directly bound to the role
	UserNames []string `json:"userNames" description:"all user names directly bound to the role"`
	// GroupNames holds all the groups directly bound to the role
	GroupNames []string `json:"groupNames" description:"all the groups directly bound to the role"`

	// RoleRef can only reference the current namespace and the global namespace
	// If the ClusterRoleRef cannot be resolved, the Authorizer must return an error.
	// Since Policy is a singleton, this is sufficient knowledge to locate a role
	RoleRef kapi.ObjectReference `json:"roleRef" description:"reference to the policy role"`
}

// ClusterPolicy is a object that holds all the ClusterRoles for a particular namespace.  There is at most
// one ClusterPolicy document per namespace.
type ClusterPolicy struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// LastModified is the last time that any part of the ClusterPolicy was created, updated, or deleted
	LastModified kutil.Time `json:"lastModified" description:"last time any part of the object was created, updated, or deleted"`

	// Roles holds all the ClusterRoles held by this ClusterPolicy, mapped by ClusterRole.Name
	Roles []NamedClusterRole `json:"roles" description:"all the roles held by this policy, mapped by role name"`
}

// ClusterPolicyBinding is a object that holds all the ClusterRoleBindings for a particular namespace.  There is
// one ClusterPolicyBinding document per referenced ClusterPolicy namespace
type ClusterPolicyBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// LastModified is the last time that any part of the ClusterPolicyBinding was created, updated, or deleted
	LastModified kutil.Time `json:"lastModified" description:"last time any part of the object was created, updated, or deleted"`

	// PolicyRef is a reference to the ClusterPolicy that contains all the ClusterRoles that this ClusterPolicyBinding's RoleBindings may reference
	PolicyRef kapi.ObjectReference `json:"policyRef" description:"reference to the cluster policy that this cluster policy binding's role bindings may reference"`
	// RoleBindings holds all the ClusterRoleBindings held by this ClusterPolicyBinding, mapped by ClusterRoleBinding.Name
	RoleBindings []NamedClusterRoleBinding `json:"roleBindings" description:"all the role bindings held by this policy, mapped by role name"`
}

type NamedClusterRole struct {
	Name string      `json:"name" description:"name of the cluster role"`
	Role ClusterRole `json:"role" description:"the cluster role"`
}

type NamedClusterRoleBinding struct {
	Name        string             `json:"name" description:"name of the cluster role binding"`
	RoleBinding ClusterRoleBinding `json:"roleBinding" description:"the cluster role binding"`
}

// ClusterPolicyList is a collection of ClusterPolicies
type ClusterPolicyList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of ClusterPolicies
	Items []ClusterPolicy `json:"items" description:"list of cluster policies"`
}

// ClusterPolicyBindingList is a collection of ClusterPolicyBindings
type ClusterPolicyBindingList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of ClusterPolicyBindings
	Items []ClusterPolicyBinding `json:"items" description:"list of cluster policy bindings"`
}

// ClusterRoleBindingList is a collection of ClusterRoleBindings
type ClusterRoleBindingList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of ClusterRoleBindings
	Items []ClusterRoleBinding `json:"items" description:"list of cluster role bindings"`
}

// ClusterRoleList is a collection of ClusterRoles
type ClusterRoleList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`

	// Items is a list of ClusterRoles
	Items []ClusterRole `json:"items" description:"list of cluster roles"`
}
