# Support for Arbiter volumes


## High level ideas

There are two different modes for arbiter that are needed:

* There are one or more nodes dedicated to only take arbiter bricks.
  The other nodes would host the data bricks. This is targeted for
  scenarios where you have two beefy storage nodes and add a more slim
  node to act as arbiter. Also applicable to situations with two
  data centers and an additional site with just a lightweight node for arbiter.

* Arbiter bricks should be spread throughout the cluster to achieve an
  overall reduction of storage on the nodes.

## Flags on Nodes / Devices

Introduce flags on nodes/devices to mark them as capable of
hosting arbiter bricks, or as only being allowed to host arbiter bricks.

Similar to Kubernetes annotations, nodes and devices will gain a
key-value store on both types, which can track semi-structured metadata.
This will intially be used to support arbiter volumes but can be
reused for assigning future metadata to nodes and devices.
The APIs will need to be adapted to allow setting them on both
device and node create as well as updating them afterwards.
For backwards compatibilty, any REST operation that lacks the
JSON field for this new substructure will be taken to mean
no-change (or in the case of a new item, no key-value pairs).

The exact name of this key-value store is TDB and needs to be
bikeshedded out still. For sake of argument the following
pseudo-code will use the term "tags".

To support arbiter a new tag key "arbiter" is defined. The
values "required", "supported", and "disabled" will have the
following specific meanings:
* required: The node or device may only host arbiter bricks.
* supported: The node or device may support data or arbiter bricks.
* disabled: The node or device must not host arbiter bricks.

Any other string will be de-facto treated the same as "disabled".

For convenience, a device that lacks a specific tag key will
"inherit" the key and value from the node it resides on.
This could be accomplished by use of a function like:
```golang
MergedTags(*NodeEntry, *DeviceEntry) map[string]string`
```
OR
```golang
MergedTag(n *NodeEntry, d *DeviceEntry, key string) (string, bool)
```

Furthermore, functions could be added that allow for convenient
checks for arbiter support such as:
```golang
// Returns true if the device is allowed to host an arbiter brick.
func CanHostArbiter(parent *NodeEntry, d *DeviceEntry) bool {
	val, _ := MergedTag(parent, d, "arbiter")
	return ArbiterOk(val)
}
```


## Request changes

A user will be able to request an Arbiter Volume by setting a
special option in the existing GlusterFS options list. Setting
the key `user.heketi.arbiter` to a value of "1" or "true" (or any
other value the
[Go standard library considers true](https://golang.org/pkg/strconv/#ParseBool)
and specifying a Replica 3 durability type will trigger Heketi
to treat the volume as an Arbiter Volume.

To create a volume in this manner a command line similar to the
following should be used:

```
heketi-cli volume create --size=80 --gluster-volume-options "user.heketi.arbiter true"
```

Gluster itself will not act on a key with the "user." prefix but will
store it. This could be useful in debugging efforts.

Earlier discussions considered defining a new type (or subtype) of durability in
Heketi that would reflect "arbiter" or (like gluster does it) add a new
volume option for the creation. However, in Kubernetes, Heketi's supported
durability types are hard-coded.
To avoid changing Kubernetes' glusterfs provisioner code significantly (which
is only possible for new releases), we have opted to go down the
volume option way. This is also closer to what glusterfs offers already.


## Need to refactor the allocator some more

* Now that the ring structure in the simple allocator is always built anew for
  each call of GetNodes(), we can augment the signature to take additional
  aspects of the volume create request as parameters and build different ring
  structures for different requests. (In particular it could take into account
  differently flagged nodes, e.g. those which should taker arbiter bricks...)
* The GetNodes() function should probably return disk sets instead of single
  disks.
* Q: should the allocator already take into account the sizes of the devices?
* Q: I.e. should the code from volume_entry_allocate.go be put into a bigger
  allocator package?

### Proposed Higher-Level Interface

In order to better support more complex layouts and internalize
more logic related to the placment of bricks within the cluster,
the following new interfaces are proposed. The core of these is
the BrickPlacer which is intended to be a more coprehensive
mechanism than the current Allocator interface.

The implementations of these interfaces may, or may not, opt to
use the existing Allocator and it is explicitly propoosed
_not_ to remove or replace the Allocator interface and
its implementation(s) at this time.

```golang
// DeviceSource is an abstraction used by the BrickPlacer to
// get an initial list of devices where bricks can be placed
// as well as converting device IDs to device entry objects.
// The idea is to keep db/connection/caching logic away from
// the placement algorithms in the placer interface.
type DeviceSource interface {
	// Devices returns a list of all the Device entries that can
	// be considered for the upcoming brick placement.
	Devices() ([]*DeviceEntry, error)
	// Device looks up a device id and resolves it to a Device
	// entry object.
	Device(id string) (*DeviceEntry, error)
}

// PlacementOpts is interface that is meant for passing the
// somewhat complex set of options needed by the placer code
// and hiding the sources of the these values away from the
// placer implementations.
type PlacementOpts interface {
	// BrickSizes returns values needed to calculate the
	// size of the brick on disk.
	BrickSizes() (uint64, float64)
	// BrickOwner returns the ID of object that will "own" the brick
	BrickOwner() string
	// BrickGid return the ID of the GID the brick will use
	BrickGid() int
	// SetSize returns the size of the Brick Sets that will be
	// allocated.
	SetSize() uint
	// SetCount returns the total number of Brick Sets that
	// will be produced.
	SetCount() uint
	// ValidDevice is a predicate function that the higher level
	// can define in order to cause the placement algorithm to
	// exlucde certain devices.
	ValidDevice(*BrickSet, *DeviceEntry) bool
}

// BrickPlacer implementations take their source devices and
// options and place new bricks on devices (if possible).
// The exact placement depends on the implementation and
// the input options.
type BrickPlacer interface {
	// PlaceAll constructs a full sequence of brick sets and
	// corresponding device sets for those bricks.
	PlaceAll(DeviceSource, PlacementOpts) (
		BrickAllocation, error)

	// Replace constructs a brick allocation constrained to
	// a single brick set where the brick set is already populated
	// but a brick with the given index into the set needs to
	// be replaced.
	Replace(DeviceSource, PlacementOpts, BrickSet, int) (
		BrickAllocation, error)
}
```

The final form of these interfaces is still subject to change but
this is a more-than-rough sketch of interfaces that should work
for both arbiter and non-arbiter volumes for the volume
create, volume expand, and brick replace use cases.


## Need to properly calculate arbiter brick sizes

Arbiter bricks are expected to be smaller than normal bricks as they only store
volume metadata. Because of this and that one of the selling points of arbiter
volumes is is space saving (and thus cost saving) feature we need to size the
arbiter bricks smaller than the data bricks but large enough to hold all the 
metadata needed.


## Touchpoints

This is a summary of the areas of the code that will need to be modified
to support arbiter volumes:

* Volume Create
* Volume Expand (?, probably)
* Allocator
* Cluster / Node / Device Metadata (?)

## References

* http://gluster.readthedocs.io/en/latest/Administrator%20Guide/arbiter-volumes-and-quorum/
* https://gist.github.com/pfactum/e8265ca07f7b19f30bb3
