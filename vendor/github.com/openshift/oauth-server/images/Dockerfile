FROM registry.svc.ci.openshift.org/openshift/release:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/oauth-server
COPY . .
RUN make build --warn-undefined-variables

FROM centos:7
COPY --from=builder /go/src/github.com/openshift/oauth-server/oauth-server /usr/bin/
ENTRYPOINT ["/usr/bin/oauth-server"]
LABEL io.k8s.display-name="OpenShift OAuth Server" \
      io.k8s.description="This is a component of OpenShift and enables cluster authentication" \
      com.redhat.component="oauth-server" \
      name="openshift/ose-oauth-server" \
      io.openshift.tags="openshift"
