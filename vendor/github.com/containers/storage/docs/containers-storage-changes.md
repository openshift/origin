## containers-storage-changes 1 "August 2016"

## NAME
containers-storage changes - Produce a list of changes in a layer

## SYNOPSIS
**containers-storage** **changes** *layerNameOrID* [*referenceLayerNameOrID*]

## DESCRIPTION
When a layer is first created, it contains no changes relative to its parent
layer.  After that is changed, the *containers-storage changes* command can be used to
obtain a summary of which files have been added, deleted, or modified in the
layer.

## EXAMPLE
**containers-storage changes f3be6c6134d0d980936b4c894f1613b69a62b79588fdeda744d0be3693bde8ec**

## SEE ALSO
containers-storage-applydiff(1)
containers-storage-diff(1)
containers-storage-diffsize(1)
