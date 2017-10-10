# V1 API

This document describes the API resource types and usage in the v1 service
catalog. Although this API will be implemented in a Kubernetes repo, other
systems are not precluded from implementing it as well.

# Resource Types

This section lists descriptions of the resources in the service catalog API.

*__Note:__ All names herein are tentative, and should be considered placeholders
for purposes of discussion. This note will be removed when names are finalized.*

## `ServiceBroker` Resource

A `ServiceBroker` represents an entity that provides service classes for use in the
catalog.

An administrator creates an instance of the `ServiceBroker` resource to indicate their
intent to make the service classes provided by that broker available in the
catalog.

The service catalog controller has a watch on the `ServiceBroker` resource.  When the
controller receives an `ADD` watch event for a new `ServiceBroker`, it contacts the
broker to determine what service classes the broker offers:

1. Makes a request against the given service broker's catalog endpoint
   (`GET /v2/catalog`)
2. Translates the response to a list of `ServiceClass`
3. Creates new `ServiceClass` instances in the API server

*TODO: What happens when a `ServiceBroker` resource is deleted?*

## `ServiceClass` Resource

A `ServiceClass` represents an offering in the service catalog.  A
`ServiceClass` is not usable directly; an `ServiceInstance` of a `ServiceClass` must be
created to be consumed by an application.

This resource is created by the service catalog's controller event loop after
it has received a `ServiceBroker` resource and successfully called the backing CF
broker's catalog endpoint.

*TODO: Make explicit the relationship between service classes and plans*

## `ServiceInstance` Resource

An `ServiceInstance` represents a provisioned instance of a `ServiceClass`, and is the
entity an application binds to.

A service consumer creates an `ServiceInstance` to indicate their intent to provision
an instance of a service class.  The `ServiceInstance` has a reference to the
`ServiceClass` to provision an instance of.

The service catalog controller has a watch on the `ServiceInstance` resource and
receives an `ADD` watch event. The controller then attempts to provision a new
instance of the service via the corresponding service broker:

1.  The controller calls the provision endpoint on the broker 
2.  The broker returns a response indicating that the broker provisioned the new
    instance, the instance was already provisioned, or the provisioning
    operation is in progress
3.  The controller updates the status of the `ServiceInstance` to indicate when the
    `ServiceInstance` is in a provisioning or provisioned condition

*TODO: codify how asynchronous responses are handled in the controller*

## `ServiceBinding` Resource

A `ServiceBinding` represents a "used by" relationship between an application and an
`ServiceInstance` of a `ServiceClass`.

*TODO: clarify exactly what constitutes an application.*

Service consumers create `ServiceBinding` resources to indicate that an application
should be bound to an `ServiceInstance`.  The `ServiceBinding` contains information about how
the application wants to use the binding information such as:

1.  The name of a Kubernetes core `Service` resource to provide a stable
    endpoint for the application to use the `ServiceInstance` via
2.  The name of a Kubernetes `Secret` resource to hold credentials necessary to
    use the service

If these values are not provided then the name of the `ServiceInstance` will be
used by default.

The service catalog controller has a watch on the `ServiceBinding` resource.  When the
controller receives an `ADD` event for a new ServiceBinding, it attempts to bind
against the service instance:

1.  The controller calls the binding endpoint on the broker
2.  The broker returns a response containing credentials and coordinates for
    the binding
3.  The controller creates a Kubernetes `Service` with the given name with the
    endpoint that fronts the `ServiceInstance`
4.  The controller creates a `Secret` with the given name containing the
    information in the `credentials` section of the broker response
5.  The controller updates the `ServiceBinding` status to reflect that the binding is
    in a bound condition
