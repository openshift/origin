# Organization Groups

## Problem
Project administrators need to be able to define groups so they can efficiently manage policy on their projects, however a project administrator is unlikely to be allowed to manage cluster-scoped group membership across all projects.  A single project administrator (or set of project administrators) is likely to own multiple related projects (think single department running multiple projects).  To allow cluster-scoped group definitions to be shared across multiple related projects, we'll introduce the concept of OrgGroups (scoped sets of users) and Organizations (things that reference Users and own OrgGroups).


## Use Cases
 1.  I want to enable a subset of users to be able to organize sets of Users into OrgGroups, but I only want those changes to affect policy evaluation across some projects.
 1.  I want to easily find all projects that a given Organization owns.


## Overview
Organizations are cluster-scoped resources that contains a set of OrgGroups, a list of admins (users that can manage the groups), and a list of members (users that can be part of the OrgGroups).  Groups are cluster-scoped sets of users that are managed by a cluster-admin.  OrgGroups are organization scoped sets of users that are managed by the Organization.AdminUsers.  Projects may have at most one owning Organization.  OrgGroups may only be referenced in RoleBindings of Projects owned by the Organization.


## OrgGroups
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

The Organization has a list of admins.  A default policy rule is bound to all-authenticated users that allows them inspect and modify OrgGroups of Organizations that they are an Admin for (whether via User or Group).  Having the policy rule described and bound in this way makes it easier to manage mutiple organizations without having a proliferation of roles and rolebindings.

The Organization has a list of members.  Only users from this list may be assigned to OrgGroups.  This prevents an overzealous organization admin from creating OrgGroups made up of non-members.  By default, an organization admin does not have rights to change Organization membership.

*****TBD*****: How are users added to the Organization???  Being able to modify the OrgGroup structure of known members seems logically distinct from being able to force users into an Organization.  I wouldn't expect the same person to do both.

Organizations will have an `organizations/members` subresource that flattens the list of members by resolving the MemberGroups.

Organizations will have an `organizations/groups` subresource.  A `GET` returns the list of OrgGroups.  A `POST` with a OrgGroup creates a new OrgGroup.  A `PUT` with a OrgGroup updates a new OrgGroup.  Subresources can't be named, so we do not have an individual get for `organizations/groups`.  This approach will require enhancements to `kubectl`, but prevents an Organization admin from being able to perform a `list` operation against `orggroups`.  The alternative is to have `list` on `orggroups` only show the `OrgGroups` you're allowed to see, but that would be different from all other `list`s.


## Tying Projects to Organizations
A Project may have at most one owner (Organization).  An Organization may own many projects.  We will indicate project ownership by having an "openshift.io/owner" label that contains a value of the form "resource/namespace/name".  Initially, the only valid resource will an Organization, so the value will look like: "organizations//MyOrg".  This allows for easy listing of projects owned by a particular Organization and allows us to extend the concept of project ownership to other resource types at a later time.


## Using OrgGroups in RoleBindings
RoleBindings will get a new field:
```
	// OrgGroups holds all the OrgGroups directly bound to the role.
	OrgGroups util.StringSet
```
The `OrgGroups` field can only refer to OrgGroups owned by the Organization that owns the Project.  Since OrgGroup names are always scope qualified, any disallowed OrgGroup name will be rejected by the RESTStorage and will not be respected by our authorizer.  Our `oc policy` commands will be able to automatically scope qualifying OrgGroups.  Having a separate field makes it possible to export rolebindings from one project and create them in another project without getting rejected by admission for having invalid OrgGroup references in the Groups field.


## user.Info and OrgGroups
user.Info is an interface shared with kubernetes.  If they won't change their interface, we'll be able to re-use it in a backwards compatible way by scope qualifying our OrgGroup names as Groups.  This means that being a member of OrgGroup "MyOrg:AOrgGroup" would result in a Group membership that looks like "org:MyOrg:AOrgGroup".  The only signficant side effect is that RoleBinding RESTStorage must reject Groups that start with "org:", but those Groups are invalid Group names anyway.
