# Support For Gluster Block Volumes

The [gluster-block project](https://github.com/gluster/gluster-block) implements
raw block volumes on top of Glusterfs volumes with a virtual-block-on-file
approach.

This document describes how heketi's `blockvolume` feature adds
convenience and higher level functionality on top of gluster-block in a similar
way that the `volume` functionality provides higher level functionality
on top of normal glusterfs volumes.

This design was originally created as part of an [end-to-end
design](https://github.com/gluster/gluster-kubernetes/blob/master/docs/design/gluster-block-provisioning.md) for
kubernetes use cases in the
[gluster-kubernetes repository](https://github.com/gluster/gluster-kubernetes)
and is now extracted to describe only the heketi parts.

## Basic Ideas and Analysis

The basic idea is to export files on GlusterFS volumes as block
devices via the iSCSI protocol (or potentially later other
access methods such as loopback devices, tcmu-qemu, etc), also
known as *virtual block on file*. The expected benefits of
this approach are better performance for certain workloads
and better scalability in number of volumes.

Well, this introduces an additional layer of indirection to
file-system-access, so why would it be any faster than direct
glusterfs filesystem access?
While data operations are fast with Gluster, the meta-data
operations are notoriously slow (GlusterFS being a distributed
file system). But with the virtual-block-on-file approach, all meta-data
operations are translated into fast reads and writes.
So this approach adds a small penalty to I/O operations, but
gives a big speed-up to meta-data operations.

Gluster's resource consumption is mainly by GlusterFS volume.
Because this approach will use one gluster file volume to host
many block device files, it should allow one to support
more block volumes with a given cluster and given hardware.


## Background About gluster-block

`gluster-block` is the gluster-level tool to make creation
and consumption of block volumes easy. It consists of
a server component `gluster-blockd` that runs on the gluster
storage nodes (or on separate hosts) and a command line client
utility `gluster-block` that is invoked on the same host
where a `gluster-blockd` is running and talks to the local
`gluster-blockd` via a local RPC mechanism.

`gluster-block(d)` takes care of creating files (to serve as block devices)
on a specifid gluster volume. These block files are then exported
as iSCSI targets with the help of the tcmu-runner mechanism,
using the gluster backend with libgfapi. This has the big
advantage that it is talking to the gluster volume directly
in user space without the need of a glusterfs fuse mount,
skipping the kernel/userspace context switches and the
user-visible mount.

The supported operations are:

* create
* list
* info
* delete
* modify

Details about the gluster-block architecture can be found
in the [gluster-block repository](https://github.com/gluster/gluster-block).


## Overview of Heketi's New `blockvolume` Functionality

For the purposes of gluster block volumes, the same heketi instance is used
as for the regular glusterfs file volumes. Heketi has a new API though for
treating blockvolume requests. Just as with the glusterfs file volume
provisioning, the logic for finding suitable clusters and file volumes
for hosting the block device files (so called **block hosting volumes**)
is part of heketi. In that regard, Heketi adds a convenience layer on top
of the raw gluster-block functionality, similar to the intelligent volume
provisioning on top of the raw glusterfs volume functionality.

The API is a variation of the `volume` API and looks as follows.

The currently supported functions are:

* `BlockVolumeCreate`
* `BlockVolumeInfo`
* `BlockVolumeDelete`
* `BlockVolumeList`

In the future, `BlockVolumeExpand` might get added.


## Details About The Individual API Elements

### BlockVolumeCreateRequest

The block volume create request takes the size and a name
and can optionally take a list of clusters and an hacount.

```golang
type BlockVolumeCreateRequest struct {
        Size       int       `json:"size"`
        Clusters   []string  `json:"clusters,omitempty"`
        Name       string    `json:"name"`
        Hacount    int       `json:"hacount,omitempty"`
        Auth       bool      `json:"auth,omitempty"
}
```

### BlockVolume

This is the basic info about a block volume.

```golang
type BlockVolume struct {
        Hosts     []string `json:"hosts"`
        Iqn       string   `json:"iqn"`
        Lun       int      `json:"lun"`
        Username  string   `json:"username"`
        Password  string   `json:"password"`
}

```

### BlockVolumeInfo

This is returned for the blockvolume info request and
upon successful creation.

```golang
type BlockVolumeInfo struct {
        Size       int       `json:"size"`
        Clusters   []string  `json:"clusters,omitempty"`
        Name       string    `json:"name"`
        Hacount    int       `json:"hacount,omitempty"`
        Id         string    `json:"id"`
        Size       int       `json:"size"`
        BlockVolume struct {
                Hosts     []string `json:"hosts"`
                Hacount   int `json:"hacount"`
                Iqn       string `json:"iqn"`
                Lun       int `json:"lun"`
        } `json:"blockvolume"`
}

```


### BlockVolumeListResponse

The block volume list request just gets a list
of block volume IDs as response.


```golang
type BlockVolumeListResponse struct {
        BlockVolumes []string `json:"blockvolumes"`
}

```

## Details About Heketi's Internal Logic

### Block-hosting volumes

The loopback files for block volumes need to be stored in
gluster file volumes. Volumes used for gluster-block volumes
should not be used for other purposes. For want of a better
term, we call these volumes that can host block-volume
loopback files **block-hosting file-volumes** or (for brevity)
**block-hosting volumes** in this document.

### Labeling block-hosting volumes

In order to satisfy a blockvolume create request, Heketi
needs to find and appropriate block-hosting volume in
the available clusters.  Hence heketi should internally
flag these volumes with a label (`block`).

### Type of block-hosting volumes

The block-hosting volumes should be regular
3-way replica volumes (possibly distributed).
One important aspect is that for performance
reasons, sharding should be enabled on these volumes.

### Block-hosting volume creation automatism

When heketi, upon receiving a blockvolume create request,
does not find a block-hosting volume with sufficient
space in any of the considered clusters, it would look for
sufficient unused space in the considered clusters and create
a new gluster file volume, or expand an existing volume
labeled `block`.

The sizes to be used for auto-creation of block-hosting
volumes will be subject to certain parameters that can
be configured and will have reasonable defaults:

* `auto_create_block_hosting_volume`: Enable auto-creation of
  block-hosting volumes?
  Defaults to **false**.
* `block_hosting_volume_size`: The size for a new block-hosting
  volume to be created on a cluster will be the minimum of the value
  of this setting and maximum size of a volume that could be created.
  This size will also be used when expanding volumes: The amount
  added to the existing volume will be the minimum of this value
  and the maximum size that could be added.
  Defaults to **1TB**.

### Internal heketi db format for block volumes

Heketi stores information about the block volumes
in it's internal DB. The information stored is

* `id`: id of this block volume
* `name`: name given to the volume
* `volume`: the id of the block-hosting volume where the loopback file resides
* `hosts`: the target ips for this volume

### Cluster selection

By default, heketi would consider all available clusters
when looking for space to create a new block-volume file.

With the clusters request parameter, this search can be
narrowed down to an explicit list of one or more clusters.

## Details On Calling `gluster-block`

Heketi calls out to `gluster-block` the same way it currently calls out to
the standard gluster cli for the normal volume create operations, i.e. it uses
an ssh or kubexec mechanism to run the command on one of the gluster nodes.
