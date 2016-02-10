# Docker-Registry Images On GlusterFS

### Assumptions

  * OSE 3.x
  * GlusterFS volume Created and Started
  * glusterfs-client installed on all Nodes
  
### DNS Configuration

Before we can initiate the docker-registry, the dnsmasq.service and the openshift DNS collision on port 53 must be corrected.

##### Edit /etc/dnsmasq.conf

On the master, edit /etc/dnsmasq.conf, adding:
```bash
# Reverse DNS record for master
host-record=<MASTER FQDN>,<MASTER IP>
# Wildcard DNS for OpenShift Applications - Points to Router
address=/apps.<MASTER FQDN>/<MASTER IP>
# Forward .local queries to SkyDNS
server=/local/127.0.0.1#8053
# Forward reverse queries for service network to SkyDNS.
# This is for default OpenShift SDN - change as needed.
server=/17.30.172.in-addr.arpa/127.0.0.1#8053
```
And uncommenting:
```bash
# Do not read /etc/resolv.conf and forward requests
# to nameservers listed there:
no-resolv
# Never forward plain names (without a dot or domain part)
domain-needed
# Never forward addresses in the non-routed address spaces.
bogus-priv
```

##### Edit /etc/origin/master/master-config.yaml
 
Change
```
dnsConfig:
   bindAddress: 0.0.0.0:53
```
to
```
dnsConfig:
        bindAddress: 127.0.0.1:8053
```

On all nodes, edit /etc/resolv.conf
```
    nameserver <MASTER IP>
    nameserver 192.168.1.1 #where this is router IP of the subnet
```

**Restart the Relavent Services on all nodes**

```bash
systemctl restart atomic-openshift-master
systemctl restart atomic-openshift-node
systemctl restart dnsmasq
```

### Run the Example

* `glusterfs-endpoints.yaml` - change `ip:` to that of each gluster node
* `gluster-pv.yaml` - change `path:` to the volume name

##### Create the persistent volume claim

```bash
oc create -f glusterfs-endpoints.yaml
oc create -f gluster-pv.yaml
oc create -f gluster-pvc.yaml
```

- Confirm the persistent volume claim is running: `oc get pvc`

##### Start the Docker Registry

Refer to the latest [Origin Docs](https://docs.openshift.org/latest/install_config/install/docker_registry.html "Deploying A Docker Registry") for deployment instructions.  See the [Production Use](https://docs.openshift.org/latest/install_config/install/docker_registry.html#production-use "Production-Use") section to implement the registry using the persistent volume claim.

