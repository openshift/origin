#
# This is the official OpenShift test image. It can be used to verify
# an installation of OpenShift completed successfully.
#
# The standard name for this image is openshift/origin-tests
#
FROM openshift/origin-cli

RUN INSTALL_PKGS=" \
      origin-tests \
      git \
      " && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all && \
    git config --system user.name test && \
    git config --system user.email test@test.com

LABEL io.k8s.display-name="OpenShift End-to-End Tests" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,tests,e2e"
