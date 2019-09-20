
# Operation Auto-Cleanup

When the server is terminated during the execution of an operation or
an operation fails rollback the operation lingers in the heketi db
until manually cleaned up. The reason Heketi retains this information
is so that the underlying storage systems can be checked that all
potential items created by Heketi are removed.

This document lays out the general plan to implement the ability
for Heketi to make use of this information to clean up after
incomplete operations in a testable & repeatable manner.


## Cleaning Operations

An operation will be considered cleanable if it meets the
CleanableOperation interface. Not all operations will be cleanable
initially. For any operation that is cleanable, the rollback
function can be implemented in terms of the clean functions.

The cleanable interface follows:
```golang
type CleanableOperation interface {
    Operation

    // Clean will undo any changes made to the storage system.
    Clean(executors.Executor) error
    // CleanDone will undo any change to the DB. It may not
    // be called before Clean completes successfully.
    CleanDone() error
}
```

Heketi will gain new functionality for iterating over one or
more operation in the db, loading the operation type from the
db and executing the clean functions. Because these functions
will use the executor each pair of clean functions should
be treated similarly to a running operation. When an operation
is being cleaned it will take a slot in the operations counter
such that the operations throttle will treat it like a running
operation. Because of this, the initial design will only
clean operations serially. This way no more than one additional
pending operation is blocked by an active clean.

A new operation state in the db will be added such that all
operations that fail are actively marked as "failed". Functions
will be able to iterate over "failed" and/or "stale" operations
and trigger clean ups.

Any time the Clean function fails the operation's status in the
Heketi db will remain unchanged in order to be able to retry
the clean up again at a later time. In order to track the
(lack of) success of cleaning each pending operation entry will
track the number of clean up attempts and a timestamp of the
last clean up attempt.


## Initiating Clean-ups

The clean up functionality can be triggered:
* Offline, through a `heketi` command line command
* Online, after Heketi server is started
* Online, a running server will periodically check for operations to clean up
* On demand, when an admin requests for clean ups using `heketi-cli`


Offline mode uses an existing Heketi db and executor configuration without
relying on a running Heketi server. This action can be seen as a high
priority cleanup driven by administrators or support or as part of a
disaster recovery scenario. In this mode, the user will run a command
such as `heketi offline cleanup-operations` which will perform only
clean ups of operations in the db and then exit.
Much like the `heketi db` subcommands, this command may only be run
with exclusive access to the Heketi db, and can not run when the server
is also running.

Online cleanups are initiated from within a running Heketi server process
and include cleanup after restart, periodic cleanup, and on-demand cleanup.

Cleanup after restart exists primarily to clean up any operations
that were running when a Heketi server was terminated. However, it will
also clean up any failed operations from previous instances of the
server. Clean up after restart will be enabled by default but can be
disabled by admins via the Heketi configuration.

Periodic cleanup is needed to avoid requiring restarts of the server
or cron jobs running heketi-cli; long running Heketi servers will
still clean up failed operations. Periodic cleanup is triggered by
an internal timer. In order to prevent running cleanups when the
server is under load, the periodic cleanup mechanism will also
consider the state of the operations counter and only trigger cleanup
after the operations counter has been zero (or some other low value)
for some time. This "cooldown" period will prevent starting cleanup
jobs when the Heketi server is needed to address incoming
operations for Kubernetes/OpenShift or admins. Both the cleanup time
and the cooldown time will be configurable but start with sensible
defaults.

There are cases where triggering clean-ups immediately are useful
and so endpoints on the heketi server and corresponding
`heketi-cli` commands will be provided to allow administrators to
trigger clean up of all failed & stale operations. Alternatively,
a specific operation ID can be provided to request clean up of
that operation. These commands will always be async and will not
poll the server for status of the clean-up.


# Management

Currently, the only tools for investigating pending operations are
the `heketi-cli server operations info` command and dumping the
heketi db. A `heketi-cli server operations list` command will be
added that will display a list of pending operations, kind, and statuses
possibly along with useful information (such as volumes/bricks/etc).
This command, along with logging, will allow users to tell if an
operation is in-flight, stale, failed, etc.

Metrics about pending operations roughly matching the operations info
subcommand will be added to the Heketi metrics endpoint.

To trigger on-demand clean-ups, a command
`heketi-cli server operations cleanup` will be added to trigger cleanups.
This command can be run without additional arguments to try to
clean up all stale and failed operations in the db. Providing one
or more operation IDs will request the server clean up that
specific list of IDs. Providing IDs not in the stale or failed
state will be an error.


# Testing

In order to properly test the feature we will need to be able to
interrupt, or simulate the interruption, of operations that would
normally succeed within the functional test framework. This will
be accomplished by adding an error-injection executor that can
intercept actions in a normal executor. For example, to test the
cleanup of a create volume operation we'd want to simulate failure
to of any of the steps in creating a brick or a volume and that
the cleanup function can undo the volume from any state on
the storage subsystems.


# Other

When clean-on-restart is implemented we will remove the default behavior
from Heketi where the server would refuse to start if stale pending
operations were found in the db. The configuration variables currently
used to disable this behavior will be ignored.


