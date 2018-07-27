OpenShift Examples
==================

This directory contains examples of using OpenShift and explaining the new concepts
available on top of Kubernetes and Docker.

* [OpenShift Sample](./sample-app) is an end-to-end application demonstrating the full
  OpenShift v3 concept chain - images, builds, deployments, and templates.
* [Jenkins Example](./jenkins) demonstrates how to enhance the [sample-app](./sample-app) by deploying a Jenkins pod on OpenShift and thereby enable continuous integration for incoming changes to the codebase and trigger deployments when integration succeeds.
* [Node.js echo Sample](https://github.com/sclorg/nodejs-ex) highlights the simple workflow from creating project, new app from GitHub, building, deploying, running and updating.
* [Configurable Git Server](./gitserver) sets up a service capable of automatic mirroring of Git repositories, intended for use within a container or Kubernetes pod.
* [QuickStarts](./quickstarts) provides templates for very basic applications using various frameworks and databases.
* [Database Templates](./db-templates) provides templates for ephemeral and persistent storage on OpenShift using MongoDB, MySQL, and PostgreSQL.
