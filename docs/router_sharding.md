- how is router configuration visualized from a user perspective
- how is router configuration visualized from an admin perspective
- how is a user notified of a route route binding and final dns
- how does a user request default dns name vs custom dns name
- router fronting with DNS, how are entries created

## Description

As an application administrator, I would like my routes to be configured with shards so they can
grow beyond a single active/active or active/passive setup.  I should be able to configure many
routers to allocate user requested routes to and be able to visualize the configuration.  

## Use Cases

The following use cases should be satisfied by this proposal:

1.  Configure routers as OpenShift resources and let the platform keep the specified configuration
    running
1.  Create a single, unsharded router
1.  Create multiple routers with shards corresponding to a resource label
1.  Allow any router to run in an HA configuration
1.  User requests default route for application
1.  User requests custom route for application
1.  Create DNS (or other front end entry points) for routers

## Existing Artifacts

1.  Routing: https://github.com/pweil-/origin/blob/master/docs/routing.md
1.  HA Routing: https://github.com/pweil-/origin/blob/master/docs/routing.md#running-ha-routers
1.  DNS Round Robin: https://github.com/pweil-/origin/blob/master/docs/routing.md#dns-round-robin

## Configuring Routers

Administering routers as a top level object allows administrators to use custom commands specific
to routers.  This provides a more use friendly mechanism of configuration and customizing routers.
However, this also introduces more code for an object that will likely be dealt with as a pod
anyway.  Routers should be a low touch configuration item that do not require many custom commands
for daily administration.

Pros:

- Configuration lives in etcd, just like any other resource
- Shards are configured via custom commands and `json` syntax
- Routers are known to OpenShift; the system ensures the proper configuration is running
- Custom administration syntax
- Deal with routers as infra
- The system knows about routers for route route binding and visualization with no extra effort

Cons: 

- More divergent from Kubernetes codebase initially, though we may be able to generalize parts of
  this approach to sharding to other resources and controllers which allow sharding

## User Requests a Route

Requesting a route is a multi-step process that includes the initial user request, router
allocation, and router configuration.  OpenShift does not provide DNS services for users who own
their own domain, users who own their own domain should point their domain name to the allocated
shard(s) for resolution.

When requesting a route the user has two options.  

1.  Requesting a specific route name in `Route.Host`: This indicates that the user owns the domain.
    The system should not manipulate the requested name but should ensure uniqueness against the
    existing routes.
2.  Requesting a route with no name specified in `Route.Host`: This indicates that the user would
    like to have system provided DNS.  The `RouteScheduler` will create a name in the format of 
    `<namespace>-<Host>.<shard>.v3.rhcloud.com` and populate the `DNS` field of the route upon 
    completion.

## DNS

OpenShift will not provide custom DNS to clients.  System provided DNS will be achieved by using a
DNS plugin or manual setup that is aware of the configured router shards.  The DNS implementation
will be set up with a wild card DNS zone for each router shard.  Below is an example of the zone
files of a router configuration with two shards.

If a plugin infrastructure is created it will be able to watch the `router` configuration to 
determine the correct zone files to set up with wildcard entries.

    
    shard1.zone:
    $ORIGIN shard1.v3.rhcloud.com.
    
    @       IN      SOA     . shard1.v3.rhcloud.com. (
                         2009092001         ; Serial
                             604800         ; Refresh
                              86400         ; Retry
                            1206900         ; Expire
                                300 )       ; Negative Cache TTL
            IN      NS      ns1.v3.rhcloud.com.
    ns1     IN      A       127.0.0.1
    *       IN      A       10.245.2.2      ; active/active DNS round robin
            IN      A       10.245.2.3      ; active/active DNS round robin
            
    shard2.zone:
    $ORIGIN shard2.v3.rhcloud.com.
    
    @       IN      SOA     . shard2.v3.rhcloud.com. (
                         2009092001         ; Serial
                             604800         ; Refresh
                              86400         ; Retry
                            1206900         ; Expire
                                300 )       ; Negative Cache TTL
            IN      NS      ns1.v3.rhcloud.com.
    ns1     IN      A       127.0.0.1
    *       IN      A       10.245.2.4      ; active/active DNS round robin
            IN      A       10.245.2.5      ; active/active DNS round robin 
                       
