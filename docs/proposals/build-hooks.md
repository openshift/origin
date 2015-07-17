# Build Hooks

## Abstract

A proposal for supporting the execution of hooks in the build lifecycle.


## Motivation

Users need to execute custom actions at different stages of the build process.
OpenShift provides full flexibility on how builds are performed through the
*Custom* build strategy, and *Source-To-Image* builds also allow customization
via its `assemble` script. However, as we'll see, those extensibility points
have their limitations.

At the time of writing, **Deployments** have a hook mechanism that supports the
execution of arbitrary commands before and after a deployment occurs,
individually supported by each *Deployment Strategy*, namely *Recreate* and
*Rolling*.
The third existing strategy, *Custom*, does not support hooks.
The implementation is done in the strategy level as to allow different
strategies to support a potentially different set of hooks.

Goals of this design:

1. Identify build hook use cases
2. Discuss and describe implementation details


## Use cases

### 1. Build pipeline

As an application developer, I want to be able to [trigger a build after an
upstream build completes](https://github.com/openshift/origin/issues/1228#issue-59933179),
creating a build pipeline.

I want to consume artifacts from an upstream build in a downstream build.
For example, an upstream build builds a Go program in a builder image containing
a Go toolchain, and later a downstream build places the binaries in a lighter
image without a Go compiler.
Similarly, a builder image with a Java SDK can be chained together with a image
containing only a JRE for application deployment.


### 2. Integration with external system

As an application developer, I want to be notified or notify others after a
build completes, email and/or otherwise, including the build UID, build result
(success or failure), and the Docker image ID that was built.

I want to trigger a Jenkins or other CI system to take some action, e.g. run a
test job, run another build, etc.

To integrate with an external system that manages builds, e.g. building RPMs, I
want to notify the external system that a build in OpenShift has started.

I want to be able to perform some data extraction from the built image, to
upload built binaries/assets to a server.


### 3. Abort push

As an application developer, I want to decide whether to push or not a image
to a registry based on some customizable logic (e.g. test results).


## Alternatives

Existing functionality that could be used to address the use cases above:

- ~~Kubernetes' container lifecycle hooks~~ *
- `assemble` script (for S2I buils)
- Changes in Dockerfile (for Docker build)
- Deployment hooks
- Events API and watchers
- Custom build

New functionality:

- Build hooks

\* **Note**: before discussing how each of those alternatives can be used to support
the use cases, it is worth noting that Kubernetes' pre-stop hook is not a
solution for any of the use cases, because it is only [executed prior to a
SIGTERM when Kubernetes is about to terminate a container](https://github.com/GoogleCloudPlatform/kubernetes/blob/596a8a40d12498b5335140f50753980bfaea4f6b/docs/user-guide/production-pods.md#lifecycle-hooks-and-termination-notice).
The builder container terminates itself, thus any pre-stop hook would
be ignored.


## Discussion on alternatives

For each use case, we compare below how each alternative could be used depending
on build strategy.


### 1. Build pipeline

For *S2I* builds, one might think that providing a custom `assemble`
script would allow for custom post build actions. However, that's not the case,
because at the time the script runs, the final state (success or failure) of the
build is unknown.

If the application defines a DeploymentConfig with an ImageChange trigger, one
can use the post-deployment hook to trigger a build. This is particularly
inelegant, fragile and hard to debug when things go wrong:

```json
{
  "kind": "DeploymentConfig",
  "apiVersion": "v1",
  "metadata": {
    "name": "chain-1-frontend"
  },
  "spec": {
    "strategy": {
      "type": "Rolling",
      "rollingParams": {
        "pre": {
          "failurePolicy": "Abort",
          "execNewPod": {
            "containerName": "chained-builds",
            "command": [
              "sh", "-c", "WEBHOOK_URL=\"https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT/osapi/v1beta3/namespaces/$TARGET_BUILDCONFIG_NAMESPACE/buildconfigs/$TARGET_BUILDCONFIG_NAME/webhooks/$GENERIC_WEBHOOK_SECRET/generic\"; test `curl -skXPOST -w %{http_code} $WEBHOOK_URL` = 200 || >&2 echo -e \"FAILURE: call to generic webhook trigger for BuildConfig $TARGET_BUILDCONFIG_NAMESPACE/$TARGET_BUILDCONFIG_NAME failed. Webhook URL: $WEBHOOK_URL\""
            ],
            "env": [
              {
                "name": "TARGET_BUILDCONFIG_NAMESPACE",
                "value": "demo"
              },
              {
                "name": "TARGET_BUILDCONFIG_NAME",
                "value": "chained-builds-app42"
              },
              {
                "name": "GENERIC_WEBHOOK_SECRET",
                "value": "${GENERIC_WEBHOOK_SECRET}"
              }
            ]
          }
        }
      }
    },
    "..."
  }
}
```

Obviously, the other downside is that it involves having a deploy just for the
sake of triggering another build.

For *Docker* builds, changing the Dockerfile is similar to changing the
`assemble` script in *S2I* builds, and has the same problem that the final build
state is unknown at the time instructions in the Dockerfile are executed.

*Custom* builds could trigger downstream builds through their generic webhooks
or OpenShift REST API. Unless you are already using *Custom* builds for a
different reason, it seems silly to have to stop using *S2I* or *Docker* builds
just to be able to execute an action after a successful build.
You should not need to fork the code for the *S2I* or *Docker* builders just to
add that feature.

For all build strategies, the events API could be used to watch for updates of
a build resource and trigger downstream builds through generic webhooks or
OpenShift REST API. You need to write a program that consumes events from the
endpoint `/oapi/v1/namespaces/{namespace}/builds?watch=true`.
This program could even be deployed on OpenShift and access OpenShift at
https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT.
By default, authenticated project admins and editors have enough permissions to
be able to use the API calls required.

If implemented, **Build hooks** would notably be an unfriendly solution for this
use case, similar to what deployment hooks offer, but without involving a
`DeploymentConfig`.

Finally, it's worth mentioning that all solutions above rely on invoking a
generic webhook or REST API endpoint, and that would be a bad move if in the
future we want to track what caused a build to happen, as auditing capability
would be weakened.


### 2. Integration with external system

Just like for the **Build pipeline** use case, `assemble` scripts and changes to
Dockerfiles are not viable solutions, because the build outcome is still
unknown.

Deployment hooks are not a solution, not even a hacky solution, as it cannot be
used to notify a failed build.

The events API could be used requiring a similar custom program like for the
**Build pipeline** use case.

*Custom* build can send notifications whatever way it wants, but again we should
not force users to move way from *S2I* or *Docker* for this sole reason.

If implemented, **Build hooks** would be a flexible way to send out
notifications without having to write your own program that consumes the
OpenShift REST API.


### 3. Abort push

Obviously, deployment hooks are not a solution.

For *S2I* builds, tests can be run as part of the `assemble` script. If the
`assemble` script fails (exit code != 0), the build fails and the image is not
pushed to the registry.

For *Docker* builds, tests can be as a `RUN` entry in the `Dockerfile`. If the
test command fails, the Docker build fails and no image is pushed.

For *Custom* builds, there's full flexibility whether to push or not.

In this use case, the events API cannot be used to prevent pushing a new image
to the registry.

If implemented, a pre-push **Build hook** could be used to execute tests before
pushing a new image to the registry. If tests fail, the image is not pushed.


## Hooks

### Hook types

To cover the use cases above, we might have these hooks:

1. Pre-build

2. Pre-push

3. Post-push

4. Post-build


### Hook Input and Output

The Docker container executing a hook action can get build metadata through
environment variables and/or mounted volumes.

Input to be provided through environment variables:

- Build UID
- Build status/phase
- Docker image ID produced by the build (if build suceeded)

The hooks return no specific output, however their *exit status* (zero or
non-zero) can be used to e.g. prevent a push from happening if a pre-push hook
fails. That said, hooks are expected to produce side effects most likely via the
network.

The build object is tagged with a label describing the status of the hook
execution, and if a Docker image is built as a result of the build, it is as
well labeled with hook execution status.
