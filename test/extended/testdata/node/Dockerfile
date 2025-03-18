FROM quay.io/crio/nested-container:v5.4.0

WORKDIR /home/podman/go/src/github.com/containers/podman
COPY --chown=podman:podman --chmod=755 skip_tests.sh skip_tests.sh
RUN ./skip_tests.sh
