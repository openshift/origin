## containers-storage-mount 1 "August 2016"

## NAME
containers-storage mount - Mount a layer or a container's layer for manipulation

## SYNOPSIS
**containers-storage** **mount** [*options* [...]] *layerOrContainerNameOrID*

## DESCRIPTION
Mounts a layer or a container's layer on the host's filesystem and prints the
mountpoint.

## OPTIONS
**-l | --label** *label*

Specify an SELinux context for the mounted layer.

## EXAMPLE
**containers-storage mount my-container**

## SEE ALSO
containers-storage-unmount(1)
