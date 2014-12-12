# Idling OpenShift Pods
## Problem

Not all containers in an OpenShift cluster will be active at all times.  Stopping inactive containers and creating new ones in response to demand allows for greater deployment densities. Removing unused pods from nodes also simplifies administration.  We refer to the process of determining which containers to stop and stopping them as "idling".  The process of creating new pods for an idled service in response to a new request to that service is called "unidling".

## Use Cases

The following use cases should be explored by this proposal:

1.  As a PaaS operator, I want pods associated with a service receiving no requests to be idled
2.  As a PaaS operator, I want a new request to an idled service to trigger the creation of new pods for that service
3.  As an application author, I want requests to my idled services to be buffered while the service is being unidled
3.  As a PaaS operator, I want to define how events (eg, deployment of new versions) to idled services are handled 

## The `Idler` State Reconciler

The `Idler` consumes request statistics from the edge routing layer and determines whether which services are receiving no requests currently and can be idled.  The `Idler` will set the replica count of the correct replication controller(s?) to zero to trigger idling.

## Edge Routing Layer Concerns

When the edge router detects that a service has been idled (has zero endpoints), it must configure the backend to send requests for that service to the kube-proxy to be unidled.  This will require the edge router backend implementations to contain logic for handling services with zero endpoints.

## The role of the kube-proxy

The kube-proxy is an ideal candidate to implement the unidling behavior for the following reasons:

1.  It is a true network proxy and handles packets directly
2.  It does not cut across multiple backend router/proxy packages

### HA configuration

TODO (Andy?)

## Where should unidling be triggered?

Our prior work on geard investigated using systemd socket activation as an unidle trigger.  The key challenge with this approach is that it requires the interface/ports has to be defined before a pod is started, which today is the pod being scheduled to the host. However, the pod has to be stopped which complicates resource scheduling. In this model the scheduler cannot make resource decisions without double checking to see if pods have been unidled, and potentially you would wake too many things. 

The edge routing layer receives endpoint information for services in order to load balance across the endpoints for that service.  This makes the edge routing layer an attractive candidate to drive the unidle process because it already will have all of the information it needs. This will also allow idled pods to be deleted since unidling can occur from a state where no pods exist for a service and has less effects on resource scheduling than socket activation.  

## Example Unidle workflow

1.  Request is made to the edge router for an idled service; the edge router forwards the request to the kube-proxy
2.  The kube-proxy buffers the request while unidling the service by changing the replica count
3.  The kube-proxy waits until there is a scheduled endpoint for the service and forwards the request directly to the first available endpoint (NOTE: this defeats load balancing, but if that's important the kube-proxy could forward back to the edge)
