## containers-storage-delete 1 "August 2016"

## NAME
containers-storage delete - Force deletion of a layer, image, or container

## SYNOPSIS
**containers-storage** **delete** *layerOrImageOrContainerNameOrID*

## DESCRIPTION
Deletes a specified layer, image, or container, with no safety checking.  This
can corrupt data, and may be removed.

## EXAMPLE
**containers-storage delete my-base-layer**

## SEE ALSO
containers-storage-delete-container(1)
containers-storage-delete-image(1)
containers-storage-delete-layer(1)
