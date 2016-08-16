Generating API documents
========================

To generate docs, it currently uses 'k8s.io/kubernetes/pkg/runtime'. Run `hack/update-generated-swagger-descriptions.sh` to create a new copy of
the docs in `_output/local/docs/swagger` for the OpenShift v1 and Kubernetes v1 APIs.