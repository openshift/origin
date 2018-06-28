#
# This is the integrated OpenShift Service Serving Cert Signer.  It signs serving certificates for use inside the platform.
#
# The standard name for this image is openshift/origin-service-serving-cert-signer
#
FROM openshift/origin-base

RUN INSTALL_PKGS="origin-service-serving-cert-signer" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Service Serving Cert Signer" \
      io.k8s.description="This is a component of OpenShift that signs serving certificates for use inside the platform." \
      io.openshift.tags="openshift"

# The process doesn't require a root user.
USER 1001

CMD /usr/bin/service-serving-cert-signer
