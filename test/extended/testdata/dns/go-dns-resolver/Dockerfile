ARG GO_VERSION
ARG GO_TOOLSET_IMAGE
FROM ${GO_TOOLSET_IMAGE}:${GO_VERSION}

ENV GOCACHE=/tmp/

COPY dns_libraries_go /go/dns_libraries.go

CMD ["/bin/sh", "-c", "sleep infinity"]
