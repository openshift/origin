## containers-storage-delete-image 1 "August 2016"

## NAME
containers-storage delete-image - Delete an image

## SYNOPSIS
**containers-storage** **delete-image** *imageNameOrID*

## DESCRIPTION
Deletes an image if it is not currently being used by any containers.  If the
image's top layer is not being used by any other images, it will be removed.
If that image's parent is then not being used by other images, it, too, will be
removed, and the this will be repeated for each parent's parent.

## EXAMPLE
**containers-storage delete-image my-base-image**

## SEE ALSO
containers-storage-create-image(1)
containers-storage-delete-container(1)
containers-storage-delete-layer(1)
