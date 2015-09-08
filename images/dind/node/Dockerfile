#
# This image is used for running a node of an openshift dev cluster.
#
# The standard name for this image is openshift/dind-node
#

FROM openshift/dind-base

RUN dnf -y update && dnf -y install docker openvswitch bridge-utils ethtool \
    && dnf clean all

ENV DIND_COMMIT e253b43366abf618475237e15633c43dc2ad40e6
RUN curl -fL "https://raw.githubusercontent.com/jpetazzo/dind/${DIND_COMMIT}/wrapdocker" \
    -o /usr/local/bin/wrapdocker && chmod +x /usr/local/bin/wrapdocker

VOLUME /var/lib/docker

ADD supervisord.conf /etc/supervisord.conf

CMD [ "/usr/bin/supervisord" ]
