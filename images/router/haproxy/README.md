## Description

The `openshift/origin-haproxy-router` is an [HAProxy](http://www.haproxy.org/) router that is used as an external to internal
interface to OpenShift [services](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/DESIGN.md#the-kubernetes-node).

The router is meant to run as a pod.  When running the router you must ensure that the router can expose port 80 on the host (minion)
in order to forward traffic.  In a deployed environment the router minion should also have external ip addressess
that can be exposed for DNS based routing.  

## Creating Routes

When you create a route you specify the `hostname` and `service` that the route is connecting.  The `hostname` is the 
prefix that will be used to create a `hostname-namespace.v3.rhcloud.com` route.  See below for an example route configuration file.


## Running the router


### In the Vagrant environment

Please note, that starting the router in the vagrant environment requires it to be pulled into docker.  This may take some time.
Once it is pulled it will start and be visible in the `docker ps` list of containers and your pod will be marked as running.


#### Single machine vagrant environment
    
    $ vagrant up
    $ vagrant ssh
    [vagrant@openshiftdev origin]$ cd /data/src/github.com/openshift/origin/
    [vagrant@openshiftdev origin]$ make clean && make
    [vagrant@openshiftdev origin]$ export PATH=/data/src/github.com/openshift/origin/_output/local/bin/linux/amd64:$PATH
    [vagrant@openshiftdev origin]$ sudo /data/src/github.com/openshift/origin/_output/local/bin/linux/amd64/openshift start &
    [vagrant@openshiftdev origin]$ hack/install-router.sh {master ip}
    [vagrant@openshiftdev origin]$ openshift kube list pods
    
#### Clustered vagrant environment    


    $ export OPENSHIFT_DEV_CLUSTER=true
    $ vagrant up
    $ vagrant ssh master
    [vagrant@openshift-master ~]$ hack/install-router.sh {master ip}
  


### In a deployed environment

In order to run the router in a deployed environment the following conditions must be met:

* The machine the router will run on must be provisioned as a minion in the cluster (for networking configuration)
* The machine may or may not be registered with the master.  Optimally it will not serve pods while also serving as the router
* The machine must not have services running on it that bind to host port 80 since this is what the router uses for traffic

To install the router pod you use the `images/router/haproxy/pod.json` template and update the `MASTER_IP`.  You may then
use the `openshift kube -c <your file> create pods` command.

### Manually   

To run the router manually (outside of a pod) you should first build the images with instructions found below.  Then you
can run the router anywhere that it can access both the pods and the master.  The router exposes port 80 so the host 
that the router is run on must not have any other services that are bound to that port.  This allows the router to be 
used by a DNS server for incoming traffic.


	$ docker run --rm -it -p 80:80 openshift/origin-haproxy-router -master $kube-master-url

example of kube-master-url : http://10.0.2.15:8080

## Monitoring the router

Since the router runs as a docker container you use the `docker logs <id>` command to monitor the router.

## Testing your route

To test your route independent of DNS you can send a host header to the router.  The following is an example.

    $ ..... vagrant up with single machine instructions .......
    $ ..... create config files listed below in ~ ........
    [vagrant@openshiftdev origin]$ openshift kube -c ~/pod.json create pods
    [vagrant@openshiftdev origin]$ openshift kube -c ~/service.json create services
    [vagrant@openshiftdev origin]$ openshift kube -c ~/route.json create routes
    [vagrant@openshiftdev origin]$ curl -H "Host:hello-openshift.v3.rhcloud.com" <vm ip>
    Hello OpenShift!
    
    $ ..... vagrant up with cluster instructions .....
    $ ..... create config files listed below in ~ ........
    [vagrant@openshift-master ~]$ openshift kube -c ~/pod.json create pods
    [vagrant@openshift-master ~]$ openshift kube -c ~/service.json create services
    [vagrant@openshift-master ~]$ openshift kube -c ~/route.json create routes
    # take note of what minion number the router is deployed on
    [vagrant@openshift-master ~]$ openshift kube list pods
    [vagrant@openshift-master ~]$ curl -H "Host:hello-openshift.v3.rhcloud.com" openshift-minion-<1,2>
    Hello OpenShift!
    

    

Configuration files (to be created in the vagrant home directory)

pod.json

    {
          "id": "hello-pod",
          "kind": "Pod",
          "apiVersion": "v1beta1",
          "desiredState": {
            "manifest": {
              "version": "v1beta1",
              "id": "hello-openshift",
              "containers": [{
                "name": "hello-openshift",
                "image": "openshift/hello-openshift",
                "ports": [{
                  "containerPort": 8080
                }]
              }]
            }
          },
          "labels": {
            "name": "hello-openshift"
          }
        }

service.json

    {
      "kind": "Service",
      "apiversion": "v1beta1",
      "id": "hello-openshift",
      "port": 27017,
      "selector": {
        "name": "hello-openshift"
      },
    }   
     
route.json

    {
      "id": "hello-route",
      "apiVersion": "v1beta1",
      "kind": "Route",
      "host": "hello-openshift.v3.rhcloud.com",
      "serviceName": "hello-openshift"
    }

## Dev - Building the haproxy router image

When building the routes you use the scripts in the `${OPENSHIFT ORIGIN PROJECT}/hack` directory.  This will build both
base images and the router image.  When complete you should have a `openshift/origin-haproxy-router` container that shows
in `docker images` that is ready to use.

	$ hack/build-base-images.sh
    $ hack/build-images.sh
    
## Dev - router internals

The router is an [HAProxy](http://www.haproxy.org/) container that is run via a go wrapper (`openshift-router.go`) that 
provides a watch on `routes` and `endpoints`.  The watch funnels down to the configuration files for the [HAProxy](http://www.haproxy.org/) 
plugin which can be found in `plugins/router/haproxy/haproxy.go`.  The router is then issued a reload command.

When debugging the router it is sometimes useful to inspect these files.  To do this you must enter the namespace of the 
running container by getting the pid via `docker inspect <container id> | grep Pid` and then `nsenter -m -u -n -i -p -t <pid>`.
Listed below are the files used for configuration.

    ConfigTemplate   = "/var/lib/haproxy/conf/haproxy_template.conf"
    ConfigFile       = "/var/lib/haproxy/conf/haproxy.config"
    HostMapFile      = "/var/lib/haproxy/conf/host_be.map"
    HostMapSniFile   = "/var/lib/haproxy/conf/host_be_sni.map"
    HostMapResslFile = "/var/lib/haproxy/conf/host_be_ressl.map"
    HostMapWsFile    = "/var/lib/haproxy/conf/host_be_ws.map"
