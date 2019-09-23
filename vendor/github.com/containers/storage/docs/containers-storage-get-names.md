## containers-storage-get-names 1 "September 2017"

## NAME
containers-storage get-names - Get names of a layer/image/container

## SYNOPSIS
**containers-storage** **get-names** *layerOrImageOrContainerNameOrID*

## DESCRIPTION
In addition to IDs, *layers*, *images*, and *containers* can have
human-readable names assigned to them in *containers-storage*.  The *get-names*
command can be used to read the list of names for any of them.

## OPTIONS

## EXAMPLE
**containers-storage get-names f3be6c6134d0d980936b4c894f1613b69a62b79588fdeda744d0be3693bde8ec**

## SEE ALSO
containers-storage-add-names(1)
containers-storage-set-names(1)
