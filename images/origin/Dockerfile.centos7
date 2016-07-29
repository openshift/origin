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
    yum clean all && \
    setcap 'cap_net_bind_service=ep' /usr/bin/openshift

LABEL io.k8s.display-name="OpenShift Origin Application Platform" \
      io.k8s.description="OpenShift Origin is a platform for developing, building, and deploying containerized applications. See https://docs.openshift.org/latest for more on running OpenShift Origin."
ENV HOME=/root \
    OPENSHIFT_CONTAINERIZED=true \
    KUBECONFIG=/var/lib/origin/openshift.local.config/master/admin.kubeconfig
VOLUME /var/lib/origin
WORKDIR /var/lib/origin
EXPOSE 8443 53
ENTRYPOINT ["/usr/bin/openshift"]
