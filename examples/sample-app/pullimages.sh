#!/bin/sh
# Pull the Docker images used in this sample
#
# Args:
#   TAG: image tag, the tag should be consistent with openshift version
#
# Example:
#   ./pullimage.sh 
#   ./pullimage.sh TAG

if [ $# -ne 0 ];then
    tag=$1
    echo "using image tag: $tag"
fi

docker pull openshift/origin-docker-registry:$tag
#docker pull openshift/origin-docker-builder
docker pull openshift/origin-sti-builder:$tag
docker pull openshift/origin-deployer:$tag  
