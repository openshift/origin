## containers-storage-import-layer 1 "April 2019"

## NAME
containers-storage import-layer - Import files to a new layer

## SYNOPSIS
**containers-storage** **import-layer** [*options* [...]] [*parentLayerNameOrID*]

## DESCRIPTION
This subcommand is a combination of *containers-storage create-layer* and
*containers-storage apply-diff*.

When a layer is first created with *containers-storage create-layer*, it
contains no changes relative to its parent layer.
The layer can either be mounted read-write and its contents modified
directly, or contents can be added (or removed) by applying a layer diff
by running *containers-storage apply-diff*.

## OPTIONS
**-n** *name*

Sets an optional name for the layer. If a name is already in use, an error is
returned.

**-i | --id** *ID*

Sets the ID for the layer. If none is specified, one is generated.

**-f | --file** *filename*

Specifies the name of a file from which the diff should be read. If this
option is not used, the diff is read from standard input.

**-l | --label** *mount-label*

Sets the label which should be assigned as an SELinux context when mounting the
layer.

**-r | --readonly**

Mark the layer as readonly.

**-j | --json**

Prefer JSON output.

**--uidmap**

UID map specified in the format expected by *subuid*. It cannot be specified simultaneously with *--hostuidmap*.

**--gidmap**

GID map specified in the format expected by *subgid*. It cannot be specified simultaneously with *--hostuidmap*.

**--hostuidmap**

Force host UID map. It cannot be specified simultaneously with neither *--uidmap* nor *--subuidmap*.

**--hostgidmap**

Force host GID map. It cannot be specified simultaneously with neither *--gidmap* nor *--subgidmap*.

**--subuidmap** *username*

Create UID map for username using the data from /etc/subuid. It cannot be specified simultaneously with *--hostuidmap*.

**--subgidmap** *group-name*

Create GID map for group-name using the data from /etc/subgid. It cannot be specified simultaneously with *--hostuidmap*.

## EXAMPLE
**containers-storage import-layer -f 71841c97e320d6cde.tar.gz -n new-layer somelayer**

## SEE ALSO
containers-storage-create-layer(1)
containers-storage-apply-diff(1)
