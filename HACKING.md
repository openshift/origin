## Installing Godep

OpenShift and Kubernetes use [Godep](https://github.com/tools/godep) for dependency management.  Godep allows versions of dependent packages to be locked at a specific commit by *vendoring* them (checking a copy of them into `Godeps/_workspace/`).  This means that everything you need for OpenShift is checked into this repository, and the `hack/config-go.sh` script will set your GOPATH appropriately.  To install `godep` locally run:

    $ go get github.com/tools/godep

If you are not updating packages you should not need godep installed.

## Updating Godeps from upstream

To update to a new version of a dependency that's not already included in Kubernetes, checkout the correct version in your GOPATH and then run `godep save <pkgname>`.  This should create a new version of `Godeps/Godeps.json`, and update `Godeps/workspace/src`.  Create a commit that includes both of these changes.

To update the Kubernetes version, checkout the new "master" branch from openshift/kubernetes (within your regular GOPATH directory for Kubernetes), and run `godep restore ./...` from the Kubernetes dir.  Then switch to the OpenShift directory and run `godep save ./...` 

## Running OpenShift in Docker

The only prerequisites for building and running OpenShift entirely in Docker are Docker 1.2 and the OpenShift source. The following commands will build OpenShift in Docker, launch a cluster using the built source, and automatically watch the logs of all the containers:

    $ git clone https://github.com/openshift/origin.git
    $ cd origin
    $ hack/build-docker.sh -lw

The first launch will take a few minutes to build the base builder and runner images.

Once up, to shut down and remove all traces of the cluster, simply press `Ctrl-C`.

### Docker development workflows

In addition to a full rebuild and launch, there are a variety of ways to use the Docker build tools to facilitate rapid OpenShift development locally.

Compile the code, overriding a Godeps dependency with local source from `GOPATH` on the host:

    $ hack/build-docker.sh -d github.com/GoogleCloudPlatform/Kubernetes

See `hack/build-docker.sh -h` for a full list options for workflow composition.

### How it works

The OpenShift Docker build process:

  1. Uses `docker build` to make the `origin-build` Docker image, which is runnable and contains the environment necessary to build OpenShift and an `origin` Docker volume to store and expose build artifacts.
  2. Uses `docker run` to create the `origin-build` container with local source mounted read-only. The build output is stored in the `origin` volume.
  3. Discards or preserves the `origin-build`  container and volume, depending on use.

The launch process extends the build process by running a Docker container for each OpenShift/Kubernetes component.

TODO: Update these docs, which originally reflected the non-DIND approach