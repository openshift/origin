# Organizations

## Problems
 1.  We need to be able to allocate quota to a group of projects.
 2.  We need to be able to limit a particular project's usage of shared quota (think htb rate/ceil for tc qdisc classes).
 3.  We need to be able to manage groups across related projects, but not across the entire cluster.
 4.  We need to be able to limit how many projects a given User/Organization can have.  (limits provisioning)

We can do this with an Organization entity that can manage multiple projects.  An Organization can have multiple org-admins who are allowed to manage quota allocation to owned projects and OrgGroups (groups that scoped to projects owned by an Organization).  A project is owned by at most one Organization, an Organization can own muliple projects.

### Open Questions
 1.  Should we have a two layer scoping `/api/v1/organization/<org name>/namespace/<namespace name>` or keep the current namespace structure?

     Keeping the current namespace structure mitigates changes, but prevents two orgs from having the same namespace and makes storing additional org-scoped resources like quota and groups more difficult.
     I'm in favor of keeping the current structure, but allowing deeper nesting of subresources and having named subresources.

 2.  Should we allow projects to be owned by Users or only allow ownership by Organizations?

     Allowing Users to own projects eliminates the creation of another "linked" resource that needs to be protected and maintained.
     Having a single entity allowed to own projects limits some duplication of things like quota.

 3.  Can we punt on adding users as organization members?

     Organization-admins should not be able to force users into an Org.  Users should not be able to join any Org they want.  I'd like to get the structure correct and punt on the ability to add members.  It would require some sort of invitation/request/approval flow.

 4.  Can we punt on project transfering?

     An org-admin shouldn't be able to take a project from someone else and they shouldn't be allowed to orphan their own project.  That means we'd need an offer/request/approval flow.



## Organizations
An Organization is a cluster scoped API object that references Users and has subresources for accessing and modifying Organization scoped OrgGroups.
```
// Organization contains the set of Users that are members of the Organization.
type Organization struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// AdminUsers is a list of Users that can administer this Organization.  By default a policy rule exists
	// that allows all Users to inspect and modify OrgGroups of Organizations that they are an Admin for.
	// Admins may NOT modify Organization membership.
	AdminUsers util.StringSet
	// AdminGroups is a list of Groups that can administer this Organization.  By default a policy rule exists
	// that allows all Users to inspect and modify OrgGroups of Organizations that they are an Admin for.
	// Admins may NOT modify Organization membership.
	AdminGroups util.StringSet

	// MemberUsers is a list of Users that are members of the Organization.  Only members may be assigned to OrgGroups.
	MemberUsers util.StringSet
	// MemberGroups is a list of Groups whose constituent users are considered members of the Organization.  Only members may be assigned to OrgGroups.
	MemberGroups util.StringSet
}
```

The Organization has a list of admins.  Admins have the power to:
 1.  Reserve quota for particular projects.
 2.  Accept ownership of a project.
 3.  Offer ownership of a project.
 4.  Remove (but not add) group members.
 5.  Add other AdminUsers and AdminGroups.  A cluster-admin will have to add the initial AdminUser or AdminGroup.
 5.  Create and modify organization groups.
 6.  Modify RoleBindings in projects the organization owns.  (This prevents accidentally orphaning a project.)

The Organization has a list of members.  Only users from this list may be assigned to OrgGroups.  This prevents an overzealous organization admin from creating OrgGroups made up of non-members.

Organizations will have an `organizations/members` subresource that flattens the list of members by resolving the constituent Users of the MemberGroups.


## Quota allocation
An Organization can be assigned quota by a cluster-admin.  The simplest way to model this is to have a cluster-scoped `ResourceQuota` (We can't call it that because `ResourceQuota` is a namespace-scope resource and it can't be in both scope.) that is linked to an Organization.  The existing quota admission controller would remain in place, but no limits would be set by default.  If an Organization admin decided that a particular project was starving the rest, he could assign hard limits to that Project's `ResourceQuota` to prevent it from taking more.  An additional admission controller would be responsible for doing the rollup across all projects in the Organization and preventing over-use of resources.

This approach is relatively easy to build, but it is limited because it doesn't reserve quota for a given project.  A more complicated approach would be to allow killing of "extra" resources and then provide a rate and ceiling like tc.  This would have an overall cap and allow "borrowing" of quota when resources are plentiful, but then selectively kill resources when resources are scarce to allow for project guarantees.  With enough effort, this sort of solution could be layered on top of kube by setting the ceiling for the existing quota controller and having a separate resource for the guarantee and a separate controller for the killer.

I'm sure there will be complaints no matter what we do.


## Default Policy
org-admins should have power somewhere between a cluster-admin and a project-admin.  Since the idea of an Organization doesn't scope cleanly, we can make use of the `attributeRestrictions` to run custom logic for a cluster-scoped rule.  We can create a `ClusterRoleBinding` to `system:authenticated` that allows a `ClusterRole` with the correct access to every project that the is owned by a Organization that the user is an org-admin for.  That makes it impossible for an org-admin to accidentally (or intentionally) remove his powers on a project.


## Project ownership
Projects will have an annotation that indicates which Organization owns them.  If we don't allow modifications to project by default, then a cluster-admin can sort it out for now until we make an offer/request/approval flow.


## Org Member powers
A given user can:
 1.  See which orgs he's a part of.  This will require a reverse-lookup map/cache and probably a subresource on user.
 2.  Leave an organization that he's tied to by MemberUser, but not one that he is tied to by MemberGroup.  That's an odd wrinkle.


## OrgGroups Overview
Project administrators need to be able to define groups so they can efficiently manage policy on their projects, however a project administrator is unlikely to be allowed to manage cluster-scoped group membership across all projects.  A single project administrator (or set of project administrators) is likely to own multiple related projects (think single department running multiple projects).  To allow cluster-scoped group definitions to be shared across multiple related projects, we'll introduce the concept of OrgGroups (scoped sets of users) and Organizations (things that reference Users and own OrgGroups).

Organizations are cluster-scoped resources that contains a set of OrgGroups, a list of admins (users that can manage the groups), and a list of members (users that can be part of the OrgGroups).  Groups are cluster-scoped sets of users that are managed by a cluster-admin.  OrgGroups are organization scoped sets of users that are managed by the Organization.AdminUsers.  Projects may have at most one owning Organization.  OrgGroups may only be referenced in RoleBindings of Projects owned by the Organization.


### Use Cases
 1.  I want to enable a subset of users to be able to organize sets of Users into OrgGroups, but I only want those changes to affect policy evaluation across some projects.

### OrgGroups
OrgGroups look a lot like Groups, but they are always scoped to a particular Organization and they behave differently.  OrgGroups are managed by Organization.Admins (Groups are managed by cluster admins), OrgGroups can only contain Users that are Organization.Members (Groups can contain any user), OrgGroups can only be bound to roles in projects owned by the Organization (Groups can be bound to roles in any project).  They will be cluster scoped resources, but we'll indicate the Organization scope of an OrgGroup by prefixing every OrgGroup name with Organization name.  This prefixing allows two organizations to have groups with the same name.
```
// OrgGroup contains the set of Users that belong to a given OrgGroup.  The name of a OrgGroup is required to 
// to be in the form of "OrganizationName:OrgGroupName".
type OrgGroup struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Users is a list of Users that are members of the OrgGroup
	Users util.StringSet
}
```

A reverse lookup index will be built in memory (similar to the project cache) to be allow the authenticator to make fast lookups for OrgGroup membership determination.  A valid OrgGroup.Name can never be used as a Group.Name and vice-versa.  This makes it impossible for anyone (including cluster-admins) to create a Group that will collide with an OrgGroup.


### user.Info and OrgGroups
user.Info is an interface shared with kubernetes.  If they won't change their interface, we'll be able to re-use it in a backwards compatible way by scope qualifying our OrgGroup names as Groups.  This means that being a member of OrgGroup "MyOrg:AOrgGroup" would result in a Group membership that looks like "org:MyOrg:AOrgGroup".  The only signficant side effect is that RoleBinding RESTStorage must reject Groups that start with "org:", but those Groups are invalid Group names anyway.


## Tying Projects to Organizations
A Project may have at most one owner (Organization).  An Organization may own many projects.  We will indicate project ownership by having an "openshift.io/owner" label that contains a value of the form "resource/namespace/name".  Initially, the only valid resource will be an Organization, so the value will look like: "organizations//MyOrg".  This allows for easy listing of projects owned by a particular Organization and allows us to extend the concept of project ownership to other resource types at a later time.


