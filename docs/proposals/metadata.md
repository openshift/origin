# Image Metadata

## Abstract

A proposal for the additional metadata for Docker images which will provide more
context about the content of the image and define relations between Docker
images.

## Motivation

Image metadata are needed to do automatic generation of the OpenShift v3
resource templates, like PodTemplates, ReplicationControllers, BuildConfigs
etc. Automatic generation of a PodTemplate is also required for
DeploymentConfigs.

## Constraints and Assumptions

This document only defines the metadata needed by the current set of use cases.
Additional metadata and/or use cases may be added in the future.

The metadata should be set using the [LABEL](https://docs.docker.com/reference/builder/#label)
Docker instruction.

Once the Docker image is built the metadata can be obtained by running the
`docker inspect` command. The metadata should be available in the `Labels`
section.

For more default about conventions or guidelines about LABEL, visit:
https://docs.docker.com/userguide/labels-custom-metadata

> The *LABEL* instruction is supported in Docker starting from the version
> 1.6.0. This instruction wont work on older Docker.

## Use cases

1. As an author of Docker image that is going to be consumed by the OpenShift
   platform, I want to express what categories/tags my Docker image will belong
   to, so the platform then can then consume the tags to improve the generation
   workflow based on them.

2. As an user of OpenShift I want to get reliable suggestions about services
   that the Docker image I'm going to use might require to operate properly. As
   an author of the Docker image, I need to have a way to record what services my
   Docker image want to consume.

3. As an author of the Docker image, I need to have a way to indicate whether
   the container started from my Docker image does not support scaling.
   The UI should then communicate this information with the end consumers.

4. As an author of the Docker image, I need to have a way to indicate what
   additional service(s) my Docker image might need to work properly, so the UI
   or generation tools can suggest them to end users.

## Namespaces

The LABEL names should typically be namespaced. The namespace should be set
accordingly to reflect the project that is going to pick up the labels and use
them. For OpenShift the namespace should be set to `openshift.io/` and for
Kubernetes the namespace is `k8s.io/`. For simple labels, like `displayName` or
`description` there might be no namespace set if they end up as standard in
Docker.

## Image Metadata

Name                                  | Type     | Target Namespace |
--------------------------------------|--------- | ------------------
[`tags`](#tags)                       | []string | openshift.io
[`wants`](#wants)                     | []string | openshift.io
[`display-name`](#display-name)       |   string | k8s.io
[`description`](#description)         |   string | k8s.io
[`expose-services`](#expose-services) | []string | openshift.io
[`non-scale`](#non-scale)             |     bool | openshift.io
[`min-cpu`](#min-cpu)                 |   string | openshift.io(?)
[`min-memory`](#min-memory)           |   string | openshift.io(?)


### `tags`

This label contains a list of tags represented as list of comma separated string
values. The tags are the way to categorize the Docker images into broad areas of
functionality. Tags help UI and generation tools to suggest relevant Docker
images during the application creation process.

*Example:*

```
LABEL openshift.io/tags   mongodb,mongodb24,nosql
```

### `wants`

Specifies a list of tags that the generation tools and the UI might use to
provide relevant suggestions if you don't have the Docker images with given tags
already.
For example, if the Docker image wants 'mysql' and 'redis' and you don't have
the Docker image with 'redis' tag, then UI might suggest you to add this image
into your deployment.

*Example:*

```
LABEL openshift.io/wants   mongodb,redis
```

### `display-name`

This label provides a human readable name of the Docker image. The Docker image
name might be complex and might be hard to display on the UI pages. This label
should contain short, human readable version of the Docker image name.

*Example:*

```
LABEL k8s.io/display-name MySQL 5.5 Server
```

### `description`

This label can be used to give the Docker image consumers more detailed
information about the service or functionality this image provides.  The UI can
then use this description together with the Docker image name to provide more
human friendly information to end users.

*Example:*

```
LABEL k8s.io/description The MySQL 5.5 Server with master-slave replication support
```

### `expose-services`

This label contains a list of service ports that match with the EXPOSE instructions
in the Dockerfile and provide more descriptive information about what actual service
on the given port provides to consumers.

The format is `PORT[/PROTO]:NAME` where the `[PROTO]` part is optional
and it defaults to `tcp` if it is not specified.

*Example:*

```
LABEL openshift.io/expose-services 2020/udp:ftp,8080:https
```

### `non-scalable` (post-3.0)

An image might use this variable to suggest that it does not support scaling.
The UI will then communicate this to consumers of that image.
Being not-scalable basically means that the value of 'replicas' should initially
not be set higher than 1.

*Example:*

```
LABEL openshift.io/non-scalable     true
```

### `min-cpu` and `min-memory` (post-3.0)

This label suggests how much resources the Docker image might need in order
to work properly. The UI might warn the user that deploying this Docker image
may exceed their user quota.

The values must be compatible with [Kubernetes
quantity](https://github.com/kubernetes/kubernetes/blob/master/docs/design/resources.md#resource-quantities) values for CPU and memory.

*Example:*

```
LABEL openshift.io/min-memory 8Gi
LABEL openshift.io/min-cpu     4
```
