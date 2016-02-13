#
# This is an example HTTP server for demonstrating deployments
#
# The standard name for this image is openshift/deployment-example:v2
#
FROM scratch

MAINTAINER Clayton Coleman <ccoleman@redhat.com>
COPY bin/deployment /deployment

EXPOSE 8080
ENV COLOR="#b5d4a8"
ENTRYPOINT ["/deployment", "v2"]
