Building Entire Applications with Templates
===========================================

This example demonstrates how to use a template to generate the shared passwords for
an application. Templates let you parameterize a set of Kubernetes and OpenShift
objects all at once, and then **apply** them using the command line client.

The example is based on the [Kubernetes Guestbook](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/examples/guestbook/README.md) - see the Guestbook for more descriptions of how the individual objects work together.

Deploy
------

1. Start an OpenShift all-in-one server

        openshift start

2. Use the command line to transform the template, and then send each object to the server:

        openshift kube process -c template.json | openshift kube apply -c -

   Note: `-c -` tells the CLI to read a file from STDIN - you can use this in other places as well.

3. It's ready! Access the server with:

        $ curl http://localhost:8080/
