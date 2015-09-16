# OpenShift Builds

## Problem/Rationale

Kubernetes creates Docker containers from images that were built elsewhere and pushed to a Docker registry. Building Docker images is a foundational use-case in Docker-based workflows for application development and deployment. Without support for builds in Kubernetes, if a system administrator wanted a system that could build images, he or she would have to select a pre-existing build system or write a new one, and then figure out how to deploy and maintain it on or off Kubernetes. However, in most cases operators would wish to leverage the ability of Kubernetes to schedule task execution into a pool of available resources, and most build systems would want to take advantage of that mechanism.

Offering an API for builds makes OpenShift a viable back-end for arbitrary third-party Docker image build systems which require resource constraints and scheduling capabilities, and allows organizations to orchestrate Docker builds from their existing continuous integration processes. OpenShift enables CI/CD flows around Docker images.

Most build jobs share common characteristics - a set of inputs to a build, the need to run a certain build process to completion, the capture of the logs from that build process, publishing resources from successful builds, and the final status of the build. In addition, the image-driven deployment flow that Kubernetes advocates depends on having images available.

Builds should take advantage of resource restrictions – specifying limitations on things such as CPU usage, memory usage, and build (pod) execution time – once support for this exists in Kubernetes. Additionally, builds should be repeatable and consistent (same inputs = same output).

Although there are potentially several different types of builds that produce other types of output, OpenShift by default provides the ability to build Docker images.

Here are some possible user scenarios for builds in OpenShift:

1.   As a user of OpenShift, I want to build an image from a source URL and push it to a registry (for eventual deployment in OpenShift).
2.   As a user of OpenShift, I want to build an image from a binary input (Docker context, artifact) and push it to a registry (for eventual deployment in OpenShift).
3.   As a provider of a service that involves building Docker images, I want to offload the resource allocation, scheduling, and garbage collection associated with that activity to OpenShift instead of solving those problems myself.
4.   As a developer of a system which involves building Docker images, I want to take advantage of OpenShift to perform the build, but orchestrate from an existing CI in order to integrate with my organization’s devops SOPs.

### Example Use: Cloud IDE

Company X offers a Docker-based cloud IDE service and needs to build Docker images at scale for their customers’ hosted projects. Company X wants a turn-key solution for this that handles scheduling, resource allocation, and garbage collection. Using the build API, Company X can leverage OpenShift for the build work and concentrate on solving their core business problems.

### Example Use: Enterprise Devops

Company Y wants to leverage OpenShift to build Docker images, but their Devops SOPs mandate the use of a third-party CI server in order to facilitate actions such as triggering builds when an upstream project is built and promoting builds when the result is signed off on in the CI server. Using the build API, company Y implements workflows in the CI server that orchestrate building in OpenShift which integrates with their organization’s SOPs.

## Build Strategies

The OpenShift build system provides extensible support for build strategies based on selectable types specified in the build API. By default, two strategies are supported: Docker builds, and [Source-To-Images (sti)](https://github.com/openshift/source-to-image#source-to-image-sti) builds.

### Docker Builds

OpenShift supports Docker builds. Using this strategy, users may supply a URL to a Docker context which is used as the basis for a [Docker build](https://docs.docker.com/reference/commandline/cli/#build).

#### How It Works

To implement Docker builds, OpenShift provides build containers access to a node’s Docker daemon.

During a build, a pod containing a single container–a build container–is created. The node’s Docker socket is bind mounted into the build container. The build container executes `docker build` using the the supplied Docker context, and all interaction with Docker occurs via the node's Docker daemon.

**Advantages**

1.  Allows Docker builds in unprivileged containers
2.  Minimizes image storage requirements
3.  Reduces the number of Docker daemons required

**Disadvantages**

1.  Constraining resources per-user is made more difficult
2.  Containers created during the build are created outside the scope of the kubelet
3.  Container processes created during the build are children of a remote Docker process, making container cleanup more difficult

There are viable paths to alleviate or resolve each of these disadvantages, and this mechanism is considered a work in progress.

##### Why not Docker-in-Docker?

It's theoretically possible to implement builds using a nested Docker daemon within a Docker container (Docker-in-Docker). On the surface, this approach offers some compelling advantages:

1.  Build process resources can be naturally constrained to the user’s acceptable limits (cgroups)
2.  Containers created during the build have the build container as their parent process, making container cleanup simple

In practice, however, there are (at present) some serious problems with the approach which render it unusable:

1.  Requires a privileged container, which is a show-stopping security concern with no solution on the horizon
    * In addition, this nullifies the theoretical benefit of cgroups isolation, as the process could break out the container
2.  With devicemapper, it's very easy to leak both loopback devices and storage on the host
3.  No easy way to share storage of images/layers among build containers, requiring each Docker-in-Docker instance to store its own unique, full copy of any image(s) downloaded during the build process.
    * A caching proxy running on the node could at least minimize the number of times an image is pulled from a remote registry, but that doesn’t eliminate the need for each build container to have its own copy of the images.

For these reasons, Docker-in-Docker is not considered a viable build strategy for a secure, multi-tenant production environment.

### STI (Source-to-Image) Builds

OpenShift also supports [Source-To-Images (sti)](https://github.com/openshift/source-to-image#source-to-image-sti) builds.

Source-to-images (sti) is a tool for building reproducible Docker images. It produces ready-to-run images by injecting a user source into a docker image and assembling a new Docker image which incorporates the base image and built source, and is ready to use with `docker run`. STI supports incremental builds which re-use previously downloaded dependencies, previously built artifacts, etc.

### Custom Builds

The custom build strategy is very similar to *Docker build* strategy, but users might
customize the builder image that will be used for build execution. The *Docker build* uses [openshift/origin-custom-docker-builder](https://hub.docker.com/r/openshift/origin-custom-docker-builder/) image by default. Using your own builder image allows you to customize your build process.

An example JSON of a custom build strategy:

```json
"strategy": {
  "type": "Custom",
    "customStrategy": {
      "image": "my-custom-builder-image",
      "exposeDockerSocket": true,
      "env": [
        { "name": "EXPOSE_PORT", "value": "8080" }
      ]
    }
}
```

The `exposeDockerSocket` option will mount the Docker socket from host into your
builder container and allows you to execute the `docker build` and `docker push` commands.
Note that this might be restricted by the administrator in future.

The `env` option allows you to specify additional environment variables that will
be passed to the builder container environment. By default, these environment
variables are passed to the build container:

* `$BUILD` contains the JSON representation of the Build
* `$OUTPUT_IMAGE` contains the output Docker image name as configured in Build
* `$OUTPUT_REGISTRY` contains the output Docker registry as configured in Build
* `$SOURCE_URI` contains the URL to the source code repository
* `$SOURCE_REF` contains the branch, tag or ref for source repository
* `$DOCKER_SOCKET` contains full path to the Docker socket

