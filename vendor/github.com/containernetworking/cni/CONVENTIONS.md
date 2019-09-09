# Extension conventions                                                        
                                                                                 
There are three ways of passing information to plugins using the Container Network Interface (CNI), none of which require the [spec](SPEC.md) to be updated. These are 
- plugin specific fields in the JSON config
- `args` field in the JSON config
- `CNI_ARGS` environment variable 

This document aims to provide guidance on which method should be used and to provide a convention for how common information should be passed.
Establishing these conventions allows plugins to work across multiple runtimes. This helps both plugins and the runtimes.

## Plugins
* Plugin authors should aim to support these conventions where it makes sense for their plugin. This means they are more likely to "just work" with a wider range of runtimes.
* Plugins should accept arguments according to these conventions if they implement the same basic functionality as other plugins. If plugins have shared functionality that isn't coverered by these conventions then a PR should be opened against this document.

## Runtimes
* Runtime authors should follow these conventions if they want to pass additional information to plugins. This will allow the extra information to be consumed by the widest range of plugins.
* These conventions serve as an abstraction for the runtime. For example, port forwarding is highly implementation specific, but users should be able to select the plugin of their choice without changing the runtime.

# Current conventions
Additional conventions can be created by creating PRs which modify this document.

## Dynamic Plugin specific fields (Capabilities / Runtime Configuration)
[Plugin specific fields](https://github.com/containernetworking/cni/blob/master/SPEC.md#network-configuration) formed part of the original CNI spec and have been present since the initial release.
> Plugins may define additional fields that they accept and may generate an error if called with unknown fields. The exception to this is the args field may be used to pass arbitrary data which may be ignored by plugins.

A plugin can define any additional fields it needs to work properly. It should return an error if it can't act on fields that were expected or where the field values were malformed.

This method of passing information to a plugin is recommended when the following conditions hold:
* The configuration has specific meaning to the plugin (i.e. it's not just general meta data)
* the plugin is expected to act on the configuration or return an error if it can't

Dynamic information (i.e. data that a runtime fills out) should be placed in a `runtimeConfig` section. Plugins can request
that the runtime insert this dynamic configuration by explicitly listing their `capabilities` in the network configuration.

For example, the configuration for a port mapping plugin might look like this to an operator (it should be included as part of a [network configuration list](https://github.com/containernetworking/cni/blob/master/SPEC.md#network-configuration-lists).
```json
{
  "name" : "ExamplePlugin",
  "type" : "port-mapper",
  "capabilities": {"portMappings": true}
}
```

But the runtime would fill in the mappings so the plugin itself would receive something like this.
```json
{
  "name" : "ExamplePlugin",
  "type" : "port-mapper",
  "runtimeConfig": {
    "portMappings": [
      {"hostPort": 8080, "containerPort": 80, "protocol": "tcp"}
    ]
  }
}
```

### Well-known Capabilities
| Area  | Purpose | Capability | Spec and Example | Runtime implementations | Plugin Implementations |
| ----- | ------- | -----------| ---------------- | ----------------------- | ---------------------  |
| port mappings | Pass mapping from ports on the host to ports in the container network namespace. | `portMappings` | A list of portmapping entries.<br/>  <pre>[<br/>  { "hostPort": 8080, "containerPort": 80, "protocol": "tcp" },<br />  { "hostPort": 8000, "containerPort": 8001, "protocol": "udp" }<br />  ]<br /></pre> | kubernetes | CNI `portmap` plugin |
| ip ranges | Dynamically configure the IP range(s) for address allocation. Runtimes that manage IP pools, but not individual IP addresses, can pass these to plugins. | `ipRanges` | The same as the `ranges` key for `host-local` - a list of lists of subnets. The outer list is the number of IPs to allocate, and the inner list is a pool of subnets for each allocation. <br/><pre>[<br/> [<br/>  { "subnet": "10.1.2.0/24", "rangeStart": "10.1.2.3", "rangeEnd": 10.1.2.99", "gateway": "10.1.2.254" } <br/>  ]<br/>]</pre> | none | CNI `host-local` plugin |
| bandwidth limits | Dynamically configure interface bandwidth limits | `bandwidth` | Desired bandwidth limits. Rates are in bits per second, burst values are in bits. <pre> { "ingressRate": 2048, "ingressBurst": 1600, "egressRate": 4096, "egressBurst": 1600 } </pre> | none | CNI `bandwidth` plugin |
| Dns | Dymaically configure dns according to runtime | `dns` | Dictionary containing a list of `servers` (string entries), a list of `searches` (string entries), a list of `options` (string entries). <pre>{ <br> "searches" : [ "internal.yoyodyne.net", "corp.tyrell.net" ] <br> "servers": [ "8.8.8.8", "10.0.0.10" ] <br />} </pre> | kubernetes | CNI `win-bridge` plugin, CNI `win-overlay` plugin |


## "args" in network config
`args` in [network config](https://github.com/containernetworking/cni/blob/master/SPEC.md#network-configuration) were introduced as an optional field into the `0.2.0` release of the CNI spec. The first CNI code release that it appeared in was `v0.4.0`. 
> args (dictionary): Optional additional arguments provided by the container runtime. For example a dictionary of labels could be passed to CNI plugins by adding them to a labels field under args.

`args` provide a way of providing more structured data than the flat strings that CNI_ARGS can support.

`args` should be used for _optional_ meta-data. Runtimes can place additional data in `args` and plugins that don't understand that data should just ignore it. Runtimes should not require that a plugin understands or consumes that data provided, and so a runtime should not expect to receive an error if the data could not be acted on.

This method of passing information to a plugin is recommended when the information is optional and the plugin can choose to ignore it. It's often that case that such information is passed to all plugins by the runtime without regard for whether the plugin can understand it.

The conventions documented here are all namespaced under `cni` so they don't conflict with any existing `args`.

For example:
```json
{  
   "cniVersion":"0.2.0",
   "name":"net",
   "args":{  
      "cni":{  
         "labels": [{"key": "app", "value": "myapp"}]
      }
   },
   <REST OF CNI CONFIG HERE>
   "ipam":{  
     <IPAM CONFIG HERE>
   }
}
```

| Area  | Purpose| Spec and Example | Runtime implementations | Plugin Implementations |
| ----- | ------ | ------------     | ----------------------- | ---------------------- |
| labels | Pass`key=value` labels to plugins | <pre>"labels" : [<br />  { "key" : "app", "value" : "myapp" },<br />  { "key" : "env", "value" : "prod" }<br />] </pre> | none | none |
| ips   | Request static IPs | Spec:<pre>"ips": ["\<ip\>[/\<prefix\>]", ...]</pre>Examples:<pre>"ips": ["10.2.2.42/24", "2001:db8::5"]</pre>The plugin may require the IP address to include a prefix length. | none | host-local |

## CNI_ARGS
CNI_ARGS formed part of the original CNI spec and have been present since the initial release.
> `CNI_ARGS`: Extra arguments passed in by the user at invocation time. Alphanumeric key-value pairs separated by semicolons; for example, "FOO=BAR;ABC=123"

The use of `CNI_ARGS` is deprecated and "args" should be used instead. If a runtime passes an equivalent key via `args` (eg the `ips` `args` Area and the `CNI_ARGS` `IP` Field) and the plugin understands `args`, the plugin must ignore the CNI_ARGS Field.

| Field  | Purpose| Spec and Example | Runtime implementations | Plugin Implementations |
| ------ | ------ | ---------------- | ----------------------- | ---------------------- |
| IP     | Request a specific IP from IPAM plugins | Spec:<pre>IP=\<ip\>[/\<prefix\>]</pre>Example: <pre>IP=192.168.10.4/24</pre>The plugin may require the IP addresses to include a prefix length. | *rkt* supports passing additional arguments to plugins and the [documentation](https://coreos.com/rkt/docs/latest/networking/overriding-defaults.html) suggests IP can be used. | host-local (since version v0.2.0) supports the field for IPv4 only - [documentation](https://github.com/containernetworking/plugins/tree/master/plugins/ipam/host-local#supported-arguments).|

## Chained Plugins
If plugins are agnostic about the type of interface created, they SHOULD work in a chained mode and configure existing interfaces. Plugins MAY also create the desired interface when not run in a chain.

For example, the `bridge` plugin adds the host-side interface to a bridge. So, it should accept any previous result that includes a host-side interface, including `tap` devices. If not called as a chained plugin, it creates a `veth` pair first.

Plugins that meet this convention are usable by a larger set of runtimes and interfaces, including hypervisors and DPDK providers.
