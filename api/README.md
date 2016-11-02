OpenShift API Documentation
---------------------------

This directory contains a Swagger 1.0 and OpenAPI (Swagger 2.0) API definition for the OpenShift and Kubernetes APIs.
The `swagger-spec` and `protobuf-spec` directories are generated automatically.

* [Swagger 1.0 - OpenShift](./swagger-spec/oapi-v1.json)
* [Swagger 1.0 - Kubernetes](./swagger-spec/api-v1.json)
* [OpenAPI / Swagger 2.0 - OpenShift and Kubernetes](./swagger-spec/openshift-openapi-spec.json)
* [Protobuf .proto files for all APIs](./protobuf-spec/)

When you add a new object or field to the REST API, you should do the following:

* Ensure all of your fields and objects have the description tag
* Run `hack/update-generated-swagger-spec.sh`

To generate the docs, you need gradle 2.2+ installed, then run

    $ hack/update-generated-swagger-docs.sh

That will create docs into _output/local/docs/swagger/api/v1 and oapi/v1 for the Kube and OpenShift docs.

From the openshift-docs source repo you can generate these directly in one step after making the changes to the OpenShift origin repo (like adding descriptions or generating new swagger doc). The following assumes both origin and openshift-docs repos are at the same level in the directory structure. If not, use the environment variable *`ORIGIN_REPO`* to define the path to the origin source repo.

    $ cd ../openshift-docs
    $ rake import_api

This will invoke update-generated-swagger-docs.sh and import the API into rest_api/.  After importing you'll need to add the correct adoc metadata to the top of kubernetes_v1.adoc and openshift_v1.adoc (pulls to automate that welcome).
