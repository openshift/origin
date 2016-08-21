# Mount Volumes on Privileged Pods

**This guide will demo GlusterFS as its example use-case but this method will work for any compatible volume provider.**

##Purpose

This example gives a basic template for attaching a persistent storage volume to a pod. It provides an end to end setup that begins with the _cluster-admin_ making the persistent volume availble and a _basic-user_ requesting storage from a **privileged** pod.

_If the pod is not run as privileged, skip the **Edit Privileged scc** section_

###Assumptions:

* OSE 3.x
* NFS, GlusterFS, Ceph, or other compatible volume provider
* A cluster-admin user.  For this guide, that user is called `admin`

##Create a basic-user and User Project

_**Note:**_ This section assumes there are not yet basic users.  If you have a basic user and that user has a project, skip this section.

`$ oc login -u tom -p tom`
    
   Where "tom" is an arbitrary user name and password. 
   
   Next, create the project as tom:
   
```bash
$ oc new-project <project_name> \
--description="<description>" \
--display-name="<display_name>"
```
   
   _At a minimum, only `<project_name>` is required._

   Basic-users are bound to the project-admin role at project creation so there is no need to manually bind them.

##Edit Privileged scc

The user must be added to the privileged scc (or to a group given access to that scc) before they can run privileged pods.

_**As admin**_

```bash
$ oc edit scc privileged
```
Under `users:` add the basic-user:

```yaml
users:
- tom
```

##Make the Volume Available to Projects

_**As admin:**_

```bash
$ oc create -f gluster-endpoints.yaml
$ oc create -f gluster-endpoints-service.yaml
$ oc create -f gluster-pv.yaml
```
###Make the volume available within the user project
_**As basic-user**_

Create the PersistentVolumeClaim

`$ oc create -f gluster-pvc.yaml`

Create the privileged pod

`$ oc create -f gluster-nginx-priv-pod.yaml`


##Confirm the Setup was Successful

###Verify the Pod is Bound to the Correct scc

Get the pod name

`$ oc get pods`

Export the configuration of the pod.

`$ oc export pod <pod_name>`

Examine the output. Check that `openshift.io/scc` has the value `privileged`.

```yaml
...
metadata:
  annotations:
    openshift.io/scc: privileged
...
```

###Check the Volume is Mounted

Access the pod

```bash
$ oc rsh <pod_name>
[root@gluster-nginx-pvc /]# mount
```

Examine the output for the gluster volume.
    
	192.168.59.102:gv0 on /mnt/gluster type fuse.gluster (rw,relatime,user_id=0,group_id=0,default_permissions,allow_other,max_read=131072)


**That's it!**

##Relevent Origin Docs

For more info on:

* Setting pv/pvc's for other volume providers see [Configuring Persistent Storage](https://docs.openshift.org/latest/install_config/persistent_storage/index.html)
* SCC's, see [Managing Security Context Contraints](https://docs.openshift.org/latest/admin_guide/manage_scc.html)

