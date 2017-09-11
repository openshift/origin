# Service Catalog Use Cases

* TODO: scrub doc post glossary

## Personas

* Service Consumer
* Service Producer
* Catalog Operator

## Catalog Publishing/Curation/Discovery

* As a Catalog Operator, I want to be able to register a Service Broker with the
  Kubernetes Service Catalog, so that the Service Catalog is aware of the
  Services that ServiceBroker offers
* As a Catalog Operator, I want to be able to update a registered Service Broker
  so that the Service Catalog can maintain the most recent versions of Services
  that ServiceBroker offers
* As a Catalog Operator, I want to be able to delete a Service Broker from the
  Service Catalog, so that I can keep the Service Catalog clean of Service
  ServiceBrokers I no longer want to support

* Can I export a list of Service Brokers and types from my Service Controller?
* Is there an auth story for adding Service Brokers?
* As a Developer, working outside of the normal production cluster, I would like
  to be able to use the Services available to me from the production cluster
  from my local environment without needing to establish a formal business
  relationship with each service provider.

## Searching and Browsing Services:
    1.  As a consumer, I'm able to search my catalog for services by attributes
        such as category
    2.  As a consumer, I'm able to see metadata about a service prior to 
        creation which allows me to see if this service fits my need
    3.  As a consumer, I'm able to see all the required and optional parameters
        the service takes in order to create it
    4.  As a consumer, when listing services I see a union of the catalogs
        from all brokers that are registered. However, if I want to restrict the
        list to a specific broker I can pass that in as a flag.

* How are Services identified: name, service name/id, plan name/id?
* Who can see which Services? (TODO: Include scope? Global/Namespaced)
* Who can see which Service Instances? (TODO: Include scope? Global/Namespaced)
* The Service Catalog should contain Services and not Service Instances

### Registering a Service Broker with the Service Catalog

A User must register each Service Broker with the Service Catalog to advertise
the Services it offers in the Service Catalog. After the Service Broker has been
registered with the Service Catalog, the Service Controller makes a call to the
Service Broker's `/v2/catalog` endpoint. The Service Broker returns a list of
Services offered by that broker. Each Service has a set of plans that
differentiate the tiers of that Service.

### Updating a Service Broker

ServiceBroker operators make changes to the services their brokers offer. To refresh
the services a broker offers, the Service Controller should re-list the
`/v2/catalog` endpoint.  The Service Controller should apply the result of
re-listing the broker to its internal representation of that broker's services:

1. New services present in the re-list results are added
2. Existing services are updated if a diff is present
3. Existing services missing from the re-list are deleted

TODO: spell out various update scenarios and how they affect end-users

### Delete a Service Broker

There must be a way to delete brokers from the catalog. We should evaluate
whether deleting a broker should:

1. Cascade down to the Service Instances for the broker
2. Leave orphaned Service Instances in the Service Catalog
3. Fail if Service Instances still exist for the broker


### Search and Browsing Services

#### Searching Services

Consumers should be be able to search or filter their catalog by labels. For
example, if I search for all services with 'catalog=database' the catalog
will return the list of services that match that label. This assumes, of
course, that producers are able to label their service offerings.

#### Service Metadata

Each service should have a list of metadata that it exposes in the catalog.
If we're following the Cloud Foundry model you can view the list of metadata
fields [here](https://docs.cloudfoundry.org/services/catalog-metadata.html).

We should consider what metadata needs to be exposed for a strong CLI and UI
experience. Here are some suggestions for metadata fields:

    * name
    * short description
    * long desciption
    * documentation/support urls
    * icon URL
    * image URLs - a list of images that could be displayed in a UI
    * TOS (terms of service) link
    * a list of plans
        * plan name
        * plan description
        * plan cost
    * construction parameters
        * name
        * description
        * default value
    * category label/tags
    * version
    * publisher name
    * publisher contact url
    * publisher website

#### Viewing Service Parameters

Each service offering may have a list of parameters (e.g., configuration)
required  to create that service. For example, if consuming a hosted database, I
may need to  specify the region, size, a link to a startup scripts, or other
parameters.

For each service, I'm able to see the list of required and optional parameters
that I can pass in during service creation. Service producers are able to
specify default values for these parameters.

#### Listing Services

When listing all services available to be created, users will see a union of the
catalog offerings from all brokers registered. However, users have the option of
passing in a flag to limit results to just a specific registered broker.

TODO: How to deal with name conflicts for {broker, service}.


## Requesting Services (Consumer)

* As an Application Operator, how do I cause a new Service Instance to be created from the
  Service Catalog?
* As an Application Operator, how do I bind an application to an existing Service Instance?
* How does the catalog support multiple consumers in different Kubernetes
  namespaces of the same Service Instance?
* As an Application Operator, who has requested a Service Instance, know that a request for a
  service instance has been fulfilled?
* As an Application Operator, I should be able to pass parameters to be used by the Service
  ServiceInstance or ServiceInstanceCredential when causing a new Service Instance to be created, so that
  I may change the attributes of the Service Instance or ServiceInstanceCredential.

## Provisioning a Service Instance

* As a ServiceBroker operator, I want to control the number of instances of my Service,
  so that I can control the resource footprint of my Service.

## ServiceInstanceCredential to a Service Instance

* As a ServiceBroker operator, I want to control the number of bindings to a Service
  ServiceInstance so that I may provide limits for services (e.g. free plan with 3
  bindings). (TODO: Do we care?)
* As a user of a Service Instance, I want a predictable set of Kubernetes
  resources (Secrets, ConfigMap, etc.) created after binding, so that I know how
  to configure my application to use the Service Instance.
* As a broker operator, I want to be able to discover what applications are
  bound to Service Instances I am responsible for, so that I may operate the
  service properly.
* As an Application Operator I should be able to see what Service Instances my applications are
  bound to.
* As an Application Operator I should be able to pass paramters when binding to a Service
  ServiceInstance so that I may indicate what type of binding my application needs.
  (e.g. credential type, admin binding, ro binding, rw binding)

As an Application Operator, I should be able to accomplish the following sets of bindings:

* One application may binding to many Service Instances
* Many different applications may bind to a single Service Instance
  * ...with unique credentials
  * ...with identical credentials
* One application, binding multiple times to the same Service Instance

## Using/Consuming a Service Instance

* As an Application Operator consuming a Service Instance, I need to be able to understand the structure
  of the Kubernetes resources that are created when a new ServiceInstanceCredential to a Service
  ServiceInstance is created, so that I can configure my application appropriately.
* As an Application Operator, I want to be able to understand the relationship between a Secret
  and Service Instance, so that I can properly configure my application (e.g.
  app connecting to sharded database).
* The consuming application may safely assume that network connectivity to the
  Service Instance is available

Consuming applications that need specific handling of credentials or
configuration should be able to use additional Kubernetes facilities to
adapt/transform the contents of the the credentials/configuration. This
includes, but is not limited to, side-car and init containers.

If the user were willing to change the application, then we could
drop the credentials in some "standard" place by convention. This would be
similar to how K8s service accounts work (service-account secrets are mounted at
`/var/run/secrets/kubernetes.io/serviceaccount`), as well as `VCAP_SERVICES` in
CF.

If the user were willing to change the configuration instead, they could specify
how the credentials were surfaced to the application -- which environment
variables, volumes, etc.

## Lifecycle of Service Instances

* As a Service Provider, I should be able to indicate that a Service Instance
  _may_ be upgraded (plan updateable), so that I can communicate Service
  capabilities to end users.
* As an Application Operator of a Service Instance, I want to be able to change the Service
  ServiceInstance plan so that I may size it appropriately for my needs.
* What is the update story for bindings to a Service Instance?
* What is the versioning and update story for a Service Instance: what happens
  when a broker changes the Services it provides?

## Unbinding from a Service Instance

* TODO

## Deprovisioning a Service Instance

* TODO
