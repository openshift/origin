Hello, OpenShift!
-----------------

This example will serve an HTTP response of "Hello OpenShift!".

    $ oc create -f examples/hello-openshift/hello-pod.json

    $ oc get pod hello-openshift -o yaml |grep podIP
     podIP: 10.1.0.2

    $ curl 10.1.0.2:8080
     Hello OpenShift!

The response message can be set by using the RESPONSE environment
variable.  You will need to edit the pod definition and add an
environment variable to the container definition and run the new pod.
To do this, edit hello-pod.json and add the following to the container
section.  Just add the env clause after the image name so you end up with:
```
    "containers": [
      {
        "name": "hello-openshift",
        "image": "openshift/hello-openshift",
        "env": [
          { "name": "RESPONSE",
            "value": "Hello World!"
          }
        ],
        ...
      }
    ],
```

After that, if you are running the pod from above, delete it:

    $ oc delete pod hello-openshift

Then you can re-create the pod as with the first example, get the new IP
address, and then curl will show your new message:

    $ curl 10.1.0.2:8080
     Hello World!

To test from external network, you need to create router. Please refer to [Running the router](https://github.com/openshift/origin/blob/master/docs/routing.md)

If you need to rebuild the image:

    $ go build -tags netgo   # avoid dynamic linking (we want a static binary)
    $ mv hello-openshift bin
    $ docker build -t docker.io/openshift/hello-openshift .
