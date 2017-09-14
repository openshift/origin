# Service Catalog Introduction

The Kubernetes Service Catalog provides a Kubernetes-native interface to one
or more [Open Service Broker API](https://www.openservicebrokerapi.org/) compatible
service brokers.

# Concepts

The service catalog API has five main concepts:

- Open Service Broker API Server: A server that acts as a service broker and conforms to the 
[Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md)
specification. This software could be hosted within your own Kubernetes cluster
or elsewhere.

The remaining four concepts all map directly to new Kubernetes resource types
that are provided by the service catalog API.

- `ServiceBroker`: An in-cluster representation of a broker server. A resource of this
type encapsulates connection details for that broker server. These are created
and managed by cluster operators who wish to use that broker server to make new
types of managed services available within their cluster.
- `ServiceClass`: A *type* of managed service offered by a particular broker.
Each time a new `ServiceBroker` resource is added to the cluster, the service catalog
controller connects to the corresponding broker server to obtain a list of
service offerings. A new `ServiceClass` resource will automatically be created
for each.
- `ServiceInstance`: A provisioned instance of a `ServiceClass`. These are created
by cluster users who wish to make a new concrete _instance_ of some _type_ of
managed service to make that available for use by one or more in-cluster
applications. When a new `ServiceInstance` resource is created, the service catalog
controller will connect to the appropriate broker server and instruct it to
provision the service instance.
- `ServiceInstanceCredential`: Access credential to a `ServiceInstance`. These
are created by cluster users who wish for their applications to make use of a
service `ServiceInstance`. Upon creation, the service catalog controller will
create a Kubernetes `Secret` containing connection details and credentials for
the service instance. Such `Secret`s can be mounted into pods as usual.

These concepts and resources are the building blocks of the service catalog.

# Installation

Service Catalog installs into a Kubernetes cluster and runs behind the
[Kubernetes API Aggregator](https://kubernetes.io/docs/concepts/api-extension/apiserver-aggregation/).

The aggregator works best in Kubernetes versions 1.7 and above, so we only
provide official support for Kubernetes 1.7 or higher. We do, however,
provide instructions for versions 1.6 and lower. They are no longer 
maintained and we do not recommend using them to run a production installation
of Service Catalog.

## Kubernetes 1.7 and Above

- [Installation instructions](./install-1.7.md)
- [Demo instructions](./walkthrough-1.7.md) (this is a work in progress)

## Kubernetes 1.6 (Deprecated)

We recommend that you run Service Catalog on Kubernetes version 1.7 or higher.
The 1.6 instructions are deprecated, not maintained and may be removed in the
future.

- [Installation instructions](./install-1.6.md)
- [Demo instructions](./walkthrough-1.6.md)
