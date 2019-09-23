## containers-storage-unmount 1 "August 2016"

## NAME
containers-storage unmount - Unmount a layer or a container's layer

## SYNOPSIS
**containers-storage** **unmount** *layerOrContainerMountpointOrNameOrID*

## DESCRIPTION
Unmounts a layer or a container's layer from the host's filesystem.

## EXAMPLE
**containers-storage unmount my-container**

**containers-storage unmount /var/lib/containers/storage/mounts/my-container**

## SEE ALSO
containers-storage-mount(1)
containers-storage-mounted(1)
