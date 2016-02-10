# APIMan Integration

## Abstract
This proposes a design for integrating [APIMan](http://www.apiman.io/) with OpenShift inorder to provide  micro-services governance for API service providers.

## Motivation
Providers of services may have the need to control who and how their service is consumed.  This may be realized as policies defined by providers to control the service
in any number of ways (e.g security, throttling/quota, billing and metrics)<sup>[1](#1)</sup>. 

## Constraints and Assumptions
### Authentication
The OAuth token identifying a user in OpenShift will be utilized by APIMan to associate service policy to their identity.  This will ensure a user's identity is 
ubiquitous across the cluster when interacting with OpenShift and the APIMan management console.

### Deployment
#### Communication
Components integrated with the cluster will utilize mutual TLS for communication.

#### On-Cluster Policy Storage
[Origin Aggregated logging](https://github.com/openshift/origin-aggregated-logging)<sup>[4](#4)</sup> will be deployed before APIMan in order to utilize the existing ElasticSearch cluster for policy storage.  
The [ACL plugin](https://github.com/fabric8io/openshift-elasticsearch-plugin)<sup>[2](#2)</sup> that is deployed
as part of aggregated logging will be updated to allow access from the gateway.  ElasicSearch index management<sup>[3](#3)</sup>  will be modified so
that it will not cull APIMan policy data.

#### Off-Cluster Policy Storage
Cluster administrators can configure APIMan to use an alternate ElasticSearch cluster if desired.  Some organizations may have an existing ElasticSearch cluster that already defines
policy, and adminstrators may desire to use it instead of the one provided with aggregated logging.  It is assumed in this configuration, the APIMan management interface will
additionally rely on Kibana deployed off-cluster.  The multi-tenant features provided by aggregated logging for ElasticSearch and Kibana may not be available when configured to utilize these off-cluster
dependencies.      

## Use Cases
* **UC01** As an API provider, I want to navigate from the OpenShift web console to the policy management interface, so I can define my service policies.
* **UC02** As an API provider, I want to associate an APIMan organization with an OpenShift project, so I can control policy between organizations.
* **UC03** As an API provider, I want to expose my service, so that it is consumable according to my management policy.  
* **UC04** As a cluster administrator, I want a gateway to route API traffic only, so that it manages traffic based on a service's policy.
* **UC05** As a cluster administrator, I want to deploy APIMan and its components reusing existing infrastructure components where possible (e.g. ElasticSearch), so I can minimize infrastructure components.
* **UC06** As a cluster administrator, I want API management interface to be able to discover the route to Kibana, so it can be linked to from the management interface. 
* **UC07** As an API consumer, I want to configure my services to consume other services

## Specification

## Project Annotations
add annotation keys an sample here
## Service Annotations (and labels?)
add sample here to upstream work
do we need labels to generate backlink from apiman management to console service?

### Origin Web Console UI extension
UI exension that creates link to apiman management with context based on project/service annotations

### OpenShift CLI Modifications
service annotation command? 

### Deployment / Deployer pod

## Rationale
The technical rationale fleshes out the specification by describing what motivated the design and why particular design decisions were made. It should describe alternate designs that were considered and related work, e.g. how the feature is supported in other products.
The rationale should provide evidence of consensus within the community and discuss important objections or concerns raised during discussion.

OpenShift existing arch allows for deploying additional routers 

## Limitations
scalability - 
Cert generation
- defer to a deployer pod
- need to redeploy all of logging to reuse ES
Project creation post-hook - (Annotate projects based on APIMan orgs? identity?) Can we do this as part of the impl?

## References
* <span id="1">[1]</span> APIMan - http://www.apiman.io/
* <span id="2">[2]</span> OpenShift ElasticSearch plugin - https://github.com/fabric8io/openshift-elasticsearch-plugin
* <span id="3">[3]</span> OpenShift Aggregated Logging Index Management - https://github.com/openshift/origin-aggregated-logging/pull/57
* <span id="4">[4]</span> Origin Aggregated Logging - https://github.com/openshift/origin-aggregated-logging