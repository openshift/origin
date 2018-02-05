OpenShift Application Platform
==============================

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/origin)](https://goreportcard.com/report/github.com/openshift/origin)
[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Travis](https://travis-ci.org/openshift/origin.svg?branch=master)](https://travis-ci.org/openshift/origin)
[![Jenkins](https://ci.openshift.redhat.com/jenkins/buildStatus/icon?job=devenv_ami)](https://ci.openshift.redhat.com/jenkins/job/devenv_ami/)
[![Join the chat at freenode:openshift-dev](https://img.shields.io/badge/irc-freenode%3A%20%23openshift--dev-blue.svg)](http://webchat.freenode.net/?channels=%23openshift-dev)
[![Licensed under Apache License version 2.0](https://img.shields.io/github/license/openshift/origin.svg?maxAge=2592000)](https://www.apache.org/licenses/LICENSE-2.0)

***OpenShift Origin*** is a distribution of [Kubernetes](https://kubernetes.io) optimized for continuous application development and multi-tenant deployment.  OpenShift adds developer and operations-centric tools on top of Kubernetes to enable rapid application development, easy deployment and scaling, and long-term lifecycle maintenance for small and large teams.

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
* Our **[Trello Roadmap](https://ci.openshift.redhat.com/roadmap_overview.html)** covers the epics and stories being worked on (click through to individual items)

For questions or feedback, reach us on [IRC on #openshift-dev](https://botbot.me/freenode/openshift-dev/) on Freenode or post to our [mailing list](https://lists.openshift.redhat.com/openshiftmm/listinfo/dev).

Getting Started
---------------

### Installation

If you have downloaded the client tools from the [releases page](https://github.com/openshift/origin/releases), place the included binaries in your PATH.

* On any system with a Docker engine installed, you can run `oc cluster up` to get started immediately.  Try it out now!
* For a full cluster installation using [Ansible](https://github.com/openshift/openshift-ansible), follow the [Advanced Installation guide](https://docs.openshift.org/latest/install_config/install/advanced_install.html)
* To build and run from source, see [CONTRIBUTING.adoc](CONTRIBUTING.adoc)

The latest OpenShift Origin images are published to the Docker Hub under the `openshift` account at https://hub.docker.com/u/openshift/. We use a rolling tag system as of v3.9, where the `:latest` tag always points to the most recent alpha release on `master`, the `v3.X` tag points to the most recent build for that release (pre-release and post-release), and `v3.X.Y` is a stable tag for patches to a release.

### Concepts

OpenShift builds a developer-centric workflow around Docker containers and Kubernetes runtime concepts.  An **Image Stream** lets you easily tag, import, and publish Docker images from the integrated registry.  A **Build Config** allows you to launch Docker builds, build directly from source code, or trigger Jenkins Pipeline jobs whenever an image stream tag is updated.  A **Deployment Config** allows you to use custom deployment logic to rollout your application, and Kubernetes workflow objects like **DaemonSets**, **Deployments**, or **StatefulSets** are upgraded to automatically trigger when new images are available.  **Routes** make it trivial to expose your Kubernetes services via a public DNS name. As an administrator, you can enable your developers to request new **Projects** which come with predefined roles, quotas, and security controls to fairly divide access.

For more on the underlying concepts of OpenShift, please see the [documentation site](https://docs.openshift.org/latest/welcome/index.html).

### OpenShift API

The OpenShift API is located on each server at `https://<host>:8443/apis`. OpenShift adds its own API groups alongside the Kubernetes APIs. For more, [see the API documentation](https://docs.openshift.org/latest/rest_api).

### Kubernetes

OpenShift extends Kubernetes with security and other developer centric concepts.  Each OpenShift Origin release ships slightly after the Kubernetes release has stabilized. Version numbers are aligned - OpenShift v3.9 is Kubernetes v1.9.

If you're looking for more information about using Kubernetes or the lower level concepts that Origin depends on, see the following:

* [Kubernetes Getting Started](https://kubernetes.io/docs/tutorials/kubernetes-basics/)
* [Kubernetes Documentation](https://kubernetes.io/docs/)
* [Kubernetes API](https://docs.openshift.org/latest/rest_api)


### What can I run on OpenShift?

OpenShift is designed to run any existing Docker images.  Additionally, you can define builds that will produce new Docker images using a `Dockerfile`.

For an easier experience running your source code, [Source-to-Image (S2I)](https://github.com/openshift/source-to-image) allows developers to simply provide an application source repository containing code to build and run.  It works by combining an existing S2I-enabled Docker image with application source to produce a new runnable image for your application.

You can see the [full list of Source-to-Image builder images](https://docs.openshift.org/latest/using_images/s2i_images/overview.html) and it's straightforward to [create your own](https://blog.openshift.com/create-s2i-builder-image/).  Some of our available images include:

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

### What sorts of security controls does OpenShift provide for containers?

OpenShift runs with the following security policy by default:

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
    * Set the net-bind capability on your executables if they need to bind to ports < 1024

If you are running your own cluster and want to run a container as root, you can grant that permission to the containers in your current project with the following command:

    # Gives the default service account in the current project access to run as UID 0 (root)
    oc adm add-scc-to-user anyuid -z default 

See the [security documentation](https://docs.openshift.org/latest/admin_guide/manage_scc.html) more on confining applications.


Support for Kubernetes Alpha Features
-----------------------------------------

Some features from upstream Kubernetes are not yet enabled in OpenShift, for reasons including supportability, security, or limitations in the upstream feature.

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
Custom Resource Definitions | GA (1.9) | GA (3.9) | 
Stateful Sets | GA (1.9) | GA (3.9) |
Deployment | GA (1.9) | GA (1.9) |
Replica Sets | GA (1.9) | GA (3.9) | Replica Sets perform the same function as Replication Controllers, but have a more powerful label syntax. Both ReplicationControllers and ReplicaSets can be used.  
Ingress | Beta (1.9) | Tech Preview (3.9) | OpenShift launched with Routes, a more full featured Ingress object. Ingress rules can be read by the router (disabled by default), but because Ingress objects reference secrets you must grant the routers access to your secrets manually.  Ingress is still beta in upstream Kubernetes.
PodSecurityPolicy | Beta (1.9) | Tech Preview (3.9) | OpenShift launched with SecurityContextConstraints, and then upstreamed them as PodSecurityPolicy. We plan to enable upstream PodSecurityPolicy so as to automatically migrate existing SecurityContextConstraints. PodSecurityPolicy has not yet completed a full security review, which will be part of the criteria for tech preview. <br>SecurityContextConstraints are a superset of PodSecurityPolicy features.
NetworkPolicy | GA (1.6) | GA (3.7) |

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

Some of the components of Origin run as Docker images, including the builders and deployment tools in `images/builder/docker/*` and `images/deploy/*`.  To build them locally run

```
$ hack/build-images.sh
```

To hack on the web console, check out the [assets/README.md](assets/README.md) file for instructions on testing the console and building your changes.

Security Response
-----------------
If you've found a security issue that you'd like to disclose confidentially
please contact Red Hat's Product Security team. Details at
https://access.redhat.com/security/team/contact


License
-------

OpenShift is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).
