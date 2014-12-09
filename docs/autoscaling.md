## Abstract
Auto-scaling is a data-driven feature that allows users to increase or decrease capacity as needed by controlling the number of replicas deployed 
within the system automatically.  

## Motivation

Applications experience peaks and valleys in usage.  In order to respond to increases and decreases in load administrators 
scale their applications by adding computing resources.  In the cloud computing environment this can be 
done automatically based on statistical analysis and thresholds.

### Goals

* Provide a concrete proposal for implementing auto-scaling components within Kubernetes
    * Implementation proposal should be in line with current discussions in existing issues: 
    * Resize verb - [1629](https://github.com/GoogleCloudPlatform/kubernetes/issues/1629)
    * Config conflicts - [Config](https://github.com/GoogleCloudPlatform/kubernetes/blob/c7cb991987193d4ca33544137a5cb7d0292cf7df/docs/config.md#automated-re-configuration-processes)
    * Rolling updates - [1353](https://github.com/GoogleCloudPlatform/kubernetes/issues/1353)
    * Multiple scalable types - [1624](https://github.com/GoogleCloudPlatform/kubernetes/issues/1624)  
* Document the currently known use cases

## Constraints and Assumptions

* The auto-scale component will not be part of a replication controller 
* Data gathering semantics will not be part of an auto-scaler but an auto-scaler may use data to perform threshold checking
* Auto-scalable resources will support a resize verb ([1629](https://github.com/GoogleCloudPlatform/kubernetes/issues/1629)) 
such that the auto-scaler does not directly manipulate the underlying resource.
* Thresholds will be set by the application administrator
* The auto-scaler must be aware of user defined actions so it does not override them unintentionally (for instance someone 
explicitly setting the replica count to 0 should mean that the auto-scaler does not try to scale the application up)

## Use Cases

### Scaling based on traffic

The current, most obvious use case, is scaling an application based on traffic.  Within the Kubernetes ecosystem there 
are routing layers that serve to direct requests to underlying `endpoints`.  These routing layers are good examples of 
candidates to provide data to the auto-scaler.

Within Kubernetes a [kube proxy](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/services.md#ips-and-portals) 
running on each node directs service requests to the underlying implementation.  

External to the Kubernetes core infrastructure (but still within the Kubernetes ecosystem) lies the OpenShift routing layer.  
OpenShift routers are `pods` with externally exposed IP addresses that are used to map service requests from the external 
world to internal `endpoints` via user defined host aliases known as `routes`.  

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
* kube proxy - a request control point.  The proxy handles internal inter-pod traffic
* router - an OpenShift request control point.  The routing layer handles outside to inside traffic requests
* auto-scaler - scales replicas up and down by using the `resize` endpoint provided by scalable resources (`ReplicationController`)


### Auto-Scaler

The Auto-Scaler is a state reconciler responsible for checking data against configured scaling thresholds 
and calling the `resize` endpoint to change the number of replicas.  The scaler will 
use a client/cache implementation to receive watch data from the data aggregators and respond to them by 
scaling the application.  Auto-scalers are created and defined like other resources via REST endpoints and belong to the 
namespace just as a `ReplicationController` or `Service`.

    //The auto scaler interface
    type AutoScalerInterface interface {        
        //Adjust a resource's replica count.  Calls resize endpoint
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
                       
        //the selector that provides the scaler with access to the scalable component
        Selector string
     }
     
     
     //abstracts the data analysis from the auto-scaler
     //example: scale when RequestsPerSecond (type) are above 50 (value) for 30 seconds (duration)
     type AutoScaleThresholdInterface interface {
        //called by the auto-scaler to determine if this threshold is met or not
        ShouldScale() boolean
     }
        
     //generic type definition
     type AutoScaleThreshold struct {
         //scale based on this threshold (see below for definition)
         Type Statistic
         //after this duration
         Duration time.Duration
         //when this value is passed
         Value float
     } 

### Data Aggregator

Data aggregation is opaque to the the auto-scaler resource.  The auto-scaler is configured to use `AutoScaleThresholds` 
that know how to work with the underlying data in order to know if an application must be scaled up or down.   Data aggregation 
must feed a common data structure to ease the development of `AutoScaleThreshold`s but it does not matter to the 
auto-scaler whether this occurs in a push or pull implementation.  For the purposes of this design I will propose a solution 
for the existing routing layers that uses a pull mechanism.


    //common statistics type for monitoring routers that can be used by threshold implementations
    type Statistics struct {
        //resource type that this statistic belongs to: router, job, etc
        ResourceType string
        //resource name that stats are being reported for
        ResourceName string
        //reporter name, to indicate where the statistics came from.
        ReporterName string
        //interval start date/time
        StartTime time.Time
        //interval stop date/time
        StopTime time.Time
        //the statistics
        Stats map[Statistic]float
    }
    
    //some initial stat types geared toward the routing layer
    type Statistic string
    
    const (
        RequestsPerSecond Statistic = "requestPerSecond"
        SessionsPerSecond Statistic = "sessionsPerSecond"
        BytesIn Statistic = "bytesIn"
        BytesOut Statistic = "bytesOut"
        CPUUsage Statistic = "cpuUsage"
        AvgRequestDuration Statistic = "avgRequestDuration"
    )


    //implementation for routing layers specified in use cases above
    type StatsGatherer interface {
        GatherStats() []Statistics
    }
    
    //Gather stats from the proxy, uses configured minions to find proxies
    type KubeProxyStatsController struct {}
    
    //OpenShift specific
    type RouterStatsController struct {
        //GatherStats delegates to router implementation which may be socket based, http based, etc.
        RouterList []router.Router
    }

Not shown is the initialization of a `StatsGatherer`.  When creating a `StatsGatherer` a registry will be given so that 
the gatherer can save data that the `AutoScaleThreshold`s act upon.  This means that other services storing statistics 
potentially can piggyback in this registry.


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
         
1.  System creates new auto-scaler with defined thresholds.  
1.  Periodically the system loops through defined thresholds calling `threshold.ShouldScale()`
1.  The threshold looks for the `requestPerSecond` statistic for `myapp-rps` in the configured registry
1.  The threshold compares the historical data and current data and determines if the app should be scaled
1.  If the app must be scaled the auto-scaler calls the `resize` endpoint for `myapp-replcontroller`
    
