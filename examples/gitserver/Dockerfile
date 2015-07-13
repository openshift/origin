#
# This is an example Git server for OpenShift Origin.
#
# The standard name for this image is openshift/origin-gitserver
#
FROM openshift/origin

ADD bin/gitserver /usr/bin/gitserver
ADD hooks/ /var/lib/git-hooks/
RUN mkdir -p /var/lib/git
VOLUME /var/lib/git

ENTRYPOINT ["/usr/bin/gitserver"]
