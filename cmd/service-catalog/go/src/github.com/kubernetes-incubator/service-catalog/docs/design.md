# Design of the Service Catalog

Table of Contents
- [Overview](#overview)
- [Terminology](#terminology)
- [Open Service Broker API](#open-service-broker-api)
- [Service Catalog Design](#service-catalog-design)
- [Current Design](#current-design)

## Overview

The Service Catalog is an implementation of the
[Open Service Broker API](https://github.com/openservicebrokerapi) for
Kubernetes. It allows for:
- a Service Broker to register with Kubernetes
- a Service Broker to specify the set of Services (and variantions of those
  Services) to Kubernetes that should then be made available to Kubernetes'
  users
- a user of Kubernetes to discover the Services that are available for use
- a user of Kubernetes to request for a new ServiceInstance of a Service
- a user of Kubernetes to link an ServiceInstance of a Service to a set of Pods

This infrastructure allows for a loose-coupling between Applications
running in Kubernetes and the Services they use.
The Service Broker, in its most basic form, is a blackbox entity. Whether
it is running within Kubernetes itself is not relevant. This allows for
the Application that uses those Services to focus on its own business logic
while leaving the management of these Services to the entity that owns
them.

## Terminology

- **Application** : Kubernetes uses the term "service" in a different way
  than Service Catalog does, so to avoid confusion the term *Application*
  will refer to the Kubernetes deployment artifact that will use a Service
  Instance.
- **ServiceInstanceCredential**, or *Service Instance Credential* : a link between a Service Instance
  and an Application. It expresses the intent for an Application to
  reference and use a particular Service Instance.
- **ServiceBroker**, or *Service Broker* : a entity, available via a web endpoint,
  that manages a set of one or more Services.
- **Credentials** : Information needed by an Application to talk with a
  Service Instance.
- **ServiceInstance**, or *Service Instance* : Each independent use of a Service
  Class is called a Service Instance.
- **Service Class**, or *Service* : one type of Service that a Service Broker
  offers.
- **Plan**, or *Service Plan* : one type of variant of a Service Class. For
  example, a Service Class may expose a set of Plans that offer
  varying degrees of quality-of-services (QoS), each with a different
  cost associated with it.

## Open Service Broker API

The Service Catalog is a compliant implementation of the
[Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md) (OSB API). The OSB API specification is the evolution of
the [Cloud Foundry Service Broker API](https://docs.cloudfoundry.org/services/api.html).

This document will not go into specifics of how the OSB API works, so for
more information please see:
[Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker).
The rest of this document assumes that the reader is familiar with the
basic concepts of the OSB API specification.

## Service Catalog Design

The following diagram represents the design of the Service Catalog:

<img src="images/desired.png" width="75%" height="75%">

Note that the current state of the project does not full support everything
as described in this design yet, but it is useful to start with our goals
and then point out the places (via a **[DIFF]** marker) where our current
state of the project differs.

At the core of the Service Catalog, as with the Kubernetes core, is an
API Server and a Controller. The API Server is an HTTP(s) RESTful front-end for
a storage component. Users of the system, as well as other components of
the system, interact with the API server to perform CRUD type of operations
on the Service Catalog's resource model. As with Kubernetes itself, the
`kubectl` command line tool can be used to interact with the Service Catalog
resource model.

The storage component behind the Service Catalog's API Server can either be
[etcd](https://github.com/coreos/etcd) or
[Third Party Resources](https://kubernetes.io/docs/user-guide/thirdpartyresources/) (TPRs).
The `rest.storage` interface abstracts the specific persistent storage
facility being used.
When etcd is used, the instance(s) of etcd will be distinct from the etcd
instances of the Kubernetes core - meaning, the Service Catalog will have its
own persistent storage that is separate from the Kubernetes core.
When TPRs are used, those resources will be stored in the Kubernetes core
and therefore a separate persistent storage (from Kubernetes) is not needed.

**[DIFF]** *As of now the API Server can only use etcd as its persistent
storage. The plan is to add support for TPRs to the `rest.storage` interface
of the API Server in the near future.*

The Service Catalog resource model is defined within a file called
`pkg/apis/servicecatalog/types.go` and the initial (current) version
of the model is in `pkg/apis/servicecatalog/v1alpha1/`.  As of now there is
only one version of the model but over time additional versions will be
created and each will have its own sub-directory under
`pkg/apis/servicecatalog/`.

**TODO** add a brief discussion of how resources are created and we'll
use the Status section to know when its fully realized.  Instead of the
"claim" model that is used by other parts of Kube.

The Controller is the brains of the Service Catalog. It monitors the
resource model (via watches on the API server), and takes the appropriate
actions based on the changes it detects.

To understand the Service Catalog resource model, it is best to walk through
a typical workflow:

### Registering a Service Broker

**TODO** Talk about namespaces - ServiceBrokers, ServiceClasses are not in a ns.
But ServiceInstances, ServiceInstanceCredentials, Secrets and ConfigMaps are. However, instances
can be in different NS's than the rest (which must all be in the same).

Before a Service can be used by an Application it must first be registered
with the Kubernetes platform. Since Services are managed by Service Brokers
we must first register the Service Broker by creating an instance of a
`ServiceBroker`:

    kubectl create -f broker.yaml

where `broker.yaml` might look like:

    apiVersion: servicecatalog.k8s.io/v1alpha1
    kind: ServiceBroker
    metadata:
      name: BestDataBase
    spec:
      url: http://bestdatabase.com

**TODO** beef-up theses sample resource snippets

After a `ServiceBroker` resource is created the Service Catalog Controller will
receive an event indicating its addition to the datastore. The Controller
will then query the Service Broker (at the `url` specified) for the list
of available Services. Each Service will then have a corresponding
`ServiceClass` resource created:

    apiVersion: servicecatalog.k8s.io/v1alpha1
    kind: ServiceClass
    metadata:
      name: smallDB
      brokerName: BestDataBase
      plans...

Notice that each Service can have one or more Plans associated with it.

**TODO** Anything special about the CF flows we need to discuss?

Users can then query for the list of available Services:

    kubectl get services

### Creating a Service Instance

Before a Service can be used, a new ServiceInstance of it must be created. This is
done by creating a new `ServiceInstance` resource:

    kubectl create -f instance.yaml

where `instance.yaml` might look like:

    apiVersion: servicecatalog.k8s.io/v1alpha1
    kind: ServiceInstance
    metadata:
      name: johnsDB
    spec:
      serviceClassName: smallDB

Within the `ServiceInstance` resource is the specified Plan to be used. This allows
for the user of the Service to indicate which variant of the Service they
want - perhaps based on QoS type of variants.

**TODO** Discuss the parameters that can be passed in

Once an `ServiceInstance` resource is created, the Controller talks with the
specified Service Broker to create a new ServiceInstance of the desired Service.

There are two modes for provisioning:
[synchronous and asynchronous](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#synchronous-and-asynchronous-operations)

For synchronous operations, a request is made to the Service Broker and upon
successful completion of the request (200 OK), Service Instance can now be used by
Application.

Some brokers support
[asynchronous](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#asynchronous-operations)
flows. When a Controller makes a request to Service Broker to
create/update/deprovision a Service Instance, the Service Broker responds with
202 ACCEPTED, and will provide endpoint at
GET /v2/service_instances/<service_instance_id>/last_operation
where the Controller can poll the status of the request.

Service Broker may return a last_operation field that then should be sent
for each last_operation request. Controller will poll while the state of
the poll request is 'in_progress'. Controller can also implement a max
timeout that it will poll before considering the provision failed and will
stop polling and mark the provisioning as failed.

While a Service Instance has an asynchronous operation in progress, controller
must ensure that there no other operations (provision,deprovision,update,bind,unbind).

**TODO** test to see if we have checks to block people from using an ServiceInstance
before its fully realized. We shouldn't let the SB be the one to detect this.

### Using a Service Instance

Before a Service Instance can be used it must be "bound" to an Application.
This means that a link, or usage intent, between an Application and the
Service Instance must be established. This is done by creating a new
`ServiceInstanceCredential` resource:

    kubectl create -f binding.yaml

where `instance.yaml` might look like:

    apiVersion: servicecatalog.k8s.io/v1alpha1
    kind: ServiceInstanceCredential
    metadata:
      name: johnsServiceInstanceCredential
    spec:
      secretName: johnSecret
      ...Pod selector labels...

The Controller, upon being notified of the new `ServiceInstanceCredential` resource, will
then talk to the Service Broker to create a new ServiceInstanceCredential for the specified
Service Instance.

Within the ServiceInstanceCredential object that is returned from the Service Broker are
a set of Credentials. These Credentials contain all of the information
needed for the application to talk with the Service Instance. For example,
it might include things such as:
- coordinates (URL) of the Service Instance
- user-id and password to access the Service Instance

The OSB API specification does not mandate what properties might appear
in the Credentials, so the Application is required to understand the
specified data returned and how to use it properly. This is typically done
by reading the documentation of the Service.

The Credentials will not be stored in the Service Catalog's datastore.
Rather, they will be stored in the Kubenetes core as Secrets and a reference
to the Secret will be saved within the `ServiceInstanceCredential` resource. If the
ServiceInstanceCredential `Spec.SecretName` is not specified then the Controller will
use the ServiceInstanceCredential `Name` property as the name of the Secret.

ServiceInstanceCredentials are not required to be in the same Kubenetes Namespace
as the Service Instance. This allows for sharing of Service Instances
across Applications and Namespaces.

In addition to the Secret, the Controller will also create a Pod Injection
Policy (PIP) resource in the Kubernetes core. See the
[PIP Proposal](https://github.com/kubernetes/community/pull/254) for more
information, but in short, the PIP defines how to modify the specification
of a Pod during its creation to include additional volumes and environment
variables.
In particular, Service Catalog will use PIPs to allow the Application
owner to indicate how the Secret should be made available to its Pods. For
example, they may define a PIP to indicate that the Secret should be mounted
into its Pods. Or perhaps the Secret's names/values should be exposed as
environment variables.

PIPs will use label selectors to indicate which Pods will be modified.
For example:

    kind: PodInjectionPolicy
    apiVersion: extensions/v1alpha1
    metadata:
      name: allow-database
      namespace: myns
    spec:
      selector:
        matchLabels:
          role: frontend
      env:
        - name: DB_PORT
          value: 6379

defines a PIP that will add an environment variable called `DB_PORT` with
a value of `6379` to all Pods that have a label of `role` with a value
of `frontend`.

Eventually, the OSB API specification will hopefully have additional metadata
about the Credentials to indicate which fields are considered "secret" and
which are not. When that support is available expect the non-secret Credential
information to be placed into a ConfigMap instead of a Secret.

Once the Secret is made available to the Application's Pods, it is then up
to the Application code to use that information to talk to the Service
ServiceInstance.

### Deleting Service Instances

As with all resources in Kubernetes, you can delete any of the Service
Catalog resource by doing an HTTP DELETE to the resource's URL. However,
it is important to note the you can not delete a Service Instance while
there are ServiceInstanceCredentials associated with it.  In other words, before a Service
ServiceInstance can be delete, you must first delete all of its ServiceInstanceCredentials.
Attempting to delete an ServiceInstance that still has a ServiceInstanceCredential will fail
and generate an error.

Deleting a ServiceInstanceCredential will also, automatically, delete any Secrets or ConfigMaps
that might be associated with it.

**TODO** what happens to the Pods using them?

## Current Design

The sections above describe the current plans and design for the Service
Catalog. However, there are certain pieces that are not in place yet and
so the code does not necessarily align with it. The current design actually
looks more like this:

<img src="images/current.png" width="75%" height="75%">

Below are the key aspects of the code that differ from the design above:

- The API Server can only use etcd as its persistent store.
- The API Server is not connected to the Controller, which means it's not
  actually used as part of the running system yet. Any resources created 
  by talking to the API Server will be stored but nothing beyond storing
  them will happen.
- Creating Third Party Resource versions of the Service Catalog resources
  in the Kubernetes core API Server is the current way the system works.
  The Controller will then talk to the Kubernetes core API Server
  and monitor the TPR version of the Service Catalog resources and take
  all appropriate actions.
