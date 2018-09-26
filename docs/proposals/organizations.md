# Organizations

## Problems
 1.  Cluster administrators want to allocate a pool of resources to a subset of their company, and delegate the authority to manage how that pool is assigned to an organization administrator.
 2.  An organization administrator needs to be able to limit a particular project's usage of shared quota (think htb rate/ceil for tc qdisc classes).
 3.  Cluster administrators want to delegate control over groups of users to an organization administrator - that organization administrator should be able to manage the access of a set of users to individual projects under that organization umbrella.
 4.  Cluster administrators want to allow self-service of users on the cluster, but the total allocated resources those self service users can get access to is limited (to prevent abuse / unfair use of resources).

We can do this by introducing the concept of project ownerhsip.  A Project is owned by at most one (Organization xor User).  We'll also introduce the idea of a `ClusterResourceQuota` object that can be associated with an organization or a user. An Organization can have multiple org-owners who are allowed to manage quota allocation to owned projects and OrgGroups (groups that scoped to projects owned by an Organization).

### Open Questions
 3.  Can we punt on adding users as organization members?  Still don't see a clear answer on this.

     Organization-admins should not be able to force users into an Org.  Users should not be able to join any Org they want.  I'd like to get the structure correct and punt on the ability to add members.  It would require some sort of invitation/request/approval flow.

 4.  Can we punt on project transfering?  Sounds like yes.  A cluster-admin could transfer ownership.

     An org-owner shouldn't be able to take a project from someone else and they shouldn't be allowed to orphan their own project.  That means we'd need an offer/request/approval flow.

 6.  What things can org-owners do?

 	There's a list below proposing things.


## ClusterResourceQuota
`ClusterResourceQuota` mirrors `ResourceQuota`, but at the cluster-scope.  (We can't call it that because `ResourceQuota` is a namespace-scope resource and it can't be in both scope.)  A `ClusterResourceQuota` has an optional owner that is either a user or an organization and an optional label selector.  An empty label selector is `true`.  If both are set, then the `ClusterResourceQuota` applies to all namespaces matching the selector **and** owned by the owner.

The existing quota admission controller would remain in place, but no limits would be set by default.  If a project owner decided that a particular project was starving the rest, he could assign hard limits to that Project's `ResourceQuota` to prevent it from taking more.  An additional admission controller would be responsible for doing the rollup across all projects in the Organization and preventing over-use of resources.

```go
type ClusterResourceQuota struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Spec defines the desired quota
	Spec ResourceQuotaSpec 

	// Status defines the actual enforced quota and its current usage
	Status ResourceQuotaStatus 
}

type ClusterResourceQuotaSpec struct {
	Owner    *kapi.ObjectReference
	Selector map[string]string 

	kapi.ResourceQuotaSpec
}
```

#### Limitations
This approach is relatively easy to build, but it is limited because it doesn't reserve quota for a given project.  A more complicated approach would be to allow killing of "extra" resources and then provide a rate and ceiling like tc.  This would have an overall cap and allow "borrowing" of quota when resources are plentiful, but then selectively kill resources when resources are scarce to allow for project guarantees.  With enough effort, this sort of solution could be layered on top of kube by setting the ceiling for the existing quota controller and having a separate resource for the guarantee and a separate controller for the killer.

I'm sure there will be complaints no matter what we do.


## Project Owners
Projects will have an label that indicates which Subject owns them.  If we don't allow modifications to project by default, then a cluster-admin can sort out project transfers.
All project owners (both Users and Organizations) have a need to perform some actions scoped to them as an owner.  These will be represented as subresources under `oapi/v1/users/foo/<blah>` and `oapi/v1/organizations/foo/<blah>`.

1. `resourcequotas` - Supports `list`.  This provides readonly access to all the `ClusterResourceQuotas` that apply to this owner.  Eventually we need to sort out `create` to allow additional subdivision by a user, but that would imply a need for `update` and `delete`.  `Update` and `delete`, should only be allowed on `resourcequotas` that the project owner has made.  That means we'd want some sort of "protected" flag or we could choose to model it as a different subresource backed by a different cluster-scope resource.
1. `ownedprojects` - Supports `list`.  This provides readonly access to all the projects that you own.  We don't claim `projects`, because that's a likely spot to hang "all the projects you can see".



## Organizations
An Organization is a cluster scoped API object that references Users and has subresources for accessing and modifying Organization scoped OrgGroups.
```go
// Organization contains the set of Users that are members of the Organization.
type Organization struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Owners is a list of Subjects that can administer this Organization.  By default a policy rule exists
	// that allows all Users to inspect and modify OrgGroups of Organizations that they are an Admin for.
	// Admins may NOT modify Organization membership.
	Owners []kapi.ObjectReference

	// Members is a list of Subjects that are members of the Organization.  Only members may be assigned to OrgGroups.
	Members []kapi.ObjectReference
}
```

The Organization has a list of admins.  Admins have the power to:
 1.  Reserve quota for particular projects.
 2.  Remove (but not add) group members.
 3.  Add other Owners.  A cluster-admin will have to add the initial Owner.
 4.  Create and modify organization groups.
 5.  Modify RoleBindings in projects the organization owns.  (This prevents accidentally orphaning a project.)

The Organization has a list of members.  Only users from this list may be assigned to OrgGroups.  This prevents an overzealous organization admin from creating OrgGroups made up of non-members.

Organizations will have an `organizations/members` subresource that flattens the list of members by resolving the constituent Subjects of Members.


## Default Policy
org-owners should have power somewhere between a cluster-admin and a project-admin.  Since the idea of an Organization doesn't scope cleanly, we can make use of the `attributeRestrictions` to run custom logic for a cluster-scoped rule.  We can create a `ClusterRoleBinding` to `system:authenticated` that allows a `ClusterRole` with the correct access to every project that the is owned by a Organization that the user is an org-owner for.  That makes it impossible for an org-owner to accidentally (or intentionally) remove his powers on a project.


## Org Member powers
A given user can:
 1.  See which orgs he's a part of.  This will require a reverse-lookup map/cache and probably a subresource on user.
 2.  Leave an organization that he's tied to by direct Subject reference, but not one that he is tied to by a Group suibject.  That's an odd wrinkle.


## OrgGroups Overview
Organization owner need to be able to define groups so they can efficiently manage policy on their projects, however a organization owner is unlikely to be allowed to manage cluster-scoped group membership across all projects.  A single organization owner (or set of organization owners) is likely to own multiple related projects (think single department running multiple projects).  To allow cluster-scoped group definitions to be shared across multiple related projects, we'll introduce the concept of OrgGroups (scoped sets of users) that are logicaly owned by Organizations.

OrgGroups look a lot like Groups, but they are always logically scoped to a particular Organization and they behave differently.  They will be cluster scoped resources, but we'll indicate the Organization scope of an OrgGroup by prefixing every OrgGroup name with Organization name.  This prefixing allows two organizations to have groups with the same name.  OrgGroups are managed by Organization.Owners (Groups are managed by cluster admins), OrgGroups can only contain Users that are Organization.Members (Groups can contain any user), OrgGroups can only be bound to roles in projects owned by the Organization (Groups can be bound to roles in any project).


### Use Cases
 1.  I want to enable a subset of users to be able to organize sets of Users into OrgGroups, but I only want those changes to affect policy evaluation across some projects.

### OrgGroups
```go
// OrgGroup contains the set of Users that belong to a given OrgGroup.  The name of a OrgGroup is required to 
// to be in the form of "OrganizationName:OrgGroupName".
type OrgGroup struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Users is a list of Users that are members of the OrgGroup
	Users []string
}
```

A reverse lookup index will be built in memory (similar to the project cache) to be allow the authenticator to make fast lookups for OrgGroup membership determination.  A valid OrgGroup.Name can never be used as a Group.Name and vice-versa.  This makes it impossible for anyone (including cluster-admins) to create a Group that will collide with an OrgGroup.


### user.Info and OrgGroups
user.Info is an interface shared with kubernetes.  If they won't change their interface, we'll be able to re-use it in a backwards compatible way by scope qualifying our OrgGroup names as Groups.  This means that being a member of OrgGroup "MyOrg:AOrgGroup" would result in a Group membership that looks like "org:MyOrg:AOrgGroup".  The only signficant side effect is that RoleBinding RESTStorage must reject Groups that start with "org:", but those Groups are invalid Group names anyway.


## Tying Projects to Organizations
A Project may have at most one owner (Organization).  An Organization may own many projects.  We will indicate project ownership by having an "openshift.io/owner" label that contains a value of the form "resource/namespace/name".  Initially, the only valid resource will be an Organization, so the value will look like: "organizations//MyOrg".  This allows for easy listing of projects owned by a particular Organization and allows us to extend the concept of project ownership to other resource types at a later time.


