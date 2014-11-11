# Openshift Deployments - Pod Templates 

## Problem/Rationale

Although Kubernetes can create primitives such as replication controllers and pods, it currently
lacks functionality for transitioning a set of those primitives between logical versions.  We
call this use case ‘deployment’, and suggest that offering the ability to deploy a new version of
a single pod-template can be seen as a higher level component above a replication controller that
can abstract consumers from dealing with replication controllers directly.

Moving from one logical configuration of a pod template version to another is a fundamental concept
which follows from being able to create a single replication controller or pod.  A simple pod
template iteration mechanism should provide: 

*  Ability to declaratively define a pod template deployment configuration and see the system 
   eventually deploy the desired configuration for that template
*  Ability to rollback to a previous pod template
*  Audit history of deployed pod template configurations
*  Ability to specify tht certain events should trigger a new deployment:
    *  When a new version of a referenced image becomes available
    *  When an update to the pod template is made
*  Ability to select from multiple deployment strategies, such as:
    *  Canary or A/B deployment
    *  Rolling deployment
    *  User-defined strategy; allowing ad-hoc strategies or decoration of existing strategies
*  Ability to pause or cancel in-progress deployments
*  Ability to manage multiple replication controllers with the same mechanism

As a simplifying assumption, it is assumed that the majority of transitions occur between pod
templates with services and other abstractions isolating sets of pods from the details of that
transition.  Therefore, this proposal focuses on migrating a single pod template and its
associated replication controllers.  An alternative approach may be to manage changes as a delta
between two states of resource configuration across many resources and a transition - we do not
view this proposal and that alternative as mutually exclusive.

## Use Cases

1.  As a user, define a new pod template manually and directly define a “deployment” that will
    result in the provisioned resources being transitioned to a new state
2.  Easily rollback to a previous deployed state - including exact versions of images referenced in
    that state.
3.  As a user, be able to easily see the historical record of a set of changes to a cluster
4.  Allow a change to an input (viz: image change, pod template change) to trigger a redeployment of
    a pod template
5.  Allow custom deployment code to be run in the cluster as transient jobs, enabling user code to
    be run securely and define the entire scope of a process
6.  Provide a simple building block for higher level transformations and/or serve as a limited scope
    transition resource for a single pod template
7.  Allow a user to define new replication controllers under a deployment label and allow the 
    deployment process to also manage changing those controllers (for regional deployments)

## Deployment Triggers

A user will always be able to manually trigger that a configuration should be deployed.
Additionally, users should be able to create configurations for pod templates that are triggered by
certain events such as a change to the pod template or the availability of a new version of a
referenced image.  

Likewise, when a deployment is triggered, either by a manual user action or in response to an event,
information about the cause(s) should be captured and related to the deployed version that is created.

## Generation of Pod Templates

We call the process of transforming deployment configuration inputs into a deployable pod template
*generation*.  The deployment workflow begins with requesting an updated pod template from a
generator.  Thus the generator not only controls the process of pod template creation from
inputs but also whether a new deployment should be generated at all.  For example, the manual
deployment process would be:

1.  An actor calls the generator for a deployment configuration
2.  The generator creates a new pod template from the configured inputs
3.  The generator determines whether the new pod template matches the currently deployed state and
    annotates the pod template as representing a new version to deploy
4.  The generator returns the new pod template to the requester
5.  The requester updates the pod template
6.  The system receives an event about the updated pod template and checks to see whether it
    represents a new version
7.  If the updated pod template represents a new version, the system deploys the new state.

## Performing Deployments - Possible Approaches

1.  **External Script:** Handle deployment via end-user scripts that tell Kubernetes what to do via
    core APIs - this proposal does not limit, prevent, or obsolete that mechanism
2.  **Generic transition between two sets of API resources:** Define a global model for transitioning
    between two definitions of a set of resources and apply that model to pod templates
3.  **Deployment API Resource; platform executes deployment:** Add deployments as a top-level API
    resource in Kubernetes that can be used to accomplish the requirements above. Deployment is
    handled by code that executes on the Kubernetes master.
4.  **Deployment API Resource; deployment executed by run-once container in cluster:** Similar to 
    above, but deployment happens in a pod and thus can take advantage of scheduling / resource
    constraint in kubernetes, and custom deployment strategies are possible.

## Performing Deployments - Evaluation of Options

### External Script:

**Pros:**

*  Completely flexible, can be done today

**Cons:**

*  No consistency between implementations around history, rollbacks, etc
*  No portability of approaches guaranteed across implementations
*  Nothing to offer from Kubernetes OTS

### Generic transition across two sets of API resources

**Pros:**

*  Flexibility, ability to deal with higher level transition concepts

**Cons:**

*  Requires definition of transition strategies in a generic way
*  Requires more complex ordering logic
*  Larger scope requires larger design and implementation effort

### Deployment API Resource - deployment executed by platform:

**Pros:**

*  Declarative description of a single pod template
*  Can be used by a higher-level transformation like #2

**Cons:**

*  Doesn’t take advantage of scheduling / resource allocation in OpenShift/Kubernetes

### Deployment API Resource - deployment executed in container:

**Pros:**

*  Well defined API for deployment images
*  Platform-offered and user-defined deployment strategies are equals
*  Can utilize cluster resources to shedule and perform deployment work
*  Can run untrusted user code for managing changes in the cluster
*  Can be used by a higher-level transformation like #2

**Cons:**

*  Container requires authentication and coordinates to use the API with the master

## Proposed Design

### API

#### Deployment

Add a new `Deployment` which corresponds to a specific PodTemplate state and accompanying image
states to deploy.  A `Deployment` is a snapshot of a `DeploymentConfig` at a particular time.  Its
field include:

*  Strategy: a `DeploymentStrategy` which defines how the deployment should be carried out
*  ControllerTemplate: a `ReplicationControllerState` capturing the pod template to deploy
*  Status: a `DeploymentStatus` corresponding to the current status of the deployment
*  Details: a `DeploymentDetails` capturing information about the causes of the deployment

#### DeploymentConfig

Add a new `DeploymentConfig` type that will be used to record the state of a deployment
configuration.  Its fields include:

*  Labels: a map of labels associated with the pod doing the deployment
*  Template: a `DeploymentTemplate` describing what to deploy and how to deploy it.
*  Trigger Policies: Determine when a deployment should be triggered; values include:
   *  Configuration change: trigger a deployment when the deployment config changes
   *  Image change: trigger deployment when a deployed image changes
*  Details: a `DeploymentDetails` capturing information about the cause of the latest update to the 
   `DeploymentConfig`
*  LatestVersion: an integer that tracks the latest version of this config.  This field is
   incremented by a configuration generator when a new version should be deployed.

Add appropriate registries and storage for `DeploymentConfig` and register `/deploymentConfigs`
with the apiserver.

#### DeploymentStrategy

Add a new `DeploymentStrategy` type encapsulates how a deployment should be carried out.  This type
will also container parameters for the selected strategy.

#### DeploymentTemplate

The `DeploymentTemplate` type encapsulates the information necessary to create a `Deployment` from
a `DeploymentConfig`: the what - the replication controller state to deploy, and the how - the 
deployment strategy to use to deploy it.  Its fields are:

*  Strategy: the `DeploymentStrategy` to use when 
*  ControllerTemplate: the `ReplicationControllerTemplate` to deploy

#### DeploymentTriggerPolicy

The `DeploymentTriggerPolicy` type encapsulates information about an event that triggers the
redeployment of a `DeploymentConfig`.  A `DeploymentTriggerPolicy` has two fields:

*  Type: the type of trigger the policy describes.  In our current design there re two trigger
   types:
   *  Image change: a deployment should be triggered when an image referenced by the template
      changes
   *  Config change: a deployment should be triggered when the structure of the pod template is
      updated.
*  ImageChangeParams: describes the inputs to a single image change trigger.

#### DeploymentImageChangeParams

The `DeploymentImageChangeParams` type encapsulates parameters to a single image referenced by the
pod template of a `DeploymentConfig`.  Its fields include:

*  Automatic: Whether a change satisfying this trigger should result in a new deployment
*  RepositoryName: The name of the docker repository
*  Tag: the specific tag for the docker repository that is referenced
*  ContainerNames: an array of the names of containers in the pod template that reference the image
   described by this trigger

## Proposed Design - Triggers

#### ImageChangeTriggerController

The `ImageChangeTriggerController` is responsible for detecting changes to images that are
referenced by a `DeploymentConfig`.  This controller watches the `DeploymentConfig` and
`ImageRepository` OpenShift resources and regenerates `DeploymentConfig`s when an automatic
image change trigger is fired.

#### ConfigChangeController

The `ConfigChangeController` is responsible for detecting changes to pod template of a
`DeploymentConfig`.  This controller watches the `DeploymentConfig` resource and regenerates
configs when an update that does not change the `LatestVersion` field occurs.

## Proposed Design - Generator

A generator is modeled as a REST endpoint that receives as a parameter the ID of the
`DeploymentConfig` to generate a new state for.  Currently, there is a single generator and
corresponding REST endpoint only.  In the future, the generator to use for a `DeploymentConfig`
will be modeled as a field of that type.  The workflow around the generator is as follows:

1.  An actor makes an HTTP GET call to `/generateDeploymentConfigs/<id>`
2.  The generator returns a `DeploymentConfig` with the latest versions of the images and tags
    interpolated into the pod template and the `LatestVersion` field incremented if a new
    deployment should be created
3.  The actor makes an HTTP PUT call to `/deploymentConfigs/<id>` and updates the state of the
    `DeploymentConfig` with the result of the generator call
4.  A `DeploymentConfigController` watches for updates to the `DeploymentConfig` resource -- if
    an event is received for a `DeploymentConfig` which does not have a `Deployment` corresponding
    to its `LatestVersion` field, this controller creates a new `Deployment`.    

## Proposed Design - Pod Based Deployments

#### CustomPodDeploymentController

Responsible for realizing a deployment by launching a single-container pod based on the deployment
image which implements the specific deployment behavior for a service.

### Deployer Image:

The deployer image is responsible for communicating with the Kubernetes master in order to
transform the current deployed state into the desired state.

Timeline of a Deployment:

1.  A trigger criteria is met (manual REST endpoint requested, new image available, etc) and drives
    the generation of a new Deployment from a DeploymentConfig
2.  There is a process which is watching the relevant images, configs, and receiving notifications
    from a REST endpoint. This process creates a new Deployment containing a copy of the 
    `DeploymentConfig` on which it is based, with status New, and image repositories resolved to a
    unique version of each image
3.  The DeploymentController recognizes the creation of a new Deployment and creates a pod to carry
    out the deployment using the configured deployment image, and updates the Deployment status to 
    Pending
4.  The DeploymentController queries the status of the pod for the deployment and updates the 
    status of the Deployment to Running once the pod is running
5.  When the pod exits, the Deployment controller updates the status of the pod to `Complete` or 
    `Failed` based on the pod’s exit code:
    1.  Exit code 0 maps to Complete
    2.  Exit code != 0 maps to Failed
    3.  Deployer Image Specification

In addition, deployer images are provided a mechanism for authorizing calls to the master to perform
manual orchestration.

#### Example: Destroy and Recreate Strategy

This example of a deployer image implements a simple strategy that destroys the replication
controllers and pods associated with the current deployment and creates the ones associated with
the new desired state.  The image must:

1.  GET the deployment record for the new deployment
2.  GET the deployment record for the current deployment
3.  Determine and make the calls necessary to destroy the current replication controller and its 
    pods
4.  Ensure that the old replication controller and pods are actually destroyed
5.  Determine and make the calls necessary to create the replication controller and pods associated
    with the new desired state
6.  Ensure that the new replication controller and pod are actually created
