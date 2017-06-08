# Terminology

The terminology of service-catalog has some inconvenient and entirely
unavoidable overloading with certain terms in the
[Kubernetes](https://github.com/kubernetes/kubernetes) lexicon.  This page lists
the definitions of terms as used in this project.

## Roles

**Service Consumer**: Any person or application that will use a Service from
the catalog.

**Application operator**: The person or team responsible for deploying an
application. Users in this role, at minimum, have access to their own
application's namespace. In some cases, users in this role may also be an
application developer or a cluster operator

**Cluster operator**: The person or team responsible for operating a Kubernetes
cluster. This team may operate the cluster on behalf of other users, or may
operate the cluster to facilitate their own work

**Catalog operator**: The person or team responsible for adminstration of the
Service Catalog, including catalog curation and Service Broker registration

**Broker operator**: The person or team responsible for running and managing one
or more **Service Brokers**.

**Service Producer**: The person or team who authors and/or operates a Service
available from the Service Catalog. As part of creating a service, the Service
Producer may also be running a Service Broker.

## Lexicon

**Service**: Running code that is made available for use by an application.
Traditionally, services are available via HTTP REST endpoints, but this is not a
requirement.

**Service Broker**: An endpoint that manages a set of services. Responsible for
translating Service Catalog activities (like provision, bind, unbind,
deprovision) into appropriate actions for the service.

**Service Catalog**: An endpoint that manages (1) a set of registered Service
Brokers and (2) the list of services that are available for instantiation from
those Service Brokers.

**Service Instance**: Each request for a unique use of a Service will result in
the Service Catalog requesting a new Service Instance from the owning Service
Broker.

**Application**: Code that will access or consume a Service. While in Kubernetes
the code that is deployed is often called a "Service", to avoid confusion, this
document will refer to the code that accesses a service as an "application".

**Resource type**: A logical Kubernetes concept. Examples include:

  - [Pods](http://kubernetes.io/docs/user-guide/pods/)
  - [Services](http://kubernetes.io/docs/user-guide/services/)
  - [Secrets](http://kubernetes.io/docs/user-guide/secrets/)

**Resource**: A specific instantiation of an aforementioned resource type,
often represented as a YAML or JSON file that is submitted or retrieved via the
standard Kubernetes API (or via `kubectl`)

**Binding**: Represents a relationship between an Application and a Service
Instance. A Binding contains the information necessary for the Application to
make use of the Service Instance.