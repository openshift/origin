# Policy

## Problem
Policy extends the existing authorization rules in Kubernetes (https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/authorization.md) to fulfill requirements around extended verb sets, verb specific attributes, role based management of attribute based access control, and more efficient evaluation of authorization status at larger scales.  For discussion about verbs, see: https://github.com/GoogleCloudPlatform/kubernetes/issues/2877.  For discussion about allowing mutation of some fields in a resource and not others, see: https://github.com/GoogleCloudPlatform/kubernetes/issues/2726.

## Use Cases
 1.  I want to apply the same set of policy rules to large numbers of projects.
 1.  I want to change the policy rules for a role and have it applied to all projects using that role.
 1.  I want to have policy changes take effect quickly, without any down time.
 1.  I want to have distinct policy rules to control group membership, project metadata, policy rules for roles, and policy rules for users.
 1.  I want to have sensible roles provided by default.  OpenShift believes there are four fundamental roles for managing a cluster.  They are: view, edit, admin, and cluster-admin.

## Exclusions
 1. UI (cli or graphical).  This document describes what policy can do, the data required to make policy work, and runtime interactions to fulfill the policy.  It does **not** attempt to describe a UI (cli or graphical) on top of basic resource CRUD.
 1. Groups.  This document assumes that a UserInfo object exists and that it has `UserInfo.getUserName() string` and `UserInfo.getGroupNames() []string`.  How that group information is populated and stored is beyond the scope of making effective policy.

  
## Policy Example
There are four fundamental roles: view, edit, admin, and cluster-admin.  Viewers are able to see all resources.  Editors are able to view and edit all resources, except ones related to policy and membership.  Admins are able to modify anything everything except policy rules.  Cluster-admins can modify aything.  The roles are expressed like this:
```
{
	"kind": "role",
	"name": "view",
	"namespace": "master",
	"rules": [
		{ "verbs": ["watch", "list", "get"], "resourceKinds": ["*", "-roles", "-roleBindings", "-policyBindings", "-policies"] },
	]
}
{
	"kind": "role",
	"name": "edit",
	"namespace": "master",
	"rules": [
		{ "verbs": ["*"], "resourceKinds": ["*", "-roles", "-roleBindings", "-policyBindings", "-policies", "-resourceAccessReview"] }
	]
}
{
	"kind": "role",
	"name": "admin",
	"namespace": "master",
	"rules": [
		{ "verbs": ["*", "-create", "-update", "-delete"], "resourceKinds": ["*"] }
		{ "verbs": ["create", "update", "delete"], "resourceKinds": ["*", "-roles", "-policyBindings"] }
	]
}
{
	"kind": "role",
	"name": "cluster-admin",
	"namespace": "master",
	"rules": [
		{ "verbs": ["*"], "resourceKinds": ["*"] }
	]
}
```

`Clark` is the cluster admin: the guy who owns the openshift installation.  `Clark` creates a project `hammer` and assigns `Hubert` as the project admin using the predefined role.  The permissions are expressed like this:
```
{
	"kind": "roleBinding",
	"name": "ProjectAdmins",
	"namespace": "hammer",
	"roleRef": {
		"namespace": "master",
		"name": "admin"
	}
	"userNames": ["Hubert"]
}
```

`Hubert` now has the power to manage policy on the `hammer` project.  He can grant permission to users who should have access.  In this case, he grants edit permission to `Edgar` using the predefined role.
```
{
	"kind": "roleBinding",
	"name": "Editors",
	"namespace": "hammer",
	"referencesGlobalRole": true,
	"roleName": "edit",
	"userNames": ["Edgar"]
}
```

That covers most cases where cluster admins want to create projects and delegate the administration of a project to a project admin.  Then the project admin wants to delegate project level permissions to other users.  


## Basic Concepts
 1.  A PolicyRule expresses a permission containing: Deny, []Verb, []ResourceKinds (pod, deploymentConfig, *-(all kinds), etc)
 1.  A Role is a way to name a set of PolicyRules
 1.  A Policy is a container for Roles.  There can only be one Policy per namespace.
 1.  A RoleBinding is a way to associate a Role with a given user or group.  A RoleBinding references (but does not include) a Role.
 1.  A PolicyBinding is a container for RoleBindings.  There can be many PolicyBinding objects per namespace.  RoleBindings may reference Roles in another namespace.  In usage, a RoleBinding that references a missing Role will produce an error.
 1.  A master namespace exists.  This namespace is configurable from the command line and it is special.  Roles defined in the master namespace can be referenced from any other namespace.  Bindings defined in the master namespace apply to **all** namespaces (an admin in the master namespace can edit any resource in any namespace).  RoleBinding policy rules cannot be overridden by any local binding (a local PolicyBindingRule that denies actions to the cluster admin will be overriden by the binding defined in the master namespace).
 1.  Roles versus Groups.  A Role is a grouping of PolicyRules.  A group is a grouping of users.  A RoleBinding associates a Role with a set of users and/or groups.  This distinction makes Groups easier to share across projects.
 1.  A Verb or Resource kind can be negated.  For example, if you want to express "all kinds except for roles", you can do that with resourceKinds = ["*", "-roles"].  This is different than having a separate deny rule.  Deny rules trump allow rules, so having a separate deny rule means that any user bound to the denying role can never have access to what is denies.  The negation allows you to express a limitted allow that doesn't have to have pre-knowledge of every potential kind or verb.

Policy Evaluation
In order to determine whether a request is authorized, the AuthorizationAttributes are tested in the following order:
  1. all deny RoleBinding PolicyRules in the master namespace - short circuit on match
  1. all allow RoleBinding PolicyRules in the master namespace - short circuit on match
  1. all deny RoleBinding PolicyRules in the namespace - short circuit on match
  1. all allow RoleBinding PolicyRules in the namespace - short circuit on match
  1. deny by default

### Configtime Authorization Types
These are the types used in the policy example above.  They allow us: to quickly find the policy rules that apply to namespace in etcd, to easily delegate project level control to a project admin, and to easily segregate all role control (a project admin can create/update/delete RoleBindings, but not Roles).
```
// PolicyRule holds information that describes a policy rule, but does not contain information 
// about who the rule applies to or which namespace the rule applies to.
type PolicyRule struct {
	// Deny is true if any request matching this rule should be denied.  If false, any request matching this rule is allowed.
	Deny bool
	// Verbs is a list of Verbs that apply to ALL the ResourceKinds and AttributeRestrictions contained in this rule.  "*" represents all kinds.
	Verbs []Verb
	// AttributeRestrictions will vary depending on what the Authorizer/AuthorizationAttributeBuilder pair supports.
	// If the Authorizer does not recognize how to handle the AttributeRestrictions, the Authorizer should report an error.
	AttributeRestrictions runtime.EmbeddedObject
	// ResourceKinds is a list of kinds this rule applies to.  "*" represents all kinds.
	ResourceKinds []string
}

// Role is a logical grouping of PolicyRules that can be referenced as a unit by RoleBindings.
type Role struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	Rules []PolicyRule
}

// RoleBinding references a Role, but not contain it.  It adds who and namespace information.  
// It can reference any Role in the same namespace or in the master namespace.
type RoleBinding struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	UserNames    []string
	GroupNames    []string

	// Since Policy is a singleton per namespace, this is sufficient knowledge to locate a role
	// RoleRefs can only reference the current namespace and the master namespace
	// If the RoleRef cannot be resolved, the Authorizer must return an error.
	// RoleRef can only point to a Role contained in the Policy referenced by its containing PolicyBinding
	// If a RoleRef points to the master namespace, then the PolicyBinding object can be created automatically.
	// if a RoleRef does NOT point to the master namespace, then the PolicyBinding must exist BEFORE attempting
	// to add the RoleBinding
	RoleRef kapi.ObjectReference
}

// Policy is a object that holds all the Roles and RoleBindings for a particular namespace.  There is at most
// one Policy document per namespace.
// It is an attempt to mitigate referential integrity problems, but it is worth noting that it does not solve them.
// Referential integrity problems still exist for references to the master namespace.  If that problem is resolved
// then it seems more reasonable to destroy this resource and use the referential integrity solution to solve
// all cases.
type Policy struct{
	kapi.TypeMeta
	kapi.ObjectMeta

	LastModified time.Time

	Roles map[string]Role
}

// PolicyBinding holds a reference to a Policy and then a set of RoleBindings that must only point to Roles in the
// reference Policy.  The PolicyBinding name is the same as the namespace of the PolicyRef.  Only a PolicyBinding that
// points to the master namespace can be provisioned automatically.
type PolicyBinding struct{
	kapi.TypeMeta
	kapi.ObjectMeta

	LastModified time.Time

	// PolicyRef limits the scope to which any RoleBinding may point to Roles contained within that Policy
	PolicyRef kapi.ObjectReference

	RoleBindings map[string]RoleBindings
}
```

### Verb Set
Currently, we only have "readonly" and everything else.  This does not give sufficient control.  We need to support all of the standard HTTP verbs, plus different concepts such as "watch" that don't have equivalents.  Distinctions between various mutating verbs are important because the ability to update is different than the ability to create (kubelets can't create new pods, but they need to be able to update status on existing ones).  Distinctions between non-mutating verbs are useful when trying to control how much performance impact a user can have (a bad watch or list is a lot more punishing than a bad get).  Verbs are not hierarchical and each request should have exactly one verb.  Our authorizer will start with the following verbs, but other implementations could choose different ones.
  1.  exec
  1.  watch
  1.  list
  1.  get
  1.  create
  1.  update
  1.  delete
  1.  proxy
  1.  permission-request
  1.  * - represents all verbs

### Verb Specific Attributes
Different verbs will require different attributes.  For instance, a list would have a kind, a get would have an kind and an id, and an update may have a kind, an id, and a list of the fields being modified.  Although exec isn't well defined, it could very well have an entirely distinct set of attributes.  The exact contract rules will be chosen by the Authorizer/AuthorizationAttributeBuilder pair, but basic resource access for openshift should include kind in order to be compatible with existing kubernetes authorization.

#### Why allow GroupNames instead of simply saying that RoleBindings only have a list of UserNames?  
Ease of maintenance and separation of powers.  Groups span namespaces, but RoleBindings do not.  If you specify the full set of users on every RoleBinding, then you have to repeat it for every namespace.  If that membership changes, you now have to update it in multiple locations.  As for the separation of powers, if you have rights to modify a RoleBinding, you have the power to change its permissions (RoleTemplateName) instead of only having the power to change its membership.  It seems reasonable to allow someone to modify group membership ("chubba is mechanic") without also having the ability to modify permissions associated with the group itself ("mechanics are super-admins").  Having Groups distinct from Roles and RoleBindings makes it easy to express this in policy.

## API
### Basic CRUD
The basic CRUD for Roles and RoleBindings, and Policy is not as straightforward as usual.  

The REST API will respond to gets, lists, creates, updates, and deletes (not watches) at the expected locations for Roles and RoleBindings.  The REST API will respond to gets, lists, and watches (not creates, updates, and deletes) at the expected location for Policy.  

Only one Policy document is allowed per namespace.  Roles and RoleBindings are not stored in etcd.  Instead, requests against their REST endpoints will result in inspection and modification of the Policy document.  This makes it easier to express restrictions on how policy can be manipulated, while making it possible to guarantee of referential integrity for references inside the local namespace.

#### /api/{version}/ns/{namespace}/resourceAccessReview
What users and groups can perform the specified verb on the specified resourceKind.  ResourceAccessReview will be a new type with associated RESTStorage that only accepts creates.  The caller POSTs a ResourceAccessReview to this URL with the `spec` values filled in.  He gets a ResourceAccessReview back, with the `status` values completed.  Here is an example of a call and its corresponding return.
```
{
	"kind": "ResourceAccessReview",
	"apiVersion": "v1beta3",
	"metadata": {
		"name": "create-pod-check",
		"namespace": "default"
		},
	"spec": {
		"verb": "create",
		"resourceKind": "pods"
	}
}

curl -X POST /api/{version}/ns/{namespace}/resourceAccessReviews -d @resource-access-review.json
or 
accessReviewResult, err := Client.ResourceAccessReviews(namespace).Create(resourceAccessReviewObject)

{
	"kind": "ResourceAccessReview",
	"apiVersion": "v1beta3",
	"metadata": {
		"name": "create-pod-check",
		"namespace": "default"
		},
	"spec": {
		"verb": "create",
		"resourceKind": "pods"
	},
	"status": {
		"userNames": ["Clark", "Hubert"],
		"groupNames": ["cluster-admins"]
	}
}
```
Verbs are the standard RESTStorage verbs: get, list, watch, create, update, and delete.

#### /api/{version}/ns/{namespace}/subjectAccessReview
Can the user or group (use authenticated user if none is specified) perform a given request?  SubjectAccessReview will be a new type with associated RESTStorage that only accepts creates.  The caller POSTs a SubjectAccessReview to this URL with the `spec` values filled in.  He gets a SubjectAccessReview back, with the `status` values completed.  Here is an example of a call and its corresponding return.
```
// input
{
	"kind": "SubjectAccessReview",
	"apiVersion": "v1beta3",
	"metadata": {
		"name": "clark-create-check",
		"namespace": "default",
		},
	"spec": {
		"verb": "create",
		"resourceKind": "pods",
		"userName": "Clark",
		"content": {
			"kind": "Pod",
			"apiVersion": "v1beta3"
			// rest of pod content
		}
	}
}

// POSTed like this
curl -X POST /api/{version}/ns/{namespace}/subjectAccessReviews -d @subject-access-review.json
// or 
accessReviewResult, err := Client.SubjectAccessReviews(namespace).Create(subjectAccessReviewObject)

// output
{
	"kind": "SubjectAccessReview",
	"apiVersion": "v1beta3",
	"metadata": {
		"name": "clark-create-check",
		"namespace": "default",
		},
	"spec": {
		"verb": "create",
		"resourceKind": "pods",
		"userName": "Clark",
		"content": {
			"kind": "Pod",
			"apiVersion": "v1beta3"
			// rest of pod content
		}
	}
	"status": {
		"allowed": true
	}
}
```

The actual Go objects look like this:
```
type SubjectAccessReviewSpec struct{
	// Verb is one of: get, list, watch, create, update, delete
	Verb string

	// ResourceKind is one of the existing resource types
	ResourceKind string

	// UserName is optional and mutually exclusive to GroupName.  If both UserName and GroupName are empty,
	// the current authenticated username is used.
	UserName string

	// GroupName is optional and mutually exclusive to UserName.
	GroupName string

	// Content is the actual content of the request for create and update
	Content runtime.EmbeddedObject

	// ResourceName is the name of the resource being requested for a "get" or deleted for a "delete"
	ResourceName string
}

type SubjectAccessReviewStatus struct{
	// Allowed is required.  True if the action would be allowed, false otherwise.
	Allowed bool

	// DenyReason is optional.  It indicates why a request was denied.
	DenyReason string

	// AllowReason is optional.  It indicates why a request was allowed.
	AllowReason string

	// EvaluationError is optional.  It indicates why a SubjectAccessReview failed during evaluation
	EvaluationError string
}

type SubjectAccessReview struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	Spec    SubjectAccessReviewSpec
	Status  SubjectAccessReviewStatus
}
```



### Runtime Authorization Types
```
type Authorizer interface{
	// Authorize can return both allowedBy and deniedBecause strings at the same time.  
	// This can happen when one rule allows an action, but another denies the action.
	// Allowing both to be returned makes it easier to track why policy is allowing or
	// denying a particular action.  Allowed by must be non-empty and deniedBecause must
	// be empty in order for an action to be allowed.
	Authorize(a AuthorizationAttributes) (allowedBy string, deniedBecause string, error)

	// GetAllowedSubjects takes a set of attributes, ignores the UserInfo() and returns back
	// the users and groups who are allowed to make a request that has those attributes.  This 
	// API enables the ResourceBasedReview requests below
	GetAllowedSubjects(attributes AuthorizationAttributes) (users []string, groups []string, error)
}

// AuthorizationAttributeBuilder takes a request and creates AuthorizationAttributes.  
// Since the attributes returned can vary based on verb type, AuthorizationAttributeBuilders 
// are paired with Authorizers during registration.
type AuthorizationAttributeBuilder interface{
	GetAttributes(request http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface{
	GetUserInfo() UserInfo  // includes GetName() string GetGroups() []string
	GetVerb() string
	GetNamespace() string
	GetResourceKind() string
	// GetRequestAttributes is of type interface{} because different verbs and different 
	// Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements
	GetRequestAttributes() interface{}
}
```
A single Authorizer/AuthorizationAttributeBuilder pair may be registered with the apiserver.


### Complex examples
Here are some examples of more complex behavior.

#### Project scoped Roles
We can continue the simple example from above with `Clark` the cluster admin, `Hubert` the project admin of the `hammer` project, and `Edgar` an editor in the `hammer` project.  Let's say that `Hubert` wants to create custom roles and restrictions.

By default, `Hubert` does not have sufficient power to create new Roles because there is not an existing Policy object in the `hammer` namespace.  `Hubert` has rights to create Roles, but the requests don't work because no existing Policy object exist.  `Clark` must first create a Policy and a PolicyBinding object:
```
{
	"kind": "Policy",
	"name": "Policy",
	"namespace": "hammer"
}
{
	"kind": "PolicyBinding",
	"name": "hammer",
	"namespace": "hammer"
}
```

Once those container resources are created, `Hubert` can now create Roles and RoleBindings that use them.

`Hubert` wants to create a bot that only has permission to touch labels on DeploymentConfigs.  He can create a role like this (note that a role is required because we want to have a set of two rules):
```
{
	"kind": "role",
	"name": "deploymentConfigLabelers",
	"namespace": "hammer",
	"rules": [
		{ "verbs": ["watch", "list", "get"], "resourceKinds": ["DeploymentConfig"] },
		{ "verbs": ["update"], "resourceKinds": ["DeploymentConfig"], "attributeRestrictions" : {"fieldsMutatable": ["labels"]} }
	]
}
{
	"kind": "roleBinding",
	"name": "DeploymentConfigLabelerBots",
	"namespace": "hammer",
	"roleRef": {
		"namespace": "hammer",
		"name": "deploymentConfigLabelers"
	}
	"userNames": ["ProtectorBot", "DeprotectorBot"]
}
```

Finally, `Hubert` wants to prevent `Edgar` from ever deleting a protected deploymentConfig again, so he creates this role and roleBinding:
```
{
	"kind": "role",
	"name": "fatFingeredEditor",
	"namespace": "hammer",
	"rule": [
		{ "Deny": "true", "Verbs": ["delete"], "ResourceKinds": ["DeploymentConfig"], "AttributeRestrictions" : {"labelsContain": ["protected"]} }
	]
}
{
	"kind": "roleBinding",
	"name": "FatFingeredEditors",
	"namespace": "hammer",
	"roleRef": {
		"namespace": "hammer",
		"name": "fatFingeredEditor"
	}
	"userNames": ["Edgar"]
}
```


#### Kubelets can modify Pods that they are running, but no others
In addition to constraining basic resources, there are some rules that are more difficult to request.  For some of these, it is easier to build a special evaluation method than trying to specify a generic set of property to express the restriction.  One way to express this is to create a rule for each individual pod that allows that particular kubelet use to access it.  This results in a tremendous number of rules and a lot of churn in the rules themselves.  Rather than do this, we can add a special kind of AttributeRestriction to a single rule.
```
{
	"kind": "role",
	"name": "kubelet",
	"namespace": "master",
	"rules": [
		{
			"verbs": ["*"], 
			"resourceKinds": ["Pod"],  
			"attributeRestrictions": {
				"kind": "sameMinionRestriction"
			}
		},
	]
}
{
	"kind": "roleBinding",
	"name": "Kubelets",
	"namespace": "master",
	"roleRef": {
		"namespace": "master",
		"name": "kubelet"
	}
	"userNames": ["kubelet-01", "kubelet-02"]
}
```

When Authorizer.Authorize() is called, it will see a 
```
// pretend authorizationAttributes
{
	"user": "kubelet",
	"verb": "get",
	"namespace": "pod's namespace",
	"resourceKind": "pod"
}

// kubelet policy rule it extracted based on the role
{
	"verbs": ["*"], 
	"resourceKinds": ["Pod"],  
	"attributeRestrictions": {
		"kind": "sameMinionRestriction"
	}
},

```
Based on this information, the authorizer will know that it has to locate and evaluate something called, "sameMinionRestriction".  The code could be as simple as:
```
// psuedo code
switch policyRule.AttributeRestrictions.(type){
case "sameMinionRestriction":
	// check to see if the user is kubelet, if not fail
	// check which minion the kubelet is running on
	// get the pod that the minion is trying to modify
	// check to see if that pod is running on the same minion as the kubelet
	// if so, return authorized.  if not, return denied
}

// do normal resource kind restrictions	

```

Making judicious use of the attributeRestrictions to describe difficult rules will make it easier to specify and understand the overall policy.  This is especially valuable for rules that are based on live environmental information as opposed to information that is statically determinant from the request itself.