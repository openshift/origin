# How To Use NFS Persistent Volumes

The purpose of this guide is to create Persistent Volumes with NFS. It is part of [OpenShift persistent storage guide](../README.md), which explains how to use these Persistent Volumes as data storage for applications.

## NFS Provisioning

We'll be creating NFS exports on the local machine.  The instructions below are for Fedora.  The provisioning process may be slightly different based on linux distribution or the type of NFS server being used.

Create two NFS exports, each of which will become a Persistent Volume in the cluster.

```
# the directories in this example can grow unbounded
# use disk partitions of specific sizes to enforce storage quotas
mkdir /home/data/pv0001
mkdir /home/data/pv0002

# data written to NFS by a pod gets squashed by NFS and is owned by 'nfsnobody'
# we'll make our export directories owned by the same user
chown -R /home/data nfsnobody:nfsnobody

# security needs to be permissive currently, but the export will soon be restricted 
# to the same UID/GID that wrote the data
chmod -R 777 /home/data/

# Add to /etc/exports
/home/data/pv0001 *(rw,sync,no_root_squash)
/home/data/pv0002 *(rw,sync,no_root_squash)

# Enable the new exports without bouncing the NFS service
exportfs -a

```

## Security

### SELinux

By default, SELinux does not allow writing from a pod to a remote NFS server. The NFS volume mounts correctly, but is read-only.

To enable writing in SELinux on each node:

```
# -P makes the bool persistent between reboots.
$ setsebool -P virt_use_nfs 1
```

## NFS Persistent Volumes

Each NFS export becomes its own Persistent Volume in the cluster.

```
# Create the persistent volumes for NFS.
$ oc create -f examples/wordpress/nfs/pv-1.yaml
$ oc create -f examples/wordpress/nfs/pv-2.yaml
$ oc get pv

NAME      LABELS    CAPACITY     ACCESSMODES   STATUS      CLAIM     REASON
pv0001    <none>    1073741824   RWO,RWX       Available             
pv0002    <none>    5368709120   RWO           Available             

```

Now the volumes are ready to be used by applications in the cluster.
