# This base image should be comparable to the base image used by the default
# Dockerfile in ../custom-build, but can be pulled without requiring any
# credentials.  Otherwise this Dockerfile should look just like that one.
FROM quay.io/buildah/stable:latest
# For simplicity, /tmp/build contains the inputs we’ll be building when we
# run this custom builder image. Normally the custom builder image would
# fetch this content from some location at build time. (e.g. via git clone).
ADD Dockerfile.sample /tmp/input/Dockerfile
ADD build.sh /usr/bin
RUN chmod a+x /usr/bin/build.sh
# /tmp/build/build.sh contains the actual custom build logic that will be executed when
# this custom builder image is executed.
ENTRYPOINT ["/usr/bin/build.sh"]
