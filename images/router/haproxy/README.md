Building the haproxy router image
---------------------------------

The openshift-router is supposed be run as a container. To build the image,

	$ hack/build-base-images.sh
  $ hack/build-images.sh

Running the router
------------------

Take the image above and run it anywhere where the networking allows the container to reach other pods. Only notable requirement is that the port 80 needs to be exposed to the node, so that DNS entries can point to the host/node where the router container is running.

	$ docker run --rm -it -p 80:80 origin/openshift-haproxy-router -master $kube-master-url

example of kube-master-url : http://10.0.2.15:8080
