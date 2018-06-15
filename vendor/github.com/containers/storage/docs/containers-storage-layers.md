## containers-storage-layers 1 "August 2016"

## NAME
containers-storage layers - List known layers

## SYNOPSIS
**containers-storage** [*options* [...]] **layers**

## DESCRIPTION
Retrieves information about all known layers and lists their IDs and names, the
IDs and names of any images which list those layers as their top layer, and the
IDs and names of any containers for which the layer serves as the container's
own layer.

## OPTIONS
**-t | --tree**

Display results using a tree to show the hierarchy of parent-child
relationships between layers.

## EXAMPLE
**containers-storage layers**
**containers-storage layers -t**
