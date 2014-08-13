# Build example

These examples demonstrate some practical use cases for builds, including a build image which allows user-defined Docker builds within Kubernetes.

Each of the following commands should be executed from the root Kubernetes directory.

### Prerequisite: Create builder images

Build the Docker-in-Docker support image:

    docker build -t kubernetes-fedora-dind examples/builds/kubernetes-fedora-dind

Build the Docker builder image. Change the tag to match your Docker Hub ID. Pushing the image to the public index is necessary, as Kubernetes always does a `docker pull` of container images in a pod.

    docker build -t myrepo/docker-builder examples/builds/docker-builder
    docker push myrepo/docker-builder

### Start Kubernetes

Start Kubernetes, configuring the build controller to launch `docker` type builds using your `myrepo/docker-builder` image:

    DOCKER_BUILDER_IMAGE=myrepo/docker-builder hack/local-up-cluster.sh

### Launch a build

Use `curl` to launch a simple Docker-in-Docker build:

    curl -H "Content-Type: application/json" --data @examples/builds/docker-build.json http://127.0.0.1:8080/api/v1beta1/builds

NOTE: The example build JSON has a hard-coded image tag which may need adjusted if you're using your own tag from the previous steps. The `docker-build.json` file can be used as a template.