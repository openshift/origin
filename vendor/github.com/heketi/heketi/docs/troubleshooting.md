Troubleshooting Guide
========================================

## Deployment

## Setup

1. Error "Unable to open topology file":
    * You use old syntax of single '-' as prefix for json option. Solution: Use new syntax of double hyphens.
1. Cannot create volumes smaller than 4GiB:
    * This is due to the limits set in Heketi.  If you want to change the limits, please update the config file using the [advanced settings](admin/server.md#advanced-options).  If you are using Heketi as a container, then you must create a new container based on Heketi which just updates the config file `/etc/heketi/heketi.json`.

## Management

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
