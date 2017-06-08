#
# This is the OpenShift Origin Federation image, used for running the
# federation apiserver and controller manager components.
#
# The standard name for this image is openshift/origin-federation
#
FROM openshift/origin-base

RUN INSTALL_PKGS="origin-federation-services" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all && \
    ln -s /usr/bin/hyperkube /hyperkube

LABEL io.k8s.display-name="OpenShift Origin Federation" \
      io.k8s.description="This is a component of OpenShift Origin and contains the software for running federation servers."
