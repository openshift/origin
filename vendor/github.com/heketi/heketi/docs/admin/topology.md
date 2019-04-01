You must provide Heketi with the information about the topology of the systems.  This allows Heketi to determine which nodes, disks, and clusters to use.

# Preparation
Before informing Heketi of the topology of the data center, you need to determine the node failure domains and clusters of nodes.  Failure domains, called _zones_ in the [API](../api/api.md#add-node), is a value given to a set of nodes which share the same switch, power supply, or anything else that would cause them to fail at the same time. Heketi uses this information to make sure that replicas are created across failure domains, thus providing cloud services volumes which are resilient to both data unavailability and data loss.  For example you may have 16 nodes where each four nodes share the same power bar.  In this example model, you would have four nodes per zone.

You also need to determine which nodes would constitute a cluster.  Heketi supports multiple GlusterFS clusters, which gives cloud services the option of specifying a set of clusters where a volume must be created.  This provides cloud services and administrators the option of creating SSD, SAS, SATA, or any other type of cluster which provide a specific quality of service to users.

In the [demo](http://github.com/heketi/heketi/wiki/Demo),
the topology is setup of a single cluster composed of four nodes across two zones.

# Topology setup
Cloud services can provide their own method of informing Heketi of the topology by using the REST API.  For simplicity, the heketi-cli client can be used as an example of how to interact with Heketi.

## Loading a topology with heketi-cli
You can use the command line client to create a cluster, then add nodes to that cluster, then add disks to each one of those nodes.  This process can be quite tedious from the command line.  For that reason, the command line client supports the option of loading this information to Heketi using a _topology_ file, which describes clusters, their nodes and disks on each node.

To load a topology file with heketi-cli, you would type the following:

```
$ export HEKETI_CLI_SERVER=http://<heketi server and port>
$ heketi-cli topology load --json=<topology>
```

Where _topology_ is a file in JSON format describing the clusters, nodes, and disks to add to Heketi.  The format of the file is as follows:

* clusters: _array of clusters_, Array of clusters
    * Each element on the array is a _map_ which describes the cluster as follows
        * nodes: _array of nodes_, Array of nodes in a cluster
            * Each element on the array is a _map_ which describes the node as follows
                * node: _map_, Same map as [Node Add](../api/api.md#add-node) except there is no need to supply the cluster id.
                * devices: _array of strings_, Name of each disk to be added, which should be raw block storage, and not a file system.

## Example
An example topology file is available at
[client/cli/go/topology-sample.json](https://github.com/heketi/heketi/blob/master/client/cli/go/topology-sample.json)
in the Heketi repository.


# Next
[Create a volume](./volume.md)

