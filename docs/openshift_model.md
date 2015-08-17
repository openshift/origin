# OpenShift Model

## Overview
OpenShift extends the base Kubernetes model to provide a more feature rich development lifecycle platform.

## Build

## BuildConfig

### Build Strategies
The OpenShift build system provides extensible support for build strategies based on selectable types specified in the build API. By default, two strategies are supported: Docker builds, and Source-to-Image builds.

#### Docker build
OpenShift supports pure Docker builds. Using this strategy, users may supply a URL to a Docker context which is used as the basis for a [Docker build](https://docs.docker.com/reference/commandline/cli/#build).

#### Custom build
The custom build strategy is very similar to *Docker build* strategy, but users might customize the builder image that will be used for build execution. The Docker build uses [openshift/origin-custom-docker-builder](https://hub.docker.com/r/openshift/origin-custom-docker-builder/) image by default. Using your own builder image allows you to customize your build process.

#### Source-to-Image
[Source-to-image](https://github.com/openshift/source-to-image) (STI) is a tool for building reproducible Docker images. It produces ready-to-run images by injecting a user source into a docker image and assembling a new Docker image which incorporates the base image and built source, and is ready to use with `docker run`. STI supports incremental builds which re-use previously downloaded dependencies, previously built artifacts, etc.

##### So why would you want to use this?

There were a few goals for STI.

* **Image flexibility**: STI allows you to use almost any existing Docker image as the base for your application. STI scripts can be written to layer application code onto almost any existing Docker image, so you can take advantage of the existing ecosystem. (Why only "almost" all images? Currently STI relies on tar/untar to inject application source so the image needs to be able to process tarred content.)
* **Speed**: Adding layers as part of a Dockerfile can be slow. With STI the assemble process can perform a large number of complex operations without creating a new layer at each step. In addition, STI scripts can be written to re-use dependencies stored in a previous version of the application image rather than re-downloading them each time the build is run.
* **Patchability**: If an underlying image needs to be patched due to a security issue, OpenShift can use STI to rebuild your application on top of the patched builder image.
* **Operational efficiency**: By restricting build operations instead of allowing arbitrary actions such as in a Dockerfile, the PaaS operator can avoid accidental or intentional abuses of the build system.
* **Operational security**: Allowing users to build arbitrary Dockerfiles exposes the host system to root privilege escalation by a malicious user because the entire docker build process is run as a user with docker privileges. STI restricts the operations performed as a root user, and can run the scripts as an individual user
* **User efficiency**: STI prevents developers from falling into a trap of performing arbitrary "yum install" type operations during their application build, which would result in slow development iteration.
* **Ecosystem**: Encourages a shared ecosystem of images with best practices you can leverage for your applications.

## BuildLog

Model object that stores the logs from a particular build for later inspection.

## Deployment

A deployment is a specially annotated [replicationController](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/replication-controller.md), specifying the desired configuration of that controller. See the [deployments](deployments.md) document.

## DeploymentConfig

A DeploymentConfig specifies an existing deployment, triggers that can result in replacing that deployment with a new one, the strategy for doing so, and the history of such changes. See the [deployments](deployments.md) document.

## Image

Metadata added to the concept of a Docker image (such as repository, tag, environment variables, etc.) that runs in a container.

## ImageRepository

## ImageRepositoryMapping

## Template

## TemplateConfig

## Route

A named method of accessing an externally-exposed Kubernetes Service that represents an external endpoint (such as a web server, message queue, or database). See the [routing document](routing.md).

## Project

An OpenShift-level grouping of pod deployments and attendant resources. May be used for identifying components in an "application" and authorizing collaboration on same.

## User

A user identity that may be authenticated and authorized for a set of capabilities. Can correspond to an actual person or a service account. Refer to the [capabilities proposal](proposals/capabilities.md) for more context around the user and access model.

