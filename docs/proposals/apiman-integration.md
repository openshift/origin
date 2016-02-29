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
APIMan Components (e.g. gateway, management interface) integrated with the cluster will utilize mutual TLS for internal communication.

#### Storage
APIMan is capable of using several backends (e.g. ElasticSearch, MySql) to store configuration, policy, and metrics.  The initial integration will utilize ElasticSearch with the goal of reusing the same ElasticSearch cluster that is deployed for aggregated logging.  It is assumed when the backend resides on the cluster, [Origin Aggregated logging](https://github.com/openshift/origin-aggregated-logging)<sup>[4](#r4)</sup> will be deployed before APIMan. 
The [ACL plugin](https://github.com/fabric8io/openshift-elasticsearch-plugin)<sup>[2](#r2)</sup> that is deployed
as part of aggregated logging will be updated to allow access from the gateway. Additionally, the ElasicSearch index management<sup>[3](#r3)</sup>  will be modified so that it will not cull APIMan data.


Cluster administrators can configure APIMan to use an alternate, off cluster ElasticSearch instance if desired.  Some organizations may have an existing instance of ElasticSearch cluster that already defines
policy, and adminstrators may desire to use it instead of the one provided with aggregated logging.

## Use Cases
* **UC01** As an API provider, I want to navigate from the OpenShift web console to the policy management interface, so I can define my service policies.
* **UC02** As an API provider, I want to explicitly expose my service when navigating to the policy managment interface, so that it is consumable according to my management policy.  
* **UC03** As a cluster administrator, I want a gateway to route API traffic only, so that it manages traffic based on a service's policy.
* **UC04** As a cluster administrator, I want to deploy APIMan and its components reusing existing infrastructure components where possible (e.g. ElasticSearch), so I can minimize infrastructure components.
* **UC05** As a cluster administrator, I want to reuse the aggregated logging CA when generating certs for the APIMan components, so I can reuse the ElasticSearch cluster.
* **UC06** As an API consumer, I want to configure my services to consume other services

## Specification

API services will be scoped to OpenShift projects. Projects will have one-to-one relationship to an APIMan namespace (formally organization). APIMan will utilize namespace
to manage policy regarding the service.

### Deployment Scenario #1 - APIMan Gateway Fronted by a Router

The primary deployment scenario is to deploy the APIMan gateway in conjunction with a supported OpenShift router.  Network traffic to a managed endpoint is initially routed to the gateway and then further routed by the gateway to the targeted service.  This deployment can be seen in **Figure 1** where the APIMan gateway relies upon the service proxy to ultimately find a pod.

![APIMan request flow with router](apiman_request_wi_router.png "Figure 1")

### Deployment Scenario #2 - APIMan Gateway Acting Without a Router
 
### Service Annotations
Services intended to be managed and exposed as API Endpoints will be annotated<sup>[5](#r5)</sup>.  The annotations are repeated below for convenience:

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
Additionally, services that are intended to be managed by APIMan will be further annotated so the cluster infra structure can handle the service if needed.  The proposed annotation is:
```
  openshift.io/service-managed-by: apiman
```

Service providers will need to explicitly publish a service using the APIMan user interface.  Future iterations may include functionality to automatically publish a service when these annotations are applied.

### Origin Web Console UI extension
A custom extension<sup>[6](#r6)</sup> will be created and hooked into the web console to support API endpoints.  The extension
will allow a user to navigate to the APIMan management interface to:
* Manage an API endpoint
* Expose an annotated service as a new API endpoint

The extension will provide the following details to the APIMan gateway:
* Service name
* Service namespace
* User's Oauth token
* Back link to the openshift web console

Calls from the extension to the APIMan management interface will utilize a REST POST call where the oauth token is part of the payload.  It will
utilize a similiar design<sup>[7](#r7)</sup> that is realized by the OpenShift origin aggregated logging integration and the auth proxy<sup>[8](#r8)</sup>.  It is necessary for
security reasons to not add the token as a query parameter.  

### API Services Gateway
The APIMan gateway will be deployed as a cluster-wide infrastructure component to handle services exposed as API endpoints.  Internal and external clients can use the gateway to access APIs.  Admins can configure the cluster to limit direct access to services and encourage consumers of API endpoints to utilize the gateway by deploying the multi-tenant SDN plugin.  This plugin controls service to service communication between projects and would force traffic throught the gateway.  A route will be created as a well known endpoint to the APIMan gateway through which all API services traffic will flow.

#### Service Consumption & Authentication
The details of service authentication and consumption depend upon the type of published service and the polices associated with it. Essentially, user's of public service need to simply fullfill the policies associated with the service.  User's of contracted services will provide an APIMan generated API token with their request.  These are fundamental features of APIMan and are described here to provide a simple understanding of consuming APIMan controlled services.  Additional information can be found in the [APIMan](http://www.apiman.io/) documentation.

### OpenShift CLI Modifications
Modifications to the openshift client binary are a stretch goal for the initial integration.  It will be updated to allow a user to:
* Expose an API endpoint.  
Exposing the service will update the service annotation as described by the service annotation section.  Possible usage syntax:<p>
``` oc set api-service SERVICENAME --path PATH```<p>
The output of the command should return a route to the service.

* Hide an API endpoint.  
Hiding an API endpoint will remove the annotations from a service.  The change will additionally cause APIMan to remove this service endpoint.  Possible usage syntax:<p>
```oc unset api-service SERVICENAME```

### Deployment / Deployer pod
* APIMan will be deployed as a cluster level service for managing API service end points.
* APIMan will be deployed to reuse the existing ElasticSearch cluster.  Communication between APIMan and ElasticSearch will make use of CA certificate that was used for aggregated logging.  Modifications to the aggregated logging deployer will be made to save the CA as a secret.  This will allow the APIMan deployer to reuse the certificate to create client certificates.

## Concerns
### Scalability
There is a concern that reusing the aggregated logging ElasticSearch cluster could inhibit the APIMan gateway from retrieving its data in situations where there is large volume of logging traffic.  Performance testing will need to be conducted to confirm the impact of co-locating the data.  An alternative solution is to deploy a separate HA clustered back-end (e.g. second ElasticSearch instance) to strictly support APIMan.  Additionally, APIMan stores some metrics data that could be offloaded to the existing origin metrics<sup>[9](#r9)</sup> solution.  There is an existing RFE to investigate this change.

### Certificates
This proposal would allow the CA to be saved as a secret and reused as needed to create additional client certificates.  Further understanding on how this affects the OpenShift cluster security should be undertaken.  Alternatively, the aggregated logging deployer could be modified to mint an APIMan client certificate during its deployment. 

### Sticky Sessions
We need to better  understand and document if there is any change to the 'sticky session' behavior of the router.  It needs to be confirmed if there are any changes to functionality introduced by the APIMan gateway proxying requests to the service IP.

## References
* <span id="r1">[1]</span> APIMan - http://www.apiman.io/
* <span id="r2">[2]</span> OpenShift ElasticSearch plugin - https://github.com/fabric8io/openshift-elasticsearch-plugin
* <span id="r3">[3]</span> OpenShift Aggregated Logging Index Management - https://github.com/openshift/origin-aggregated-logging/pull/57
* <span id="r4">[4]</span> Origin Aggregated Logging - https://github.com/openshift/origin-aggregated-logging
* <span id="r5">[5]</span> Service Discover - https://github.com/kubernetes/kubernetes/blob/master/docs/proposals/service-discovery.md
* <span id="r6">[6]</span> Web Console Extensions - https://docs.openshift.org/latest/install_config/web_console_customization.html
* <span id="r7">[7]</span> Kibana API Discovery - https://github.com/openshift/origin/blob/master/assets/app/scripts/directives/logViewer.js#L337
* <span id="r8">[8]</span> OpenShift Auth Proxy - https://github.com/fabric8io/openshift-auth-proxy
* <span id="r9">[9]</span> Origin Metrics - https://github.com/openshift/origin-metrics