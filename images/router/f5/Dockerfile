#
# This is the F5 router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-f5-router
#
FROM openshift/origin-control-plane

RUN INSTALL_PKGS="openssh-clients" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Origin F5 Router" \
      io.k8s.description="This is a component of OpenShift Origin and programs a BigIP F5 router to expose services within the cluster." \
      io.openshift.tags="openshift,router,f5"
ENTRYPOINT ["/usr/bin/openshift-f5-router"]
