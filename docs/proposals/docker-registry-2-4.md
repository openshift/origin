# Support for Docker Registry 2.4
## Abstract

A proposal to update the docker/distribution to v2.4.0.

## Motivation

Primary goal is to migrate to Schema 2 [1].

## Compatibility

docker/distribution maintains compatibility with old clients that don't know
how to work with Schema 2.

##### Docker 1.9

When the manifest is pulled by digest or tag with any docker version, a Schema 1 manifest will be returned.

##### Docker 1.10

Docker Engine 1.10 tries to construct a manifest in the new format when pushing an image.
If uploading this manifest fails, presumably because the registry only supports the old format,
Docker will fall back to uploading a manifest in the old format.

When the manifest is pulled by digest or tag with Docker Engine 1.10, a Schema 2 manifest will be returned.

When the manifest is pulled by tag with Docker Engine 1.9 and older, the manifest is converted on-the-fly
to Schema 1 and sent with the response. Docker 1.9 is compatible with this older format.

If a manifest is pulled from a registry by digest with Docker Engine 1.9 and older,
and the manifest was pushed with Docker Engine 1.10, a security check will cause the Engine
to receive a manifest it cannot use and the pull will fail.

##### Openshift registry

For users who do not want to use Schema 2 at all: we can reject Schema 2 by config parameter and
Docker Engine 1.10 will push the images in old format in accordance with the push strategy.

## API changes

* `MediaType` needs to be added to `ImageLayer`.
* `Image.DockerImageMetadata` will be empty (except ID and Size). We don't have any data to fill it.

## Upgrade process

No data migration is required; existing metadata will be gradually replaced with new records.

## Manifest List

I propose not to add support for Manifest Lists. The manifest list is the "fat manifest" which
points to specific image manifests for one or more platforms. In other words, this is a group
of images. This is a new object for openshift and it's too dificult to add it with the upgrade.

## Links

1. [manifest-v2-2.md](https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-2.md)
1. [proof-of-concept](https://github.com/legionus/origin/commits/docker-registry-v2.4.0)
