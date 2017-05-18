## Improve docker-registry

### Motivation

Openshift's docker-registry always resolve and store in the etcd the last digest. So we always know what object
we need to request. But we can't get access to it without mapping to the repository through which it was uploaded.

In `docker/distribution` there is no way to read/write an object without using the `linkedBlobStore`.
This module deals with the comparison of objects and links to them from user repositories. These links are made
to reduce the amount of disk space.

We need to stop using `linkedBlobStore` from `docker/distribution`. The main purpose for this module is to store information
about blobs because `docker/distribution` don't have own database. It places information in the filesystem.

The `docker/distribution` allow you to add middleware for
[Registry](https://github.com/openshift/origin/blob/master/Godeps/_workspace/src/github.com/docker/distribution/registry/middleware/registry/middleware.go),
[Repository](https://github.com/openshift/origin/blob/master/Godeps/_workspace/src/github.com/docker/distribution/registry/middleware/repository/middleware.go) and
[Storage driver](https://github.com/openshift/origin/blob/master/Godeps/_workspace/src/github.com/docker/distribution/registry/storage/driver/middleware/storagemiddleware.go)
(there are several different types of middlewares but they do not fit for us).
The first two can't access the blobs directly because path to them will be created by `linkedBlobStore`.
The third has access to blobs, but cannot build a path. The path is created in a higher layer.

### Solution

Openshift already uses an etcd as a database. All infomation about images/imagestreams are there. We can hold information
about the blobs in repository in the database as well. As result, we need to store only the blobs's payload in the filesystem
using our own objects layout. In this case, we get complete control over the blobs.

To do this we will need to implement part of the functionality of the `linkedBlobStore`. We can create our own middleware for
repository and storage driver. The middleware for storage driver will be used for reading and writing in old (docker/distribution)
and new layout.

We need to make a new layout because the layout that uses `docker/distribution` private and upstream doesn't
want to open it. The important point is that we don't have to replace the whole old layout. We need only the part
that is used to store blobs.

### New objects layout

In the layout I propose to use are similar to `docker/distribution` but simpler:
```
uploadDataPathSpec:      /openshift/v1/repositories/<name>/_uploads/<id>/data
uploadStartedAtPathSpec: /openshift/v1/repositories/<name>/_uploads/<id>/startedat
uploadHashStatePathSpec: /openshift/v1/repositories/<name>/_uploads/<id>/hashstates/<algorithm>/<offset>

blobPathSpec:            /openshift/v1/blobs/<algorithm>/<first two hex bytes of digest>/<hex digest>
blobDataPathSpec:        /openshift/v1/blobs/<algorithm>/<first two hex bytes of digest>/<hex digest>/data
```
That's all we need. Everything else is stored in the database or in old layout.

### Benefits

* Cross repository access to blobs;
* Reduce the number of patches for the `docker/distibution`;
* Possibility of applying the quota at the stage of upload.

### Negative aspects

* We have to implement part of the functionality of the `blobStore` and `linkedBlobStore`.
