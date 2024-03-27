ARG GO_VERSION
FROM registry.redhat.io/ubi8/go-toolset:${GO_VERSION}

ENV GOCACHE=/tmp/

COPY dns_libraries_go /go/dns_libraries.go

CMD ["/bin/sh", "-c", "sleep infinity"]
