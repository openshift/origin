## SDN with Isolation for Openshift

This document describes the architecture, benefits, and limitations of the isolation/multitenant implementation for openshift-sdn.

#### Network Architecture

The main components on a node are:

* **br0** - the Open vSwitch bridge, which handles all isolation tasks through OpenFlow rules
* **vethXXXXX** - the veth pairs that connect a pod's network namespace to the OVS bridge
* **tun0** - an OVS internal port assigned the OpenShift node gateway address (10.1.x.1/24) for outside network communication.  iptables rules NAT traffic from tun0 to the outside network.
* **lbr0** - the docker bridge, handles IPAM for all docker containers and OpenShift pods
* **vovsbr**/**vlinuxbr** - veth pair that connects the docker bridge (lbr0) to the OVS bridge, to allow docker-only containers to talk to OpenShift pods and to access the outside network through tun0
* **vxlan0** - an OVS VXLAN tunnel for communication with all other cluster nodes; directed to destination node with OF rules

See `isolation-node-interfaces-diagram.pdf` for a diagram of how all these interfaces relate to each other.

#### IP Address allocation

When using Vagrant to configure the cluster, the master and each node are assigned cluster-private IP addresses in the 10.245.2.x/24 subnet through statements in the Vagrantfile.  The master receives 10.245.2.2 and the nodes recieve an address above that.  The cluster uses these 10.245.2.x addresses for all communication between master and nodes.  Vagrant provisioning then writes the IP addresses of the master into the master's openshift-master systemd startup script, the configuration files passed to each openshift node process with --config, each node's 'kubeconfig' (referenced from the nodes --config YAML file), and each node's /etc/hosts file.

When manual or other provisioning is used, the nodes must be told of the master's IP address through the --config argument and/or the kubeconfig files.  The master must be told its IP address with the --master argument.

Regardless of how the cluster's public IP addresses are provisioned, each node is also assigned a cluster-private /24 in the 10.1.x.x/16 range by the openshift master process, either through the --network-cidr argument or the NewDefaultNetworkArgs() function.  Each node's OpenShift multitenant plugin then uses this address (retrieved from the master's etcd service) for local IP address allocation to pods.  OpenShift (through the openshift-sdn-multitenant-setup.sh script) creates lbr0 and assigns it 10.1.x.1/24, then tells docker to use that interface.  Docker then automatically begins providing IPAM on that interface for all containers on the system, both docker-only and OpenShift originated ones.

#### Isolation

Isolation is provided on an OpenShift "project" basis.  The default OpenShift project "default" receives the Virtual Network ID (VNID) 0; all other projects receive non-zero VNIDs.  VNID 0 is privileged in that all other VNIDs can send to VNID 0, and VNID 0 can send to all other VNIDs.  However, non-zero VNIDs cannot talk to each other.

All traffic from local pods is tagged with a VNID based on its port number when it enters the OVS bridge.  The port:VNID mapping is determined when the pod is created by asking etcd on the master for the VNID associated with the pod's project name.  Incoming VXLAN traffic from other nodes already has a VNID which is added by the other node before sending across the VXLAN tunnel.

OpenFlow rules will prevent delivery of any traffic to a pod's port that is not tagged with that pod's VNID (except for VNID 0 as previous discussed).  This ensures each project's traffic is isolated from other projects.

#### Outside Network Access

The tun0 interface is an OVS internal port assigned the IP address 10.1.x.1/24 based on the node's assigned subnet range in the 10.1.x.x/16 address space.  You may notice that this interface has the same IP address as the lbr0 device, but this is only because we need Docker to do IPAM on lbr0, but we also need to control the default gateway.  As such, iptables rules are disabled on lbr0 by openshift-sdn-multitenant-setup.sh and all pod traffic destined for the default gateway (10.1.x.1) traffic exiting the node eventually ends up at tun0, where it is NAT-ed to the host's physical interface.

#### openshift-sdn Kubernetes plugin

Kubernetes (and therefore OpenShift) makes use of network plugins, of which openshift-sdn's multitenant code is only one.  Network plugins are selected by passing the --network-plugin argument to the OpenShift master process.  Kubernetes usually looks for the plugin you specify in the /usr/libexec/kubernetes/kubelet-plugins/net/exec/ directory (which contains directories into which the plugin places its main binary), but when openshift-sdn is linked directly into Origin, the openshift-sdn multitenant plugin is instantiated directly by some specific code in the master and nodes that looks for the multitenant plugin's name.

The most interesting pieces of the multitenant plugin are:

* **ovssubnet/controller/multitenant/bin/openshift-ovs-multitenant**: This script is run every time a pod is started/stopped to set up/tear down the network namespace that all containers of the pod share.  It handles taking the container's veth endpoint out of lbr0 and adding it to the OVS bridge instead.  It then adds pod-specific OVS flow rules to provide traffic flow and isolation based on the VNID.

* **ovssubnet/controller/multitenant/bin/openshift-sdn-multitenant-setup.sh**: this script is run every time the openshift-node process starts or stops.  It does initial setup, like configuring the lbr0 bridge, adding the OVS bridge, adding the OVS VXLAN port, setting up the tun0 port and NAT rules, and configuring the non-pod-specific OVS rules.

* **ovssubnet/controller/multitenant/multitenant.go**: this module watches etcd for indications of nodes added to or removed from the cluster, and updates the OVS rules to ensure each node can be reached through the VXLAN tunnel.

