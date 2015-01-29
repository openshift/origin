package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
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
	GlobalNamespace = "global"
)

// PolicyRule holds information that describes a policy rule, but does not contain information
// about who the rule applies to or which namespace the rule applies to.
type PolicyRule struct {
	// Deny is true if any request matching this rule should be denied.  If false, any request matching this rule is allowed.
	Deny bool `json:"deny"`
	// Verbs is a list of Verbs that apply to ALL the ResourceKinds and AttributeRestrictions contained in this rule.  "*" represents all kinds.
	Verbs []string `json:"verbs"`
	// AttributeRestrictions will vary depending on what the Authorizer/AuthorizationAttributeBuilder pair supports.
	// If the Authorizer does not recognize how to handle the AttributeRestrictions, the Authorizer should report an error.
	AttributeRestrictions kruntime.RawExtension `json:"attributeRestrictions"`
	// ResourceKinds is a list of kinds this rule applies to.  "*" represents all kinds.
	ResourceKinds []string `json:"resourceKinds""`
}

// Role is a logical grouping of PolicyRules that can be referenced as a unit by RoleBindings.
type Role struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	Rules []PolicyRule `json:"rules"`
}

// RoleBinding references a Role, but not contain it.  It adds who and namespace information.
// It can reference any Role in the same namespace or in the global namespace.
type RoleBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	UserNames  []string `json:"userNames"`
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
	kapi.ObjectMeta `json:"metadata,omitempty"`

	LastModified kutil.Time `json:"lastModified"`

	Roles []NamedRole `json:"roles"`
}

// PolicyBinding is a object that holds all the RoleBindings for a particular namespace.  There is
// one PolicyBinding document per referenced Policy namespace
type PolicyBinding struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	LastModified kutil.Time `json:"lastModified"`

	PolicyRef    kapi.ObjectReference `json:"policyRef"`
	RoleBindings []NamedRoleBinding   `json:"roleBindings"`
}

type NamedRole struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

type NamedRoleBinding struct {
	Name        string      `json:"name"`
	RoleBinding RoleBinding `json:"roleBinding"`
}

type PolicyList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Policy `json:"items"`
}
type PolicyBindingList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []PolicyBinding `json:"items"`
}
