FROM registry.redhat.io/rhel8/buildah:latest
# For simplicity, /tmp/build contains the inputs we’ll be building when we
# run this custom builder image. Normally the custom builder image would
# fetch this content from some location at build time. (e.g. via git clone).
ADD Dockerfile.sample /tmp/input/Dockerfile
ADD build.sh /usr/bin
RUN chmod a+x /usr/bin/build.sh
# /usr/build/build.sh contains the actual custom build logic that will be executed when
# this custom builder image is executed.
ENTRYPOINT ["/usr/bin/build.sh"]
