#
# This image is used for running a master node of an openshift dev cluster.
#
# The standard name for this image is openshift/dind-master
#

FROM fedora:22

RUN dnf -y update && dnf -y install supervisor git golang hg tar make \
    hostname bind-utils iproute iputils which procps-ng \
    && dnf clean all

ADD supervisord.conf /etc/supervisord.conf

CMD [ "/usr/bin/supervisord" ]
