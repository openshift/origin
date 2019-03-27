# Contents
* [Overview](#overview)
* [Adding Capacity](#adding-capacity)
    * [Adding new devices](#adding-new-devices)
    * [Increasing cluster size](#increasing-cluster-size)
    * [Adding a new cluster](#adding-a-new-cluster)
* [Reducing Capacity](#reducing-capacity)


# Overview

Heketi allows administrators to add and remove storage capacity by managing
one or more GlusterFS clusters.

# Adding Capacity

There are multiple ways to add additional storage capacity using Heketi.
One can add new devices, increase the cluster size, or add an entirely
new cluster.

## Adding new devices

When adding more devices, please keep in mind to add devices as a set.
For example, if volumes are using replica 2 you should add a device to two
nodes (one device per node). If using replica 3, then add a device to three
nodes.

Devices can be added to nodes by directly accessing the
[Heketi API](../api/api.md).

Using the Heketi cli, a single device can be added to a node:

```
$ heketi-cli device add \
      --name=/dev/sdb
      --node=3e098cb4407d7109806bb196d9e8f095
```

A much simpler way to add many devices at once is to add the new device
to the node description in your topology file used to setup the cluster.
Then rerun the command to load the new topology.
Here is an example where we added a new `/dev/sdj` drive to the node:

```
$ heketi-cli topology load --json=topology.json
...
        Found node 192.168.10.100 on cluster 3e21671bc4f290fca6bce464ae7bb6e7
                Found device /dev/sdb
                Found device /dev/sdc
                Found device /dev/sdd
                Found device /dev/sde
                Found device /dev/sdf
                Found device /dev/sdg
                Found device /dev/sdh
                Found device /dev/sdi
                Adding device /dev/sdj ... OK
...
```

## Increasing cluster size

In addition to adding new devices to existing nodes new nodes can be
added to the cluster. As with devices one can add a new node to an
existing cluster by either using the [API](../api/api.md), using the cli,
or modifying your topology file.

The following shows an example of how to add a new node using the cli:

```
$ heketi-cli node add \
      --zone=3 \
      --cluster=3e21671bc4f290fca6bce464ae7bb6e7 \
      --management-host-name=node1-manage.gluster.lab.com \
      --storage-host-name=172.18.10.53

Node information:
Id: e0017385b683c10e4166492e78832d09
State: online
Cluster Id: 3e21671bc4f290fca6bce464ae7bb6e7
Zone: 3
Management Hostname node1-manage.gluster.lab.com
Storage Hostname 172.18.10.53

$ heketi-cli device add \
      --name=/dev/sdb \
      --node=e0017385b683c10e4166492e78832d09
Device added successfully

$ heketi-cli device add \
      --name=/dev/sdc \
      --node=e0017385b683c10e4166492e78832d09
Device added successfully
```

A much easier way is to expand a cluster is to add a new node to your
topology file file. When adding the new node you **must** add this node
information after the existing ones so that the Heketi cli figures out
which cluster this new node should be part of. Here is an example:

```
...
        Found node 192.168.10.103 on cluster 3e21671bc4f290fca6bce464ae7bb6e7
                Found device /dev/sdb
                Found device /dev/sdc
                Found device /dev/sdd
                Found device /dev/sde
                Found device /dev/sdf
                Found device /dev/sdg
                Found device /dev/sdh
                Found device /dev/sdi
        Creating node 192.168.10.105 ... ID: be0e8f7fba6ec1e5aa0337141f356013
                Adding device /dev/sdb ... OK
                Adding device /dev/sdc ... OK
```

## Adding a new cluster

Storage capacity can also be increased by adding new clusters of GlusterFS.
Just as before, one can use the [API](../api/api.md) directly, use the
`heketi-cli` to manually add clusters, nodes and devices, or create another
topology file to define the new nodes and devices which will compose this
cluster.

# Reducing Capacity

Heketi also supports the reduction of storage capacity. This is possible
by deleting devices, nodes, and clusters. These changes can be
performed  using the [API](../api/api.md) directly or by using `heketi-cli`.
Here is an example of how to delete devices with no volumes from Heketi:

```
sh-4.2$ heketi-cli topology info
 
Cluster Id: 6fe4dcffb9e077007db17f737ed999fe
 
    Volumes:
 
    Nodes:
 
        Node Id: 61d019bb0f717e04ecddfefa5555bc41
        State: online
        Cluster Id: 6fe4dcffb9e077007db17f737ed999fe
        Zone: 1
        Management Hostname: gprfc053.o.internal
        Storage Hostname: 172.18.10.53
        Devices:
                Id:e4805400ffa45d6da503da19b26baad6   Name:/dev/sdc            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
                Id:ecc3c65e4d22abf3980deba4ae90238c   Name:/dev/sdd            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
 
        Node Id: e97d77d0191c26089376c78202ee2f20
        State: online
        Cluster Id: 6fe4dcffb9e077007db17f737ed999fe
        Zone: 2
        Management Hostname: gprfc054.o.internal
        Storage Hostname: 172.18.10.54
        Devices:
                Id:3dc3b3f0dfd749e8dc4ee98ed2cc4141   Name:/dev/sdd            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
                Id:4122bdbbe28017944a44e42b06755b1c   Name:/dev/sdc            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
                Id:b5333d93446565243f1a7413be45292a   Name:/dev/sdb            State:online    Size (GiB):279     Used (GiB):0       Free (GiB):279
                        Bricks:
sh-4.2$
sh-4.2$ d=`heketi-cli topology info | grep Size | awk '{print $1}' | cut -d: -f 2`
sh-4.2$ for i in $d ; do
> heketi-cli device delete $i
> done
Device e4805400ffa45d6da503da19b26baad6 deleted
Device ecc3c65e4d22abf3980deba4ae90238c deleted
Device 3dc3b3f0dfd749e8dc4ee98ed2cc4141 deleted
Device 4122bdbbe28017944a44e42b06755b1c deleted
Device b5333d93446565243f1a7413be45292a deleted
sh-4.2$ heketi-cli node delete $node1
Node 61d019bb0f717e04ecddfefa5555bc41 deleted
sh-4.2$ heketi-cli node delete $node2
Node e97d77d0191c26089376c78202ee2f20 deleted
sh-4.2$ heketi-cli cluster delete $cluster
Cluster 6fe4dcffb9e077007db17f737ed999fe deleted
```
