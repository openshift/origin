OpenShift Application Platform
==============================

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/origin)](https://goreportcard.com/report/github.com/openshift/origin)
[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Travis](https://travis-ci.org/openshift/origin.svg?branch=master)](https://travis-ci.org/openshift/origin)
[![Jenkins](https://ci.openshift.redhat.com/jenkins/buildStatus/icon?job=devenv_ami)](https://ci.openshift.redhat.com/jenkins/job/devenv_ami/)
[![Join the chat at freenode:openshift-dev](https://img.shields.io/badge/irc-freenode%3A%20%23openshift--dev-blue.svg)](http://webchat.freenode.net/?channels=%23openshift-dev)
[![Licensed under Apache License version 2.0](https://img.shields.io/github/license/openshift/origin.svg?maxAge=2592000)](https://www.apache.org/licenses/LICENSE-2.0)

***OpenShift Origin*** is a distribution of [Kubernetes](https://kubernetes.io) optimized for continuous application development and multi-tenant deployment.  Origin enables teams of all sizes to quickly develop, deploy, and scale applications, reducing the length of development cycles and operational effort.

[![Watch the full asciicast](docs/openshift-intro.gif)](https://asciinema.org/a/49402)

**Features:**

**For developers:**

* Easily build applications with integrated service discovery and persistent storage.
* Quickly and easily scale applications to handle periods of increased demand.
  * Support for automatic high availability, load balancing, health checking, and failover.
* Push source code to your Git repository and automatically deploy containerized applications.
* Web console and command-line client for building and monitoring applications.

**For system administrators:**

* Centralized administration and management of an entire stack, team, or organization.
  * Create reusable templates for components of your system, and iteratively deploy them over time.
  * Roll out modifications to software stacks to your entire organization in a controlled fashion.
  * Integration with your existing authentication mechanisms, including LDAP, Active Directory, and public OAuth providers such as GitHub.
* Multi-tenancy support, including team and user isolation of containers, builds, and network communication.
  * Allow developers to run containers securely with fine-grained controls in production.
  * Limit, track, and manage the developers and teams on the platform.
* Integrated Docker registry, automatic edge load balancing, cluster logging, and integrated metrics.

**Learn More:**

* **[Public Documentation](https://docs.openshift.org/latest/welcome/)**
  * **[API Documentation](https://docs.openshift.org/latest/rest_api/openshift_v1.html)**
* **[Technical Architecture Presentation](https://docs.google.com/presentation/d/1Isp5UeQZTo3gh6e59FMYmMs_V9QIQeBelmbyHIJ1H_g/pub?start=false&loop=false&delayms=3000)**
* **[System Architecture](https://github.com/openshift/openshift-pep/blob/master/openshift-pep-013-openshift-3.md)** design document
* The **[Trello Roadmap](https://ci.openshift.redhat.com/roadmap_overview.html)** covers the epics and stories being worked on (click through to individual items).

For questions or feedback, reach us on [IRC on #openshift-dev](https://botbot.me/freenode/openshift-dev/) on Freenode or post to our [mailing list](https://lists.openshift.redhat.com/openshiftmm/listinfo/dev).

Getting Started
---------------

### Installation

* If you intend to develop applications to run on an existing installation of the OpenShift platform, you can download the client tools and place the included binaries in your `PATH`.
* For local development/test or product evaluation purposes, we recommend using the quick install as described in the [Getting Started Install guide](https://docs.openshift.org/latest/getting_started/administrators.html).
* For production environments, we recommend using [Ansible](https://github.com/openshift/openshift-ansible) as described in the [Advanced Installation guide](https://docs.openshift.org/latest/install_config/install/advanced_install.html).
* To build and run from source, see [CONTRIBUTING.adoc](CONTRIBUTING.adoc).

### Concepts

The [Origin walkthrough](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md) is a step-by-step guide that demonstrates the core capabilities of OpenShift throughout the development, build, and test cycle.  The walkthrough is accompanied by a blog series on [blog.openshift.com](https://blog.openshift.com/openshift-v3-deep-dive-docker-kubernetes/) that goes into more detail.  It's a great place to start.

### Origin API

The Origin API is located on each server at `https://<host>:8443/oapi/v1`. These APIs are described via [Swagger v1.2](https://www.swagger.io) at `https://<host>:8443/swaggerapi/oapi/v1`. For more, [see the API documentation](https://docs.openshift.org/latest/rest_api/openshift_v1.html).

### Kubernetes

Since OpenShift Origin builds on Kubernetes, it is helpful to understand underlying concepts such as Pods and Replication Controllers. The following are good references:

* [Kubernetes User Guide](http://kubernetes.io/docs/user-guide/)
* [Kubernetes Getting Started](http://kubernetes.io/docs/whatisk8s/)
* [Kubernetes Documentation](https://github.com/kubernetes/kubernetes/blob/master/docs/README.md)
* [Kubernetes API](https://docs.openshift.org/latest/rest_api/kubernetes_v1.html)

### Troubleshooting

The [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md) provides advice for diagnosing and correcting any problems that you may encounter while installing, configuring, or running Origin.

FAQ
---

1. How does Origin relate to Kubernetes?

    Origin is a distribution of Kubernetes optimized for enterprise application development and deployment, and is the foundation of OpenShift 3 and Atomic Enterprise.  Origin extends Kubernetes with additional functionality, offering a simple, yet powerful, development and operational experience.  Both Origin and the upstream Kubernetes project focus on building and deploying applications in containers.

    You can run the core Kubernetes server components with `openshift start kube` and use `openshift kube` in place of `kubectl`.  Additionally, the Origin release archives include versions of `kubectl`, `kubelet`, `kube-apiserver`, and other core components.  You can see the version of Kubernetes included with Origin by invoking `openshift version`.

2. How does Atomic Enterprise relate to Origin and OpenShift?

    Two products are built from Origin, Atomic Enterprise and OpenShift. Atomic Enterprise adds
    operational centric tools to enable easy deployment and scaling and long-term lifecycle
    maintenance for small and large teams and applications. OpenShift provides a number of
    developer-focused tools on top of Atomic Enterprise such as image building, management, and
    enhanced deployment flows.

3. What can I run on Origin?

    Origin is designed to run any existing Docker images.  Additionally, you can define builds that will produce new Docker images using a `Dockerfile`.

    However, the real magic of Origin is [Source-to-Image (S2I)](https://github.com/openshift/source-to-image) builds, which allow developers to simply provide an application source repository containing code to build and execute.  It works by combining an existing S2I-enabled Docker image with application source to produce a new runnable image for your application.

    We are continuing to grow the [ecosystem of Source-to-Image builder images](https://docs.openshift.org/latest/using_images/s2i_images/overview.html) and it's straightforward to [create your own](https://blog.openshift.com/create-s2i-builder-image/).  Our available images are:

    * [Ruby](https://github.com/openshift/sti-ruby)
    * [Python](https://github.com/openshift/sti-python)
    * [Node.js](https://github.com/openshift/sti-nodejs)
    * [PHP](https://github.com/openshift/sti-php)
    * [Perl](https://github.com/openshift/sti-perl)
    * Other community-supported images, including [WildFly](https://github.com/openshift-s2i/s2i-wildfly)

    Your application image can be easily extended with a database service with our [database images](https://docs.openshift.org/latest/using_images/db_images/overview.html). Our available database images are:

    * [MySQL](https://github.com/openshift/mysql)
    * [MongoDB](https://github.com/openshift/mongodb)
    * [PostgreSQL](https://github.com/openshift/postgresql)

4. Why doesn't my Docker image run on OpenShift?

    Security! Origin runs with the following security policy by default:

    * Containers run as a non-root unique user that is separate from other system users.
      * They cannot access host resources, run privileged, or become root.
      * They are given CPU and memory limits defined by the system administrator.
      * Any persistent storage they access will be under a unique SELinux label, which prevents others from seeing their content.
      * These settings are per project, so containers in different projects cannot see each other by default.
    * Regular users can run Docker, source, and custom builds.
      * By default, Docker builds can (and often do) run as root. You can control who can create Docker builds through the `builds/docker` and `builds/custom` policy resource.
    * Regular users and project admins cannot change their security quotas.

    Many Docker containers expect to run as root, and therefore expect the ability to modify all contents of the filesystem. The [Image Author's guide](https://docs.openshift.org/latest/creating_images/guidelines.html#openshift-specific-guidelines) provides recommendations for making your image more secure by default:

    * Don't run as root.
    * Make directories you want to write to group-writable and owned by group id 0.
    * Set the net-bind capability on your executables if they need to bind to ports &lt;1024
      (e.g. `setcap cap_net_bind_service=+ep /usr/sbin/httpd`).

    Although we recommend against it for security reasons, it is also possible to relax these restrictions as described in the [security documentation](https://docs.openshift.org/latest/admin_guide/manage_scc.html).

5. How do I get networking working?

    The Origin and Kubernetes network model assigns each Pod (group of containers) an IP address that is expected to be reachable from all nodes in the cluster. The default configuration uses Open vSwitch to provide Software-Defined Networking (SDN) capabilities, which requires communication between nodes in the cluster using port 4679.  Additionally, the Origin master processes must be able to reach pods within the network, so they may require the SDN plugin.

    Other networking options are available such as Calico, Flannel, Nuage, and Weave. For a non-overlay networking solution, existing networks can be used by assigning a different subnet to each host, and ensuring routing rules deliver packets bound for that subnet to the host it belongs to. This is called [host subnet routing](https://docs.openshift.org/latest/admin_guide/native_container_routing.html).

6. Why can't I run Origin in a Docker image on boot2docker or Ubuntu?

    Versions of Docker distributed by the Docker team don't allow containers to mount volumes on the host and write to them (mount propagation is private). Kubernetes manages volumes and uses them to expose secrets into containers, which Origin uses to give containers the tokens they need to access the API and run deployments and builds. Until mount propagation is configurable in Docker you must use Docker on Fedora, CentOS, or RHEL (which have a patch to allow mount propagation) or run Origin outside of a container. Tracked in [openshift/origin issue #3072](https://github.com/openshift/origin/issues/3072).

Contributing
------------

You can develop [locally on your host](CONTRIBUTING.adoc#develop-locally-on-your-host) or with
a [virtual machine](CONTRIBUTING.adoc#develop-on-virtual-machine-using-vagrant).

If you just want to try Origin, [download the latest pre-built binaries](CONTRIBUTING.adoc#download-from-github)
for Linux, MacOS X (client only), or Windows (client only).

First, **get up and running with the** [**Contributing Guide**](CONTRIBUTING.adoc).

All contributions are welcome - Origin uses the Apache 2 license and does not require any contributor agreement to submit patches.  Please open issues for any bugs or problems you encounter, ask questions on the OpenShift IRC channel (#openshift-dev on freenode), or get involved in the [Kubernetes project](https://github.com/kubernetes/kubernetes) at the container runtime layer.

See [HACKING.md](https://github.com/openshift/origin/blob/master/HACKING.md) for more details on developing on Origin including how different tests are setup.

To run the test suite, run the following commands from the root directory (e.g. `origin`):

```
# run the unit tests
$ make check

# run a command-line integration test suite
$ hack/test-cmd.sh

# run the integration server test suite
$ hack/test-integration.sh

# run the end-to-end test suite
$ hack/test-end-to-end.sh

# run all of the tests above
$ make test
```

You'll need [etcd](https://github.com/coreos/etcd) installed and on your path for the integration and end-to-end tests to run, and Docker must be installed to run the end-to-end tests.  To install etcd you should be able to run:

```
$ hack/install-etcd.sh
```

Some of the components of Origin run as Docker images, including the builders and deployment tools in `images/builder/docker/*` and `images/deploy/*`.  To build them locally, run:

```
$ hack/build-images.sh
```

The OpenShift Origin Management Console (also known as Web Console) is distributed in a separate repository; see the [origin-web-console project](https://github.com/openshift/origin-web-console) for instructions on building and testing your changes.

Copyright and License
---------------------

Copyright 2014-2016 by Red Hat, Inc. and other contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
