#
# This image is used for running a master node of an openshift dev cluster.
#
# The standard name for this image is openshift/dind-master
#

FROM openshift/dind-base

ADD supervisord.conf /etc/supervisord.conf

CMD [ "/usr/bin/supervisord" ]
