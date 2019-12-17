
# Support for "Tag Matching" when placing bricks


## Use case

Users frequently request creating different performance tiers or service
levels using Heketi managed Gluster. Currently, this is not possible
unless the devices are split across different nodes such that nodes with
device types X form one Gluster TSP and nodes with device type Y form
a different Gluster TSP. Then Heketi can be requested to allocate volumes
from one cluster or another. If one wants to deploy gluster on bare-metal
with a mix of device types this workaround fails.

## Summary

This design proposes a simple "tag matching" approach to require Heketi
only use certain devices when placing a volume. To keep the design simple
no sort of auto-detection will be performed. The user is required to
specify a key-value pair on a specific set of devices. Then a new
volume option with a "user.heketi.device-tag-match" key and a simple matching
rule (see below) must be specified when the volume is created.

Then Heketi will ensure that bricks for this volume will only be
placed on devices where the tag matching rule applies.

This approach largely re-uses existing mechanisms established during the
development of the arbiter feature as well as the zone-checking
features.

## Workflow Example

Assume three nodes each with three existing devices each. One of the
devices on the node is a smaller high-performance device. The
remaining two devices are large slower devices.

For each device on one example node:
```
   $ heketi-cli device settags abc1230901344aef90 tier:gold
   $ heketi-cli device settags bbc1230901344aef91 tier:silver
   $ heketi-cli device settags cbc1230901344aef92 tier:silver
```

Now that devices have been tagged a volume can be requested to
make use of the tagging:
```
    $ heketi-cli volume create --size=5 --gluster-volume-options 'user.heketi.device-tag-match tier=gold'
    $ heketi-cli volume create --size=100 --gluster-volume-options 'user.heketi.device-tag-match tier=silver'
```

## Tag Match Rule

The syntax of the volume option is intended to be very simple. All values
must match the following regex: `([a-zA-Z0-9.-_]+)(\!?=)([a-zA-Z0-9.-_]+)`.
The rule can be split into three parts, the tag-key, the op, and the tag-value.
The tag-key and tag-value are (initially) simple strings that require
minimal validation.

If the op is "=" the device must have a tag with a key matching the supplied
tag-key and a value matching the supplied tag-value. If the op is "!=" then
the device must not have a tag matching the supplied tag-key and value.

All matches are exact and case sensitive. Only one device-tag-match may be
specified.


## Implementation

Internally, when a tag match rule is specified Heketi will construct a new
device-filter-function, similar to how zone-checking works. The device filter
will ensure only devices matching the rule are selected for placement of
the volume. Thus the system ensures that volumes so specified are only
placed on specific devices.


## Other Considerations

### Matching devices by default

In some cases volumes may be allocated prior to normal use of Heketi being
possible. This includes situations like bootstrapping the installation to
use a "heketidbstorage" volume for the Heketi db on gluster. It can also
include a situation where an installer creates a default storage class that
can be used for initial volume creation. In these cases the user can not
establish a tagging policy prior to the creation of volumes, possibly using
storage that was meant to be dedicated for some other purpose.
In these cases we expect that the installer tools establish a policy of
providing Heketi with default tag-matching rules such that the user can
add tags to devices and nodes in the topology and the system will avoid
the use of such devices. Similarly, these systems can configure default
storage classes or Heketi's environment to default to tag matching rules.

As an example, we want to avoid placing the "heketidbstorage" volume on
fast and expensive storage. The installer must create the volume with the
rule "user.heketi.device-tag-match builtinvols!=disabled". Then the
user must tag all devices that must not be used for the "heketidbstorage"
volume as "builtinvols:disabled". This scheme as the advantage as imposing
no additional policy on how users expect these devices to be used.


### Changing tag match rules

Currently, once a volume is created the volume options list is fixed. While
it is intentional that the tag-match rules persist with the volume metadata
for volume expansion and brick replacement purposes it is possible that
policies will change over time. If the policy about where a volumes tag-match
rule change there is currently no method to change this short of manipulating
the db outside of the Heketi API. While this design does not require the
ability to change volume options after a volume has been created it seems
like this is something that should be investigated to improve the UX
related to this feature.
