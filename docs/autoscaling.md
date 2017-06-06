## Abstract
Auto-scaling is a data-driven feature that allows users to increase or decrease capacity as needed by controlling the 
number of pods deployed within the system automatically.  

## Motivation

Applications experience peaks and valleys in usage.  In order to respond to increases and decreases in load administrators 
scale their applications by adding computing resources.  In the cloud computing environment this can be 
done automatically based on statistical analysis and thresholds.

### Goals

* Provide a concrete proposal for implementing auto-scaling pods within Kubernetes
* Implementation proposal should be in line with current discussions in existing issues: 
    * Resize verb - [1629](https://github.com/GoogleCloudPlatform/kubernetes/issues/1629)
    * Config conflicts - [Config](https://github.com/GoogleCloudPlatform/kubernetes/blob/c7cb991987193d4ca33544137a5cb7d0292cf7df/docs/config.md#automated-re-configuration-processes)
    * Rolling updates - [1353](https://github.com/GoogleCloudPlatform/kubernetes/issues/1353)
    * Multiple scalable types - [1624](https://github.com/GoogleCloudPlatform/kubernetes/issues/1624)  

## Constraints and Assumptions

* This proposal is for horizontal scaling only.  Vertical scaling will be handled in by [issue 2072](https://github.com/GoogleCloudPlatform/kubernetes/issues/2072)
* `ReplicationControllers` will not know about the auto-scaler, they are the target of the auto-scaler.  The `ReplicationController` responsibilities are 
constrained to only ensuring that the desired number of pods are operational per the [Replication Controller Design](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/replication-controller.md#responsibilities-of-the-replication-controller)
* Auto-scalers will be loosely coupled with data gathering components in order to allow a wide variety of input sources
* Auto-scalable resources will support a resize verb ([1629](https://github.com/GoogleCloudPlatform/kubernetes/issues/1629)) 
such that the auto-scaler does not directly manipulate the underlying resource.
* Initially, most thresholds will be set by application administrators. It should be possible for an autoscaler to be 
written later that sets thresholds automatically based on past behavior (CPU used vs incoming requests).
* The auto-scaler must be aware of user defined actions so it does not override them unintentionally (for instance someone 
explicitly setting the replica count to 0 should mean that the auto-scaler does not try to scale the application up)
* It should be possible to write and deploy a custom auto-scaler without modifying existing auto-scalers

## Use Cases

### Scaling based on traffic

The current, most obvious use case, is scaling an application based on network traffic like requests per second.  Most 
applications will expose one or more network endpoints for clients to connect to. Many of those endpoints will be load 
balanced or situated behind a proxy - the data from those proxies and load balancers can be used to estimate client to 
server traffic for applications. This is the primary, but not sole, source of data for making decisions.

Within Kubernetes a [kube proxy](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/services.md#ips-and-portals) 
running on each node directs service requests to the underlying implementation.  

While the proxy provides internal inter-pod connections, there will be L3 and L7 proxies and load balancers that manage 
traffic to backends. OpenShift, for instance, adds a "route" resource for defining external to internal traffic flow. 
The "routers" are HAProxy or Apache load balancers that aggregate many different services and pods and can serve as a 
data source for the number of backends.

### Scaling based on predictive analysis

Scaling may also occur based on predictions of system state like anticipated load, historical data, etc.  Hand in hand 
with scaling based on traffic, predictive analysis may be used to determine anticipated system load and scale the application automatically.  

### Scaling based on arbitrary data

Administrators may wish to scale the application based on any number of arbitrary data points such as job execution time or
duration of active sessions.  There are any number of reasons an administrator may wish to increase or decrease capacity which 
means the auto-scaler must be a configurable, extensible component.

## Specification

In order to facilitate talking about auto-scaling the following definitions are used:

* `ReplicationController` - the first building block of auto scaling.  Pods are deployed and scaled by a `ReplicationController`.
* kube proxy - The proxy handles internal inter-pod traffic, an example of a data source to drive an auto-scaler
* L3/L7 proxies - A routing layer handling outside to inside traffic requests, an example of a data source to drive an auto-scaler
* auto-scaler - scales replicas up and down by using the `resize` endpoint provided by scalable resources (`ReplicationController`)


### Auto-Scaler

The Auto-Scaler is a state reconciler responsible for checking data against configured scaling thresholds 
and calling the `resize` endpoint to change the number of replicas.  The scaler will 
use a client/cache implementation to receive watch data from the data aggregators and respond to them by 
scaling the application.  Auto-scalers are created and defined like other resources via REST endpoints and belong to the 
namespace just as a `ReplicationController` or `Service`.

There are two options for implementing the auto-scaler:

1.  Annotations on a `ReplicationController`
    
    Pros:
        
      * uses an existing resource, not another component that must be defined separately
      * easy to know what the target of the auto-scaler is since the config for the scaler is attached to the target      
      
    Cons:
    
      * Configuration in annotations is marginally more difficult than plain old json.  
      * Rather than watching explicitly for new auto-scaler definitions, the auto-scaler controller must watch all 
      `ReplicationController`s and create auto-scalers when appropriate.  As new, auto-scalable resources are defined the 
      auto-scaler controller must also watch those resources.
      
1.  As a new resource
    
    Pros:
        
      * auto-scalers are managed by the user independent of the `ReplicationController`
      * flexible by using a selector to the scalable resource (that implements the `resize` verb), future implementations 
      *may* require no extra work on the auto-scaler side
      
    Cons:
    
      * one more resource to store, manage, and monitor
  
For this proposal, the auto-scaler is a resource:

    //The auto scaler interface
    type AutoScalerInterface interface {        
        //Adjust a resource's replica count.  Calls resize endpoint.  Args to this are based on what the endpoint
        //can support.  See https://github.com/GoogleCloudPlatform/kubernetes/issues/1629
        ScaleApplication(num int) error
    }

    type AutoScaler struct {       
        //Thresholds
        AutoScaleThresholds []AutoScaleThreshold
        
        //turn auto scaling on or off
        Enabled boolean 
        //max replicas that the auto scaler can use, empty is unlimited
        MaxAutoScaleCount int
        //min replicas that the auto scaler can use, empty == 0 (idle) 
        MinAutoScaleCount int 
                       
        //the label selector that points to a resource implementing the resize verb.  Right now this is a ReplicationController
        //in the future it could be a job or any resource that implements resize
        ScalableTargetSelector string
     }
     
     
     //abstracts the data analysis from the auto-scaler
     //example: scale when RequestsPerSecond (type) are above 50 (value) for 30 seconds (duration)
     type AutoScaleThresholdInterface interface {
        //called by the auto-scaler to determine if this threshold is met or not
        ShouldScale() boolean
     }
      
     type StatisticType string
        
     //generic type definition
     type AutoScaleThreshold struct {
         //scale based on this threshold (see below for definition)
         //example: RequestsPerSecond StatisticType = "requestPerSecond"
         Type StatisticType
         //after this duration
         Duration time.Duration
         //when this value is passed
         Value float
     } 

### Data Aggregator

This section has intentionally been left empty.  I will defer to folks who have more experience gathering and analyzing 
time series statistics.  

Data aggregation is opaque to the the auto-scaler resource.  The auto-scaler is configured to use `AutoScaleThresholds` 
that know how to work with the underlying data in order to know if an application must be scaled up or down.   Data aggregation 
must feed a common data structure to ease the development of `AutoScaleThreshold`s but it does not matter to the 
auto-scaler whether this occurs in a push or pull implementation, whether or not the data is stored at a granular level,
or what algorithm is used to determine the final statistics value.  Ultimately, the auto-scaler only requires that a statistic 
resolves to a value that can be checked against a configured threshold.

Of note: If the statistics gathering mechanisms can be initialized with a registry other components storing statistics can
potentially piggyback on this registry.


## Use Case Realization

### Scaling based on traffic

1.  User defines the application's auto-scaling resources    
    
         {
            "id": "myapp-autoscaler",
            "kind": "AutoScaler",
            "apiVersion": "v1beta1",
            "maxAutoScaleCount": 50,
            "minAutoScaleCount": 1,
            "thresholds": [
                {
                    "id": "myapp-rps",
                    "kind": "AutoScaleThreshold",
                    "type": "requestPerSecond", 
                    "durationVal": 30,
                    "durationInterval": "seconds",
                    "value": 50,
                }
            ],
            "selector": "myapp-replcontroller"
         }
         
1.  The auto-scaler controller watches for new `AutoScaler` definitions and creates the resource   
1.  Periodically the auto-scaler loops through defined thresholds and determine if a threshold has been exceeded
1.  If the app must be scaled the auto-scaler calls the `resize` endpoint for `myapp-replcontroller`
    
