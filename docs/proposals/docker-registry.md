## Improve docker-registry

We need to stop using `linkedBlobStore` from `docker/distribution`. The main purpose for this module is to store information
about blobs because `docker/distribution` don't have own database. It places information in the filesystem.

Openshift already uses an etcd as a database. All infomation about images/imagestreams are there. We can hold information
about the blobs in repository in the database as well. As result, we need to store only the blobs's payload in the filesystem
using our own objects layout. In this case, we get complete control over the blobs.

To do this we will need to implement part of the functionality of the `linkedBlobStore`. We can create our own middleware for
repository and storage driver. The middleware for storage driver will be used for reading and writing in old (docker/distribution)
and new layout.

We need to make a new layout because the layout that uses `docker/distribution` private and upstream doesn't
want to open it.

### Negative aspects

* We have to implement part of the functionality of the `blobStore` and `linkedBlobStore`.

### New objects layout

In the layout I propose to use are similar to `docker/distribution` but simpler:
```
blobPathSpec:     /openshift/v1/blobs/<algorithm>/<first two hex bytes of digest>/<hex digest>
blobDataPathSpec: /openshift/v1/blobs/<algorithm>/<first two hex bytes of digest>/<hex digest>/data
```
That's all we need. Everything else is stored in the database.

### What is missing in docker/distribution

* Would be very good to have [validateBlob](https://github.com/openshift/origin/blob/master/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/blobwriter.go#L137)
as a public function.
