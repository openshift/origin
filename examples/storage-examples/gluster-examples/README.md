# OpenShift and GlusterFS – running NGINX applications with Storage
---
### Environment:
This environment consists of 4 hosts: 
* 2 GlusterFS nodes consisting of the gluster cluster (gluster1.rhs and gluster2.rhs)
* 2 RHEL7 Atomic Hosts running OpenShift and GlusterFS Client (master= ose1.rhs and node1 = ose2.rhs)

OpenShift is installed from [OpenShift Admin Guide – quick installation](https://docs.openshift.com/enterprise/3.0/admin_guide/install/quick_install.html) and operational (meaning the server is running and you can login to the OpenShift Console via the GUI - https://master-host:8443/console).  Also the gluster cluster has been tested with all RHEL Atomic Nodes and is working properly and accessible to the OpenShift nodes.  

Below is a summary of the Gluster Volume Info used for this example:


| Attribute       | Value                 |
|:--------------- | --------------------- | 
| Volume Name     | myVol1                |
| Status          | Started               |
| Num Bricks      | 2                     |
| Brick1          | gluster1.rhs:/mnt/brick1/myVol1 |
| Brick2          | gluster2.rhs:/mnt/brick1/myVol1 |

| Attribute       | Value                 |
|:--------------- | --------------------- |
| Volume Name     | myVol2                |
| Status          | Started               |
| Num Bricks      | 2                     |
| Brick1          | gluster1.rhs:/mnt/brick2/myVol2 |
| Brick2          | gluster2.rhs:/mnt/brick2/myVol2 |

**Assumptions:**

1.  OpenShift Enterprise v3 is installed on at least two nodes running  RHEL 7 Atomic hosts
2.  An active Glusterfs cluster exists and is accessible by the Atomic hosts
3.  All necessary post install configurations and setup were performed per install guides and both gluster and OpenShift clusters are useable.
4.  Atomic hosts (all nodes) have glusterfs-client installed and enabled (modeprobe fuse)
5.  Basic understanding of Docker/Containers and Kubernetes


**Summary:**

The goal of these examples are to validate/verify the glusterfs plugin as it pertains to existing OpenShift and Gluster environments.  These examples are designed to target potential users at a high level to begin the process of learning and experimenting with Gluster and Persistent Storage Capabilities with the Atomic OpenShift Platform.


- [example 1](./nginx_gluster_host):  Simple NGINX app, deployed using OpenShift Console that utilizes existing manually created GlusterFS fuse mounts on the RHEL Atomic Host for Distributred Gluster Storage and mapping those mounts to the hostPath of the NGINX application

- [example 2](./nginx_gluster_plugin):  Deploy NGINX application, using OpenShift Console that creates the relationship and mount to the GlusterFS volume via the POD definition using the glusterfs plugin.

- [example 3](./nginx_gluster_pvc):  Deploy NGINX application, using OpenShift Console and PersistentVolume and PersistentVolumeClaims using glusterfs plugin 

- [example 4](./nginx_template):  Create an OpenShift v3 Template that deploys a NGINX application out of the box for OpenShift users – applied via the OpenShift Console Web GUI


===

[Previous Example - Local Storage](../local-storage-examples)  |  [First Example](./nginx_gluster_host)

===



