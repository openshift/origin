# Cluster Groups

## Problem
To allow cluster-scoped group definitions to be shared across multiple related projects, we'll introduce the concept of Groups (cluster-scoped sets of users) that are managed by cluster-admins.


## Use Cases
 1.  I want to be able to define some Groups that will affect policy evaluation across all projects.
 1.  I want to be able to efficiently manage group membership.


## Groups
Groups are cluster-scoped sets of users that are managed by cluster-admins and may contain any user.  Groups can be bound to roles in all projects.  That means that if UserA is added to GroupB and ProjectC has a rolebinding that says "GroupB is an editor", UserA now has editor permissions in ProjectC.  Groups will be described using an API object that looks like this:
```
// Group contains the set of users that belong to a given Group.  The name may not contain ":"
type Group struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// Users is a list of Users that are members of the Group
	Users util.StringSet
}
```

A reverse lookup index will be built in memory (similar to the project cache) to be allow the authenticator to make fast lookups for Group membership determination.
