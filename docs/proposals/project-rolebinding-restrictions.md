# Project RoleBinding Restrictions

## Problem
A project-admin may not be allowed to bind any subject he chooses into his project.  Doing so can produce problems
for the bindee like unexpected typo shadowing or an unviewable console (too many projects).  On the binder side,
the project-admin may accidentally expose confidential projects to unauthorized users (think private repos on github).

To prevent those issues, a cluster-admin should be able to restrict the set of subjects (users, groups, serviceaccounts)
that can be bound to roles in a given project.

 1. Project ownerA somehow establishes a relationship with userB (invite/request/approve flow).  Once established
    userB should be bindable into any project owned by ownerA.
 2. Binding subject restrictions should be possible on a per-project basis: userA can be bound into project1,
    but not project2.  This allows a project owner to restrict a project admin, but still delegate bounded bindings.
 3. Project owner's are likely to have associated groups (pseudo-org things), they'll want to restrict subject binding
    based on group literals or group labels.



## Mechanism
The restrictions are naturally per project, but the selected sets of subjects are likely to be re-used across related
projects.  Even without first class organizations, the concept of "related projects" has been introduced with 
ClusterResourceQuota.  Some projects are managed together.  That relatedness, along with the scale of subjects versus
projects, means that we're going to want to describe the restrictions on the projects instead of on the subjects.

Since projects are not first class entities, we'll have to use annotations to link projects to subjects.

### SystemGroups and SystemUsers
SystemGroups are used sparingly in the system and can only originate from certificate-based users (very rare and 
highly privileged) and synthetic groups like anonymous, unauthenticated, and all.  We probably want to be able to restrict
the synthetics.  Since synthetic groups are unlabelable (no resource representation) and the known set is small,
the most straightforward way to do this is with a set of literal strings.  Keep this in mind.

### Groups
To restrict the set of groups that a user can bind, we'll want to select a set of groups.  The mechanism to do that 
kube/openshift is label selection.  We can introduce the `authorization.openshift.io/bindable-groups` annotation to hold
a list of allowed literal projects (remember SystemGroups?) and a label selector to choose groups which can be bound 
into the project.  The result is the union of the label selection and the literals.

In terms of representation, two fields either means two annotations (blech) or serialized kind (also blech).  I'd choose the
serialized json for ease of migration later in life.

### Users
Restricting the set of users is more difficult.  You probably want to select based on:
 1.  literals - remember SystemUsers?
 2.  members of a literal set of groups - groups are the way to manage sets of users.  We should re-use it.
 3.  members of a label selected set of groups - If you have < 5 groups, you use a literal.  If you have more than 5, 
     you probably use a label selector.

Again with the representation choices, looks like serialized json to me.

### ServiceAccounts
If the user binding the service account has access to the SA token secret, then that user logically "owns" the SA
and is allowed to bind it into his project.

### "I own this subject"
If the user binding the subject has the power to add a user to the allowed subjects, should he be able to bind
any subject he wants?  If you can edit the group that is used to restrict users, should you be able to bind any user?
I assert, no.  The purpose for these restrictions is not just to prevent a project-admin from doing something, but also
preventing unexpected subjects from joining the project.  If the user doing the binding has the power to make the 
subject allowed, he should have to perform that step first.  It helps prevent fat-fingering.

## Serialized JSON
It may look ugly inside of annotation, but it gives us clean versioning if we want to phase in various aspects of the 
restrictions or change it in the future.  We also have a structured way to migrate these fields wholesale using
the migration tools if it becomes necessary.  If we use unstructured lists, we lose the easy path for making that work.