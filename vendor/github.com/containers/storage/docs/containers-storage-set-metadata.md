## containers-storage-set-metadata 1 "August 2016"

## NAME
containers-storage set-metadata - Set metadata for a layer, image, or container

## SYNOPSIS
**containers-storage** **set-metadata** [*options* [...]] *layerOrImageOrContainerNameOrID*

## DESCRIPTION
Updates the metadata associated with a layer, image, or container.  Metadata is
intended to be small, and is expected to be cached in memory.

## OPTIONS
**-f | --metadata-file** *filename*

Use the contents of the specified file as the metadata.

**-m | --metadata** *value*

Use the specified value as the metadata.

## EXAMPLE
**containers-storage set-metadata -m "compression: gzip" my-layer**

## SEE ALSO
containers-storage-metadata(1)
