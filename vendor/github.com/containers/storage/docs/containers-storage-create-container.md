## containers-storage-create-container 1 "August 2016"

## NAME
containers-storage create-container - Create a container

## SYNOPSIS
**containers-storage** **create-container** [*options*...] *imageNameOrID*

## DESCRIPTION
Creates a container, using the specified image as the starting point for its
root filesystem.

## OPTIONS
**-n | --name** *name*

Sets an optional name for the container.  If a name is already in use, an error
is returned.

**-i | --id** *ID*

Sets the ID for the container.  If none is specified, one is generated.

**-m | --metadata** *metadata-value*

Sets the metadata for the container to the specified value.

**-f | --metadata-file** *metadata-file*

Sets the metadata for the container to the contents of the specified file.

## EXAMPLE
**containers-storage create-container -f manifest.json -n new-container goodimage**

## SEE ALSO
containers-storage-create-image(1)
containers-storage-create-layer(1)
containers-storage-delete-container(1)
