## containers-storage-copy 1 "April 2018"

## NAME
containers-storage copy - Copy content into a layer

## SYNOPSIS
**containers-storage** **copy** [--chown UID[:GID]] [*sourceLayerNameOrID*:]/path [...] *targetLayerNameOrID*:/path

## DESCRIPTION
Copies contents from a layer, or outside of layers, into a layer, performing ID mapping as appropriate.

## OPTIONS
**--chown** *UID[:GID]*

Set owners for the new copies to the specified UID/GID pair.  If the GID is not
specified, the UID is used.  The owner IDs are mapped as appropriate for the
target layer.

## EXAMPLE
**containers-storage copy layer-with-mapping:/root/config.txt layer-with-different-mapping:/root/config.txt**

## SEE ALSO
containers-storage(1)
