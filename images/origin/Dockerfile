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

COPY bin/openshift /usr/bin/openshift
RUN ln -s /usr/bin/openshift /usr/bin/oc && \
    ln -s /usr/bin/openshift /usr/bin/oadm && \
    ln -s /usr/bin/openshift /usr/bin/origin && \
    ln -s /usr/bin/openshift /usr/bin/kubectl && \
    ln -s /usr/bin/openshift /usr/bin/openshift-deploy && \
    ln -s /usr/bin/openshift /usr/bin/openshift-recycle && \
    ln -s /usr/bin/openshift /usr/bin/openshift-router && \
    ln -s /usr/bin/openshift /usr/bin/openshift-docker-build && \
    ln -s /usr/bin/openshift /usr/bin/openshift-sti-build && \
    ln -s /usr/bin/openshift /usr/bin/openshift-f5-router && \
    setcap 'cap_net_bind_service=ep' /usr/bin/openshift

LABEL io.k8s.display-name="OpenShift Origin Application Platform" \
      io.k8s.description="OpenShift Origin is a platform for developing, building, and deploying containerized applications. See https://docs.openshift.org/latest for more on running OpenShift Origin."
ENV HOME=/root \
    OPENSHIFT_CONTAINERIZED=true \
    KUBECONFIG=/var/lib/origin/openshift.local.config/master/admin.kubeconfig
WORKDIR /var/lib/origin
EXPOSE 8443 53
ENTRYPOINT ["/usr/bin/openshift"]
