## Description

The `openshift/origin-haproxy-router` is an [HAProxy](http://www.haproxy.org/) router that is used as an external to internal
interface to OpenShift [services](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/services.md).

The router is meant to run as a pod.  When running the router you must ensure that the router can expose port 80 on the host (minion)
in order to forward traffic.  In a deployed environment the router minion should also have external ip addresses
that can be exposed for DNS based routing.  

## Creating Routes

When you create a route you specify the `hostname` and `service` that the route is connecting.  The `hostname` is the
web host that the router will use to direct traffic.  This host name should be a domain name that you
already own, for instance `www.example.com`.   Alternatively, you may leave the host name
blank and a system generated host name will be created.  It is important to note that at this point
DNS resolution of host names is external to the OpenShift system.


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

    If running in https mode, ensure oc can authenticate to the master
    [vagrant@openshiftdev origin]$ export KUBECONFIG=/data/src/github.com/openshift/origin/openshift.local.config/master/admin.kubeconfig
    [vagrant@openshiftdev origin]$ sudo chmod a+r "$KUBECONFIG"
    [vagrant@openshiftdev origin]$ sudo chmod a+r openshift.local.config/master/openshift-router.kubeconfig
    [vagrant@openshiftdev origin]$ oadm router --create --credentials="openshift.local.config/master/openshift-router.kubeconfig"
    [vagrant@openshiftdev origin]$ oc get pods

#### Clustered vagrant environment


    $ export OPENSHIFT_DEV_CLUSTER=true
    $ vagrant up
    $ vagrant ssh master
    [vagrant@openshift-master ~]$ oadm router --create --credentials="${KUBECONFIG}"



### In a deployed environment

In order to run the router in a deployed environment the following conditions must be met:

* The machine the router will run on must be provisioned as a minion in the cluster (for networking configuration)
* The machine may or may not be registered with the master.  Optimally it will not serve pods while also serving as the router
* The machine must not have services running on it that bind to host port 80 since this is what the router uses for traffic

To install the router pod you use the `oadm router` command line, passing the flag `--credentials=<kubeconfig_file>`.
The credentials flag controls the identity that the router will use to talk to the master (and the address of the master) so in most
environments you can use the `${CONFIG_DIR}/master/openshift-router.kubeconfig` file. Once you run this command you can check the configuration
of the router by running `oc get dc router` to check the deployment status.

`oadm router` offers other options for deploying routers - run `oadm router --help` for more details.

### Manually

To run the router manually (outside of a pod) you should first build the images with instructions found below.  Then you
can run the router anywhere that it can access both the pods and the master.  The router exposes port 80 so the host 
that the router is run on must not have any other services that are bound to that port.  This allows the router to be 
used by a DNS server for incoming traffic.


	$ docker run --rm -it -p 80:80 openshift/origin-haproxy-router --master $kube-master-url

example of kube-master-url : https://10.0.2.15:8443

## Monitoring the router

Since the router runs as a docker container you use the `docker logs <id>` command to monitor the router.

## Testing your route

To test your route independent of DNS you can send a host header to the router.  The following is an example.

    $ ..... vagrant up with single machine instructions .......
    $ ..... create config files listed below in ~ ........
    [vagrant@openshiftdev origin]$ oc create -f ~/pod.json
    [vagrant@openshiftdev origin]$ oc create -f ~/service.json
    [vagrant@openshiftdev origin]$ oc create -f ~/route.json
    [vagrant@openshiftdev origin]$ curl -H "Host:hello-openshift.v3.rhcloud.com" <vm ip>
    Hello OpenShift!

    $ ..... vagrant up with cluster instructions .....
    $ ..... create config files listed below in ~ ........
    [vagrant@openshift-master ~]$ oc create -f ~/pod.json
    [vagrant@openshift-master ~]$ oc create -f ~/service.json
    [vagrant@openshift-master ~]$ oc create -f ~/route.json
    # take note of what minion number the router is deployed on
    [vagrant@openshift-master ~]$ oc get pods
    [vagrant@openshift-master ~]$ curl -H "Host:hello-openshift.v3.rhcloud.com" openshift-minion-<1,2>
    Hello OpenShift!




Configuration files (to be created in the vagrant home directory)

pod.json

    {
      "kind": "Pod",
      "apiVersion": "v1beta3",
      "metadata": {
        "name": "hello-pod",
        "labels": {
          "name": "hello-openshift"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "hello-openshift",
            "image": "openshift/hello-openshift",
            "ports": [
              {
                "containerPort": 8080,
                "protocol": "TCP"
              }
            ],
            "resources": {},
            "terminationMessagePath": "/dev/termination-log",
            "imagePullPolicy": "IfNotPresent",
            "capabilities": {},
            "securityContext": {
              "capabilities": {},
              "privileged": false
            }
          }
        ],
        "restartPolicy": "Always",
        "dnsPolicy": "ClusterFirst"
      }
    }

service.json

    {
      "kind": "Service",
      "apiVersion": "v1beta3",
      "metadata": {
        "name": "hello-openshift"
      },
      "spec": {
        "ports": [
          {
            "protocol": "TCP",
            "port": 27017,
            "targetPort": 0,
            "nodePort": 0
          }
        ],
        "selector": {
          "name": "hello-openshift"
        },
        "portalIP": "",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      }
    }
    
route.json

    {
      "kind": "Route",
      "apiVersion": "v1beta3",
      "metadata": {
        "name": "hello-route"
      },
      "spec": {
        "host": "hello-openshift.v3.rhcloud.com",
        "to": {
          "kind": "Service",
          "name": "hello-openshift"
        }
      }
    }

## Securing Your Routes

Creating a secure route to your pods can be accomplished by specifying the TLS Termination of the route and, optionally,
providing certificates to use.  As of writing, OpenShift beta1 TLS termination relies on SNI for serving custom certificates.
In the future, the ability to create custom frontends within the router will allow all traffic to serve custom certificates.

TLS Termination falls in the following configuration buckets:

#### Edge Termination
Edge termination means that TLS termination occurs prior to traffic reaching the destination.  TLS certificates are served
by the frontend of the router.

Edge termination is configured by setting `TLS.Termination` to `edge` on your `route` and by specifying the `CertificateFile`
and `KeyFile` (at a minimum).  You may also specify your `CACertificateFile` to complete the entire certificate chain.

#### Passthrough Termination
Passthrough termination is a mechanism to send encrypted traffic straight to the destination without the router providing
TLS termination.    

Passthrough termination is configured by setting `TLS.Termination` to `passthrough` on your `route`.  No other information is required.
The destination (such as an Nginx, Apache, or another HAProxy instance) will be responsible for serving certificates for 
the traffic.

#### Re-encryption Termination
Re-encryption is a special case of edge termination where the traffic is first decrypted with certificate A and then 
re-encrypted with certificate B when sending the traffic to the destination.
 
Re-encryption termination is configured by setting `TLS.Termination` to `reencrypt` and providing the `CertificateFile`,
`KeyFile`, the `CACertificateFile`, and a `DestinationCACertificateFile`.  The edge certificates remain the same as in the edge
termination use case.  The `DestinationCACertificateFile` is used in order to validate the secure connection from the 
router to the destination.

### Special Notes About Secure Routes
At this point, password protected key files are not supported.  HAProxy prompts you for a password when starting up and 
does not have a way to automate this process.  We will need a follow up for `KeyPassPhrase`.  To remove a passphrase from 
a keyfile you may run `openssl rsa -in passwordProtectedKey.key -out new.key`

## Running HA Routers

Highly available router setups can be accomplished by running multiple instances of the router pod and fronting them with
a balancing tier.  This could be something as simple as DNS round robin or as complex as multiple load balancing layers.

### DNS Round Robin

As a simple example, you may create a zone file for a DNS server like [BIND](http://www.isc.org/downloads/bind/) that maps
multiple A records for a single domain name.  When clients do a lookup they will be given one of the many records, in order
as a round robin scheme.  The files below illustrate an example of using wild card DNS with multiple A records to achieve
the desired round robin.  The wild card could be further distributed into shards with `*.<shard>`.  Finally, a test using
`dig` (available in the `bind-utils` package) is shown from the vagrant environment that shows multiple answers for the 
same lookup.  Doing multiple pings show the resolution swapping between IP addresses.

#### named.conf - add a new zone that points to your file
    zone "v3.rhcloud.com" IN {
            type master;
            file "v3.rhcloud.com.zone";
    };


#### v3.rhcloud.com.zone - contains the round robin mappings for the DNS lookup
    $ORIGIN v3.rhcloud.com.

    @       IN      SOA     . v3.rhcloud.com. (
                         2009092001         ; Serial
                             604800         ; Refresh
                              86400         ; Retry
                            1206900         ; Expire
                                300 )       ; Negative Cache TTL
            IN      NS      ns1.v3.rhcloud.com.
    ns1     IN      A       127.0.0.1
    *       IN      A       10.245.2.2
            IN      A       10.245.2.3


#### Testing the entry


    [vagrant@openshift-master ~]$ dig hello-openshift.shard1.v3.rhcloud.com

    ; <<>> DiG 9.9.4-P2-RedHat-9.9.4-16.P2.fc20 <<>> hello-openshift.shard1.v3.rhcloud.com
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 36389
    ;; flags: qr aa rd; QUERY: 1, ANSWER: 2, AUTHORITY: 1, ADDITIONAL: 2
    ;; WARNING: recursion requested but not available

    ;; OPT PSEUDOSECTION:
    ; EDNS: version: 0, flags:; udp: 4096
    ;; QUESTION SECTION:
    ;hello-openshift.shard1.v3.rhcloud.com. IN A

    ;; ANSWER SECTION:
    hello-openshift.shard1.v3.rhcloud.com. 300 IN A	10.245.2.2
    hello-openshift.shard1.v3.rhcloud.com. 300 IN A	10.245.2.3

    ;; AUTHORITY SECTION:
    v3.rhcloud.com.		300	IN	NS	ns1.v3.rhcloud.com.

    ;; ADDITIONAL SECTION:
    ns1.v3.rhcloud.com.	300	IN	A	127.0.0.1

    ;; Query time: 5 msec
    ;; SERVER: 10.245.2.3#53(10.245.2.3)
    ;; WHEN: Wed Nov 19 19:01:32 UTC 2014
    ;; MSG SIZE  rcvd: 132

    [vagrant@openshift-master ~]$ ping hello-openshift.shard1.v3.rhcloud.com
    PING hello-openshift.shard1.v3.rhcloud.com (10.245.2.3) 56(84) bytes of data.
    ...
    ^C
    --- hello-openshift.shard1.v3.rhcloud.com ping statistics ---
    2 packets transmitted, 2 received, 0% packet loss, time 1000ms
    rtt min/avg/max/mdev = 0.272/0.573/0.874/0.301 ms
    [vagrant@openshift-master ~]$ ping hello-openshift.shard1.v3.rhcloud.com
    ...



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
