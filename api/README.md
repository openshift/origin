OpenShift API Documentation
---------------------------

This directory contains a Swagger API definition for the OpenShift and Kubernetes APIs.  The `swagger-spec` directory is generated automatically, and the other directories contain content that is used to generate the official documentation.

When you add a new object or field to the REST API, you should do the following:

* Ensure all of your fields have the description tag
* Run `hack/update-swagger-spec.sh`
* If you've added a new object, add a simple description to `api/definitions/v1.objectname/description.adoc` (object name is all lower case for your Kind)
  * For an example, see `api/definitions/v1.persistentvolumeclaim/description.adoc`

To generate the docs, you need gradle 2.2+ installed, then run

    $ hack/gen-swagger-docs.sh

That will create docs into _output/local/docs/swagger/api/v1 and oapi/v1 for the Kube and OpenShift docs.

From openshift-docs you can generate these directly in one step - make the changes to the OpenShift origin repo (like adding descriptions or generating new swagger doc), then run

    $ cd ../openshift-docs
    $ rake import_api

This will invoke gen-swagger-docs.sh and import the API into rest_api/.  After importing you'll need to add the correct adoc metadata to the top of kubernetes_v1.adoc and openshift_v1.adoc (pulls to automate that welcome).
