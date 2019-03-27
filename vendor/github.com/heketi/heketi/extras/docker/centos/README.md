# Heketi Container Image based on packages from the CentOS Storage SIG

The CentOS Storage SIG provides a stable version of Heketi as and RPM. This
container aims to be suitable for productions deployments where CentOS is the
preferred operating system.

This directory contains two files from creating images:

1. `Dockerfile` for building an image from released packages
2. `Dockerfile.testing` for building an image from packages in testing

These images are maintained by the Heketi Developers, for questions or problem
reports they can be reached on
[heketi-devel@gluster.org](mailto:heketi-devel@gluster.org), on Freenode IRC in
#heketi or by opening an [issue on
Gitub](https://github.com/heketi/heketi/issues/new).


## Build Process

Images are build by the [CentOS Community Container Pipeline
Service](https://github.com/CentOS/container-pipeline-service). The
specification for the job is split over two files:

1. the `cccp.yaml` contains the `job-id` which is used for the image name
2. a [`gluster.yaml` in the container-index
   repository](https://github.com/CentOS/container-index/blob/master/index.d/gluster.yaml)
   that points to this repository and its `Dockerfile`s
