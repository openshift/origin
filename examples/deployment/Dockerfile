#
# This is an example HTTP server for demonstrating deployments
#
# The standard name for this image is openshift/deployment-example
#
FROM scratch

MAINTAINER Clayton Coleman <ccoleman@redhat.com>
ADD bin/deployment /deployment

EXPOSE 8080
ENV COLOR="#006e9c"
ENTRYPOINT ["/deployment", "v1"]
