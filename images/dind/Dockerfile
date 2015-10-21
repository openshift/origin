#
# This image is used for running a host of an openshift dev cluster.
#
# The standard name for this image is openshift/dind
#

FROM fedora:21

RUN yum -y update && yum -y install supervisor git golang hg tar make \
  hostname bind-utils iproute iputils which procps-ng \
  # Node-specific packages
  docker openvswitch bridge-utils ethtool \
  && yum clean all

ENV DIND_COMMIT 4e899d64e020a67ca05f913d354aa8d99a341a7b
RUN curl -fL "https://raw.githubusercontent.com/docker/docker/${DIND_COMMIT}/hack/dind" \
  -o /usr/local/bin/dind && chmod +x /usr/local/bin/dind

# Use a bash script to work around supervisord's inability to allow
# arbitrary environment variables in command strings.
ADD wrapdind /usr/local/bin/wrapdind
RUN chmod +x /usr/local/bin/wrapdind

VOLUME /var/lib/docker

ADD supervisord.conf /etc/supervisord.conf

CMD [ "/usr/bin/supervisord" ]
