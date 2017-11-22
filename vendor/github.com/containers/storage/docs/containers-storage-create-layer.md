## containers-storage-create-layer 1 "August 2016"

## NAME
containers-storage create-layer - Create a layer

## SYNOPSIS
**containers-storage** **create-layer** [*options* [...]] [*parentLayerNameOrID*]

## DESCRIPTION
Creates a new layer which either has a specified layer as its parent, or if no
parent is specified, is empty.

## OPTIONS
**-n** *name*

Sets an optional name for the layer.  If a name is already in use, an error is
returned.

**-i | --id** *ID*

Sets the ID for the layer.  If none is specified, one is generated.

**-m | --metadata** *metadata-value*

Sets the metadata for the layer to the specified value.

**-f | --metadata-file** *metadata-file*

Sets the metadata for the layer to the contents of the specified file.

**-l | --label** *mount-label*

Sets the label which should be assigned as an SELinux context when mounting the
layer.

## EXAMPLE
**containers-storage create-layer -f manifest.json -n new-layer somelayer**

## SEE ALSO
containers-storage-create-container(1)
containers-storage-create-image(1)
containers-storage-delete-layer(1)
