## containers-storage-get-container-data 1 "August 2016"

## NAME
containers-storage get-container-data - Retrieve lookaside data for a container

## SYNOPSIS
**containers-storage** **get-container-data** [*options* [...]] *containerNameOrID* *dataName*

## DESCRIPTION
Retrieves a piece of named data which is associated with a container.

## OPTIONS
**-f | --file** *file*

Write the data to a file instead of stdout.

## EXAMPLE
**containers-storage get-container-data -f config.json my-container configuration**

## SEE ALSO
containers-storage-list-container-data(1)
containers-storage-set-container-data(1)
