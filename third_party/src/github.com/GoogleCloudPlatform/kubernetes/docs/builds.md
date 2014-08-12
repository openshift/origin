This document describes a build implementation inspired by #503.

We think a comprehensive platform should include deployment capabilities and a means to build images without requiring external infrastructure. To build images, you need hosting infrastructure. At scale, we'd prefer to use the cluster’s resources where possible, and schedule builds just like any other task (i.e. pod). In order to model this problem, we need a notion of a pod that runs only once.  We wanted to get a feel for what this integration might feel like.

To that end, we've been working on a prototype to add the ability to build images in Kubernetes.

A **build** is a user's request to create a new Docker image from one or more inputs (such as a Dockerfile or Docker context).  It has a status and a reference to the Pod which implements the build. In our POC we implement Dockerfile builds - we expect to support multiple build types such as STI (source to images), packer, Dockerfile2, etc.  We are especially interested in feedback about how this problem should be modeled to facilitate other build extensions. 

A new **build controller** (similar to the replication controller) looks for new builds in storage and acts on them. It creates a run-once pod for the build and monitors its status to completion (success or failure).

The build controller can support different build implementations, with the initial prototype defining a container that runs its own Docker daemon (Docker-in-Docker) and then executes `docker build` using the Docker context specified as a parameter to the Kubernetes build.

Implementation Notes:

We had to prototype/provide a couple of new capabilities to implement this proof of concept:

- Run-once containers
- Launching privileged containers (for Docker-in-Docker)

Supplementals:

Dockerfile: https://gist.github.com/pmorie/b7a0270bab01b86091aa
Build descriptor: https://gist.github.com/ncdc/17dce7b517ff6ab4118e
Docker-in-docker image: https://github.com/ironcladlou/fedora-dind
Docker builder image: https://github.com/ironcladlou/openshift-docker-builder