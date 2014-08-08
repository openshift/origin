OpenShift Origin 3.0
====================

This is the source repository for the next version of OpenShift - the third architectural revision.  It is based around [Docker](https://www.docker.io) containers and images and the [Kubernetes](https://github.com/GoogleCloudPlatform/kubernetes) container management solution.  OpenShift adds developer  centric and organization centric workflows on top of Kubernetes, and much of the core functionality of OpenShift is designed as plugins to the core Kubernetes concepts.

Please see the [OpenShift 3 Project Enhancement Proposal (PEP)](https://github.com/openshift/openshift-pep/blob/master/openshift-pep-013-openshift-3.md) for a deeper discussion of the features you see here.

NOTE: This is a very early prototype, and as such is designed for rapid iteration around core concepts.

Getting Started
---------------

You'll need Docker and the Go language compilation tools installed.

1.  [Install Docker](https://docs.docker.com/installation/#installation)
2.  [Install the Go language toolkit](http://golang.org/doc/install) and set your GOPATH
3.  Clone this git repository through the Go tools:

        $ go get github.com/openshift/origin
        $ cd $GOPATH/src/github.com/openshift/origin
   
4.  Run a build

        $ go get github.com/coreos/etcd
        $ hack/build-go.sh
    
5.  Start an OpenShift all-in-one server (includes everything you need to try OpenShift)

        $ output/go/bin/openshift start
    
6.  In another terminal window, switch to the directory:

        $ cd $GOPATH/src/github.com/openshift/origin
        $ output/go/bin/openshift kube create services -c examples/test-service.json
    
Coming soon: Vagrant environments supporting OpenShift - see [Kubernetes README.md](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/README.md) for now.

API
---

The OpenShift APIs are exposed at `http://localhost:8081/osapi/v1beta1/*`.  

* `http://localhost:8081/osapi/v1beta1/services` (stub)

The Kubernetes APIs are exposed at `http://localhost:8080/api/v1beta1/*`:

* `http://localhost:8080/api/v1beta1/pods`
* `http://localhost:8080/api/v1beta1/services`
* `http://localhost:8080/api/v1beta1/replicationControllers`
* `http://localhost:8080/api/v1beta1/operations`

An draft of the proposed API is available [in this repository](https://rawgit.com/csrwng/oo-api-v3/master/oov3.html).  Expect significant changes.


Contributing
------------

Contributions are welcome - a more formal process is coming soon.  In the meantime, open issues as necessary, ask questions on the OpenShift IRC channel (#openshift-dev on freenode), or get involved in the [Kubernetes project](https://github.com/GoogleCloudPlatform/kubernetes).


License
-------

OpenShift is licensed under the Apache Software License 2.0.
