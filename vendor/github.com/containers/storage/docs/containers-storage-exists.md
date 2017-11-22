## containers-storage-exists 1 "August 2016"

## NAME
containers-storage exists - Check if a layer, image, or container exists

## SYNOPSIS
**containers-storage** **exists** [*options* [...]] *layerOrImageOrContainerNameOrID* [...]

## DESCRIPTION
Checks if there are layers, images, or containers which have the specified
names or IDs.

## OPTIONS
**-c | --container**

Only succeed if the names or IDs are that of containers.

**-i | --image**

Only succeed if the names or IDs are that of images.

**-l | --layer**

Only succeed if the names or IDs are that of layers.

**-q | --quiet**

Suppress output.

## EXAMPLE
**containers-storage exists my-base-layer**
