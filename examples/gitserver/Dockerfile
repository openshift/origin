#
# This is an example Git server for OpenShift Origin.
#
# The standard name for this image is openshift/origin-gitserver
#
FROM openshift/origin

ADD bin/gitserver /usr/bin/gitserver
ADD hooks/ /var/lib/git-hooks/
RUN mkdir -p /var/lib/git && \
    mkdir -p /var/lib/gitconfig && \
    chmod 777 /var/lib/gitconfig && \
    ln -s /usr/bin/gitserver /usr/bin/gitrepo-buildconfigs
ADD gitconfig /var/lib/gitconfig/.gitconfig
ENV HOME=/var/lib/gitconfig
VOLUME /var/lib/git

ENTRYPOINT ["/usr/bin/gitserver"]
