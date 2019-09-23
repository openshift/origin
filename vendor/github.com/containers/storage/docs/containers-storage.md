## containers-storage 1 "August 2016"

## NAME
containers-storage - Manage layer/image/container storage

## SYNOPSIS
**containers-storage** [**subcommand**] [**--help**]

## DESCRIPTION
The *containers-storage* command is a front-end for the *containers/storage* library.
While it can be used to manage storage for filesystem layers, images, and
containers directly, its main use cases are centered around troubleshooting and
querying the state of storage which is being managed by other processes.

Notionally, a complete filesystem layer is composed of a container filesystem
and some bookkeeping information.  Other layers, *children* of that layer,
default to sharing its contents, but any changes made to the contents of the
children are not reflected in the *parent*.  This arrangement is intended to
save disk space: by storing the *child* layer only as a set of changes relative
to its *parent*, the *parent*'s contents should not need to be duplicated for
each of the *parent*'s *children*.  Of course, each *child* can have its own
*children*.  The contents of *parent* layers should not be modified.

An *image* is a reference to a particular *layer*, along with some bookkeeping
information.  Presumably, the *image* points to a *layer* which has been
modified, possibly in multiple steps, from some general-purpose *parent*, so
that it is suitable for running an intended application.  Multiple *images* can
reference a single *layer*, while differing only in the additional bookkeeping
information that they carry.  The contents of *images* should be considered
read-only.

A *container* is essentially a *layer* which is a *child* of a *layer* which is
referred to by an *image* (put another way, a *container* is instantiated from
an *image*), along with some bookkeeping information.  They do not have
*children* and their *layers* can not be directly referred to by *images*.
This ensures that changes to the contents of a *container*'s layer do not
affect other *images* or *layers*, so they are considered writeable.

All of *layers*, *images*, and *containers* can have metadata which
*containers-storage* manages attached to them.  Generally this metadata is not
expected to be large, as it is cached in memory.

*Images* and *containers* can also have arbitrarily-named data items attached
to them.  Generally, this data can be larger than metadata, and is not kept in
memory unless it is being retrieved or written.

It is expected that signatures which can be used to verify an *image*'s
contents will be stored as data items for that *image*, along with any template
configuration data which is recommended for use in *containers* which derive
from the *image*.  It is also expected that a *container*'s run-time
configuration will be stored as data items.

Files belonging to a *readonly* *layer* will become deduplicated with *OSTree* if the configuration option *storage.ostree_repo* for saving the corresponding OSTree repository is provided.
This option won't work if *containers-storage* gets built without support for OSTree.

## SUB-COMMANDS
The *containers-storage* command's features are broken down into several subcommands:
 **containers-storage add-names(1)**           Add layer, image, or container name or names

 **containers-storage applydiff(1)**           Apply a diff to a layer

 **containers-storage changes(1)**             Compare two layers

 **containers-storage container(1)**           Examine a container

 **containers-storage containers(1)**          List containers

 **containers-storage create-container(1)**    Create a new container from an image

 **containers-storage create-image(1)**        Create a new image using layers

 **containers-storage create-layer(1)**        Create a new layer

 **containers-storage delete(1)**              Delete a layer or image or container, with no safety checks

 **containers-storage delete-container(1)**    Delete a container, with safety checks

 **containers-storage delete-image(1)**        Delete an image, with safety checks

 **containers-storage delete-layer(1)**        Delete a layer, with safety checks

 **containers-storage diff(1)**                Compare two layers

 **containers-storage diffsize(1)**            Compare two layers

 **containers-storage exists(1)**              Check if a layer or image or container exists

 **containers-storage get-container-data(1)**  Get data that is attached to a container

 **containers-storage get-image-data(1)**      Get data that is attached to an image

 **containers-storage image(1)**               Examine an image

 **containers-storage images(1)**              List images

 **containers-storage layers(1)**              List layers

 **containers-storage list-container-data(1)** List data items that are attached to a container

 **containers-storage list-image-data(1)**     List data items that are attached to an image

 **containers-storage metadata(1)**            Retrieve layer, image, or container metadata

 **containers-storage mount(1)**               Mount a layer or container

 **containers-storage mounted(1)**             Check if a file system is mounted

 **containers-storage set-container-data(1)**  Set data that is attached to a container

 **containers-storage set-image-data(1)**      Set data that is attached to an image

 **containers-storage set-metadata(1)**        Set layer, image, or container metadata

 **containers-storage set-names(1)**           Set layer, image, or container name or names

 **containers-storage shutdown(1)**            Shut down graph driver

 **containers-storage status(1)**              Check on graph driver status

 **containers-storage unmount(1)**             Unmount a layer or container

 **containers-storage version(1)**             Return containers-storage version information

 **containers-storage wipe(1)**                Wipe all layers, images, and containers

## OPTIONS
**--help**

Print the list of available sub-commands.  When a sub-command is specified,
provide information about that command.

**--debug, -D**

Increases the amount of debugging information which is printed.

**--graph, -g=/var/lib/containers/storage**

Overrides the root of the storage tree, used for storing layer contents and
information about layers, images, and containers.

**--run, -R=/var/run/containers/storage**

Overrides the root of the runtime state tree, currently used mainly for noting
the location where a given layer is mounted (see **containers-storage mount**) so that
it can be unmounted by path name as an alternative to unmounting by ID or name.

**--storage-driver, -s**

Specifies which storage driver to use.  If not set, but *$STORAGE_DRIVER* is
set in the environment, its value is used.  If the storage tree has previously
been initialized, neither needs to be provided.  If the tree has not previously
been initialized and neither is set, a hard-coded default is selected.

**--storage-opt=[]**

Set options which will be passed to the storage driver.  If not set, but
*$STORAGE_OPTS* is set in the environment, its value is treated as a
comma-separated list and used instead.  If the storage tree has previously been
initialized, these need not be provided.

## EXAMPLES
**containers-storage layers -t**

## BUGS
This is still a work in progress, so some functionality may not yet be
implemented, and some will be removed if it is found to be unnecessary.  That
said, if anything isn't working correctly, please report it to [the project's
issue tracker] (https://github.com/containers/storage/issues).
