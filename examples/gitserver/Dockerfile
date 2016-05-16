#
# This is an example Git server for OpenShift Origin.
#
# The standard name for this image is openshift/origin-gitserver
#
FROM openshift/origin-base

COPY bin/oc /usr/bin/oc
COPY bin/gitserver /usr/bin/gitserver
COPY hooks/ /var/lib/git-hooks/
COPY gitconfig /var/lib/gitconfig/.gitconfig
RUN mkdir -p /var/lib/git && \
    mkdir -p /var/lib/gitconfig && \
    chmod 777 /var/lib/gitconfig && \
    ln -s /usr/bin/gitserver /usr/bin/gitrepo-buildconfigs
VOLUME /var/lib/git
ENV HOME=/var/lib/gitconfig

ENTRYPOINT ["/usr/bin/gitserver"]
