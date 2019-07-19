Snapshots are read-only copies of volumes (once the snapshot is activated) that
can be used for cloning new volumes. It is possible to create a snapshot
through the [Snapshot Create API](#create-a-snapshot) or the commandline
client.

From the command line client, you can type the following to create a snapshot
from an existing volume:

```
$ heketi-cli volume snapshot <vol_uuid> [--name=<snap_name>]
```

The new snapshot can be used to create a new volume:

```
$ heketi-cli snapshot clone <snap_uuid> [--name=<clone_name>]
```

The clones of snapshots are new volumes with the same properties as the
original. The cloned volumes can be deleted with the `heketi-cli volume delete`
command. In a similar fashion, snapshots can be removed with the `heketi-cli
snapshot delete` command.


## Implementation Detail for Heketi internal Snapshot objects
The proposed API and CLI can be used for both file-volumes and block-volumes.
In order to provide the expected outcome of the cloning, the Heketi internal
snapshot object should have a `Type: {volume|blockvolume}`. This makes it
possible to have a single `/snapshots/{snapshot_uuid}` endpoint that can handle
both volume types.


# Proposed CLI

```
$ heketi-cli volume snapshot <vol_uuid> [--name=<snap_uuid>] [--description=<string>]
$ heketi-cli snapshot clone <snap_uuid> [--name=<vol_uuid>]
$ heketi-cli snapshot delete <snap_uuid>
$ heketi-cli snapshot list
$ heketi-cli snapshot info <snap_uuid>
```

Convenience call for direct volume cloning:

```
$ heketi-cli volume clone <vol_uuid>
```


# API Proposal

The API is layed out here for file volume types only. The same API will be
added for block-volumes at a later time.

### Create a Snapshot
* **Method:** _POST_
* **Endpoint**:`/volumes/{volume_uuid}/snapshot`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#asynchronous-operations)
* **Temporary Resource Response HTTP Status Code**: 303, `Location` header will contain `/snapshots/{snapshot_uuid}`. See [Snapshot Info](#snapshot_info) for JSON response.
* **JSON Request**:
    * name: _string_, _optional_, Name of snapshot. If not provided, the name of the snapshot will be `snap_{id}`, for example `snap_728faa5522838746abce2980`
    * description: _string_, _optional_, Description of the snapshot. If not provided, the description will be empty.
    * Example:

```json
{
    "name": "midnight",
    "description": "nightly snapshot"
}
```

### Clone a Volume from a Snapshot
* **Method:** _POST_
* **Endpoint**:`/snapshots/{snapshot_uuid}/clone`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#asynchronous-operations)
* **Temporary Resource Response HTTP Status Code**: 303, `Location` header will depend on the type of the snapshot object:
    * For `file-volumes` the `Location` header will contain `/volumes/{id}`. See [Volume Info](#volume_info) for JSON response.
    * For `block-volumes` the `Location` header will contain `/blockvolumes/{id}`. The `BlockVolumeInfo` JSON resonse is not documented.
* **JSON Request**:
    * name: _string_, _optional_, Name of volume. If not provided, the name of the volume will be `vol_{id}`, for example `vol_728faa5522838746abce2980`
    * Example:

```json
{
    "name": "new-vol-from-snap"
}
```

### Delete a Snapshot
* **Method:** _DELETE_
* **Endpoint**:`/snapshots/{snapshot_uuid}`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Temporary Resource Response HTTP Status Code**: 204

### List Snapshots
* **Method:** _GET_
* **Endpoint**:`/snapshots`
* **Response HTTP Status Code**: 200
* **JSON Response**:
    * snapshots: _array strings_, List of snapshot UUIDs.
    * Example:

```json
{
    "snapshots": [
        "aa927734601288237463aa",
        "70927734601288237463aa"
    ]
}
```

### Get Snapshot Information
* **Method:** _GET_
* **Endpoint**:`/snapshots/{snapshot_uuid}`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
* **JSON Response**:
    * id: _string_, Snapshot UUID
    * name: _string_, Name of the snapshot
    * description: _string_, Description of the snapshot
    * created: _int_, Seconds after the Epoch when the snapshot was taken
    * volume: _string_, UUID of the volume that this snapshot belongs to
    * cluster: _string_, UUID of cluster which contains this snapshot
    * type: _string_, type of the original object that was snapshot'd, either `volume` or `blockvolume`
    * Example:

```json
{
    "id": "70927734601288237463aa",
    "name": "midnight",
    "description": "nightly snapshot",
    "created": "1518712323",
    "volume": "aa927734601288237463aa",
    "cluster": "67e267ea403dfcdf80731165b300d1ca",
    "type": "volume"
}
```

### Clone a Volume directly

This is a flavor that clones a volume directly.
Implicitly, it creates a snapshot, activates
and clones it, and then deletes the snapshot again.
It is a convenience method for cloning for users
only interested in the clone and not in the snapshots.

* **Method:** _POST_
* **Endpoint**:`/volumes/{volume_uuid}/clone`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#asynchronous-operations)
* **Temporary Resource Response HTTP Status Code**: 303, `Location` header will contain `/volumes/{new_volume_uuid}`. See [Volume Info](#volume_info) for JSON response.
* **JSON Request**:
    * name: _string_, _optional_, Name of the clone. If not provided, the name of the snapshot will be `snap_{id}`, for example `snap_728faa5522838746abce2980`
    * Example:

```json
{
    "name": "my_clone",
}
```


# Future API Extensions


### Activate a Snapshot
This is an internal function for Gluster, there is currently no known use-case that needs this exposed.

* **Method:** _POST_
* **Endpoint**:`/snapshots/{snapshot_uuid}/activate`

### Deactivate a Snapshot
This is an internal function for Gluster, there is currently no known use-case that needs this exposed.

* **Method:** _POST_
* **Endpoint**:`/snapshots/{snapshot_uuid}/deactivate`

### Remove all Snapshots of a Volume
* **Method:** _DELETE_
* **Endpoint**:`/volumes/{volume_uuid}/snapshot`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Temporary Resource Response HTTP Status Code**: 204

The matching CLI extension might look like

```
$ heketi-cli volume delete-snapshots <vol_uuid>
```

### Cloning of BlockVolumes
The `/blockvolumes/(block_uuid}/clone` endpoint should behave like
`/volumes/{volume_uuid}/clone`. Additional snapshot features can be implemented
under `/snaphots/{snapshot_uuid}`. The Heketi internal snapshot object
representation should have a `Type: {file-volume|block-volume}` so that the
different volume types can be handled as expected.


# Kubernetes Snapshotting Proposal

[Volume
Snapshotting](https://github.com/kubernetes-incubator/external-storage/blob/master/snapshot/doc/volume-snapshotting-proposal.md)
in the Kubernetes external-storage provisioner.

# Gluster Snapshot CLI Reference
```
$ gluster --log-file=/dev/null snapshot help

gluster snapshot commands
=========================

snapshot activate <snapname> [force] - Activate snapshot volume.
snapshot clone <clonename> <snapname> - Snapshot Clone.
snapshot config [volname] ([snap-max-hard-limit <count>] [snap-max-soft-limit <percent>]) | ([auto-delete <enable|disable>])| ([activate-on-create <enable|disable>]) - Snapshot Config.
snapshot create <snapname> <volname> [no-timestamp] [description <description>] [force] - Snapshot Create.
snapshot deactivate <snapname> - Deactivate snapshot volume.
snapshot delete (all | snapname | volume <volname>) - Snapshot Delete.
snapshot help - display help for snapshot commands
snapshot info [(snapname | volume <volname>)] - Snapshot Info.
snapshot list [volname] - Snapshot List.
snapshot restore <snapname> - Snapshot Restore.
snapshot status [(snapname | volume <volname>)] - Snapshot Status.
```

- [Snapshot](https://github.com/gluster/glusterfs-specs/blob/master/done/GlusterFS%203.6/Gluster%20Volume%20Snapshot.md)
- [Cloning](https://github.com/gluster/glusterfs-specs/blob/master/done/GlusterFS%203.7/Clone%20of%20Snapshot.md)
