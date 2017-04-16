# Parameters

A parameter is a key-value pair that describes configuration information or information about
another system entity.

Modeling configuration parameters as an API resource has utility that cuts across several different
use-cases.  Both system components in OpenShift/kubernetes and software running in pods can benefit
from a central configuration store:

1.  Cluster operators could store configuration data for system components, enabling components to
    self-bootstrap with just a master URL
2.  Users could model external shared services and consume information about them in pods without
    having to redefine parameters in each container
3.  Users could model supplemental information about services as parameters and consume them in the
    pods that consumed those services
4.  Users could store configuration info for their components as parameters and consume them in the
    appropriate pods

## Use-Case: Configuring system components

Upstream issue [kubernetes/1627](https://github.com/GoogleCloudPlatform/kubernetes/issues/1627)
discusses storing configuration for system components in a resource with a `map[string]string`
structure.  If a component is configured with a namespace/name, it should be able to pull its
configuration from the API server.  A `Parameters` resource would solve this use case; a system
component would only need the URL for the API server and the coordinates for its configuration
in order to discover its configuration.  It could also watch the `Parameters` resource and receive
events when its configuration was changed.

An alternative to a parameters resource that has been discussed for configuring system components
upstream is to store configuration data in etcd directly.  Using `Parameters` to sure
configuration about services has several advantages to storing them directly in etcd or using CLI
arguments exclusively:

1.  Eliminates the need to query etcd directly, resulting in lower complexity; consumers only need
    to know how to query the API server instead of the API server and etcd
2.  Keeps invocation simple by eliminating the need to pass tons of CLI arguments; components only
    need to be configured with the coordinates of the API server and their namespace/name
3.  Allows components to easily watch for configuration changes and adapt to new configurations
    without restart

## Use-Case: Metadata about services

A `Parameters` resource can also hold information about how to use a service.  For example, a
service that provides a mysql database could store the database name and username as parameters.
Pods consuming the mysql service would also consume the mysql parameters instance in order to be
configured with the username and database name.  Declaring pod dependencies on services has been
proposed upstream in [kubernetes/1768](https://github.com/GoogleCloudPlatform/kubernetes/issues/1768)
via `ServiceLink` type.  A similar notion could be introduced for `Parameters` -- containers in a
pod would declare `ParametersLink`s to indicate that a container consumed the referenced
`Parameters` instance.

## Use-Case: External services

A `Parameters` resource can also be used to model information about an external service which is
shared amongst pods in a namespace.  For example, if many pods in a namespace need information
about a SaaS endpoint, a user could create a `Parameters` instance to hold coordinates for the
endpoint and then consume it from the appropriate pods.  For this use case, the appropriate
containers would declare a `ParametersLink` to indicate the relationship with the necessary
`Parameters` instance.

## Use-Case: Configuring software in user pods

A `Parameters` resource can also store ordinary configuration parameters for user software that runs
in pods.  Containers would declare `ParametersLink`s as for the other user pods use-cases.

## Use-Case: Optional Parameters

There are some use-cases where it's convenient to express that a `ParametersLink` should be
optional to the consumer.  The case for optional parameter links is similar to the one described
for optional service links in 
[kubernetes/1768](https://github.com/GoogleCloudPlatform/kubernetes/issues/1768).  Consider a set
of images which are designed to work together via Kubernetes/OpenShift.  For example, `ruby` may
want to know how to consume the parameters for `mysql` if it's present, but doesn't require `mysql`
to operate.  The `ParametersLink` resource should be able to capture the notion of required versus
optional dependencies between pods and parameters.

## Advantages of using Parameters for userspace configuration

All of the use-cases describe for userspace concerns (service parameters, external services, etc)
can be solved by setting environment variables on the appropriate containers manually.  However,
using `Parameters` to model shared parameters has a number of advantages over this approach:

1.  Eliminates the need to redeclare shared parameters on every container that needs them
2.  Parameters only have to be updated in a single place
3.  Allows deployment infrastructure to trigger on parameter changes and redeploy all consumers on
    parameter change

## The `Parameters` resource

We propose the following structure for `Parameters`:

    type Parameters struct {
    	TypeMeta
    	ObjectMeta

    	Params      map[string]string
    	Labels      map[string]string
    	Annotations map[string]string
    }

`Parameters` are presented to the containers in a pod that consume them.  Parameters may be presented
as environment variables to containers, but they differ from environment variables in a number of
ways:

1.  `Parameters` separate presentation from representation
2.  `Parameters` are about storing information in a usage-neutral manner so they can be applied to
     many different types of use-cases
3.  `Parameters` are a distinct resource that can be queried, listed, watched, referred to with an
    `ObjectReference`, etc

## The `ParametersLink` resource

We propose the following structure for `ParametersLink`:

    type ParametersLink struct {
    	Target   ObjectReference
    	Required bool
    }

The `Container` type would be altered to have a list of `ParametersLink` so that a container could
declare a dependency on multiple `Parameters`.

## How are Parameters consumed?

The representation of `Parameters` and how consuming pods are injected with parameters should be
separated.  Upstream issue 
[kubernetes/1768](https://github.com/GoogleCloudPlatform/kubernetes/issues/1768) discusses
different possible ways of presenting parameter information to pods.  For now, we will assume that
presentation / adaptation will be handled in subsequent work; they are not dealt with here.  For
now, `Parameters` will be injected as environment variables into containers that consume them.
Future work will address adapting information `Parameters` and information about services to meet
the needs of arbitrary images which may need to consume information in a specific way (example:
non-standard environment variables, special locations on container filesystem).

## Parameters and OpenShift deployments

If a pod consumes a set of configuration parameters then it is also a valid use case to trigger
a new deployment if a pod's configuration parameters change.  A new trigger type for OpenShift
deployments could be added in order to express that a pod template should be redeployed when a
specific `Parameters` instance was updated.  This trigger would contain:

1.  The coordinates of the `Parameters` instance
2.  Which containers in the pod template consume the parameters

The deployment generator would mutate the PodTemplate in order to include the state of the
referenced parameters so that the deployed pod template would capture the exact state to rollback
to.  For the presentation mode where `Parameters` are consumed as environment variables, this would
mean mutating the appropriate containers' environments so that the exact state at the time of
generation would be captured explicitly in the deployed `PodTemplate`.  For a presentation mode
where parameters were presented on the filesystem, this would be more complex; it would probably
involve generating a unique host volume for each container and referencing that in container's
volume spec.

### Use Case: Application creation via OpenShift console/cli

When a user creates an application, they should be prompted to:

1.  Create a set of parameters for each service
2.  Create a set of parameters for each container in each `DeploymentConfig`

### Use Case: Add service via OpenShift console/cli

When a user adds a service to an application via the OpenShift console, they should receive a
prompt to specify a `Parameters` instance that the consumers of that service will consume.

### Use Case: Add new `DeploymentConfig` to application via console/cli

When a user adds an new `DeploymentConfig` to an application via the OpenShift console, they
should be prompted to:

1.  Specify which existing `Parameters` instances the `PodTemplate` should consume
2.  Create a set of parameters for each container in the `PodTemplate` to consume 

### Use Case: Delete `DeploymentConfig` from application via OpenShift console/cli

When a user deletes a `DeploymentConfig` from an application, they should be prompted to confirm
that the `Parameters` associated with the `DeploymentConfig` should be deleted as well.

## Detailed examples

Let's walk through some detailed examples to illustrate using `Parameters` in various use-cases.

### Example: User creates an application with `ruby` and `mysql` from a template

A simple use case to start with is the case where a user creates an application with a ruby image
and a mysql image.  The template to create this app would contain:

-  A template parameters section with a database name and root password for the mysql database
-  A `Parameters` that would hold the parameters for mysql; written in terms of the template
   parameters
-  A `DeploymentConfig` for the ruby pods; pods have a `ParametersLink` to mysql parameters; the
   config has a trigger to redeploy when the `Parameters` changes
-  A `DeploymentConfig` for the mysql pods; pods have a `ParametersLink` to mysql parameters; the
   config has a trigger to redeploy when the `Parameters` changes

The workflow for creating this app would be:

1.  The template generator generates a config; the config contains the `Parameters` instance with
    the generated values for root password and database name
2.  The user posts the config
3.  The deployment generator generates a new materialized replication controller for each of the
    deployment configs; it resolves the `ParametersLink` on each container against the triggers
    for that deployment config and mutates the PodTemplate to contain the current values of the
    referenced `Parameters` in the environment.

