FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/origin
COPY . .
RUN make build WHAT=vendor/github.com/openshift/template-service-broker/cmd/template-service-broker; \
    mkdir -p /tmp/build; \
    cp /go/src/github.com/openshift/origin/_output/local/bin/linux/$(go env GOARCH)/template-service-broker /tmp/build/template-service-broker

FROM registry.svc.ci.openshift.org/ocp/4.2:base
COPY --from=builder /tmp/build/template-service-broker /usr/bin/
LABEL io.k8s.display-name="OpenShift Template Service Broker" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift"
CMD [ "/usr/bin/template-service-broker" ]
