#
# This is the base image for an openshift docker-in-docker dev cluster.
#
# The standard name for this image is openshift/dind-base
#

FROM fedora:22

RUN dnf -y update && dnf -y install supervisor git golang hg tar make \
    hostname bind-utils iproute iputils which procps-ng \
    && dnf clean all
