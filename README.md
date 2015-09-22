OpenShift Application Platform
==============================

[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Travis](https://travis-ci.org/openshift/origin.svg?branch=master)](https://travis-ci.org/openshift/origin)

This is the source repository for [OpenShift Origin](https://openshift.github.io), based on top of
[Docker](https://www.docker.io) containers and the
[Kubernetes](https://github.com/kubernetes/kubernetes) container cluster manager.
Origin is a distribution of Kubernetes optimized for enterprise application development and deployment,
used by OpenShift 3 and Atomic Enterprise.  Origin adds developer and operational centric tools
on top of Kubernetes to enable rapid application development,
easy deployment and scaling, and long-term lifecycle maintenance for small and large teams and applications.

**Features:**

* Build web-scale applications with integrated service discovery, DNS, load balancing, failover, health checking, persistent storage, and fast scaling
* Push source code to your Git repository and have image builds and deployments automatically occur
* Easy to use client tools for building web applications from source code
  * Templatize the components of your system, reuse them, and iteratively deploy them over time
* Centralized administration and management of application component libraries
  * Roll out changes to software stacks to your entire organization in a controlled fashion
* Team and user isolation of containers, builds, and network communication in an easy multi-tenancy system
  * Allow developers to run containers securely by preventing root access and isolating containers with SELinux
  * Limit, track, and manage the resources teams are using

**Learn More:**

* **[Public Documentation](http://docs.openshift.org/latest/welcome/index.html)**
* The **[Trello Roadmap](https://ci.openshift.redhat.com/roadmap_overview.html)** covers the epics and stories being worked on (click through to individual items)
* **[Technical Architecture Presentation](https://docs.google.com/presentation/d/1Isp5UeQZTo3gh6e59FMYmMs_V9QIQeBelmbyHIJ1H_g/pub?start=false&loop=false&delayms=3000)**
* **[System Architecture](https://github.com/openshift/openshift-pep/blob/master/openshift-pep-013-openshift-3.md)** design document
* **[API Documentation](http://docs.openshift.org/latest/rest_api/openshift_v1.html)**

For questions or feedback, reach us on [IRC on #openshift-dev](https://botbot.me/freenode/openshift-dev/) on Freenode or post to our [mailing list](https://lists.openshift.redhat.com/openshiftmm/listinfo/dev).


Getting Started
---------------

### Installation

* For a quick install of Origin, see the [Getting Started Install guide](https://docs.openshift.org/latest/getting_started/administrators.html).
* For an advanced installation using [Ansible](https://github.com/openshift/openshift-ansible), follow the [Advanced Installation guide](https://docs.openshift.org/latest/install_config/install/advanced_install.html)
* To build and run from source, see [CONTRIBUTING.adoc](CONTRIBUTING.adoc)

### Concepts

We highly recommend trying out the [Origin walkthrough](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md) which covers the core concepts in Origin.  The walkthrough is accompanied by a blog series on [blog.openshift.com](https://blog.openshift.com/openshift-v3-deep-dive-docker-kubernetes/) that goes into more detail.  It's a great place to start.

### Origin API

The Origin API is located on each server at `https://<host>:8443/oapi/v1`. These APIs are described via [Swagger v1.2](https://www.swagger.io) at `https://<host>:8443/swaggerapi/oapi/v1`. For more, [see the API documentation](http://docs.openshift.org/latest/rest_api/openshift_v1.html).

### Kubernetes

If you're looking for more information about using Kubernetes or the lower level concepts that Origin depends on, see the following:

* [Kubernetes Getting Started](https://github.com/kubernetes/kubernetes/blob/master/README.md)
* [Kubernetes Documentation](https://github.com/kubernetes/kubernetes/blob/master/docs/README.md)
* [Kubernetes API](http://docs.openshift.org/latest/rest_api/kubernetes_v1.html)

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

    You can run the core Kubernetes server components with `openshift start kube`, use `kubectl` via
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

    Origin is designed to run any existing Docker images.  In addition you can define builds that will produce new Docker images from a Dockerfile.  However the real magic of Origin can be seen when using [Source-To-Image](https://github.com/openshift/source-to-image) builds which allow you to simply supply an application source repository which will be combined with an existing Source-To-Image enabled Docker image to produce a new runnable image that runs your application.  We are continuing to grow the ecosystem of Source-To-Image enabled images and documenting them [here](http://docs.openshift.org/latest/using_images/s2i_images/overview.html). Our available images are:

    * [Ruby](https://github.com/openshift/sti-ruby)
    * [Python](https://github.com/openshift/sti-python)
    * [NodeJS](https://github.com/openshift/sti-nodejs)
    * [PHP](https://github.com/openshift/sti-php)
    * [Perl](https://github.com/openshift/sti-perl)
    * [Wildfly](https://github.com/openshift/wildfly-8-centos)

    Your application image can be easily extended with a database service with our [database images](http://docs.openshift.org/latest/using_images/db_images/overview.html). Our available database images are:

    * [MySQL](https://github.com/openshift/mysql)
    * [MongoDB](https://github.com/openshift/mongodb)
    * [PostgreSQL](https://github.com/openshift/postgresql)

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

    Other networking options are available such as Calico, Flannel, Nuage, and Weave. For a non-overlay networking solution, existing networks can be used by assigning a different subnet to each host, and ensuring routing rules deliver packets bound for that subnet to the host it belongs to. This is called [host subnet routing](https://docs.openshift.org/latest/admin_guide/native_container_routing.html).

6. Why can't I run Origin in a Docker image on boot2docker or Ubuntu?

    Versions of Docker distributed by the Docker team don't allow containers to mount volumes on the host and write to them (mount propagation is private). Kubernetes manages volumes and uses them to expose secrets into containers, which Origin uses to give containers the tokens they need to access the API and run deployments and builds. Until mount propagation is configurable in Docker you must use Docker on Fedora, CentOS, or RHEL (which have a patch to allow mount propagation) or run Origin outside of a container. Tracked in [this issue](https://github.com/openshift/origin/issues/3072).


Contributing
------------

You can develop [locally on your host](CONTRIBUTING.adoc#develop-locally-on-your-host) or with a [virtual machine](CONTRIBUTING.adoc#develop-on-virtual-machine-using-vagrant), or if you want to just try out Origin [download the latest Linux server, or Windows and Mac OS X client pre-built binaries](CONTRIBUTING.adoc#download-from-github).

First, **get up and running with the** [**Contributing Guide**](CONTRIBUTING.adoc).

All contributions are welcome - Origin uses the Apache 2 license and does not require any contributor agreement to submit patches.  Please open issues for any bugs or problems you encounter, ask questions on the OpenShift IRC channel (#openshift-dev on freenode), or get involved in the [Kubernetes project](https://github.com/kubernetes/kubernetes) at the container runtime layer.

See [HACKING.md](https://github.com/openshift/origin/blob/master/HACKING.md) for more details on developing on Origin including how different tests are setup.

If you want to run the test suite, make sure you have your environment set up, and from the `origin` directory run:

```
# run the unit tests
$ make check

# run a simple server integration test
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
