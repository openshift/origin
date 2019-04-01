
# Arbiter Volumes

[Arbiter volumes](https://docs.gluster.org/en/latest/Administrator%20Guide/arbiter-volumes-and-quorum/)
are a type of volume supported by GlusterFS
that aim to reduce total space used while retaining the
resiliency of a replica three volume.

Heketi supports creating and managing Arbiter volumes. This document
provides an overview on how Arbiter volumes are managed via Heketi.


## Creating an Arbiter Volume

### Heketi CLI

To create an Arbiter volume using the Heketi CLI one must request a
replica 3 volume as well as provide the Heketi-specific volume
option `user.heketi.arbiter true` that will instruct the system to
create the Arbiter variant of replica 3.

Example:
```bash
$ heketi-cli volume create --size=4 --gluster-volume-options='user.heketi.arbiter true'
```

### API and API Consumers

To create an Arbiter volume using the Heketi API, or using another
system that consumes the Heketi API, such as a Kubernetes storage
provisioner, the volume option `user.heketi.arbiter true` must be
provided to the *glustervolumeoptions* list. Other options may also be
provided if needed.


## Managing an existing Arbiter Volume

A volume configured for the Arbiter feature behaves the same as
other replica 3 volumes for the purposes of volume expansion and
volume delete.

## Controlling Arbiter Brick Sizing

A standard replica 3 volume has the same sized bricks in each set.
For example a 4GB replica 3 volume with one brick set will have
three bricks all of size 4GB. However, an Arbiter volume will have
one brick in the brick set that can be smaller than the data bricks.

In order to better optimize the sizing of the Arbiter brick, Heketi
allows the user to provide an average file size value that is used
to calculate the final size of the Arbiter brick. This is done using
the volume option `user.heketi.average-file-size NUM` where NUM is
an integer value in KiB. By default Heketi uses a value of 64KiB.

For example if you planned on using to
store larger files, about 1MiB on average you would specify
`user.heketi.average-file-size 1024`.

**Note**: The Arbiter brick size is only directly proportional to the
data brick size. Multiple brick sets with varying sizes may make up
a single volume. Additionally, other factors, such as a volume's
snapshot configuration, will impact the total size the volume
consumes on the underlying devices.

### Setting Average File Size with the Heketi CLI

To create an Arbiter volume with a custom average file size
using the heketi-cli command line tool the volume options
`user.heketi.arbiter true` and `user.heketi.average-file-size 1024`
must be provided.

Example:
```bash
$ heketi-cli volume create --size=4 --gluster-volume-options='user.heketi.arbiter true,user.heketi.average-file-size 1024'
```

### Setting Average File Size with the Heketi CLI

To create an Arbiter volume with a custom average file size
using the Heketi API, or using another system that consumes
the Heketi API, such as a Kubernetes storage provisioner,
specify the string `user.heketi.average-file-size NUM`
to the *glustervolumeoptions* list, replacing NUM with size
of desired size as an integer in KiB. If this option is
provided without a corresponding user.heketi.arbiter option
this value will be preserved but ignored.


## Controlling Arbiter Brick Placement

There are use cases where one might desire the Arbiter brick to be
placed on certain nodes or devices (or avoid certain nodes and devices).

To accomplish the task of controlling where arbiter bricks are placed,
Heketi uses specific node and device tags. For the Arbiter feature,
the tag "arbiter" can be applied to a node or device with the values of
"supported", "required", or "disabled".

The values mean the following:
* supported: both arbiter bricks and data bricks are allowed
* required: only arbiter bricks are allowed, data bricks are rejected
* disabled: only data bricks are allowed, arbiter bricks are rejected

The default behavior of an untagged node/device is "supported".

A device without an explicit tag will automatically inherit the arbiter
tag value from the node it is connected to. An explicit tag on
the device always has priority over the node's tag. For example,
a node N hosts devices A, B, & C. If node N is tagged arbiter disabled,
and there are no tags on A, B, & C, the system will not place an
arbiter brick on devices A, B, or C. However, if the tag arbiter
required is added to device C (A & B are left untagged as before),
the system will not place arbiter bricks on A & B, but will place
arbiter bricks (and not place data bricks) on C.

### Setting Tags with the Heketi CLI

To set tags on nodes and device via the heketi-cli command line tool
the subcommands `heketi-cli node settags` and `heketi-cli device settags`
can be used.

Examples:
```bash
$ heketi-cli node settags e2a792a43ca9a6bac4b9bfa792e89347 arbiter:disabled

$ heketi-cli device settags 167fe2831ad0a91f7173dac79172f8d7 arbiter:required
```

A tag can be explicitly removed from a node or device with the subcommands
`heketi-cli node rmtags` and `heketi-cli device settags`.

Examples:
```bash
$ heketi-cli node rmtags e2a792a43ca9a6bac4b9bfa792e89347 arbiter

$ heketi-cli device rmtags 167fe2831ad0a91f7173dac79172f8d7 arbiter
```

### Viewing Tags with the Heketi CLI

To view tags use the commands `heketi-cli node info` and
`heketi-cli device info`. If the node or device has any tags
it will be displayed in a list below the heading "Tags:".

Examples:

```bash
$ heketi-cli node info e2a792a43ca9a6bac4b9bfa792e89347
Node Id: e2a792a43ca9a6bac4b9bfa792e89347
State: online
Cluster Id: ddb14817873c13c5bb42a5c04969daf9
Zone: 1
Management Hostname: 192.168.10.100
Storage Hostname: 192.168.10.100
Tags:
  arbiter: disabled
  test: demonstration
Devices:
Id:0b39f89c0677e8c0b796caf00204e726   Name:/dev/vdb            State:online    Size (GiB):500     Used (GiB):0       Free (GiB):500     Bricks:0
Id:167fe2831ad0a91f7173dac79172f8d7   Name:/dev/vdg            State:online    Size (GiB):500     Used (GiB):0       Free (GiB):500     Bricks:0

$ heketi-cli device info 167fe2831ad0a91f7173dac79172f8d7
Device Id: 167fe2831ad0a91f7173dac79172f8d7
Name: /dev/vdg
State: online
Size (GiB): 500
Used (GiB): 0
Free (GiB): 500
Tags:
  arbiter: required
  foobar: magic
Bricks:
```

### Settings Tags with the Heketi API

The Heketi API supports setting tags on nodes and devices at any time
via set tags APIs. It also allows tags to be specified on initial
create of nodes and devices.
Please refer to the [Heketi API documentation](../api/api.md) for more detail.

### Viewing Tags with the Heketi API

The Heketi API supports fetching the tags from a node or device using
the *node info* and *device info* APIs, respectively.
Please refer to the [Heketi API documentation](../api/api.md) for more detail.
