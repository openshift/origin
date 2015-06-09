Hello, OpenShift!
-----------------

This example will serve an http response of "Hello OpenShift!" to [http://localhost:6061](http://localhost:6061).  To create the pod run:

        $ oc create -f examples/hello-openshift/hello-pod.json

If you need to rebuild the image:
$ go build -tags netgo   # ensures static binary
$ mv hello-openshift bin
$ docker build -t docker.io/openshift/hello-openshift .
