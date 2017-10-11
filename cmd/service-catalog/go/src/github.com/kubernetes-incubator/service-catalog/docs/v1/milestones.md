# v1 Milestones

**NOTE: THIS DOC IS A WORK IN PROGRESS**

This document captures concensus on the identity and content of milestones.

## Legos-1

Tentative target date: Feb 1, 2017

1.  `servicecatalog/v1alpha1` API complete and functional:
  1.  `ServiceBroker`, `ServiceClass`, `ServiceInstance`, `ServiceBinding`
  2.  Integrators should be able to program against this REST API
2.  API server serves `servicecatalog/v1alpha1`
  1.  It should be possible to use stock `kubectl` for raw CRUD operations
      against the API server
3.  Golang client for API in (1) is auto-generated

## MVP-1

Tentative target date: March 1, 2017

The MVP1 milestone is the barest skeleton of an MVP that has the right
architecture.

High-level functional requirements:

1.  `servicecatalog/v1alpha1` API complete and functional:
  1.  `ServiceBroker`, `ServiceClass`, `ServiceInstance`, `ServiceBinding`
  2.  Integrators should be able to program against this REST API
  3.  Status subresources for `ServiceBroker`, `ServiceInstance`, and `ServiceBinding`
2.  Golang client for API in (1) is auto-generated
3.  Native-k8s-style CLI experience
  1.  Formatted list
  2.  Formatted describe
4.  Integration with Open Service Broker API:
  1.  Add broker
  2.  Provision service instance
  3.  Bind to service instance
  4.  Unbind from service instance
  5.  Deprovision service instance
  6.  Remove broker
  7.  Update broker
  8.  Delete broker
5.  ServiceBindings manifest as a Secret in the the k8s core; users will be expected
    to explicitly reference this secret in their Pod specs

High-level architectural requirements:

1.  We are moved off TPR and fully onto `servicecatalog/v1alpha1` API
2.  Code generators for API are in place:
  1.  Deep copy
  2.  Defaults
  3.  Conversions
  4.  YAML parser
  5.  API client
3.  Controller(s) use k8s controller infrastructure
4.  CLI interface:
  1.  Vendor in `kubectl` guts
  2.  listers and describers for `servicecatalog/v1alpha1` API resources
