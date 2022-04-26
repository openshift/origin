FROM registry.access.redhat.com/ubi8/go-toolset

ARG DNS_CLUSTER_IP
ENV DNS_CLUSTER_IP=$DNS_CLUSTER_IP

COPY dns_libraries.go /go/dns
RUN chown 1001 /go/dns
USER 1001
