## Auto-Scaling

Auto-scaling is a feature that allows users to increase capacity as needed by controlling the number of replicas deployed 
within the system based on request statistics. Auto-scaling is bidirectional, it will scale up to a specified limit as 
requests increase and scale down to minimal capacity as the requests decrease.

### The Auto-Scaling Architecture

* `ReplicationController` - the building block of auto scaling.  Pods are deployed and scaled by a `ReplicationController`
* router - a request control point.  The routing layer handles outside to inside traffic requests and maintains statistics for the scaler
* kube proxy - a request control point.  The proxy handles internal traffic and maintains statistics for the scaler
* data aggregator - gathers data from control points
* auto-scaler - scales replicas up and down by increasing or decreasing the replication count of the `ReplicationController` according 
to scaling rules

### Auto-Scaling Data

Data used for auto scaling is based upon routes and the requests for those routes.  The auto-scaler cares about the following 
data points:

1.  network statistics (Requests Per Second, Sessions Per Second, Bytes In, Bytes Out, CPU Usage, Avg Request Duration)
1.  how the route relates to replication controllers (and by definition, the pods they control)
1.  whether or not the replication controller is auto-scalable and the thresholds set
1.  auto-scaling thresholds for requests, both up and down.

Example: A `ReplicationController` is configured to auto-scale up when it reaches a threshold of 50 requests per second that is sustained 
for at least 30 seconds.  

#### Data Retention

- we care about real time data and data patterns over time
- we only care about the route and summary statistics for that route.  More granular data used to calculate statistics 
is not important to retain and should be available in the router if needed
- statistics must be maintained over time to determine sustained requests thresholds being broken


### Data Aggregator

Data aggregation is a function of two components: the routers and the data aggregator itself.  

Routers will implement a function that will serve stats in a specified format.  This format will be the same for all implementations and allows 
the data aggregator to deal with each implementation in the same manner.  OpenShift will provide a base image for each 
supported implementation (HAProxy, NGINX, and Apache) that will provide the server install binary as well as a default 
implementation of stats gathering that works with the functions defined below.  
 
The data aggregator is responsible for periodically pinging each known router's function to gather the statistics for a 
time period (last request until now).  Then, if configured, the data aggregator may make a prediction about the expected 
traffic and store the statistics information so that the auto-scaler can respond by scaling applications up or down.

The data aggregator will be implemented as a controller and be given a registry to store statistics.

     type RouterStatsController struct {
          //Method to incrementally gather statistics in a loop.  Delegates to router implementation which may 
          //be socket based, http based, etc.
          GatherStats   func()
          //Injectable list of routers to deal with.  Injectable for testability
          RouterList    []router.Router
          //Algorithm for generating the next requests per second prediction
          GeneratePrediction func() float          
     }
     
In order to allow the stats aggregator to delegate statistics gathering details each router must provide a way to access 
its statistics.  First, the routers must be configured as proposed in the [Router Sharding Proposal](https://github.com/openshift/origin/pull/506). 
Each router will implement a function that serves to gather statistics.  The underlying implementation may vary from 
embedded endpoints that serve data to centralized storage locations that the function just reads from (while the router 
writes the data).

    type RouterStatistics struct {
        .... important info noted below, some already exists in reusable structures ....
        //application namespace
        Namespace
        //name of the app/repl controller/id used to uniquely identify what is being monitored in the namespace
        AppName
        //route being monitored
        RouteName    
        //interval start date/time
        StartTime time.Time
        //interval stop date/time
        StopTime time.Time
        //actual values used for scaling, populated from real time data
        Statistics Stats        
        //predicted values used for scaling
        Predictions Stats
    }
    
    type Stats struct{            
        RequestsPerSecond int
        SessionsPerSecond int
        BytesIn int
        BytesOut int
        CPUUsage int
        AvgRequestDuration int
    }

    type Router interface {
        .... fields omitted ....
        //pluggable implementation for retrieving stats from a router that can be called from the aggregator.  
        //This implementation may be reading from a centralized store or pinging a router directly through an exposed 
        //endpoint periodically.  It is up to each router implementation (or the base implementation) to provide the 
        //mechanism for providing stats.  
        Stats func() []RouterStatistics
    }        
    
#### Predictive Analysis
    
The stats aggregator may provide predictive analysis based on data trends.  When scaling, the auto-scaler may use both 
actual and predicted data to scale an application up.  The `GeneratePrediction` is a pluggable algorithm that will have two 
initial implementations: 

* No-op: make no predictive analysis, use actual values only, return `nil`
* Theil-Sen: make predictive analysis based on [Thiel-Sen](http://en.wikipedia.org/wiki/Theil%E2%80%93Sen_estimator)
    
### Auto-Scaler

The Auto-Scaler is a state reconciler responsible for checking predictions and real data against configured scaling thresholds 
and manipulating the `ReplicationControllers` to change the number of replicas an application is running.  The scaler will 
use a client/cache implementation to receive watch data from the `RouterStatsController` and respond to them by 
scaling the application.

     type AutoScaler struct {
        //Get the next statistic to check against thresholds
        NextStat func() RouterStatistics        
        //Adjust an application's repl controller replica count
        ScaleApplication(appInfo, numReplControllers) error
     }

### Auto-Scaling Configuration

Auto-scaling config parameters will be added to the `ReplicationController`

    type AutoScaleConfig struct {
        //turn auto scaling on or off
        Enabled boolean
        //max replicas that the auto scaler can use, empty is unlimited
        MaxAutoScaleCount int
        //min replicas that the auto scaler can use, empty == 0 (idle)
        //an idle configuration overrides the ReplicationControllerState.Replicas setting 
        MinAutoScaleCount int
        //Thresholds
        AutoScaleThresholds []AutoScaleThreshold
    }
    
    //RequestsPerSecond, SessionsPerSecond, BytesIn, BytesOut, CPUUsage, AvgRequestDuration
    type AutoScaleType string
    
    //holds the settings for when to scale an application
    //example: scale when RequestsPerSecond (type) are above 15 (value) for 10 minutes (duration)
    type AutoScaleThreshold struct {
        //scale based on this threshold
        type AutoScaleType
        //after this duration
        duration time.Duration
        //when this value is passed
        value int
    }    

    type ReplicationControllerState struct {
        AutoScaleConfig
    }
    
    type Route struct {
        .... fields omitted ....
        //selector for the replication controller that controls the underlying endpoints for the service
        //used for scaling the application up and down if auto-scaling is enabled.
        ReplicationControllerName string
    }

## Implementations

#### HAProxy Implementation
TODO 
http://cbonte.github.io/haproxy-dconv/configuration-1.4.html#9 (socat on a socket can be used to pull stats)

[example from Rajat using socat](https://github.com/rajatchopra/geard-router-haproxy/blob/rc/autoscaler.rb)

#### NGINX Implementation
TODO

#### Apache Implementation
TODO
mod_status http://httpd.apache.org/docs/2.2/mod/mod_status.html, http://www.tecmint.com/monitor-apache-web-server-load-and-page-statistics/, http://freecode.com/projects/apachetop/, http://www.opennms.org/wiki/Monitoring_Apache_with_the_HTTP_collector

#### Kube Proxy Implementation

TODO (will need to make stats implementation for proxy and register it as a non-replication controller controller
piece of infa - ie. do not try and manage these routers)

The intention is to have the OpenShift router be able to run in a mode that will allow it to replace the Kube Proxy in 
the future.  This proposal assumes that if we want to gather statistics from the Kube Proxy prior to the OpenShift router 
implementation we will be able to modify it to expose statistics like any other router.  This proposal also assumes 
that we will be able to automatically register the Kube Proxy as a non-controlled (not managed by Kubernetes, part of infra ) 
router and have the stats gathering mechanism ping it's endpoint like any other implementation.
