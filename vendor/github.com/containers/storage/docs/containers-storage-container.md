## containers-storage-container 1 "August 2016"

## NAME
containers-storage container - Examine a single container

## SYNOPSIS
**containers-storage** **container** *containerNameOrID*

## DESCRIPTION
Retrieve information about a container: any names it has, which image was used
to create it, any names that image has, and the ID of the container's layer.

## EXAMPLE
**containers-storage container f3be6c6134d0d980936b4c894f1613b69a62b79588fdeda744d0be3693bde8ec**
**containers-storage container my-awesome-container**

## SEE ALSO
containers-storage-containers(1)
