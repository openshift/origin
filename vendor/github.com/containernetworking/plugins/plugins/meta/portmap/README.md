## Port-mapping plugin

This plugin will forward traffic from one or more ports on the host to the
container. It expects to be run as a chained plugin.

## Usage
You should use this plugin as part of a network configuration list. It accepts
the following configuration options:

* `snat` - boolean, default true. If true or omitted, set up the SNAT chains
* `conditionsV4`, `conditionsV6` - array of strings. A list of arbitrary `iptables` 
matches to add to the per-container rule. This may be useful if you wish to 
exclude specific IPs from port-mapping

The plugin expects to receive the actual list of port mappings via the 
`portMappings` [capability argument](https://github.com/containernetworking/cni/blob/master/CONVENTIONS.md)

So a sample standalone config list (with the file extension .conflist) might
look like:

```json
{
        "cniVersion": "0.3.1",
        "name": "mynet",
        "plugins": [
                {
                        "type": "ptp",
                        "ipMasq": true,
                        "ipam": {
                                "type": "host-local",
                                "subnet": "172.16.30.0/24",
                                "routes": [
                                        {
                                                "dst": "0.0.0.0/0"
                                        }
                                ]
                        }
                },
                {
                        "type": "portmap",
                        "capabilities": {"portMappings": true},
                        "snat": false,
                        "conditionsV4": ["!", "-d", "192.0.2.0/24"],
                        "conditionsV6": ["!", "-d", "fc00::/7"]
                }
        ]
}
```



## Rule structure
The plugin sets up two sequences of chains and rules - one "primary" DNAT
sequence to rewrite the destination, and one additional SNAT sequence that
rewrites the source address for packets from localhost. The sequence is somewhat
complex to minimize the number of rules non-forwarded packets must traverse.


### DNAT
The DNAT rule rewrites the destination port and address of new connections.
There is a top-level chain, `CNI-HOSTPORT-DNAT` which is always created and
never deleted. Each plugin execution creates an additional chain for ease
of cleanup. So, if a single container exists on IP 172.16.30.2 with ports 
8080 and 8043 on the host forwarded to ports 80 and 443 in the container, the 
rules look like this:

`PREROUTING`, `OUTPUT` chains:
- `--dst-type LOCAL -j CNI-HOSTPORT-DNAT`

`CNI-HOSTPORT-DNAT` chain:
- `${ConditionsV4/6} -j CNI-DN-xxxxxx` (where xxxxxx is a function of the ContainerID and network name)

`CNI-DN-xxxxxx` chain: 
- `-p tcp --dport 8080 -j DNAT --to-destination 172.16.30.2:80`
- `-p tcp --dport 8043 -j DNAT --to-destination 172.16.30.2:443`

New connections to the host will have to traverse every rule, so large numbers
of port forwards may have a performance impact. This won't affect established
connections, just the first packet.

### SNAT 
The SNAT rule enables port-forwarding from the localhost IP on the host.
This rule rewrites (masquerades) the source address for connections from
localhost. If this rule did not exist, a connection to `localhost:80` would
still have a source IP of 127.0.0.1 when received by the container, so no 
packets would respond. Again, it is a sequence of 3 chains. Because SNAT has to
occur in the `POSTROUTING` chain, the packet has already been through the DNAT
chain.

`POSTROUTING`:
- `-s 127.0.0.1 ! -d 127.0.0.1 -j CNI-HOSTPORT-SNAT`

`CNI-HOSTPORT-SNAT`:
- `-j CNI-SN-xxxxx`

`CNI-SN-xxxxx`:
- `-p tcp -s 127.0.0.1 -d 172.16.30.2 --dport 80 -j MASQUERADE`
- `-p tcp -s 127.0.0.1 -d 172.16.30.2 --dport 443 -j MASQUERADE`

Only new connections from the host, where the source address is 127.0.0.1 but
not the destination will traverse this chain. It is unlikely that any packets
will reach these rules without being SNATted, so the cost should be minimal.

Because MASQUERADE happens in POSTROUTING, it means that packets with source ip
127.0.0.1 need to pass a routing boundary. By default, that is not allowed
in Linux. So, need to enable the sysctl `net.ipv4.conf.IFNAME.route_localnet`,
where IFNAME is the name of the host-side interface that routes traffic to the
container.

There is no equivalent to `route_localnet` for ipv6, so SNAT does not work
for ipv6. If you need port forwarding from localhost, your container must have
an ipv4 address.


## Known issues
- ipsets could improve efficiency
- SNAT does not work with ipv6.
