# Overview

The tools used to deploy Gluster and Heketi within an OpenShift cluster has matured significantly since Heketi was first used with OpenShift.   

It is now recommended to deploy Heketi on OpenShift using dedicated tools. Specifically, we recommend
using [OpenShift Ansible](https://github.com/openshift/openshift-ansible) and configuring the inventory file for [GlusterFS storage](https://docs.openshift.org/latest/install_config/install/advanced_install.html#advanced-install-containerized-glusterfs-persistent-storage).

Alternatively, the [gluster-kubernetes project](https://github.com/gluster/gluster-kubernetes) may also be used with OpenShift if needed.


# Details

Using Heketi within an OpenShift cluster works much like using Heketi on a Kubernetes cluster.
If you wish to understand more about how Heketi is interoperates with Kubernetes or OpenShift
please refer to the [kubernetes install doc](install-kubernetes.md). 
Please note that this doc is primarily meant for education purposes and we strongly recommend using
a deployment tool in production.


# Historical Demo

The original demo for Heketi on OpenShift is provided below:

[![demo](https://github.com/heketi/heketi/wiki/images/aplo_demo.png)](https://asciinema.org/a/50531)

For simplicity, you can deploy an OpenShift cluster using the configured [Heketi Vagrant Demo](https://github.com/heketi/vagrant-heketi)
