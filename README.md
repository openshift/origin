OpenShift 3 Alpha
=================

This is the source repository for the next version of OpenShift - the third architectural revision.
It is based around [Docker](https://www.docker.io) containers and images and the
[Kubernetes](https://github.com/GoogleCloudPlatform/kubernetes) container management solution.
OpenShift adds developer centric and organization centric workflows on top of Kubernetes, and much
of the core functionality of OpenShift is designed as plugins to the core Kubernetes concepts.

Please see the [OpenShift 3 Project Enhancement Proposal (PEP)](https://github.com/openshift/openshift-pep/blob/master/openshift-pep-013-openshift-3.md) for a deeper discussion of the features you see here.

NOTE: OpenShift is in alpha and is not intended for production use. However we welcome feedback and testing as we approach our first beta.

[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Travis](https://travis-ci.org/openshift/origin.svg?branch=master)](https://travis-ci.org/openshift/origin)

Getting Started
---------------
The simplest way to start is to run OpenShift Origin in a Docker container:

    $ docker run -v /var/run/docker.sock:/var/run/docker.sock --net=host --privileged openshift/origin start

Note that this won't hold any data after a restart, so you'll need to use a data container or mount a volume at `/var/lib/openshift` to preserve that data.  Once the container is started, run:

    $ docker run openshift/origin kube --help

to see the command line options you can use.

### Start Developing

You can develop [locally on your host](CONTRIBUTING.adoc#develop-locally-on-your-host) or with a [virtual machine](CONTRIBUTING.adoc#develop-on-virtual-machine-using-vagrant), or if you want to just try out OpenShift [download the latest Linux server, or Windows and Mac OS X client pre-built binary](CONTRIBUTING.adoc#download-from-github).

First, **get up and running with the** [**Contributing Guide**](CONTRIBUTING.adoc).

Once setup, you can:

1.  Run a build

        $ hack/build-go.sh

2.  Start an OpenShift all-in-one server (includes everything you need to try OpenShift)

        $ _output/local/go/bin/openshift start

3.  In another terminal window, switch to the directory and start an app:

        $ cd $GOPATH/src/github.com/openshift/origin
        $ _output/local/go/bin/openshift kube create pods -c examples/hello-openshift/hello-pod.json

Once that's done, open a browser on your machine and open [http://localhost:6061](http://localhost:6061); you should see a 'Welcome to OpenShift' message.

### How Does This Work?

This example runs the ['openshift/hello-openshift' Docker image](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello-pod.json#L11) inside a Docker container, but managed by OpenShift and Kubernetes.

* At the Docker level, that image [binds to port 8080](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello_openshift.go#L16) within a container and [prints out a simple 'Hello OpenShift' message on access](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello_openshift.go#L9).
* At the Kubernetes level, we [map that bound port in the container](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello-pod.json#L13) [to port 6061 on the host](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello-pod.json#L14) so that we can access it via the host browser.
* When you created the container, Kubernetes decided which host to place the container on by looking at the available hosts and selecting one with available space.  The agent that runs on each node (part of the OpenShift all-in-one binary, called the Kubelet) saw that it was now supposed to run the container and instructed Docker to start the container.

OpenShift brings all of these pieces (and a client) together in a single, easy to use binary.  The following examples show the other OpenShift specific features that live above the Kubernetes runtime like image building and deployment flows.

### Other Examples

* [OpenShift full walkthrough](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md)
* [Kubernetes walkthrough](https://github.com/GoogleCloudPlatform/kubernetes/tree/master/examples/walkthrough)
* [Kubernetes guestbook](https://github.com/GoogleCloudPlatform/kubernetes/tree/master/examples/guestbook)
* [OpenShift guestbook template](https://github.com/openshift/origin/blob/master/examples/guestbook) takes the previous example and templatizes it

Remember, you can pass a URL to `-c` when using the `kube` command, so you can [download the latest
release](CONTRIBUTING.adoc#download-from-github) and pass a URL to the content on GitHub so you
don't even need clone the source.

### Docker registry

In order to use an image built from an OpenShift build, you'll need to push that image into a Docker registry.
You can use a private [Docker registry](https://github.com/docker/docker-registry) or the [DockerHub](https://hub.docker.com/).

#### Private docker registry

To setup private docker registry you can either follow the [registry quick-start](https://github.com/docker/docker-registry#quick-start)
or use [sample-app example](https://github.com/openshift/origin/blob/master/examples/sample-app) to host a registry on OpenShift. In your `buildConfig` you should pass the fully qualified registry name of the image you want to push `myregistry.com:8080/username/imagename`.

#### DockerHub

To push images to the DockerHub you need to login using `docker login` command. This command will create a file named `.dockercfg` in your home directory containing your Hub credentials. If you're running the OpenShift all-in-one as a different user, you'll need to copy this file into that other user's home directory. When the build completes this file will be read by Docker, and the credentials inside of it will be used to push your image.

NOTE: You must tag your built image as `<username-for-credentials>/<imagename>` when using the DockerHub.

Design Documents
----------------

OpenShift designs:

* [OpenShift 3 PEP](https://github.com/openshift/openshift-pep/blob/master/openshift-pep-013-openshift-3.md)
* [Orchestration Overview](https://github.com/openshift/origin/blob/master/docs/orchestration.md)

Kubernetes designs are in [the Kubernetes docs dir](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/)

API
---

The OpenShift APIs are exposed at `http://localhost:8080/osapi/v1beta1/*`.

* Builds
 * `http://localhost:8080/osapi/v1beta1/builds`
 * `http://localhost:8080/osapi/v1beta1/buildConfigs`
 * `http://localhost:8080/osapi/v1beta1/buildLogs`
 * `http://localhost:8080/osapi/v1beta1/buildConfigHooks`
* Deployments
 * `http://localhost:8080/osapi/v1beta1/deployments`
 * `http://localhost:8080/osapi/v1beta1/deploymentConfigs`
* Images
 * `http://localhost:8080/osapi/v1beta1/images`
 * `http://localhost:8080/osapi/v1beta1/imageRepositories`
 * `http://localhost:8080/osapi/v1beta1/imageRepositoryMappings`
* Templates
 * `http://localhost:8080/osapi/v1beta1/templateConfigs`
* Routes
 * `http://localhost:8080/osapi/v1beta1/routes`
* Projects
 * `http://localhost:8080/osapi/v1beta1/projects`
* Users
 * `http://localhost:8080/osapi/v1beta1/users`
 * `http://localhost:8080/osapi/v1beta1/userIdentityMappings`
* OAuth
 * `http://localhost:8080/osapi/v1beta1/accessTokens`
 * `http://localhost:8080/osapi/v1beta1/authorizeTokens`
 * `http://localhost:8080/osapi/v1beta1/clients`
 * `http://localhost:8080/osapi/v1beta1/clientAuthorizations`

The Kubernetes APIs are exposed at `http://localhost:8080/api/v1beta2/*`:

* `http://localhost:8080/api/v1beta2/pods`
* `http://localhost:8080/api/v1beta2/services`
* `http://localhost:8080/api/v1beta2/replicationControllers`
* `http://localhost:8080/api/v1beta2/operations`

A draft of the proposed API is available at http://rawgit.com/openshift/origin/master/api/openshift3.html and is developed under the [api](./api) directory.  Expect significant changes.


FAQ
---

1. How does OpenShift relate to Kubernetes?

    OpenShift embeds Kubernetes and adds additional functionality to offer a simple, powerful, and
    easy-to-approach developer and operator experience for building applications in containers.
    Kubernetes today is focused around composing containerized applications - OpenShift adds
    building images, managing them, and integrating them into deployment flows.  Our goal is to do
    most of that work upstream, with integration and final packaging occurring in OpenShift.  As we
    iterate through the next few months, you'll see this repository focus more on integration and
    plugins, with more and more features becoming part of Kubernetes.

    OpenShift tracks the Kubernetes upstream at
    [github.com/openshift/kubernetes](https://github.com/openshift/kubernetes).  See the wiki in
    that project for more on how we manage the process of integrating prototyped features.

2. What about [geard](https://github.com/openshift/geard)?

    Geard started as a prototype vehicle for the next generation of the OpenShift node - as an
    orchestration endpoint, to offer integration with systemd, and to prototype network abstraction,
    routing, SSH access to containers, and Git hosting.  Its intended goal is to provide a simple
    way of reliably managing containers at scale, and to offer administrators tools for easily
    composing those applications (gear deploy).

    With the introduction of Kubernetes, the Kubelet, and the pull model it leverages from etcd, we
    believe we can implement the pull-orchestration model described in
    [orchestrating geard](https://github.com/openshift/geard/blob/master/docs/orchestrating_geard.md),
    especially now that we have a path to properly
    [limit host compromises from affecting the cluster](https://github.com/GoogleCloudPlatform/kubernetes/pull/860).  
    The pull-model has many advantages for end clients, not least of which that they are guaranteed
    to eventually converge to the correct state of the server. We expect that the use cases the geard
    endpoint offered will be merged into the Kubelet for consumption by admins.

    systemd and Docker integration offers efficient and clean process management and secure logging
    aggregation with the system.  We plan on introducing those capabilities into Kubernetes over
    time, especially as we work with the Docker upstream to limit the impact of the Docker daemon's
    parent child process relationship with containers, where death of the Docker daemon terminates
    the containers under it

    Network links and their ability to simplify how software connects to other containers is planned
    for Docker links v2 and is a capability we believe will be important in Kubernetes as well ([see issue 494 for more details](https://github.com/GoogleCloudPlatform/kubernetes/issues/494)).

    The geard deployment descriptor describes containers and their relationships and will be mapped
    to deployment on top of Kubernetes.  The geard commandline itself will likely be merged directly
    into the `openshift` command for all-in-one management of a cluster.


Contributing
------------

All contributions are welcome - OpenShift uses the Apache 2 license and does not require any contributor agreement to submit patches.  Please open issues for any bugs or problems you encounter, ask questions on the OpenShift IRC channel (#openshift-dev on freenode), or get involved in the [Kubernetes project](https://github.com/GoogleCloudPlatform/kubernetes) at the container runtime layer.

See [HACKING.md](https://github.com/openshift/origin/blob/master/HACKING.md) for more details on
developing on OpenShift including how different tests are setup.

If you want to run the test suite, make sure you have your environment from above set up, and from
the origin directory run:

```
# run the unit tests
$ hack/test-go.sh

# run a simple server integration test
$ hack/test-cmd.sh

# run the integration server test suite
$ hack/test-integration.sh
```

You'll need [etcd](https://github.com/coreos/etcd) installed and on your path for the last step to run.  To install etcd you should be able to run:

```
$ hack/install-etcd.sh
```

Some of the components of OpenShift run as Docker images, including the builders and deployment tools in `images/builder/docker/*` and 'images/deploy/*`.  To build them locally run

```
$ hack/build-images.sh
```


License
-------

OpenShift is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).
