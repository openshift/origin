#
# This is the official OpenShift Origin image. It has as its entrypoint the OpenShift
# all-in-one binary.
#
# The standard name for this image is openshift3/ose
#
FROM openshift3/ose-base

RUN yum install -y openshift && \
    yum clean all

RUN ln -s /usr/bin/openshift /usr/bin/osc && \
    ln -s /usr/bin/openshift /usr/bin/osadm && \
    ln -s /usr/bin/openshift /usr/bin/kubectl && \
    ln -s /usr/bin/openshift /usr/bin/openshift-deploy && \
    ln -s /usr/bin/openshift /usr/bin/openshift-docker-build && \
    ln -s /usr/bin/openshift /usr/bin/openshift-sti-build && \
    ln -s /usr/bin/openshift /usr/bin/openshift-router && \
    setcap 'cap_net_bind_service=ep' /usr/bin/openshift

ADD scripts/recycler.sh /usr/share/openshift/scripts/volumes/recycler.sh

ENV HOME /root
ENV OPENSHIFT_CONTAINERIZED true
ENV KUBECONFIG /var/lib/openshift/openshift.local.config/master/admin.kubeconfig
WORKDIR /var/lib/openshift
EXPOSE 8443 53
ENTRYPOINT ["/usr/bin/openshift"]
