This is `containers-storage`, a command line tool for manipulating local
layer/image/container stores.

It depends on `storage`, which is a pretty barebones wrapping of the graph
drivers that exposes the create/mount/unmount/delete operations and adds enough
bookkeeping to know about the relationships between layers.

On top of that, `storage` provides a notion of a reference to a layer which is
paired with arbitrary user data (i.e., an `image`, that data being history,
configuration, and other metadata).  It also provides a notion of a type of
layer, which is typically the child of an image's topmost layer, to which
arbitrary data is directly attached (i.e., a `container`, where the data is
typically configuration).

Layers, images, and containers are each identified using IDs which can be set
when they are created (if not set, random values are generated), and can
optionally be assigned names which are resolved to IDs automatically by the
various APIs.

The containers-storage tool is a CLI that wraps that as thinly as possible, so
that other tooling can use it to import layers from images.  Those other tools
can then either manage the concept of images on their own, or let the API/CLI
handle storing the image metadata and/or configuration.  Likewise, other tools
can create container layers and manage them on their own or use the API/CLI for
storing what I assume will be container metadata and/or configurations.

Logic for importing images and creating and managing containers will most
likely be implemented elsewhere, and if that implementation ends up not needing
the API/CLI to provide a place to store data about images and containers, that
functionality can be dropped.
