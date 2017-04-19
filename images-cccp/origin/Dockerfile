#
# This is the official OpenShift Origin image. It has as its entrypoint the OpenShift
# all-in-one binary.
#
# While this image can be used for a simple node it does not support OVS based
# SDN or storage plugins required for EBS, GCE, Gluster, Ceph, or iSCSI volume
# management. For those features please use 'openshift/node' 
#
# The standard name for this image is openshift/origin
#
FROM openshift/origin-base

RUN INSTALL_PKGS="origin" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

RUN mkdir -p /var/lib/origin

RUN setcap 'cap_net_bind_service=ep' /usr/bin/openshift

ENV HOME /root
ENV OPENSHIFT_CONTAINERIZED true
ENV KUBECONFIG /var/lib/origin/openshift.local.config/master/admin.kubeconfig
WORKDIR /var/lib/origin
EXPOSE 8443 53
ENTRYPOINT ["/usr/bin/openshift"]
