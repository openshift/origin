# How To Use Fibre Channel Persistent Volumes

The purpose of this guide is to create Persistent Volumes with Fibre Channel. It is part of [OpenShift persistent storage guide](../README.md), which explains how to use these Persistent Volumes as data storage for applications.

## Setting up Fibre Channel Target

On your FC SAN Zone manager, allocate and mask LUNs so Kubernetes hosts can access them.

## Creating the PV with Fibre Channel persistent storage

In the *fc* volume, you need to provide *targetWWNs* (array of Fibre Channel target's World Wide Names), *lun*,  *fsType* that designates the filesystem type that has been created on the lun, and *readOnly* boolean.

## Fibre Channel Persistent Volumes

Each Fibre Channel Volume becomes its own Persistent Volume in the cluster.

```
# Create the persistent volumes for Fibre Channel.
$ oc create -f examples/wordpress/fc/pv-1.yaml
$ oc create -f examples/wordpress/fc/pv-2.yaml
$ oc get pv

NAME      LABELS    CAPACITY   ACCESSMODES   STATUS      CLAIM     REASON    AGE
pv0001    <none>    1Gi        RWO           Available                       2m
pv0002    <none>    1Gi        ROX           Available                       5s

```

Now the volumes are ready to be used by applications in the cluster.
