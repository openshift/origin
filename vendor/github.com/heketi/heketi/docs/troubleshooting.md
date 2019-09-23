Troubleshooting Guide
========================================

## Deployment

## Setup

1. Error "Unable to open topology file":
    * You use old syntax of single '-' as prefix for json option. Solution: Use new syntax of double hyphens.
1. Cannot create volumes smaller than 4GiB:
    * This is due to the limits set in Heketi.  If you want to change the limits, please update the config file using the [advanced settings](admin/server.md#advanced-options).  If you are using Heketi as a container, then you must create a new container based on Heketi which just updates the config file `/etc/heketi/heketi.json`.

## Management

### Heketi starts up with a warning saying:

    "Heketi has existing pending operations in the db."

In the newest versions of Heketi the system has support for
automatically cleaning up most operations that are either failed
(started, encountered an error, but failed to rollback) and stale
(server restarted before operation finished) operation.

Until an operation can be cleaned up from the storage system an
operation will remain in the Heketi database so that Heketi knows what
must be checked on the storage system. The items (bricks, volumes, etc.)
associated with that operation continue to virtually take up space so
that Heketi does not double allocate that space on the storage system
until it is sure that the items have been fully removed. Clean up may be
retried many times.

To view operations & pending operation metadata on a running server the
commands `heketi-cli server operations info` and `heketi-cli server
operations list` may be used. The first gives a tally of the state of
the operations the server knows about.  The latter command lists all
pending operations stored in db.  The command `heketi-cli server
operations info <OP-ID>` can be used to get more detailed information
about a particular pending operation.

The server will attempt to clean up a short time after it starts and
every so often while it is running. Additionally, an administrator may
request that Heketi run the clean up process by running the command
`heketi-cli server operations cleanup` or `heketi-cli server operations
cleanup [OP-ID-1] [OP-ID-2] ... [OP-ID-N]`.  The first command will
instruct Heketi to try and clean up all stale or failed operations in
the db. The latter command requests Heketi try and clean up only those
operations with the given IDs.

Current versions of Heketi always start up regardless of the presence of
stale pending operations in the db. If needed, the cleanup procedure of
exporting the db to JSON, editing it, and re-importing the db still
applies to the current version.


### Preventing Stale Pending Operations

To prevent stale pending operations from appearing in the system one
needs to avoid stopping the Heketi server while it is processing
existing operations. To do this, the server must be prevented from
accepting new requests while the existing requests are processed and
stopping the server only when the number of in-flight operations is
zero.

One simple approach to this is activate one of Heketi's administrative
modes.  To do this run `heketi-cli server mode set local-client` or
`heketi-cli server mode set read-only`. The former command will stop the
server from accepting new change requests from all hosts except the
localhost (127.0.0.1) and the latter will stop the server from accepting
new change requests from all hosts.  Informational requests (GETs to the
API) continue to be accepted so clients will be able to continue to wait
for results and admins can monitor the server.

Run the `heketi-cli server operations info` command and wait until the
"in-flight" counter is zero. Once it is zero the server can be stopped
knowing no new operations will be accepted.


### Checking database consistency

In older versions of Heketi, the database could become inconsistent due to bugs
in those versions of the software. To check the consistency of the database, the
cli variant of the command is `heketi-cli db check` and the server variant of
the command is `heketi db check`. These commands only need access to the
database. They don't collect or compare data with gluster.


### Comparing state in heketi database with the state of Gluster

Heketi manages Gluster Storage pools and stores the state in its database. In some cases, the state of Gluster may diverge from that stored in the database either due to bugs or due to activities performed directly on Gluster Storage pools bypassing Heketi. To summarize such differences, run the `heketi-cli server state examine gluster` command or `heketi offline state examine gluster --config /path/to/heketi.config.json` command. It fetches the following data from Gluster pools:
  1. Gluster volume info
  2. Blockvolumes list
  3. Bricks mount information
  4. Device information from LVM

The command reports the data collected and also the following comparisons
  1. Volume list of heketi with that of gluster volume info.

Known issues:
offline mode might not work with kubeexec executor if not run with right privileges.

## Management (Older Versions)

### Heketi refuses to start and the log lines contain the string:

    "Heketi was terminated while performing one or more operations."

In Heketi 6.0 support for pending operations were added to the system.
These pending operations are logged in the db before Heketi
configures Gluster and then are removed when the action is
complete. However, if the Heketi process is terminated while it is
performing these actions the pending operations will remain
in the db. Because these stale pending operations exist in the db
Heketi (by default) refuses to start, in order to avoid many of
these stale operation entries from piling up and giving the
administrator an opportunity to clean up any half-completed
items on the Gluster side.

In order to start Heketi again one can:
* Set the environment variable `HEKETI_IGNORE_STALE_OPERATIONS=true`.
  If provided, the value should be "true" or "false".
  When set to "true" the variable will force Heketi to start despite the
  existence of stale pending operation entries in the db.
  _Important_: This option exists for expedience and forcing Heketi
  to start will not resolve the stale operations and any inconsistencies
  between Heketi and Gluster.

  One should schedule some planned downtime and use the methods
  below to really resolve the situation.
  If you are running Heketi within a container this variable can
  be passed into the container using the appropriate tools for
  your system.
* Remove stale pending operation entries from the db using the db
  export/import tools. One can use the
  `heketi db export --dbfile heketi.db --jsonfile /tmp/q.json`
  command to export the database to JSON. One can then manipulate
  the JSON such that all sub-items under "pendingoperations" are
  removed as well as removing any volumes and bricks that have a non-empty
  "Pending"/"Id" value.
  The DB can then be updated via
  `heketi db import --dbfile heketi2.db --jsonfile /tmp/q.json` and the new
  db copied over the old db (the method depends on what environment
  Heketi is running in).

  Remember to add back the free storage space of devices for bricks you
  are deleting, or resync the storage space using the Heketi resync feature
  (see help for `heketi-cli device resync`).

Before or after the Heketi DB is repaired one should examine
the GlusterFS system for orphaned volumes, bricks, and LVM volumes
and clean them up, using Gluster and LVM command line tools, if needed.
