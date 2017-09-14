# Developer's Guide to Service-Catalog

Table of Contents
- [Overview](#overview)
- [Working on Issues](#working-on-issues)
- [Prerequisites](#prerequisites)
- [Workflow](#workflow)
- [Building](#building)
- [Testing](#testing)
- [Advanced Build Steps](#advanced-build-steps)
- [Deploying to Kubernetes](#deploying-to-kubernetes)
- [Demo walkthrough](#demo-walkthrough)

## Overview

This repository is organized as similarly to Kubernetes itself as the developers
have found possible (or practical). Below is a summary of the repository's
layout:

    .
    ├── .glide                  # Glide cache (untracked)
    ├── bin                     # Destination for binaries compiled for linux/amd64 (untracked)
    ├── build                   # Contains build-related scripts and subdirectories containing Dockerfiles
    ├── charts                  # Helm charts for deployment
    │   └── catalog             # Helm chart for deploying the service catalog
    │   └── ups-broker          # Helm chart for deploying the user-provided service broker
    ├── cmd                     # Contains "main" Go packages for each service catalog component binary
    │   └── apiserver           # The service catalog API server binary
    │   └── controller-manager  # The service catalog controller manager binary
    ├── contrib                 # Contains examples, non-essential golang source, CI configurations, etc
    │   └── build               # Dockerfiles for contrib images (example: ups-broker)
    │   └── cmd                 # Entrypoints for contrib binaries
    │   └── examples            # Example API resources
    │   └── hack                # Non-build related scripts
    │   └── jenkins             # Jenkins configuration
    │   └── pkg                 # Contrib golang code
    │   └── travis              # Travis configuration
    ├── docs                    # Documentation
    ├── pkg                     # Contains all non-"main" Go packages
    ├── plugin                  # Plugins for API server
    ├── test                    # Integration and e2e tests
    └── vendor                  # Glide-managed dependencies

## Working on Issues

Github does not allow non-maintainers to assign, or be assigned to, issues.
As such non-maintainers can indicate their desire to work on (own) a particular
issue by adding a comment to it of the form:

	#dibs

However, it is a good idea to discuss the issue, and your intent to work on it,
with the other members via the [slack channel](https://kubernetes.slack.com/messages/sig-service-catalog)
to make sure there isn't some other work already going on with respect to that
issue.

When you create a pull request (PR) that completely addresses an open issue
please include a line in the initial comment that looks like:

	Closes: #1234

where `1234` is the issue number. This allows Github to automatically
close the issue when the PR is merged.

Also, before you start working on your issue, please read our [Code Standards](./code-standards.md)
document.

## Prerequisites

At a minimum you will need:

* [Docker](https://www.docker.com) installed locally
* GNU Make
* [git](https://git-scm.com)

These will allow you to build and test service catalog components within a
Docker container.

If you want to deploy service catalog components built from source, you will
also need:

* A working Kubernetes cluster and `kubectl` installed in your local `PATH`,
  properly configured to access that cluster. The version of Kubernetes and
  `kubectl` must be >= 1.6. See below for instructions on how to download these
  versions of `kubectl`
* [Helm](https://helm.sh) (Tiller) installed in your Kubernetes cluster and the
  `helm` binary in your `PATH`
* To be pre-authenticated to a Docker registry (if using a remote cluster)

**Note:** It is not generally useful to run service catalog components outside
a Kubernetes cluster. As such, our build process only supports compilation of
linux/amd64 binaries suitable for execution within a Docker container.

## Workflow
We can set up the repo by following a process similar to the [dev guide for k8s]( https://github.com/kubernetes/community/blob/master/contributors/devel/development.md#1-fork-in-the-cloud)

### 1 Fork in the Cloud
1. Visit https://github.com/kubernetes-incubator/service-catalog
2. Click Fork button (top right) to establish a cloud-based fork.

### 2 Clone fork to local storage

Per Go's workspace instructions, place Service Catalog's code on your GOPATH
using the following cloning procedure.

Define a local working directory:

> If your GOPATH has multiple paths, pick
> just one and use it instead of $GOPATH.

> You must follow exactly this pattern,
> neither `$GOPATH/src/github.com/${your github profile name}/`
> nor any other pattern will work.

From your shell:
```bash
# Run the following only if `echo $GOPATH` shows nothing.
export GOPATH=$(go env GOPATH)

# Set your working directory
working_dir=$GOPATH/src/github.com/kubernetes-incubator

# Set user to match your github profile name
user={your github profile name}

# Create your clone:
mkdir -p $working_dir
cd $working_dir
git clone https://github.com/$user/service-catalog.git
# or: git clone git@github.com:$user/service-catalog.git

cd service-catalog
git remote add upstream https://github.com/kubernetes-incubator/service-catalog.git
# or: git remote add upstream git@github.com:kubernetes-incubator/service-catalog.git

# Never push to upstream master
git remote set-url --push upstream no_push

# Confirm that your remotes make sense:
git remote -v
```

## Building

First `cd` to the root of the cloned repository tree.
To build the service-catalog:

    $ make build

The above will build all executables and place them in the `bin` directory. This
is done within a Docker container-- meaning you do not need to have all of the
necessary tooling installed on your host (such as a golang compiler or glide).
Building outside the container is possible, but not officially supported.

Note, this will do the basic build of the service catalog. There are more
more [advanced build steps](#advanced-build-steps) below as well.

To deploy to Kubernetes, see the
[Deploying to Kubernetes](#deploying-to-kubernetes) section.

### Notes Concerning the Build Process/Makefile

* The Makefile assumes you're running `make` from the root of the repo.
* There are some source files that are generated during the build process.
  These are:

    * `pkg/client/*_generated`
    * `pkg/apis/servicecatalog/zz_*`
    * `pkg/apis/servicecatalog/v1alpha1/zz_*`
    * `pkg/apis/servicecatalog/v1alpha1/types.generated.go`
    * `pkg/openapi/openapi_generated.go`

* Running `make clean` or `make clean-generated` will roll back (via
  `git checkout --`) the state of any generated files in the repo.
* Running `make purge-generated` will _remove_ those generated files from the
  repo.
* A Docker Image called "scbuildimage" will be used. The image isn't pre-built
  and pulled from a public registry. Instead, it is built from source contained
  within the service catalog repository.
* While many people have utilities, such as editor hooks, that auto-format
  their go source files with `gofmt`, there is a Makefile target called
  `format` which can be used to do this task for you.
* `make build` will build binaries for linux/amd64 only.

## Testing

There are two types of tests: unit and integration. The unit testcases
can be run via the `test-unit` Makefile target, e.g.:

    $ make test-unit

These will execute any `*_test.go` files within the source tree.
The integration tests can be run via the `test-integration` Makefile target,
e.g.:

    $ make test-integration

The integration tests require the Kubernetes client (`kubectl`) so there is a
script called `contrib/hack/kubectl` that will run it from within a
Docker container. This avoids the need for you to download, or install it,
youself. You may find it useful to add `contrib/hack` to your `PATH`.

The `test` Makefile target will run both the unit and integration tests, e.g.:

    $ make test

If you want to run just a subset of the unit testcases then you can
specify the source directories of the tests:

    $ TEST_DIRS="path1 path2" make test

or you can specify a regexp expression for the test name:

    $ UNIT_TESTS=TestBar* make test

To see how well these tests cover the source code, you can use:

    $ make coverage

These will execute the tests and perform an analysis of how well they
cover all code paths. The results are put into a file called:
`coverage.html` at the root of the repo.

## Advanced Build Steps

You can build the service catalog executables into Docker images yourself. By
default, image names are `quay.io/kubernetes-service-catalog/<component>`. Since
most contributors who hack on service catalog components will wish to produce
custom-built images, but will be unable to push to this location, it can be
overridden through use of the `REGISTRY` environment variable.

Examples of apiserver image names:

| `REGISTRY` | Fully Qualified Image Name | Notes |
|----------|----------------------------|-------|
| Unset; default | `quay.io/kubernetes-service-catalog/apiserver` | You probably don't have permissions to push to here |
| Dockerhub username + trailing slash, e.g. `krancour/` | `krancour/apiserver` | Missing hostname == Dockerhub |
| Dockerhub username + slash + some prefix, e.g. `krancour/sc-` | `krancour/sc-apiserver` | The prefix is useful for disambiguating similarly names images within a single namespace. |
| 192.168.99.102:5000/ | `192.168.99.102:5000/apiserver` | A local registry |

With `REGISTRY` set appropriately:

    $ make images push

This will build Docker images for all service catalog components. The images are
also pushed to the registry specified by the `REGISTRY` environment variable, so
they can be accessed by your Kubernetes cluster.

The images are tagged with the current Git commit SHA:

    $ docker images

----

## Deploying to Kubernetes

Use the [`catalog` chart](../charts/catalog) to deploy the service
catalog into your cluster.  The easiest way to get started is to deploy into a
cluster you regularly use and are familiar with.  One of the choices you can
make when deploying the catalog is whether to make the API server store its
resources in an external etcd server, or in third party resources.

If you choose etcd storage, the helm chart will launch an etcd server for you
in the same pod as the service-catalog API server. You will be responsible for
the data in the etcd server container.

If you choose third party resources storage, the helm chart will not launch an
etcd server, but will instead instruct the API server to store all resources in
the Kubernetes cluster as third party resources.

## Demo walkthrough

Check out the [introduction](./introduction.md) to get started with 
installation and a self-guided demo.
