## containers-storage-get-image-data 1 "August 2016"

## NAME
containers-storage get-image-data - Retrieve lookaside data for an image

## SYNOPSIS
**containers-storage** **get-image-data** [*options* [...]] *imageNameOrID* *dataName*

## DESCRIPTION
Retrieves a piece of named data which is associated with an image.

## OPTIONS
**-f | --file** *file*

Write the data to a file instead of stdout.

## EXAMPLE
**containers-storage get-image-data -f manifest.json my-image manifest**

## SEE ALSO
containers-storage-list-image-data(1)
containers-storage-set-image-data(1)
