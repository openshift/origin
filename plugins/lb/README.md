Building the lb
---------------

The lb/router is supposed be run as a container. To build the lb image :
$ cd plugins/lb
$ ./build.sh
$ docker build .

Done. Lets tag it, so its easy on the eyes.
docker tag <image-id> openshift-router

Push it to your private/public/shared repo?
docker push openshift-router:latest

Running the lb
--------------

Take the image above and run it anywhere where the networking allows the container to reach other pods. Only notable requirement is that the port 80 needs to be exposed to the node, so that DNS entries can point to the host/node where the router container is running.

$ docker run --rm -it -p 80:80 openshift-router /usr/bin/lb -master <kube-master-url>

example of <kube-master-url> : http://10.0.2.15:8080
