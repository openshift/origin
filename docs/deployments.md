# Deployments in OpenShift

## Overview

In OpenShift, deployment is an update to a single replication controller's pod template based on triggered events. The deployment subsystem provides:

*  [Declarative definition](#defining-a-deploymentconfig) of a desired deployment configuration which drives automated deployments by the system
*  [Triggers](#triggers) which drive new deployments in response to events
*  [Rollbacks](#rollbacks) to a previous deployment
*  [Strategies](#strategies) for deployment rollout behavior which are user-customizable
*  Audit history of deployed pod template configurations

#### Concepts

An OpenShift `deploymentConfig` describes a single `template` and a set of `triggers` for when a new `deployment` should be created. A `deployment` is simply a specially annotated `replicationController`. A `strategy` is responsible for making a `deployment` live in the cluster.

Each time a new deployment is created, the `latestVersion` field of `deploymentConfig` is incremented, and a `deploymentCause` is added to the `deploymentConfig` describing the change that led to the latest deployment.

## Defining a deploymentConfig

A `deploymentConfig` in OpenShift is a REST object which can be POSTed to the API server to create a new instance. Consider a simple configuration which should result in a new `deployment` every time a Docker image tag changes.

```
{
  "kind": "DeploymentConfig",
  "apiVersion": "v1",
  "metadata": {
    "name": "frontend"
  },
  "spec": {
    "strategy": {
      "type": "Recreate",
      "resources": {}
    },
    "triggers": [
      {
        "type": "ImageChange",
        "imageChangeParams": {
          "automatic": true,
          "containerNames": [
            "helloworld"
          ],
          "from": {},
          "lastTriggeredImage": ""
        }
      }
    ],
    "replicas": 1,
    "selector": {
      "name": "frontend"
    },
    "template": {
      "metadata": {
        "creationTimestamp": null,
        "labels": {
          "name": "frontend"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "helloworld",
            "image": "openshift/openshift/origin-ruby-sample",
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

This specification will create a new `deploymentConfig` named `frontend`. A single ImageChange `trigger` is defined, which causes a new `deployment` to be created each time the `openshift/origin-ruby-sample:latest` tag value changes. The Recreate `strategy` makes the `deployment` live by removing any prior `deployment` and increasing the replica count of the new `deployment`.

## Triggers

A `deploymentConfig` contains `triggers` which drive the creation of new deployments in response to events (both inside and outside OpenShift). The following trigger types are supported:

##### Image change triggers

The ImageChange `trigger` will result in a new deployment whenever the value of a Docker `imageRepository` tag value changes. Consider an example trigger.

```
{
  "type": "ImageChange",
  "imageChangeParams": {
    "automatic": true,
    "containerNames": [
      "helloworld"
    ],
    "repositoryName": "openshift/origin-ruby-sample",
    "tag": "latest"
  }
}
```

In this example, when the `latest` tag value for the `imageRepository` named `openshift/origin-ruby-sample` changes, the containers specified in `containerNames` for the `deploymentConfig` will be updated  with the new tag value, and a new `deployment` will be created.

If the `automatic` option is set to `false`, the trigger is effectively disabled.

##### Config change triggers

The ConfigChange `trigger` will result in a new deployment whenever changes are detected to the `template` of the `deploymentConfig`. Suppose the REST API is used to modify an environment variable in a container within the `template`.

```
{
  "type": "ConfigChange"
}
```

This `trigger` will cause a new `deployment` to be created in response to the `template` modification.

## Strategies

A `deploymentConfig` has a `strategy` which is responsible for making new deployments live in the cluster. Each application has different requirements for availability (and other considerations) during deployments. OpenShift provides out-of-the-box strategies to support a variety of deployment scenarios:

##### Recreate strategy

The Recreate `strategy` has very basic behavior.

```
{
  "type": "Recreate"
}
```

The algorithm for this `strategy` is:

1.  Find and destroy any existing `deployment` (by reducing its `replicationController` replica count to 0, and finally deleting it)
2.  Ensure that the old `replicationController` and `pods` are actually destroyed
3.  Set the replica count of the new `replicationController` to 1
4.  Ensure that pods defined by the new `replicationController` are created

##### Custom strategy

The Custom `strategy` allows users of OpenShift to provide their own deployment behavior.

```
{
  "type": "Custom",
  "customParams": {
    "image": "organization/strategy",
    "command": ["command", "arg1"],
    "environment": [
      {
        "name": "ENV_1",
        "value": "VALUE_1"
      }
    ]
  }
}
```

With this specification, the `organization/strategy` Docker image will carry out the `strategy` behavior. The optional `command` array overrides any `CMD` directive specified in the image's Dockerfile. The optional `environment` variables provided will be added to the execution environment of the `strategy` process.

Additionally, the following environment variables are provided by OpenShift to the `strategy` process:

* `OPENSHIFT_DEPLOYMENT_NAME` - the name of the `replicationController` representing the new `deployment`
* `OPENSHIFT_DEPLOYMENT_NAMESPACE` - the namespace of the `replicationController` representing the new `deployment`

The replica count of the `replicationController` for the new deployment will be 0 initially. The responsibility of the `strategy` is to make the new `deployment` live using whatever logic best serves the needs of the user.

## Rollbacks

Rolling a deployment back to a previous state is a two step process accomplished by:

1. POSTing a `rollback` API object to a special endpoint, which generates and returns a new `deploymentConfig` representing the rollback state
2. POSTing the new `deploymentConfig` to the API server

The `rollback` API object configures the generation process and provides the scope of the rollback. For example, given a previous deployment `deployment-1` and the current deployment `deployment-2`:

```
{
  "kind": "DeploymentConfigRollback",
  "apiVersion": "v1",
  "spec": {
    "from": {
      "name": "deployment-1"
    },
    "includeTriggers": false,
    "includeTemplate": true,
    "includeReplicationMeta": false,
    "includeStrategy": true
  }
}
```

With this rollback specification, a new `deploymentConfig` named `deployment-3` will be generated, containing the details of `deployment-2` with the specified portions of `deployment-1` overlayed. The generation options are:

* `includeTemplate` - whether to roll back `podTemplate` of the `deploymentConfig`
* `includeTriggers` - whether to roll back `triggers` of the `deploymentConfig`
* `includeReplicationMeta` - whether to roll back `replicas` and `selector` of the `deploymentConfig`
* `includeStrategy` - whether to roll back the `strategy` of the `deploymentConfig`

Note that `namespace` is specified on the `rollback` itself, and will be used as the namespace from which to obtain the `deployment` specified in `from`.
