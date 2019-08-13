## containers-storage-add-names "August 2016"

## NAME
containers-storage add-names - Add names to a layer/image/container

## SYNOPSIS
**containers-storage** **add-names** [*options* [...]] *layerOrImageOrContainerNameOrID*

## DESCRIPTION
In addition to IDs, *layers*, *images*, and *containers* can have
human-readable names assigned to them in *containers-storage*.  The *add-names*
command can be used to add one or more names to them.

## OPTIONS
**-n | --name** *name*

Specifies a name to add to the layer, image, or container.  If a specified name
is already used by another layer, image, or container, it is removed from that
other layer, image, or container.

## EXAMPLE
**containers-storage add-names -n my-awesome-container -n my-for-realsies-awesome-container f3be6c6134d0d980936b4c894f1613b69a62b79588fdeda744d0be3693bde8ec**

## SEE ALSO
containers-storage-get-names(1)
containers-storage-set-names(1)
