OpenShift Examples
==================

This directory contains examples of using OpenShift and explaining the new concepts
available on top of Kubernetes and Docker.

* [Hello OpenShift](./hello-openshift) is a simple Hello World style application that can be used to start a simple pod
* [OpenShift Sample](./sample-app) is an end-to-end application demonstrating the full
  OpenShift v3 concept chain - images, builds, deployments, and templates.
* [Jenkins Example](./jenkins) demonstrates how to enhance the [sample-app](./sample-app) by deploying a Jenkins pod on OpenShift and thereby enable continuous integration for incoming changes to the codebase and trigger deployments when integration succeeds.
* [Node.js echo Sample](https://github.com/openshift/nodejs-ex) highlights the simple workflow from creating project, new app from GitHub, building, deploying, running and updating.
* [Project Quotas and Resource Limits](./project-quota) demonstrates how quota and resource limits can be applied to resources in an OpenShift project.
* [Replicated Zookeper Template](./zookeeper) provides a template for an OpenShift service that exposes a simple set of primitives that distributed applications can build upon to implement higher level services for synchronization, configuration maintenance, and groups and naming.
* [Storage Examples](./storage-examples) provides a high level tutorial and templates for local and persistent storage on OpenShift using simple nginx applications.
* [Database Templates](./db-templates) provide templates for ephemeral and persistent storage on OpenShift using MongoDB, MySQL, and PostgreSQL.
* [Clustered Etcd Template](./etcd) provides a template for setting up a clustered instance of the [Etcd](https://github.com/coreos/etcd) key-value store as a service on OpenShift.
* [Configurable Git Server](./gitserver) sets up a serivce capable of automatic mirroring of Git repositories, intended for use within a container or Kubernetes pod.
