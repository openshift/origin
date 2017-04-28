# OpenShift networking requirements
This document presents the guidelines for a third party network plugin to work with OpenShift.

## Basic requirements
OpenShift networking has certain requirements over and above kubernetes essentials. The basic kubernetes requirements can be found [here](https://github.com/kubernetes/kubernetes/blob/release-1.3/docs/design/networking.md).

## CNI is the recommended way

Any external networking solution can be used to plumb networking for OpenShift as long as it follows the 'CNI' spec. Then, OpenShift needs to be launched with 'networkPluginName: "cni"' in the master/node config yaml files. 
Example:
```
networkConfig:
  networkPluginName: cni
```

When done through ansible, provide sdn_network_plugin_name=cni as the option while installing OpenShift. Be aware that OpenShift ansible installation allows a firewall passthrough for the VxLAN port (4789), so if a plugin needs other ports (for management/control/data) to be open, then the installer needs to be changed suitably.

Learn more about CNI [here](https://kubernetes.io/docs/concepts/cluster-administration/network-plugins) and [here](https://github.com/containernetworking/cni/blob/master/SPEC.md).

## Advanced requirements
Finally, these extra things should be kept in mind when writing a CNI plugin for OpenShift:

1. A plugin can follow the NetworkPolicy objects from kubernetes and implement the user/admin intent on multi-tenancy (see https://github.com/kubernetes/kubernetes/blob/master/docs/proposals/network-policy.md). Or just ignore the multi-tenancy completely. Or implement a model where multi-tenancy is based on projects i.e. kubernetes namespaces, where,
   - Each namespace should be treated like a tenant where its pods and services are isolated from another project's pods/services
   - *Support exists for operations like merge/join networks even when they belong to different namespaces

2. Certain services in the cluster will be run as infrastructure services. e.g. Load balancer, registry, DNS server(skydns). The plugin should allow for a 'global' tenant which is-accessible-by/can-access all pods of the cluster. For example, a load balancer can run in two modes - private and global. The global load balancer should have access to all tenants/namespaces of the cluster. A private load balancer is one that is launched as a pod by a particular namespace, and this should obey tenant isolation rules.

3. *Access to all pods from the host - particularly important if kube-proxy is used by the SDN solution to support kubernetes services. Please note that iptables based kube-proxy will be enabled by default in OpenShift. This will have to be overridden specially if the plugin wants a different behaviour. One could disable the proxy by giving the option "--disable proxy" to OpenShift's node process.
e.g. for the OpenShift node's systemd service, add the following option to the sysconfig file (/etc/sysconfig/origin-node in case of origin):
```
OPTIONS="--loglevel=2 --disable proxy"
```

4. Access to external networks by pods whether through NAT or direct access.

5. Build containers - as part of the developer workflow, OpenShift builds docker images. The build is run through 'docker build' api. This means that docker's default networking will be invoked for this container (CNI/kube-plugin will not run as this is not a pod). These containers still need a network and access to external network (the internet e.g.).

6. *Respect the PodSecurityContext::HostNetwork=true for infra pods. Or provide an externally routable IP address to the pod. This is used for the load balancer pods which are the entry point for all external traffic funneling into the cluster.
   - Note that the HostPort<->ContainerPort mapping will not be available by default if the CNI plugin is enabled (as the default docker networking is turned off). The plugin will have to implement this functionality by itself.



* The items marked with '*' are _not_ necessary for a functional OpenShift cluster, but some things will need to be worked around for the administrator's benefit.
