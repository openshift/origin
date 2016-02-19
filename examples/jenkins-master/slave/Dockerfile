# The FROM will be replaced when building in OpenShift
FROM openshift/base-centos7

# Install headless Java
USER root
RUN yum install -y --setopt=tsflags=nodocs --enablerepo=centosplus epel-release && \
    rpmkeys --import file:///etc/pki/rpm-gpg/RPM-GPG-KEY-EPEL-7 && \
    export INSTALL_PKGS="java-1.8.0-openjdk-headless nss_wrapper" && \
    yum install -y --setopt=tsflags=nodocs install $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    mkdir -p /opt/app-root/jenkins && \
    chown -R 1001:0 /opt/app-root/jenkins && \
    chmod -R g+w /opt/app-root/jenkins

# Copy the entrypoint
COPY contrib/openshift/* /opt/app-root/jenkins/
USER 1001

# Run the JNLP client by default
# To use swarm client, specify "/opt/app-root/jenkins/run-swarm-client" as Command
ENTRYPOINT ["/opt/app-root/jenkins/run-jnlp-client"]
