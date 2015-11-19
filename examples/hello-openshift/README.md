Hello, OpenShift!
-----------------

This example will serve an HTTP response of "Hello OpenShift!".

    $ oc create -f examples/hello-openshift/hello-pod.json

    $ oc get pod hello-openshift -o yaml |grep podIP
     podIP: 10.1.0.2

    $ curl 10.1.0.2:8080
     Hello OpenShift!

To test from external network, you need to create router. Please refer to [Running the router](https://github.com/openshift/origin/blob/master/docs/routing.md)

If you need to rebuild the image:

    $ go build -tags netgo   # avoid dynamic linking (we want a static binary)
    $ mv hello-openshift bin
    $ docker build -t docker.io/openshift/hello-openshift .
