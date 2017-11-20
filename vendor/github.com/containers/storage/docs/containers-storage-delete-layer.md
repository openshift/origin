## containers-storage-delete-layer 1 "August 2016"

## NAME
containers-storage delete-layer - Delete a layer

## SYNOPSIS
**containers-storage** **delete-layer** *layerNameOrID*

## DESCRIPTION
Deletes a layer if it is not currently being used by any images or containers,
and is not the parent of any other layers.

## EXAMPLE
**containers-storage delete-layer my-base-layer**

## SEE ALSO
containers-storage-create-layer(1)
containers-storage-delete-image(1)
containers-storage-delete-layer(1)
