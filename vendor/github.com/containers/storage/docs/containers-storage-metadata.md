## containers-storage-metadata 1 "August 2016"

## NAME
containers-storage metadata - Retrieve metadata for a layer, image, or container

## SYNOPSIS
**containers-storage** **metadata** [*options* [...]] *layerOrImageOrContainerNameOrID*

## DESCRIPTION
Outputs metadata associated with a layer, image, or container.  Metadata is
intended to be small, and is expected to be cached in memory.

## OPTIONS
**-q | --quiet**

Don't print the ID or name of the item with which the metadata is associated.

## EXAMPLE
**containers-storage metadata -q my-image > my-image.txt**

## SEE ALSO
containers-storage-set-metadata(1)
