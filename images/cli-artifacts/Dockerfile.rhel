# This Dockerfile is a used by CI to publish openshift/origin-v4.0:installer-artifacts
# It builds an image containing the Mac version of the installer layered on top of the
# Linux installer image.

FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/origin
COPY . .
RUN OS_RELEASE_WITHOUT_LINKS=y OS_BUILD_RELEASE_ARCHIVES=n OS_ONLY_BUILD_PLATFORMS="^(darwin|windows)/amd64$" bash -x hack/build-cross.sh

FROM registry.svc.ci.openshift.org/ocp/4.2:cli
COPY --from=builder /go/src/github.com/openshift/origin/_output/local/bin/darwin/amd64/oc /usr/share/openshift/mac/oc
COPY --from=builder /go/src/github.com/openshift/origin/_output/local/bin/windows/amd64/oc.exe /usr/share/openshift/windows/oc.exe
