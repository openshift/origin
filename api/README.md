OpenShift API Documentation
---------------------------

This directory contains a Swagger 1.0 and OpenAPI (Swagger 2.0) API definition for the OpenShift and Kubernetes APIs.
The `swagger-spec`, `protobuf-spec` and `docs` directories are generated automatically.

* [Swagger 1.0 - OpenShift](./swagger-spec/oapi-v1.json)
* [Swagger 1.0 - Kubernetes](./swagger-spec/api-v1.json)
* [OpenAPI / Swagger 2.0 - OpenShift and Kubernetes](./swagger-spec/openshift-openapi-spec.json)
* [Protobuf .proto files for all APIs](./protobuf-spec/)
* [API documentation for openshift-docs](./docs/)

When you add a new object or field to the REST API, you should do the following:

* Ensure all of your fields and objects have the description tag
* Run `hack/update-generated-swagger-spec.sh`

To update the REST API documentation in the openshift-docs repo, first ensure it
is up-to-date in origin.  Then in the openshift-docs repo, run:

    $ ORIGIN_REPO=path/to/origin make
