#
# This is the integrated OpenShift Service Serving Cert Signer.  It signs serving certificates for use inside the platform.
#
# The standard name for this image is openshift/origin-service-serving-cert-signer
#
FROM openshift/origin-release:golang-1.10
COPY . /go/src/github.com/openshift/service-serving-cert-signer
RUN cd /go/src/github.com/openshift/service-serving-cert-signer && go build ./cmd/service-serving-cert-signer

FROM centos:7
COPY --from=0 /go/src/github.com/openshift/service-serving-cert-signer/service-serving-cert-signer /usr/bin/service-serving-cert-signer

COPY manifests /manifests
LABEL io.openshift.release.operator=true
