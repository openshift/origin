## containers-storage-set-container-data 1 "August 2016"

## NAME
containers-storage set-container-data - Set lookaside data for a container

## SYNOPSIS
**containers-storage** **set-container-data** [*options* [...]] *containerNameOrID* *dataName*

## DESCRIPTION
Sets a piece of named data which is associated with a container.

## OPTIONS
**-f | --file** *filename*

Read the data contents from a file instead of stdin.

## EXAMPLE
**containers-storage set-container-data -f ./config.json my-container configuration**

## SEE ALSO
containers-storage-get-container-data(1)
containers-storage-list-container-data(1)
