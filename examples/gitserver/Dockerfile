#
# This is an example Git server for OpenShift Origin.
#
# The standard name for this image is openshift/origin-gitserver
#
FROM openshift/origin

ADD hooks/ /var/lib/git-hooks/
RUN ln -s /usr/bin/openshift /usr/bin/openshift-gitserver && \
    mkdir -p /var/lib/git
VOLUME /var/lib/git

ENTRYPOINT ["/usr/bin/openshift-gitserver"]
