OpenShift Application Platform
==============================

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/origin)](https://goreportcard.com/report/github.com/openshift/origin)
[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Travis](https://travis-ci.org/openshift/origin.svg?branch=master)](https://travis-ci.org/openshift/origin)
[![Jenkins](https://ci.openshift.redhat.com/jenkins/buildStatus/icon?job=devenv_ami)](https://ci.openshift.redhat.com/jenkins/job/devenv_ami/)
[![Join the chat at freenode:openshift-dev](https://img.shields.io/badge/irc-freenode%3A%20%23openshift--dev-blue.svg)](http://webchat.freenode.net/?channels=%23openshift-dev)
[![Licensed under Apache License version 2.0](https://img.shields.io/github/license/openshift/origin.svg?maxAge=2592000)](https://www.apache.org/licenses/LICENSE-2.0)

***OpenShift Origin*** is a distribution of [Kubernetes](https://kubernetes.io) optimized for continuous application development and multi-tenant deployment.  Origin adds developer and operations-centric tools on top of Kubernetes to enable rapid application development, easy deployment and scaling, and long-term lifecycle maintenance for small and large teams.

[![Watch the full asciicast](docs/openshift-intro.gif)](https://asciinema.org/a/49402)

**Features:**

* Easily build applications with integrated service discovery and persistent storage.
* Quickly and easily scale applications to handle periods of increased demand.
  * Support for automatic high availability, load balancing, health checking, and failover.
* Push source code to your Git repository and automatically deploy containerized applications.
* Web console and command-line client for building and monitoring applications.
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
* The **[Trello Roadmap](https://ci.openshift.redhat.com/roadmap_overview.html)** covers the epics and stories being worked on (click through to individual items)

For questions or feedback, reach us on [IRC on #openshift-dev](https://botbot.me/freenode/openshift-dev/) on Freenode or post to our [mailing list](https://lists.openshift.redhat.com/openshiftmm/listinfo/dev).

Getting Started
---------------

### Installation

If you have downloaded the client tools, place the included binaries in your PATH.

* For a quick install of Origin, see the [Getting Started Install guide](https://docs.openshift.org/latest/getting_started/administrators.html).
* For an advanced installation using [Ansible](https://github.com/openshift/openshift-ansible), follow the [Advanced Installation guide](https://docs.openshift.org/latest/install_config/install/advanced_install.html)
* To build and run from source, see [CONTRIBUTING.adoc](CONTRIBUTING.adoc)

### Concepts

We highly recommend trying out the [Origin walkthrough](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md) which covers the core concepts in Origin.  The walkthrough is accompanied by a blog series on [blog.openshift.com](https://blog.openshift.com/openshift-v3-deep-dive-docker-kubernetes/) that goes into more detail.  It's a great place to start.

### Origin API

The Origin API is located on each server at `https://<host>:8443/oapi/v1`. These APIs are described via [Swagger v1.2](https://www.swagger.io) at `https://<host>:8443/swaggerapi/oapi/v1`. For more, [see the API documentation](https://docs.openshift.org/latest/rest_api/openshift_v1.html).

### Kubernetes

If you're looking for more information about using Kubernetes or the lower level concepts that Origin depends on, see the following:

* [Kubernetes Getting Started](https://github.com/kubernetes/kubernetes/blob/master/README.md)
* [Kubernetes Documentation](https://github.com/kubernetes/kubernetes/blob/master/docs/README.md)
* [Kubernetes API](https://docs.openshift.org/latest/rest_api/kubernetes_v1.html)

### Troubleshooting

If you run into difficulties running Origin, start by reading through the [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md).


FAQ
---

1. How does Origin relate to Kubernetes?

    Origin is a distribution of Kubernetes optimized for enterprise application development and deployment,
    used by OpenShift 3 and Atomic Enterprise.  Origin embeds Kubernetes and adds additional
    functionality to offer a simple, powerful, and easy-to-approach developer and operator experience
    for building applications in containers.  Our goal is to do most of that work upstream, with
    integration and final packaging occurring in Origin.

    You can run the core Kubernetes server components with `openshift start kubernetes`, use `kubectl` via
    `openshift kube`, and the Origin release zips include versions of `kubectl`, `kubelet`,
    `kube-apiserver`, and other core components. You can see the version of Kubernetes included with
    Origin via `openshift version`.

2. How does Atomic Enterprise relate to Origin and OpenShift?

    Two products are built from Origin, Atomic Enterprise and OpenShift. Atomic Enterprise adds
    operational centric tools to enable easy deployment and scaling and long-term lifecycle
    maintenance for small and large teams and applications. OpenShift provides a number of
    developer-focused tools on top of Atomic Enterprise such as image building, management, and
    enhanced deployment flows.

3. What can I run on Origin?

    Origin is designed to run any existing Docker images.  Additionally, you can define builds that will produce new Docker images using a `Dockerfile`.

    However, the real magic of Origin is [Source-to-Image (S2I)](https://github.com/openshift/source-to-image) builds, which allow developers to simply provide an application source repository containing code to build and run.  It works by combining an existing S2I-enabled Docker image with application source to produce a new runnable image for your application.

    We are continuing to grow the [ecosystem of Source-to-Image builder images](https://docs.openshift.org/latest/using_images/s2i_images/overview.html) and it's straightforward to [create your own](https://blog.openshift.com/create-s2i-builder-image/).  Some of our available images include:

    * [Ruby](https://github.com/sclorg/s2i-ruby-container)
    * [Python](https://github.com/sclorg/s2i-python-container)
    * [Node.js](https://github.com/sclorg/s2i-nodejs-container)
    * [PHP](https://github.com/sclorg/s2i-php-container)
    * [Perl](https://github.com/sclorg/s2i-perl-container)
    * [WildFly](https://github.com/openshift-s2i/s2i-wildfly)

    Your application image can be easily extended with a database service with our [database images](https://docs.openshift.org/latest/using_images/db_images/overview.html):

    * [MySQL](https://github.com/sclorg/mysql-container)
    * [MongoDB](https://github.com/sclorg/mongodb-container)
    * [PostgreSQL](https://github.com/sclorg/postgresql-container)

4. Why doesn't my Docker image run on OpenShift?

    Security! Origin runs with the following security policy by default:

    * Containers run as a non-root unique user that is separate from other system users
      * They cannot access host resources, run privileged, or become root
      * They are given CPU and memory limits defined by the system administrator
      * Any persistent storage they access will be under a unique SELinux label, which prevents others from seeing their content
      * These settings are per project, so containers in different projects cannot see each other by default
    * Regular users can run Docker, source, and custom builds
      * By default, Docker builds can (and often do) run as root. You can control who can create Docker builds through the `builds/docker` and `builds/custom` policy resource.
    * Regular users and project admins cannot change their security quotas.

    Many Docker containers expect to run as root (and therefore edit all the contents of the filesystem). The [Image Author's guide](https://docs.openshift.org/latest/creating_images/guidelines.html#openshift-specific-guidelines) gives recommendations on making your image more secure by default:

    * Don't run as root
    * Make directories you want to write to group-writable and owned by group id 0
    * Set the net-bind capability on your executables if they need to bind to ports &lt;1024

    Otherwise, you can see the [security documentation](https://docs.openshift.org/latest/admin_guide/manage_scc.html) for descriptions on how to relax these restrictions.

5. How do I get networking working?

    The Origin and Kubernetes network model assigns each pod (group of containers) an IP that is expected to be reachable from all nodes in the cluster. The default setup is through a simple SDN plugin with OVS - this plugin expects the port 4679 to be open between nodes in the cluster. Also, the Origin master processes need to be able to reach pods via the network, so they may require the SDN plugin.

    Other networking options are available such as Calico, Flannel, Nuage, and Weave. For a non-overlay networking solution, existing networks can be used by assigning a different subnet to each host, and ensuring routing rules deliver packets bound for that subnet to the host it belongs to. This is called [host subnet routing](https://docs.openshift.org/latest/install_config/configuring_routing.html#install-config-configuring-native-container-routing).

6. Why can't I run Origin in a Docker image on boot2docker or Ubuntu?

    Versions of Docker distributed by the Docker team don't allow containers to mount volumes on the host and write to them (mount propagation is private). Kubernetes manages volumes and uses them to expose secrets into containers, which Origin uses to give containers the tokens they need to access the API and run deployments and builds. Until mount propagation is configurable in Docker you must use Docker on Fedora, CentOS, or RHEL (which have a patch to allow mount propagation) or run Origin outside of a container. Tracked in [this issue](https://github.com/openshift/origin/issues/3072).


Alpha and Unsupported Kubernetes Features
-----------------------------------------

Some features from upstream Kubernetes are not yet enabled in Origin, for reasons including supportability, security, or limitations in the upstream feature.

Kubernetes Definitions:

* Alpha
  * The feature is available, but no guarantees are made about backwards compatibility or whether data is preserved when feature moves to Beta.
  * The feature may have significant bugs and is suitable for testing and prototyping.
  * The feature may be replaced or significantly redesigned in the future.
  * No migration to Beta is generally provided other than documentation of the change.
* Beta
  * The feature is available and generally agreed to solve the desired solution, but may need stabilization or additional feedback.
  * The feature is potentially suitable for limited production use under constrained circumstances.
  * The feature is unlikely to be replaced or removed, although it is still possible for feature changes that require migration.

OpenShift uses these terms in the same fashion as Kubernetes, and adds four more:

* Not Yet Secure
  * Features which are not yet enabled because they have significant security or stability risks to the cluster
  * Generally this applies to features which may allow escalation or denial-of-service behavior on the platform
  * In some cases this is applied to new features which have not had time for full security review
* Potentially Insecure
  * Features that require additional work to be properly secured in a multi-user environment
  * These features are only enabled for cluster admins by default and we do not recommend enabling them for untrusted users
  * We generally try to identify and fix these within 1 release of their availability
* Tech Preview
  * Features that are considered unsupported for various reasons are known as 'tech preview' in our documentation
  * Kubernetes Alpha and Beta features are considered tech preview, although occasionally some features will be graduated early
  * Any tech preview feature is not supported in OpenShift Container Platform except through exemption
* Disabled Pending Migration
  * These are features that are new in Kubernetes but which originated in OpenShift, and thus need migrations for existing users
  * We generally try to minimize the impact of features introduced upstream to Kubernetes on OpenShift users by providing seamless
    migration for existing clusters.
  * Generally these are addressed within 1 Kubernetes release

The list of features that qualify under these labels is described below, along with additional context for why.

Feature | Kubernetes | OpenShift | Justification
------- | ---------- | --------- | -------------
Third Party Resources | Alpha (1.3) | Not Yet Secure (1.2, 1.3) | Third party resources are still under active development upstream.<br>Known issues include failure to clean up resources in etcd, which may result in a denial of service attack against the cluster.<br>We are considering enabling them for development environments only.
Garbage Collection | Alpha (1.3) | Not Yet Secure (1.3) | Garbage collection will automatically delete related resources on the server, and thus given the potential for data loss we are waiting for GC to graduate to beta and have a full release cycle of testing before enabling it in Origin.<br>At the current time, it is possible for a malicious user to trick another user into deleting a sensitive resource (like a quota or limit resource) during deletion, which must be addressed prior to enablement.
Pet Sets | Alpha (1.3) | Tech Preview (1.3) | Pet Sets are still being actively developed and no backwards compatibility is guaranteed. Also, Pet Sets allow users to create PVCs indirectly, and more security controls are needed to limit the potential impact on the cluster.
Init Containers | Alpha (1.3) | Tech Preview (1.3) | Init containers are properly secured, but are not officially part of the Kubernetes API and may change without notice.
Federated Clusters | Beta (1.3) | Tech Preview (1.3) | A Kubernetes federation server may be used against Origin clusters with the appropriate credentials today.<br>Known issues include tenant support in federation and the ability to have consistent access control between federation and normal clusters.<br>No Origin specific binary is being distributed for federation at this time.
Deployment | Alpha (1.2)<br>Beta (1.3) | Disabled Pending Migration (1.2)<br>Tech Preview (1.3) | OpenShift launched with DeploymentConfigs, a more fully featured Deployment object. The upstream Deployments are enabled in OpenShift as a tech preview so users can create both Deployments and DeploymentConfigs. We plan to provide migration from Deployment Config to Deployment in future when the feature parity with the upstream Deployment object is reached. <br>Deployment Configs are currently a superset of Deployment features.
Replica Sets | Beta (1.2)<br>Beta (1.3) | Disabled Pending Migration (1.2)<br>Tech Preview (1.3) | Replica Sets perform the same function as Replication Controllers, but have a more powerful label syntax. We are working upstream to enable a migration path forward for clusters with existing Replication Controllers deployed to be automatically migratable to Replica Sets, in order to ease the transition for clients and tooling that depend on RCs.
Ingress | Alpha (1.1)<br>Beta (1.2, 1.3) | Disabled Pending Migration (1.2)<br>Tech Preview (1.3) | OpenShift launched with Routes, a more full featured Ingress object. The upstream Ingress are enabled in OpenShift as a tech preview, so users can create both Ingress and Route resources. We plan to provide migration from Route to Ingress in future when the feature parity with the upstream Ingress is reached. <br>Routes are currently a superset of Ingress features.
PodSecurityPolicy | Alpha (1.2)<br>Beta (1.3) | Disabled Pending Migration (1.3)<br>Not Yet Secure (1.3) | OpenShift launched with SecurityContextConstraints, and then upstreamed them as PodSecurityPolicy. We plan to enable upstream PodSecurityPolicy so as to automatically migrate existing SecurityContextConstraints. PodSecurityPolicy has not yet completed a full security review, which will be part of the criteria for tech preview. <br>SecurityContextConstraints are a superset of PodSecurityPolicy features.
PodAntiAffinitySelectors | Alpha (1.3) | Not Yet Secure (1.3)<br>Tech Preview (1.4?) | End users are not allowed to set PodAntiAffinitySelectors that are not the node name due to the possibility of attacking the scheduler via denial of service.|
NetworkPolicy | Beta (1.3) | Tech Preview (1.3) | OpenShift's default network plugins (`redhat/openshift-ovs-subnet` and `redhat/openshift-ovs-multitenant`) do not support NetworkPolicy yet, but if you use a third-party network plugin, it might.

Please contact us if this list omits a feature supported in Kubernetes which does not run in Origin.


Contributing
------------

You can develop [locally on your host](CONTRIBUTING.adoc#develop-locally-on-your-host) or with a [virtual machine](CONTRIBUTING.adoc#develop-on-virtual-machine-using-vagrant), or if you want to just try out Origin [download the latest Linux server, or Windows and Mac OS X client pre-built binaries](CONTRIBUTING.adoc#download-from-github).

First, **get up and running with the** [**Contributing Guide**](CONTRIBUTING.adoc).

All contributions are welcome - Origin uses the Apache 2 license and does not require any contributor agreement to submit patches.  Please open issues for any bugs or problems you encounter, ask questions on the OpenShift IRC channel (#openshift-dev on freenode), or get involved in the [Kubernetes project](https://github.com/kubernetes/kubernetes) at the container runtime layer.

See [HACKING.md](https://github.com/openshift/origin/blob/master/HACKING.md) for more details on developing on Origin including how different tests are setup.

If you want to run the test suite, make sure you have your environment set up, and from the `origin` directory run:

```
# run the verifiers, unit tests, and command tests
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

Some of the components of Origin run as Docker images, including the builders and deployment tools in `images/builder/docker/*` and 'images/deploy/*`.  To build them locally run

```
$ hack/build-images.sh
```

To hack on the web console, check out the [assets/README.md](assets/README.md) file for instructions on testing the console and building your changes.


License
-------

Origin is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).
