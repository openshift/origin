FROM registry.svc.ci.openshift.org/openshift/builder:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/oc
COPY .. .
RUN make build --warn-undefined-variables

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/oc/oc /usr/bin/
RUN for i in kubectl openshift-deploy openshift-docker-build openshift-sti-build openshift-git-clone openshift-manage-dockerfile openshift-extract-image-content openshift-recycle; do ln -s /usr/bin/oc /usr/bin/$i; done
LABEL io.k8s.display-name="OpenShift Client" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,cli"
