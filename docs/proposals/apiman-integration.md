# APIMan Integration

## Abstract
This proposes a design for integrating [APIMan](http://www.apiman.io/) with OpenShift in order to provide  micro-services governance for API service providers.

## Motivation
Providers of services may have the need to control who and how their services are consumed.  This may be realized as policies defined by providers to control a service
in any number of ways (e.g security, throttling/quota, billing and metrics)<sup>[1](#r1)</sup>. 

## Constraints and Assumptions
### Authentication
The OAuth token identifying a user in OpenShift will be utilized by APIMan to associate service policy to their identity.  This will ensure a user's identity is 
ubiquitous across the cluster when interacting with OpenShift and the APIMan management console.

### Deployment
#### Communication
APIMan Components (e.g. gateway, management interface) integrated with the cluster will utilize mutual TLS for communication.

#### On-Cluster Policy Storage
[Origin Aggregated logging](https://github.com/openshift/origin-aggregated-logging)<sup>[4](#r4)</sup> will be deployed before APIMan in order to utilize the existing ElasticSearch cluster for policy storage.  
The [ACL plugin](https://github.com/fabric8io/openshift-elasticsearch-plugin)<sup>[2](#r2)</sup> that is deployed
as part of aggregated logging will be updated to allow access from the gateway.  ElasicSearch index management<sup>[3](#r3)</sup>  will be modified so
that it will not cull APIMan policy data.

#### Off-Cluster Policy Storage
Cluster administrators can configure APIMan to use an alternate ElasticSearch cluster if desired.  Some organizations may have an existing ElasticSearch cluster that already defines
policy, and adminstrators may desire to use it instead of the one provided with aggregated logging.  It is assumed in this configuration, the APIMan management interface will
additionally rely on Kibana deployed off-cluster.  The multi-tenant features provided by aggregated logging for ElasticSearch and Kibana may not be available when configured to utilize these off-cluster
dependencies.      

## Use Cases
* **UC01** As an API provider, I want to navigate from the OpenShift web console to the policy management interface, so I can define my service policies.
* <s> **UC02** As an API provider, I want to associate an APIMan organization with an OpenShift project, so I can control policy between organizations.</s>
* **UC03** As an API provider, I want to expose my service, so that it is consumable according to my management policy.  
* **UC04** As a cluster administrator, I want a gateway to route API traffic only, so that it manages traffic based on a service's policy.
* **UC05** As a cluster administrator, I want to deploy APIMan and its components reusing existing infrastructure components where possible (e.g. ElasticSearch), so I can minimize infrastructure components.
* **UC06** As a cluster administrator, I want API management interface to be able to discover the route to Kibana, so it can be linked to from the management interface. 
* **UC07** As an API consumer, I want to configure my services to consume other services

## Specification

API services will be scoped to OpenShift projects. Projects will have one-to-one relationship to an APIMan namespace (formally organization). APIMan will utilize namespace
to manage policy regarding the service.
 
### Service Annotations
Services intended to be managed by APIMan and exposed as API Endpoints will be annotated<sup>[5](#r5)</sup>.  The annotations are repeated below for convenience:

```
  apiVersion: "v1"
  kind: "Service"
  metadata: 
    annotations: 
      api.service.kubernetes.io/protocol: REST
      api.service.kubernetes.io/scheme: http
      api.service.kubernetes.io/path: cxfcdi
      api.service.kubernetes.io/description-path: cxfcdi/swagger.json
      api.service.kubernetes.io/description-language: SwaggerJSON
```

### Origin Web Console UI extension
A custom extension<sup>[6](#r6)</sup> will be created and hooked into the web console to support API endpoints.  The extension
will allow a user to navigate to the APIMan management interface to:
* Manage an API endpoint
* Expose a service as a new API endpoint

The extension will provide the following details to the APIMan gateway:
* Service name
* Service namespace
* User's Oauth token

Calls from the extension to the APIMan management interface will utilize a REST POST call where the oauth token is part of the payload.  It will
utilize a similiar design<sup>[7](#r7)</sup> that is realized by the OpenShift origin aggregated logging integration and the auth proxy<sup>[8](#r8)</sup>.  It is necessary for
security reasons to not add the token as a query parameter.  

### API Services Gateway
The APIMan gateway will be deployed as a cluster-wide infrastructure component to handle services exposed as API endpoints.  Internal and external clients can use the gateway to access APIs.  Admins should be able to configure the cluster to limit direct access to services and encourage consumers of API endpoints to utilize the gateway.

<s>APIman will be deployed as an additional gateway router that internal and external clients can use to access APIs. It is not an exclusive router, but admins should be able to configure the cluster to limit access to services directly and instead encourage applications bouncing off the gateway. The gateway would act as a 'router', although it will effectively be looking at annotated services directly.</s>

**Need: Document how a cluster admin might setup their cluster to control services as alluded to in this section **

**Need: Understand how we would deploy multiple gateways to control subsets of the cluster **

### OpenShift CLI Modifications
The openshift client binary will be updated to allow a user to:
* Expose an API endpoint.  
Exposing the service will update the service annotation as described by the service annotation section.  Possible usage syntax:<p>
``` oc set api-service SERVICENAME --path PATH```<p>
The output of the command should return a route to the service.

* Hide an API endpoint.  
Hiding an API endpoint will remove the annotations from a service.  The change will additionally cause APIMan to remove this service endpoint.  Possible usage syntax:<p>
```oc unset api-service SERVICENAME```

### Deployment / Deployer pod
* APIMan will be deployed as a cluster level service for managing API service end points.
* APIMan will be deployed to reuse the existing ElasticSearch cluster
* APIMan will be deployed to reuse the existing Kibana instance 

**Need:**
* What indexes does a user's profile need to display
* How do we configure apiman to know about kibana?  deployer env image var

## Rationale
The technical rationale fleshes out the specification by describing what motivated the design and why particular design decisions were made. It should describe alternate designs that were considered and related work, e.g. how the feature is supported in other products.
The rationale should provide evidence of consensus within the community and discuss important objections or concerns raised during discussion.

## Limitations
scalability - 
Cert generation
- defer to a deployer pod
- need to redeploy all of logging to reuse ES


## References
* <span id="r1">[1]</span> APIMan - http://www.apiman.io/
* <span id="r2">[2]</span> OpenShift ElasticSearch plugin - https://github.com/fabric8io/openshift-elasticsearch-plugin
* <span id="r3">[3]</span> OpenShift Aggregated Logging Index Management - https://github.com/openshift/origin-aggregated-logging/pull/57
* <span id="r4">[4]</span> Origin Aggregated Logging - https://github.com/openshift/origin-aggregated-logging
* <span id="r5">[5]</span> Service Discover - https://github.com/kubernetes/kubernetes/blob/master/docs/proposals/service-discovery.md
* <span id="r6">[6]</span> Web Console Extensions - https://docs.openshift.org/latest/install_config/web_console_customization.html
* <span id="r7">[7]</span> Kibana API Discovery - https://github.com/openshift/origin/blob/master/assets/app/scripts/directives/logViewer.js#L337
* <span id="r8">[8]</span> OpenShift Auth Proxy - https://github.com/fabric8io/openshift-auth-proxy