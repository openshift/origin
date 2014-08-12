An image stream is a collection of images that share the same metadata. It contains multiple
versions of an image as it evolves over time with new builds of the codebase. 

An image stream is one of the sources of image metadata. Metadata for an image can come from one of three places:
* The original image binary as specified by the Dockerfile - this includes environment variables, command to execute, ports exposed, etc.
* The image definition in the OpenShift environment. This can be used to override metadata from the Dockerfile
* The image stream which overrides individual image metadata.

Some images can be used as services while others can be used as single execution jobs.
