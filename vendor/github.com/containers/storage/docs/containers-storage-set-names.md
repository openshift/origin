## containers-storage-set-names 1 "August 2016"

## NAME
containers-storage set-names - Set names for a layer/image/container

## SYNOPSIS
**containers-storage** **set-names** [**-n** *name* [...]] *layerOrImageOrContainerNameOrID*

## DESCRIPTION
In addition to IDs, *layers*, *images*, and *containers* can have
human-readable names assigned to them in *containers-storage*.  The *set-names*
command can be used to reset the list of names for any of them.

## OPTIONS
**-n | --name** *name*

Specifies a name to set on the layer, image, or container.  If a specified name
is already used by another layer, image, or container, it is removed from that
other layer, image, or container.  Any names which are currently assigned to
this layer, image, or container, and which are not specified using this option,
will be removed from the layer, image, or container.

## EXAMPLE
**containers-storage set-names -n my-one-and-only-name f3be6c6134d0d980936b4c894f1613b69a62b79588fdeda744d0be3693bde8ec**

## SEE ALSO
containers-storage-add-names(1)
containers-storage-get-names(1)
