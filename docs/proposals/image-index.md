# Image Classification API Proposal

## Problem

Allow to efficiently report usage of images in the cluster by tracking the
relationship between images from the **base** image to the **application** image.

By having this system in place, we can provide a better auditing of which and
how images are used. In case of a vulnerability disclosure in one of the base
images, we can identify the affected applications built on top of this base
image and trigger an update.

Additionally, administrator should be able to limit the usage to certain image
classes (iow. On this cluster, you are only allowed to deploy applications
based on *base* image).

## Use Cases

* Classify a certain image by a label and have that label propagated to all descentants of that
  image currently present in the cluster.
  * By classification, we are giving an Image context, or role in the cluster. For example an image
    classified as "VENDOR" represents all images that are based on this "VENDOR" image.
* Be able to query:
  * ImageStreamTags that currently reference an Image classified as *foo*
  * ImageStreamTags that reference outdated version of an Image classified as *foo*
  * All Images based on the Image *foo* (descentants)
  * All Images that the *foo* images is based on (ancestors)
* Be able to track the usage of images limited by an license by giving the image carrying the licenced
  content a class
* Be able to enforce policies based on classification:
  * Allow running only *VENDOR* based images in this cluster
* Be able to perform bulk operations based on the classification:
  * Trigger a build for all BuildConfigs that have builder image based on *VENDOR*.

### Current State

The information about the Image(s) we gather is not normalized in any way. It
does not provide any information about relationships between images, other than
their affiliation with ImageStream. Furthermore, all of the information
currently available is scattered throughout the entire system, and accessed
from different parts of the system in non-unified way.

This results in the tools such as pruning or `oadm top images` always build
in-memory state of the cluster for their operation.

### Goal

To build efficient *Image Index API* that allows to querying the relations
between images and also name and map these relations into human-readable form.

### Known Limitations / Scope Of Work

* We can only keep track of Images we know about (iow. imported into OpenShift)
  * If somebody use an external image, we can’t map it to indexing API without importing
    it into an Image object
* Currently, there is no way to restrict users referencing an external images in Pods.
  * Solving this problem goes beyond the scope of this proposal
* This API is meant to only query the images/imagestreams not their usage in the Pods.
* The classification is meant to be managed and used by cluster/registry administrators,
  not by regular users

## Proposed Workflow

* As an admin I want to identify all *VENDOR* based images in the cluster:
  * I create new `classification` using the *VENDOR* ImageStream(Tag)
  * `ImageClassificationController` discovers all descentants of this Image (or any previous
     version of this Image) in the cluster
  * `ImageClassificationController` applies label/annotations to the Image(s) that include
    reference to *VENDOR*

* As an admin I want to group the classifications together, so I can send "key=value" queries, for example:
  * I want to get all images with `os=rhel7` or `vendor=redhat` or `lang=python`:
  * For that I create a new classification group named “os” and provide an “selector” that matches the labels
    in the target “classification” objects.

### Proposed API Changes

For the implementation, two new API types are needed:

```go
// ImageClass provides a single Image "class" a context that a
// certain group if Images shares. For example "rhel7" or "python".
// To create an ImageClass, you have to point it to an initial
// or "target" image first. From that point, the ImageClassController
// will track the history of that Image.
type ImageClass struct{
		unversioned.TypeMeta
		kapi.ObjectMeta

		Current      ImageChain   
		History      []ImageChain  // Should be in 'Status' ?
}

// ImageClassGroup groups the various ImageClass objects into a
// logical group. For example a "vendor" group groups all ImageClass
// objects that matches the defined selector.
type ImageClassGroup struct {
		unversioned.TypeMeta
		kapi.ObjectMeta

		Selector: map[string]string
}

// ImageChain represents an internal state of the Image layers and
// the chains based on the image layers.
type ImageChain struct {
		Chains    []string
}
```

The `ImageClass` object is needed to keep the historical informations about the
image persistent as the in-memory image chain index will only contain the
images and chains that are currently available in the cluster.

In future this allows to see which images are based on old or insecure images
and mark the affected images as outdated.


### Image Chains Index

The Image is represented by the list of its layers. To be able to identify the
base image, we have to introspect the layers and calculate the "hash chains"
for the image layers. We also need to keep this index in memory for fast
access.

For example:

* The `base` image contains 3 layers: `AAA`, `BBB` and `CCC`
  * For that we have 3 chains: `chain1(AAA)`, `chain2(AAA, BBB)`, `chain3(AAA, BBB, CCC)`

* An `ancestor` image contains 4 layers: `AAA`, `BBB`, `CCC` and `DDD`
  * To say that the "ancestor" image is based on the `base` image, we can
    assume that the "ancestor" image must contain the `chain3` in its chains list.


For the implementation we need a indexed informer that contains two indexes:

* `ChainIndex` that provides mapping between a chain and the list of images containing
  this chain:
  * `chain => [image1, image2, image3]`
* `LastChainIndex` that provides mapping between "last" chain and the image that it represents:
  * `chain => [image3]`

The memory footprint for this index can be estimated by the size of the items
that have to be stored. The `ChainIndex` needs a single key for each image layer,
and the chain itself is represented by the 64 strings (SHA256). The Images
re-use layers so the cost goes down with the number of re-used layers.

The `LastChainIndex` cost is the same as the number of images in the cluster. The
key represents an pointer to the image object.
