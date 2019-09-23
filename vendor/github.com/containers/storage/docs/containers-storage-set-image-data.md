## containers-storage-set-image-data 1 "August 2016"

## NAME
containers-storage set-image-data - Set lookaside data for an image

## SYNOPSIS
**containers-storage** **set-image-data** [*options* [...]] *imageNameOrID* *dataName*

## DESCRIPTION
Sets a piece of named data which is associated with an image.

## OPTIONS
**-f | --file** *filename*

Read the data contents from a file instead of stdin.

## EXAMPLE
**containers-storage set-image-data -f ./manifest.json my-image manifest**

## SEE ALSO
containers-storage-list-image-data(1)
containers-storage-get-image-data(1)
containers-storage-get-image-data-size(1)
containers-storage-get-image-data-digest(1)
