# Deployment Hooks

## Abstract

A proposal for a deployment code execution API integrated with the deployment lifecycle.


## Motivation

Deployment hooks are needed to provide users with a way to execute arbitrary commands necessary to complete a deployment.

Goals of this design:

1. Identify deployment hook use cases
2. Define the integration of deployment hooks and the deployment lifecycle
3. Describe a deployment hook API


## Comparison of potential approaches

There are two fundamental approaches to solving each deployment hook use case: existing upstream support for *container lifecycle hooks*, and the externalized *deployment hooks* outlined in this proposal. The following describes the two approaches, and each use case is evaluated in terms of these approaches.

##### Upstream container lifecycle hooks

Kubernetes provides *container lifecycle hooks* for containers within a Pod. Currently, post-start and pre-stop hooks are supported. For deployments, post-start is the most relevant. Because these hooks are implemented by the Kubelet, the post-start hook provides some unique guarantees:

1. The hook is executed synchronously during the pod lifecycle.
2. The status of the pod is linked to the status and outcome of the hook execution.
  1. The pod will not enter a ready status until the hook has completed successfully.
    1. Service endpoints will not be created for the pod until the pod has a ready status.
  1. If the hook fails, the pod's creation is considered a failure, and the retry behavior is restart-policy driven in the usual way.

Because deployments are represented as replication controllers, lifecycle hooks defined for containers are executed for every container in the replica set for the deployment. This behavior has complexity implications when applied to deployment use cases:

1. The hooks for all pods in the deployment will race, placing a burden on hook authors (e.g., the hooks would generally need to be tolerant of concurrent hook execution and implement manual coordination.)


##### Deployment hooks

An alternative to the upstream-provided lifecycle hooks is to have a notion of a hook which is a property of an OpenShift deployment. OpenShift deployment hooks can provide a different set of guarantees:

1. Hooks can be bound to the logical deployment lifecycle, enabling hook executions decoupled from replication mechanics.
  1. Races can be avoided by defining a hook that executes at least once per deployment regardless of the replica count.
2. Hooks defined in terms of deployments are conceptually easier to reason about from the perspective of a user defining a deployment workflow.

Hooks can be defined to execute before or after the deployment strategy scales up the deployment. When implementing a hook which runs after a deployment has been scaled up, there are special considerations to make:

1. Nothing prevents external connectivity races: when the deployment's pods are ready, they become routable to services and other pods within an application the moment their containers enter a ready state, likely before or during or before hook execution.
2. Hook execution can't be atomically linked to the deployment pods' statuses: If a hook failure should result in deployment failure, a previously scaled and exposed application must be rolled back when ideally the application shouldn't have been exposed prior to hook success.


## Use cases

1. As a Rails application developer, I want to perform a Rails database migration following an application deployment.
2. As an application developer, I want to invoke a cloud API endpoint to notify it of the presence of new code.


#### Use-case: Rails migrations

New revisions of a Rails application often contain schema or other database updates which must accompany the new code deployment. Users should be able to specify a hook which performs a Rails migration as part of the application code deployment.

Database migrations are complex and introduce downtime concerns. Here are [some examples](https://blog.rainforestqa.com/2014-06-27-zero-downtime-database-migrations) of zero-downtime Rails migration workflows.

Deployments including database migrations must make special considerations:

1.  Code must be newer than the schema in use, or all old code must be stopped before the new schema is introduced.
2.  Database or table locking must be minimized or eliminated to prevent service outages.

The workflows which are effective at ensuring zero downtime migrations are typically multi-phased. For a user orchestrating a zero downtime migration deployment, it's likely the user needs to verify each deployment step discretely, with the option to abort and rollback after each phase.

Consider this simple example of a phased deployment which adds a new column:

1. Deploy a new migration which adds the new column.
  1. The user can verify that the new column didn't break the application.
2. Deploy a new version of the code which makes use of the new column.
  1. The user can verify the new code interacts correctly with the new column.

###### Container lifecycle hooks

Container lifecycle hooks introduce problems with Rails migrations:

1. There is no way to guarantee that pods with older code are not running.
2. The migration hook will execute in the same pod as application containers, consuming resources allocated for the application.
  1. This can cause instability, as it's unlikely the application pod resource allocation takes into account the temporarily increased requirements of a transient deployment step.

###### Deployment hooks

Deployment hooks satisfy this use case by providing a means to execute the hook only once per logical deployment. The hook is expressed as a run-once pod which provides the migration with its own resource allocation decoupled from the application.


#### Use-case: Invoke a cloud API endpoint

Consider an application whose deployment should result in a cloud API call being invoked to notify it of the newly deployed code.

###### Container lifecycle hooks

Container lifecycle hooks aren't ideal for this use case because they will be fired once per pod in the deployment during scale-up rather than following the logical deployment as a whole. Consider an example deployment flow using container lifecycle hooks:

1. Deployment is created.
2. Deployment is scaled up to 10 by the deployment strategy.
3. The cloud API is invoked 10 times.
4. Deployment is considered complete concurrently with the cloud API calls.
5. Deployment is scaled up to 15 to handle increased application traffic.
6. The cloud API is invoked 5 times, outside the deployment workflow.

###### Deployment hooks

A post-deployment hook would satisfy the use case by ensuring the API call is invoked after the deployment has been rolled out. For example, the flow of this deployment would be:

1. Deployment is created.
2. Deployment is scaled up to 10 by the deployment strategy.
3. Deployment hook fires, invoking the cloud API.
4. Deployment is considered complete.
5. Deployment is scaled up to 15 to handle increased application traffic.
6. No further calls to cloud API are made until next deployment.


## Proposed design

Deployment hooks are implemented as run-once pods which can be executed at one or both of the following points during the deployment lifecycle:

1. Before the execution of the deployment strategy, which ensures that the hook is run and its outcome evaluated prior to the scale-up of the deployment. These hooks are referred to as *pre-deployment* hooks.
2. After the execution of the deployment strategy. These hooks are referred to as *post-deployment* hooks.

##### Hook failure handling

Hooks designated as *mandatory* should impact the outcome of the deployment.

There are a few possible ways to handle a failed mandatory pre-deployment hook:

1. Transition the deployment to a failed status and do not execute the strategy.
2. Delete and retry the deployment.
  1. Potentially safe because the strategy has not yet executed and the existing prior deployment is still active.
    1. Could be unsafe depending on what the hook did prior to failing.
  2. Further API considerations need made to prevent endless deployment attempts.
3. Rollback the deployment to a previous version.
  1. This is very dangerous to do automatically and is probably not realistic at this time:
    1. Requires automated rollback viability analysis/trust.
    2. Requires logic to prevent a chronically failing hook that spans historical deployments from causing unending rollbacks.

This proposal prescribes the use of option 1 as being the simplest starting point for the hook API.

Failed mandatory post-deployment hooks are more challenging:

1. The deployment has most likely already been rolled out and made live by the strategy.
2. Deleting the deployment is no longer safe due to 1.
3. Rollback is necessary and subject to the same challenges presented above.

Due to the complexities of automated rollback, this proposal limits the scope of failure handling for post-deployment hooks: post-deployment hooks cannot be considered mandatory at this time. This limitation may be lifted in the future by an separate proposal.

##### Hook failure reporting

When a deployment hook fails:

1. An error is logged via the global error handler.
2. The hook status is available as an annotation on the deployment.

More reporting capabilities could be addressed in a future proposal.


### Deployment hooks API

The `DeploymentStrategy` gains a new `Lifecycle` field:

```go
type DeploymentStrategy struct {
  // Type is the name of a deployment strategy.
  Type DeploymentStrategyType `json:"type,omitempty"`
  // CustomParams are the input to the Custom deployment strategy.
  CustomParams *CustomDeploymentStrategyParams `json:"customParams,omitempty"`
  // Lifecycle provides optional hooks into the deployment process.
  Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
}
```

```go
// Lifecycle describes actions the system should take in response to
// deployment lifecycle events. The deployment process blocks while
// executing lifecycle handlers. A HandleFailurePolicy determines what
// action is taken in response to a failed handler.
type Lifecycle struct {
  // Pre is called immediately before the deployment strategy executes.
  Pre *Handler `json:"pre,omitempty"`
  // Post is called immediately after the deployment strategy executes.
  // NOTE: AbortHandlerFailurePolicy is not supported for Post.
  Post *Handler `json:"post,omitempty"`
}
```

Each lifecycle hook is implemented with a `Handler`:

```go
// Handler defines a specific deployment lifecycle action.
type Handler struct {
  // ExecNewPod specifies the action to take.
  ExecNewPod *ExecNewPodAction `json:"execNewPod,omitempty"`
  // FailurePolicy specifies what action to take if the handler fails.
  FailurePolicy HandlerFailurePolicy `json:"failurePolicy"`
}
```

The first handler implementation is pod-based:

```go
// ExecNewPodAction runs a command in a new pod based on the specified
// container which is assumed to be part of the deployment template.
type ExecNewPodAction struct {
  // Command is the action command and its arguments.
  Command []string `json:"command"`
  // Env is a set of environment variables to supply to the action's container.
  Env []EnvVar `json:"env,omitempty"`
  // ContainerName is the name of a container in the deployment pod
  // template whose Docker image will be used for the action's container.
  ContainerName string `json:"containerName"`
}
```

Handler failure management is policy driven:

```go
// HandlerFailurePolicy describes the action to take if a handler fails.
type HandlerFailurePolicy string

const(
  // RetryHandlerFailurePolicy means retry the handler until it succeeds.
  RetryHandlerFailurePolicy HandlerFailurePolicy = "Retry"
  // AbortHandlerFailurePolicy means abort the deployment (if possible).
  AbortHandlerFailurePolicy HandlerFailurePolicy = "Abort"
  // ContinueHandlerFailurePolicy means continue the deployment.
  ContinueHandlerFailurePolicy HandlerFailurePolicy = "Continue"
)
```

`ExecNewPodAction` pods will be associated with deployments using new annotations:

```go
const (
  // PreExecNewPodActionPodAnnotation is the name of a pre-deployment
  // ExecNewPodAction pod.
  PreExecNewPodActionPodAnnotation = "openshift.io/deployment.lifecycle.pre.execnewpod.pod"
  // PreExecNewPodActionPodPhaseAnnotation is the phase of a pre-deployment
  // ExecNewPodAction pod and is used to track its status and outcome.
  PreExecNewPodActionPodPhaseAnnotation = "openshift.io/deployment.lifecycle.pre.execnewpod.phase"
  // PostExecNewPodActionPodAnnotation is the name of a post-deployment
  // ExecNewPodAction pod.
  PostExecNewPodActionPodAnnotation = "openshift.io/deployment.lifecycle.post.execnewpod.pod"
  // PostDeploymentHookPodPhaseAnnotation is the phase of a post-deployment
  // ExecNewPodAction pod and is used to track its status and outcome.
  PostExecNewPodActionPodPhaseAnnotation = "openshift.io/deployment.lifecycle.post.execnewpod.phase"
)
```

#### Validations related to hooks

Initially, valid values for `Lifecycle.Post.FailurePolicy` will be `Retry` and `Continue`. This may change in the future if deployments can be safely rolled back automatically.

TODO: `ExecNewPodAction.ContainerName`

1. Could reject container names that aren't defined in the deploymentConfig.


#### Hook and deployment status relationship

The status of a deployment hook is distinct from the status of the deployment iteself. The deployment status may be updated in response to a change in hook status.

1. The `Pre` hook executes while the deployment has a `New` status, and the hook will have a terminal status prior to the deployment transitioning past `New`.
2. The `Post` hook executes while the deployment has a `Running` status, and the hook will have a terminal status prior to the deployment transitioning past `Running`.


### Example: Rails migration

Here's an example deployment which demonstrates how to apply deployment hooks to a Rails application which uses migrations.

The application image `example/rails` is built with a `Dockerfile` based on the `rails` image from Docker Hub:

```dockerfile
FROM rails:onbuild
```

A database is exposed to the application using a service:

```json
{
  "kind": "Service",
  "apiVersion": "v1",
  "metadata": {
    "name": "mysql"
  },
  "spec": {
    "ports": [
      {
        "protocol": "TCP",
        "port": 5434,
        "targetPort": 3306,
        "nodePort": 0
      }
    ],
    "selector": {
      "name": "mysql"
    },
    "clusterIP": "",
    "type": "ClusterIP",
    "sessionAffinity": "None"
  }
}
```

A deployment configuration describes the template for application deployments:

```json
{
  "kind": "DeploymentConfig",
  "apiVersion": "v1",
  "metadata": {
    "name": "rails"
  },
  "spec": {
    "strategy": {
      "type": "Recreate",
      "resources": {}
    },
    "triggers": [
      {
        "type": "ConfigChange"
      }
    ],
    "replicas": 1,
    "selector": {
      "name": "rails"
    },
    "template": {
      "metadata": {
        "labels": {
          "name": "rails"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "rails",
            "image": "example/rails",
            "ports": [
              {
                "containerPort": 8080,
                "protocol": "TCP"
              }
            ],
            "resources": {},
            "terminationMessagePath": "/dev/termination-log",
            "imagePullPolicy": "IfNotPresent",
            "capabilities": {},
            "securityContext": {
              "capabilities": {},
              "privileged": false
            }
          }
        ],
        "restartPolicy": "Always",
        "dnsPolicy": "ClusterFirst"
      }
    }
  }
}
```

Let's consider a hypothetical timeline of events for this deployment, assuming that the initial version of the application is already deployed as `rails-1`.

1. A new version of the `example/rails` image triggers a deployment of the `rails` deployment configuration.
2. A new deployment `rails-2` is created with 0 replicas; the deployment is not yet live.
3. The `pre` hook command `rake db:migrate` is executed in a container using the `example/rails` image as specified in the `rails` container.
  1. The `rake` command connects to the database using environment variables provided for the `mysql` service.
4. When `rake db:migrate` finishes successfully, the `Recreate` strategy executes, causing the `rails-2` deployment to become live and `rails-1` to be disabled.
  1. Because `failurePolicy` is set to `Retry`, if the `rake` command fails, it will be retried and the deployment will not proceed until the command succeeds.
5. Since there is no `post` hook, the deployment is now complete.
