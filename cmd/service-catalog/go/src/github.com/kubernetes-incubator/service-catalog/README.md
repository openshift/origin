## `service-catalog`

[![Build Status](https://travis-ci.org/kubernetes-incubator/service-catalog.svg?branch=master)](https://travis-ci.org/kubernetes-incubator/service-catalog "Travis")
[![Build Status](https://service-catalog-jenkins.appspot.com/buildStatus/icon?job=service-catalog-master-testing)](https://service-catalog-jenkins.appspot.com/job/service-catalog-master-testing/ "Jenkins")

### Introduction

The service-catalog project is in incubation to bring integration with service
brokers to the Kubernetes ecosystem via the [Open Service Broker
API](https://github.com/openservicebrokerapi/servicebroker). A service broker
is an endpoint that manages a set of services.  The end-goal of the service-
catalog project is to provide a way for Kubernetes users to consume services
from brokers and easily configure their applications to use those services,
without needing detailed knowledge about how those services are created /
managed.

As an example:

Most applications need a datastore of some kind.  The service-catalog allows
Kubernetes applications to consume services like databases that exist
_somewhere_ in a simple way:

1.  A user wanting to consume a database in their application browses a list of
    available services in the catalog
2.  The user asks for a new instance of that service to be _provisioned_

     _Provisioning_ means that the broker somehow creates a new instance of a
    service.  This could mean basically anything that results in a new instance
    of the service becoming available.  Possibilities include: creating a new
    set of Kubernetes resources in another namespace in the same Kubernetes
    cluster as the consumer or a different cluster, or even creating a new
    tenant in a multi-tenant SaaS system.  The point is that the
    consumer doesn't have to be aware of or care at all about the details.
3.  The user _binds_ that service to their application

    _Binding_ means that the application is injected with the information
    necessary to use the service, such as coordinates, credentials, and
    configuration items.  Applications are injected using the existing
    application configuration primitives in Kubernetes: Services, Secrets, and
    ConfigMaps.

For more details about the design and features of this project see the
[design](docs/design.md) doc.

#### Video links

- [Service Catalog Basic Concepts](https://goo.gl/6xINOa)
- [Service Catalog Basic Demo](https://goo.gl/IJ6CV3)
- [SIG Service Catalog Meeting Playlist](https://goo.gl/ZmLNX9)

---

### Overall Status

We are currently working toward a beta-quality release to be used in conjunction with
Kubernetes 1.6. See the
[milestones list](https://github.com/kubernetes-incubator/service-catalog/milestones?direction=desc&sort=due_date&state=open) 
for information about the issues and PRs in current and future milestones.

**NOTE**: Some fields in our API will still be considered **ALPHA** after the
API graduates to **BETA**.  These fields are prefixed with `alpha` in
JSON/YAML.  Alpha fields are provided for use at your own risk, may not work
correctly, may be subject to change or removal at any time, and will not have
data migration provided for them when they graduate past alpha.  When an alpha
field graduates past alpha, the `alpha` prefix will be removed.

The project [roadmap](https://github.com/kubernetes-incubator/service-catalog/wiki/Roadmap)
contains information about our high-level goals for future milestones.

We are currently making weekly releases; see the
[release process](https://github.com/kubernetes-incubator/service-catalog/wiki/Release-Process)
for more information.

### Documentation

Our goal is to have extensive use-case and functional documentation.

See [here](./docs/v1) for [documentation](./docs/v1).

See [here](https://github.com/kubernetes-incubator/service-catalog/wiki/Examples) for examples and
[here](https://github.com/openservicebrokerapi/servicebroker/blob/master/gettingStarted.md) for
broker servers that are compatible with this software.

### Terminology

This project's problem domain contains a few inconvenient but unavoidable
overloads with other Kubernetes terms.  Check out our [terminology
page](./terminology.md) for definitions of terms as they are used in this
project.

### Contributing

Interested in contributing?  Check out the [documentation](./CONTRIBUTING.md).

Also see the [developer's guide](./docs/devguide.md) for information on how to
build and test the code.

We have weekly meetings - see
[Kubernetes SIGs](https://github.com/kubernetes/community/blob/master/sig-list.md)
(search for "Service Catalog") for the exact date and time. Our agenda/notes
doc can be found
[here](https://docs.google.com/document/d/10VsJjstYfnqeQKCgXGgI43kQWnWFSx8JTH7wFh8CmPA/edit).

### Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md).
The project was established 2016-Sept-12.  The incubator team for the project is:

- Sponsor: Brian Grant ([@bgrant0607](https://github.com/bgrant0607))
- Champion: Paul Morie ([@pmorie](https://github.com/pmorie))
- SIG: [sig-service-catalog](https://github.com/kubernetes/community/tree/master/sig-service-catalog)

For more information about sig-service-catalog such as meeting times and agenda,
check out the [community site](https://github.com/kubernetes/community/tree/master/sig-service-catalog).

### Code of Conduct

Participation in the Kubernetes community is governed by the
[Kubernetes Code of Conduct](./code-of-conduct.md).
