% kpod(1) kpod-pull - Simple tool to pull an image from a registry
% Urvashi Mohnani
# kpod-pull "1" "July 2017" "kpod"

## NAME
kpod-pull - Pull an image from a registry

## SYNOPSIS
**kpod pull**
**NAME[:TAG|@DIGEST]**
[**--help**|**-h**]

## DESCRIPTION
Copies an image from a registry onto the local machine. **kpod pull** pulls an
image from Docker Hub if a registry is not specified in the command line argument.
If an image tag is not specified, **kpod pull** defaults to the image with the
**latest** tag (if it exists) and pulls it. **kpod pull** can also pull an image
using its digest **kpod pull [image]@[digest]**. **kpod pull** can be used to pull
images from archives and local storage using different transports.

## imageID
Image stored in local container/storage

## DESTINATION

 The DESTINATION is a location to store container images
 The Image "DESTINATION" uses a "transport":"details" format.

 Multiple transports are supported:

  **dir:**_path_
  An existing local directory _path_ storing the manifest, layer tarballs and signatures as individual files. This is a non-standardized format, primarily useful for debugging or noninvasive container inspection.

  **docker://**_docker-reference_
  An image in a registry implementing the "Docker Registry HTTP API V2". By default, uses the authorization state in `$HOME/.docker/config.json`, which is set e.g. using `(docker login)`.

  **docker-archive:**_path_[**:**_docker-reference_]
  An image is stored in the `docker save` formatted file.  _docker-reference_ is only used when creating such a file, and it must not contain a digest.

  **docker-daemon:**_docker-reference_
  An image _docker-reference_ stored in the docker daemon internal storage.  _docker-reference_ must contain either a tag or a digest.  Alternatively, when reading images, the format can also be docker-daemon:algo:digest (an image ID).

  **oci:**_path_**:**_tag_
  An image _tag_ in a directory compliant with "Open Container Image Layout Specification" at _path_.

  **ostree:**_image_[**@**_/absolute/repo/path_]
  An image in local OSTree repository.  _/absolute/repo/path_ defaults to _/ostree/repo_.

**kpod [GLOBAL OPTIONS]**

**kpod pull [GLOBAL OPTIONS]**

**kpod pull NAME[:TAG|@DIGEST] [GLOBAL OPTIONS]**

## GLOBAL OPTIONS

**--help, -h**
  Print usage statement

## SEE ALSO
kpod(1), crio(8), crio.conf(5)

## HISTORY
July 2017, Originally compiled by Urvashi Mohnani <umohnani@redhat.com>
