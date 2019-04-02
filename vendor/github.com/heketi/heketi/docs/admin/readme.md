# Example
An example of the usage workflow can be seen by visiting
the [Demo](http://github.com/heketi/heketi/wiki/Demo).

# Requirements
Heketi requires ssh access to the nodes that it will manage.  For that reason, Heketi has the following requirements:

* SSH Access
    * SSH user and public key already setup on the node
    * SSH user must have password-less sudo
    * Must be able to run sudo commands from ssh.  This requires disabling `requiretty` in the /etc/sudoers file
* System must have glusterd service enabled and glusterfs-server installed
* Disks registered with Heketi must be in raw format.

# Workflow

* Installation:
  * [OpenShift](./install-openshift.md): New in v2.0, Heketi can now be used to manage GlusterFS containers deployed in an OpenShift Cluster.
  * [Standalone](./install-standalone.md): This method allows a Heketi to be installed on a system to manage GlusterFS storage nodes.
  * [Kubernetes](./install-kubernetes.md): Heketi can now be used to manage GlusterFS containers deployed in a Kubernetes Cluster. 
* [Running the server](./server.md)
* [Setting up the topology](./topology.md)
* [Creating a volume](./volume.md)
* [Cluster Maintenance](./maintenance.md)

