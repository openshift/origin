FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/sdn
COPY . .
RUN make build --warn-undefined-variables

FROM registry.svc.ci.openshift.org/ocp/4.2:base
COPY --from=builder /go/src/github.com/openshift/sdn/network-controller /usr/bin/
ENTRYPOINT ["/usr/bin/network-controller"]
LABEL io.k8s.display-name="OpenShift SDN Controller" \
      io.k8s.description="This is a component of OpenShift, it controls the network" \
      com.redhat.component="sdn-controller" \
      io.openshift.tags="openshift,sdn-controller,sdn"
