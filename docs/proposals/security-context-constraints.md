## Abstract

Security context constraints relate [security contexts](./security_contexts.md) and
[service accounts](http://docs.k8s.io/design/service_accounts.md).  They are configured for a
cluster, namespaces, and users within the cluster to govern how a [security context](./security_contexts.md) is established.

## Motivation

Administration of a multi-tenant cluster requires the ability to provide varying sets of permissions
among the tenants, the infrastructure components, and end users of the system who may themselves be
administrators within their own isolated namespace.

Actors in a cluster may include infrastructure that is managed by administrators, infrastructure
that is exposed to end users (builds, deployments), the isolated end user namespaces in the cluster, and
the individual users inside those namespaces.  Infrastructure components that operate on behalf of a
user (builds, deployments) should be allowed to run at an elevated level of permissions without
granting the user themselves an elevated set of permissions.

## Goals

1.  Associate [service accounts](http://docs.k8s.io/design/service_accounts.md) and users with
a set of constraints that dictate how a [security context](./security_contexts.md) is established for a pod.
1.  Provide the ability for users and infrastructure components to run pods with elevated privileges
on behalf of another user or within a namespace where privileges are more restrictive.
1.  Secure the ability to reference elevated permissions or to change the constraints under which
a user runs.

## Use Cases

Use case 1:
As an administrator, I can create a namespace for a person that can't create privileged containers
AND enforces that the UID of the containers is set to a certain value

Use case 2:
As a cluster operator, an infrastructure component should be able to create a pod with elevated
privileges in a namespace where regular users cannot create pods with these privileges or execute
commands in that pod.

Use case 3:
As a cluster administrator, I can allow a given namespace (or service account) to create privileged
pods or to run root pods

Use case 4:
As a cluster administrator, I can allow a project administrator to control the security contexts of
pods and service accounts within a project


## Requirements

1.  Provide a set of restrictions that controlls how a security context is created as a new, cluster-scoped, object
called SecurityContextConstraints.
1.  User information in `user.Info` must be available to admission controllers. (Completed in
https://github.com/kubernetes/kubernetes/pull/8203)
1.  Some authorizers may restrict a user’s ability to reference a service account.  Systems requiring
the ability to secure service accounts on a user level must be able to add a policy that enables
referencing specific service accounts themselves.
1.  Admission control must validate the creation of Pods against the allowed set of constraints.

## Design

### Model

A security context constraint object exists in the root scope, outside of a namespace.  The
security context constraints will reference users and groups that are allowed
to operate under the constraints.  In order to support this, `ServiceAccounts` must be mapped
to a user name or group list by the authentication/authorization layers.  This allows the security
context to treat users, groups, and service accounts uniformly.

Below is a list of security context constraints which will likely serve most use cases:

1.  A default constraints object.  This object is permissioned to something covers all actors such
as a `system:authenticated` group and will likely be the most restrictive set of constraints.
1.  A default constraints object for service accounts.  This object can be identified as serving
a group identified by `system:service-accounts` which can be imposed by the service account authenticator / token generator.
1.  Cluster admin constraints identified by `system:cluster-admins` group - a set of constraints with elevated privileges that can be used
by an administrative user or group.
1.  Infrastructure components constraints which can be identified either by a specific service
account or by a group containing all service accounts.

```go
// SecurityContextConstraints governs the ability to make requests that affect the SecurityContext that will
// be applied to a container.
type SecurityContextConstraints struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	// AllowPrivilegedContainer determines if a container can request to be run as privileged.
	AllowPrivilegedContainer bool `json:"allowPrivilegedContainer,omitempty" description:"allow containers to run as privileged"`
	// AllowedCapabilities is a list of capabilities that can be requested to add to the container.
	AllowedCapabilities []CapabilityType `json:"allowedCapabilities,omitempty" description:"capabilities that are allowed to be added"`
	// AllowHostDirVolumePlugin determines if the policy allow containers to use the HostDir volume plugin
	AllowHostDirVolumePlugin bool `json:"allowHostDirVolumePlugin,omitempty" description:"allow the use of the host dir volume plugin"`
	// SELinuxContext is the strategy that will dictate what labels will be set in the SecurityContext.
	SELinuxContext SELinuxContextStrategyOptions `json:"seLinuxContext,omitempty" description:"strategy used to generate SELinuxOptions"`
	// RunAsUser is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	RunAsUser RunAsUserStrategyOptions `json:"runAsUser,omitempty" description:"strategy used to generate RunAsUser"`

	// The users who have permissions to use this security context constraints
	Users []string `json:"users,omitempty" description:"users allowed to use this SecurityContextConstraints"`
	// The groups that have permission to use this security context constraints
	Groups []string `json:"groups,omitempty" description:"groups allowed to use this SecurityContextConstraints"`
}

// SELinuxContextStrategyOptions defines the strategy type and any options used to create the strategy.
type SELinuxContextStrategyOptions struct {
	// Type is the strategy that will dictate what SELinux context is used in the SecurityContext.
	Type SELinuxContextStrategyType `json:"type,omitempty" description:"strategy used to generate the SELinux context"`

	//TODO
}

// RunAsUserStrategyOptions defines the strategy type and any options used to create the strategy.
type RunAsUserStrategyOptions struct {
	// Type is the strategy that will dictate what RunAsUser is used in the SecurityContext.
	Type RunAsUserStrategyType `json:"type,omitempty" description:"strategy used to generate RunAsUser"`
	// UID is the user id that containers must run as.  Required for the MustRunAs strategy if not using
	// namespace/service account allocated uids.
	UID *int64 `json:"uid,omitempty" description:"the uid to always run as; required for MustRunAs"`
	// AllocatedIDAnnotation provides an annotation that the strategy can look for if it should
	// use a pre-allocated UID that exists on the namespace or service account.
	AllocatedIDAnnotation string `json:"allocatedIDAnnotation,omitempty" description:"how the strategy can discover pre-allocated ids on the namespace or service account"`
	// UIDRangeMin defines the min value for a strategy that allocates by range.
	UIDRangeMin int64 `json:"uidRangeMin,omitempty" description:"min value for range based allocators"`
	// UIDRangeMax defines the max value for a strategy that allocates by range.
	UIDRangeMax int64 `json:"uidRangeMax,omitempty" description:"max value for range based allocators"`
	// UIDRangeAnnotation provides an annotation that the strategy can look for if ranges
	// are preallocated and assigned to the namespace or service account.
	UIDRangeAnnotation string `json:"uidRangeAnnotation,omitempty" description:"how the strategy can discover pre-allocated ranges on the namespace or service account"`
}

// SELinuxContextStrategyType denotes strategy types for generating SELinux options for a
// SecurityContext
type SELinuxContextStrategyType string

// RunAsUserStrategyType denotes strategy types for generating RunAsUser values for a
// SecurityContext
type RunAsUserStrategyType string

// SecurityContextConstraintsAllocator provides the implementation to generate a new security
// context based on constraints or validate an existing security context against constraints.
type SecurityContextConstraintsAllocator interface {
	// Create a SecurityContext based on the given constraints
	CreateSecurityContext(pod *api.Pod, container *api.Container, constraints *api.SecurityContextConstraints) (*api.SecurityContext, error)
	// Ensure a container's SecurityContext is in compliance with the given constraints
	ValidateSecurityContext(pod *api.Pod, container *api.Container, constraints *api.SecurityContextConstraints) fielderrors.ValidationErrorList
}

const (
  // container must have SELinux labels of X applied.
  SELinuxStrategyMustRunAs SELinuxContextStrategyType = "MustRunAs"
  // container may make requests for any SELinux context labels.
  SELinuxStrategyRunAsAny SELinuxContextStrategyType = "RunAsAny"

  // container must run as a particular uid.
  RunAsUserStrategyMustRunAs RunAsUserStrategyType = "MustRunAs"
  // container must run as a non-root uid
  RunAsUserStrategyMustRunAsNonRoot RunAsUserStrategyType = "MustRunAsNonRoot"
  // container may make requests for any uid.
  RunAsUserStrategyRunAsAny RunAsUserStrategyType = "RunAsAny"
)
```

### Security Context Constraints Lifecycle

As reusable objects in the root scope, security context constraints follow the lifecycle of the
cluster itself.  Maintenance of constraints such as adding, assigning, or changing them is the
responsibility of the cluster administrator.  For
instance, creating a new user within a namespace should not require the cluster administrator to
define the user's security context constraints.  They should receive the default set of constraints
that the administrator has defined for users.

Finalizers should ensure that any user or group references in a security context constraint object
will be removed when the referenced object is removed.


## Default Security Context Constraints And Overrides

In order to establish security context constraints for service accounts and users there must be a way
to identify the default set of constraints that is to be used.  This is best accomplished by using
groups.  As mentioned above, groups may be used by the authentication/authorization layer to ensure
that every user maps to at least one group (with a default example of `system:authenticated`) and it
is up to the cluster administrator to ensure that a security context constraint object exists that
references the group.

If an administrator would like to provide a user with a changed set of security context permissions
they may do the following:

1.  Create a new security context constraints object and add a reference to the user or a group
that the user belongs to.
1.  Add the user (or group) to an existing security context constraints object with the proper
elevated privileges.

## Admission

Admission control using an authorizer allows the ability to control the creation of resources
based on capabilities granted to a user.  In terms of the security context constraints it means
that an admission controller may inspect the user info made available in the context to retrieve
and appropriate set of security context constraints for validation.

The appropriate set of security context constraints is defined as all of the security context
constraints available that have reference to the user or groups that the user belongs to.

Admission will use the SecurityContextConstraintsAllocator to ensure that any requests for a
specific security context constraint setting or to generate settings using the following approach:

**Pod with referenced service account**

1.  First, ensure that the user has the ability to use the security context constraints of the
service account.  This can be accomplished by finding the intersection of allowed constraints between
the user and the service account.  If none exists then the user does not have permissions to
run as the service account.
2.  Ask the SecurityContextConstraintsAllocator to create the security context.  The allocator
will not overwrite fields already set in the container.
3.  Validate that the generated SecurityContext falls within the boundaries of the security context
constraints and accept or reject the pod.

**Pods with no service account and pods whose service account does not match the requested privileges**

1.  Retrieve all security context constraints available for use by the user.
2.  Loop through the constraints to ensure that any requests on the pod fall within an allowed
security context constraint.  This constraint is what will be used in subsequent steps.  If no
acceptable constraint is found then reject the pod.
2.  Ask the SecurityContextConstraintsAllocator to create the security context.  The allocator
will not overwrite fields already set in the container.
3.  Validate that the generated SecurityContext falls within the boundaries of the security context
constraints and accept or reject the pod.


## Creation of a Security Context Based on Security Context Constraints

The creation of a security context based on security context constraints is based upon the configured
settings of the security context constraints and the cluster wide strategies enforce by the
allocator (discussed below).

There are three scenarios under which a security context constraint field may fall:

1.  Governed by a boolean: fields of this type will be defaulted to the most restrictive value.
For instance, `AllowPrivilegedContainer` will always be set to false if unspecified.
1.  Governed by an allowable set: fields of this type will be checked against the set to ensure
their value is allowed.  For example, `AllowedCapabilities` will ensure that only capabilities
that are allowed to be requested are considered valid.  `HostNetworkSources` will ensure that
only pods created from source X are allowed to request access to the host network.
1.  Governed by a strategy: Items that have a strategy to generate a value will provide a
mechanism to generate the value as well as a mechanism to ensure that a specified value falls into
the set of allowable values.  See the Types section for the description of the interfaces that
strategies must implement.

Some items of the security context only need to be allocated a single time.  For instance if the
cluster defines that each namespace should have a block of UIDs used for allocation to service
accounts then the allocator may allocate UID 1000 to service account X for the lifetime of the
service account.  This allocation can be stored as an annotation on the namespace or service account level so
that it is uneditable.  When a security context is generated on behalf of security context
constraints it should first check the annotations for pre-allocated values and use them if appropriate.

Some strategies such as a "allow pods to run as any user" will not need to store annotations.

The allocator will use the configured strategies with interfaces shown below to either validate
the settings fall within the allowed values or create a new value.

```go
// SELinuxSecurityContextConstraintsStrategy defines the interface for all SELinux constraint strategies.
type SELinuxSecurityContextConstraintsStrategy interface {
	// Generate creates the SELinuxOptions based on constraint rules.
	Generate(pod *api.Pod, container *api.Container)  (*api.SELinuxOptions, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(pod *api.Pod, container *api.Container) fielderrors.ValidationErrorList
}

// RunAsUserSecurityContextConstraintsStrategy defines the interface for all uid constraint strategies.
type RunAsUserSecurityContextConstraintsStrategy interface {
	// Generate creates the uid based on policy rules.
	Generate(pod *api.Pod, container *api.Container) (*int64, error)
	// Validate ensures that the specified values fall within the range of the strategy.
	Validate(pod *api.Pod, container *api.Container) fielderrors.ValidationErrorList
}
```

## Escalating Privileges by an Administrator

An administrator may wish to create a resource in a namespace that runs with
escalated privileges.  This is similar to saying `sudo make me a pod`.  By allowing security context
constraints to operate on both the requesting user and pod's service account administrators are able to
create pods in namespaces with elevated privileges based on the administrator's security context
constraints.

This also allows the system to guard commands being executed in the non-conforming container.  For
instance, an `exec` command can first check the security context of the pod against the security
context constraints of the user or the user's ability to reference a service account.
If it does not validate then it can block users from executing the command.  Since the validation
will be user aware administrators would still be able to run the commands that are restricted to normal users.

## Interaction with the Kubelet

In certain cases, the kubelet may need provide information about
the image in order to validate the security context.  An example of this is a cluster
that is configured to run with a UID strategy of `RunAsUserStrategyMustRunAsNonRoot`.

In this case the validation can either require that the pod be submitted with a non-root `RunAsUser`
set in the security context, use a pre-allocation strategy, or assume that images run in the cluster
will be providing the user.  In this case, if a pod reaches the kubelet and does not have a user set
in the security context the kubelet should inspect the image, determine if a user is set in the
image configuration, and ensure that the value complies with the constraints.

