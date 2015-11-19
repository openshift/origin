# Policy

## Problem
Policy extends the existing authorization rules in Kubernetes (https://github.com/kubernetes/kubernetes/blob/master/docs/admin/authorization.md) to fulfill requirements around extended verb sets, verb specific attributes, role based management of attribute based access control, and more efficient evaluation of authorization status at larger scales.  For discussion about verbs, see: https://github.com/kubernetes/kubernetes/issues/2877.  For discussion about allowing mutation of some fields in a resource and not others, see: https://github.com/kubernetes/kubernetes/issues/2726.

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
		{ "verbs": ["get", "list", "watch"], "resources": ["resourcegroup:exposedopenshift", "resourcegroup:allkube"] }
	]
}
{
	"kind": "role",
	"name": "edit",
	"namespace": "master",
	"rules": [
		{ "verbs": ["get", "list", "watch", "create", "update", "delete"], "resources": ["resourcegroup:exposedopenshift", "resourcegroup:exposedkube"] }
		{ "verbs": ["get", "list", "watch"], "resources": ["resourcegroup:allkube"] }
	]
}
{
	"kind": "role",
	"name": "admin",
	"namespace": "master",
	"rules": [
		{ "verbs": ["get", "list", "watch", "create", "update", "delete"], "resources": ["resourcegroup:exposedopenshift", "resourcegroup:granter", "resourcegroup:exposedkube"] }
		{ "verbs": ["get", "list", "watch"], "resources": ["resourcegroup:policy", "resourcegroup:allkube"] }
	]
}
{
	"kind": "role",
	"name": "cluster-admin",
	"namespace": "master",
	"rules": [
		{ "verbs": ["*"], "resources": ["*"] }
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
 1.  A PolicyRule expresses a permission containing: []Verb, []Resources (pod, deploymentConfig, resourcegroup:deployments, etc), []ResourceNames
 1.  A Role is a way to name a set of PolicyRules
 1.  A Policy is a container for Roles.  There can only be one Policy per namespace.
 1.  A RoleBinding is a way to associate a Role with a given user or group.  A RoleBinding references (but does not include) a Role.
 1.  A PolicyBinding is a container for RoleBindings.  There can be many PolicyBinding objects per namespace.  RoleBindings may reference Roles in another namespace.  In usage, a RoleBinding that references a missing Role will produce an error **if** the requested action cannot be allowed by another Role bound to the user.
 1.  A master namespace exists.  This namespace is configurable from the command line and it is special.  Roles defined in the master namespace can be referenced from any other namespace.  Bindings defined in the master namespace apply to **all** namespaces (an admin in the master namespace can edit any resource in any namespace).
 1.  Roles versus Groups.  A Role is a grouping of PolicyRules.  A group is a grouping of users.  A RoleBinding associates a Role with a set of users and/or groups.  This distinction makes Groups easier to share across projects.
 1.  There are some predefined resourcegroups.  When included as a Resource element in a PolicyRule, they grant permissions to multiple Resources.  For instance, `resourcegroup:deployments` contains `deployments`, `deploymentconfigs`, `generatedeploymentconfigs`, `deploymentconfigrollbacks`.
 1.  ResourceNames is an optional white list of names that the rule applies to.  This restriction only makes sense for verbs that will provide a name in addition to other information, so operations like `get`, `update`, and `delete`.  An empty set (the default) means that any name is allowed.

Policy Evaluation
In order to determine whether a request is authorized, the AuthorizationAttributes are tested in the following order:
  1. all allow RoleBinding PolicyRules in the master namespace - short circuit on match
  1. all allow RoleBinding PolicyRules in the namespace - short circuit on match
  1. deny by default

### Configtime Authorization Types
These are the types used in the policy example above.  They allow us: to quickly find the policy rules that apply to namespace in etcd, to easily delegate project level control to a project admin, and to easily segregate all role control (a project admin can create/update/delete RoleBindings, but not Roles).
```
// PolicyRule holds information that describes a policy rule, but does not contain information
// about who the rule applies to or which namespace the rule applies to.
type PolicyRule struct {
	// Verbs is a list of Verbs that apply to ALL the ResourceKinds and AttributeRestrictions contained in this rule.  VerbAll represents all kinds.
	Verbs util.StringSet
	// AttributeRestrictions will vary depending on what the Authorizer/AuthorizationAttributeBuilder pair supports.
	// If the Authorizer does not recognize how to handle the AttributeRestrictions, the Authorizer should report an error.
	AttributeRestrictions runtime.EmbeddedObject
	// Resources is a list of resources this rule applies to.  ResourceAll represents all resources.
	Resources util.StringSet
	// ResourceNames is an optional white list of names that the rule applies to.  An empty set means that everything is allowed.
	ResourceNames util.StringSet
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

	// Users holds all the usernames directly bound to the role
	Users util.StringSet
	// Groups holds all the groups directly bound to the role
	Groups util.StringSet

	// Since Policy is a singleton, this is sufficient knowledge to locate a role
	// RoleRefs can only reference the current namespace and the global namespace
	// If the RoleRef cannot be resolved, the Authorizer must return an error.
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
	Roles map[string]Role
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
	RoleBindings map[string]RoleBinding
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
  1.  * - represents all verbs

### Verb Specific Attributes
Different verbs will require different attributes.  For instance, a list would have a kind, a get would have an kind and an id, and an update may have a kind, an id, and a list of the fields being modified.  Although exec isn't well defined, it could very well have an entirely distinct set of attributes.  The exact contract rules will be chosen by the Authorizer/AuthorizationAttributeBuilder pair, but basic resource access for openshift should include kind in order to be compatible with existing kubernetes authorization.

#### Why allow Groups instead of simply saying that RoleBindings only have a list of Users?
Ease of maintenance and separation of powers.  Groups span namespaces, but RoleBindings do not.  If you specify the full set of users on every RoleBinding, then you have to repeat it for every namespace.  If that membership changes, you now have to update it in multiple locations.  As for the separation of powers, if you have rights to modify a RoleBinding, you have the power to change its permissions (RoleTemplateName) instead of only having the power to change its membership.  It seems reasonable to allow someone to modify group membership ("chubba is mechanic") without also having the ability to modify permissions associated with the group itself ("mechanics are super-admins").  Having Groups distinct from Roles and RoleBindings makes it easy to express this in policy.

## API
### Basic CRUD
The basic CRUD for Roles and RoleBindings, and Policy is not as straightforward as usual.

The REST API will respond to gets, lists, creates, updates, and deletes (not watches) at the expected locations for Roles and RoleBindings.  The REST API will respond to gets, lists, and watches (not creates, updates, and deletes) at the expected location for Policy.

Only one Policy document is allowed per namespace.  Roles and RoleBindings are not stored in etcd.  Instead, requests against their REST endpoints will result in inspection and modification of the Policy document.  This makes it easier to express restrictions on how policy can be manipulated, while making it possible to guarantee of referential integrity for references inside the local namespace.

#### /api/{version}/ns/{namespace}/resourceAccessReview
This API answers the question: which users and groups can perform the specified verb on the specified resourceKind.  ResourceAccessReview will be a new type with associated RESTStorage that only accepts creates.  The caller POSTs a ResourceAccessReview to this URL and he gets a ResourceAccessReviewResponse back.  Here is an example of a call and its corresponding return.
```
// input
{
	"kind": "ResourceAccessReview",
	"apiVersion": "v1beta3",
	"verb": "list",
	"resource": "replicationcontrollers"
}

// POSTed like this
curl -X POST /api/{version}/ns/{namespace}/resourceAccessReviews -d @resource-access-review.json
// or
accessReviewResult, err := Client.ResourceAccessReviews(namespace).Create(resourceAccessReviewObject)

// output
{
	"kind": "ResourceAccessReviewResponse",
	"apiVersion": "v1beta3",
	"namespace": "default"
	"users": ["Clark", "Hubert"],
	"groups": ["cluster-admins"]
}
```

The actual Go objects look like this:
```
// ResourceAccessReview is a means to request a list of which users and groups are authorized to perform the
// action specified by spec
type ResourceAccessReview struct {
	kapi.TypeMeta

	// Verb is one of: get, list, watch, create, update, delete
	Verb string
	// Resource is one of the existing resource types
	Resource string
	// Content is the actual content of the request for create and update
	Content runtime.EmbeddedObject
	// ResourceName is the name of the resource being requested for a "get" or deleted for a "delete"
	ResourceName string
}

// ResourceAccessReviewResponse describes who can perform the action
type ResourceAccessReviewResponse struct {
	kapi.TypeMeta

	// Namespace is the namespace used for the access review
	Namespace string
	// Users is the list of users who can perform the action
	Users util.StringSet
	// Groups is the list of groups who can perform the action
	Groups util.StringSet
}
```
Verbs are the standard RESTStorage verbs: get, list, watch, create, update, and delete.

#### /api/{version}/ns/{namespace}/subjectAccessReview
This API answers the question: can a user or group (use authenticated user if none is specified) perform a given action.  SubjectAccessReview will be a new type with associated RESTStorage that only accepts creates.  The caller POSTs a SubjectAccessReview to this URL and he gets a SubjectAccessReviewResponse back.  Here is an example of a call and its corresponding return.
```
// input
{
	"kind": "SubjectAccessReview",
	"apiVersion": "v1beta3",
	"verb": "create",
	"resource": "pods",
	"user": "Clark",
	"content": {
		"kind": "pods",
		"apiVersion": "v1beta3"
		// rest of pod content
	}
}

// POSTed like this
curl -X POST /api/{version}/ns/{namespace}/subjectAccessReviews -d @subject-access-review.json
// or
accessReviewResult, err := Client.SubjectAccessReviews(namespace).Create(subjectAccessReviewObject)

// output
{
	"kind": "SubjectAccessReviewResponse",
	"apiVersion": "v1beta3",
	"namespace": "default",
	"allowed": true
}
```

The actual Go objects look like this:
```
// SubjectAccessReview is an object for requesting information about whether a user or group can perform an action
type SubjectAccessReview struct {
	kapi.TypeMeta

	// Verb is one of: get, list, watch, create, update, delete
	Verb string
	// Resource is one of the existing resource types
	Resource string
	// User is optional.  If both User and Groups are empty, the current authenticated user is used.
	User string
	// Groups is optional.  Groups is the list of groups to which the User belongs.
	Groups util.StringSet
	// Content is the actual content of the request for create and update
	Content runtime.EmbeddedObject
	// ResourceName is the name of the resource being requested for a "get" or deleted for a "delete"
	ResourceName string
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
```



### Runtime Authorization Types
```
type Authorizer interface{
	// Authorize indicates whether an action is allowed or not and optionally a reason for allowing or denying.
	Authorize(a AuthorizationAttributes) (allowed bool, reason string, err error)

	// GetAllowedSubjects takes a set of attributes, ignores the UserInfo() and returns back
	// the users and groups who are allowed to make a request that has those attributes.  This
	// API enables the ResourceBasedReview requests below
	GetAllowedSubjects(attributes AuthorizationAttributes) (users util.StringSet, groups util.StringSet, error)
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
	GetResourceName() string
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
		{ "verbs": ["watch", "list", "get"], "resources": ["deploymentconfigs"] },
		{ "verbs": ["update"], "resources": ["deploymentconfigs"], "attributeRestrictions" : {"fieldsMutatable": ["labels"]} }
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
			"resources": ["pods"],
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
	"resources": ["Pod"],
	"attributeRestrictions": {
		"kind": "sameMinionRestriction"
	}
},

```
Based on this information, the authorizer will know that it has to locate and evaluate something called, "sameMinionRestriction".  The code could be as simple as:
```
// pseudo code
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
