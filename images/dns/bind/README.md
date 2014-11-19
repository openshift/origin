OpenShift BIND DNS
-------------------
The OpenShift BIND DNS plugin is meant to be run in a pod and work with routers and routes.  The plugin is an implementation
of BIND DNS that uses wild cards based on shards.  Currently, sharding is completely on the DNS side.  As of writing,
all OpenShift routers know about all OpenShift routes but only service traffic based on their shard.


Shards take the form of a route coupled with a DNS name.  For instance, in a 2 shard environment you will have 2 routers
running and a single DNS pod running.  Right now, the route must be created with a host name that includes the shard.
In the future this should be automatic as a route is allocated to a router.  Sharded route host names take the form of
`<name>.<shard>.<domain>`.  For instance `hello-openshift.shard1.v3.rhcloud.com`.


Running/Testing OpenShift BIND DNS
-------------------
Running the OpenShift BIND DNS pod currently requires you to have a container that is premade with the routers you would
like to use.  Your container uses a json configuration file to create the BIND zone configuration and start the server.
The packaged configuration file is suitable for use with the clustered vagrant environment.

TODO: make this dynamic and add it to hack/build-images.sh

### Create your container (optional for vagrant)
Example configuration file

    {
        "shards": [
    
            {
                "name": "shard-1",
                "pattern": "*.shard1",
                "routerlist": [
                    {
                        "name": "router-1",
                        "ip": "10.245.2.2"
                    },
                    {
                        "name": "router-2",
                        "ip": "10.245.2.3"
                    }
                ]
            },
            {
                "name": "shard-2",
                "pattern": "*.shard2",
                "routerlist": [
                    {
                        "name": "router-2",
                        "ip": "10.245.2.3"
                    },
                    {
                        "name": "router-1",
                        "ip": "10.245.2.2"
                    }
                ]
            }
    
        ]
    }
    
        
    

### Start your environment
Please note, this example assumes that you are using the same configuration files found in the router examples.  For this 
example we will be using two routers so you will need to modify the `pod.json` used in `hack/install-router.sh` and change
the name to make `router2.json`.  You will also need to modify the `route.json` file to include a shard by changing the host 
to `hello-openshift.shard1.v3.rhcloud.com`
    
    router2.json:
    {
        "kind": "Pod",
        "apiVersion": "v1beta1",
        "id": "openshift-router2",
        "desiredState": {
            "manifest": {
                "version": "v1beta2",
                "containers": [
                    {
                        "name": "origin-haproxy-router2",
                        "image": "openshift/origin-haproxy-router",
                        "ports": [{
                            "containerPort": 80,
                            "hostPort": 80
                        }],
                        "command": ["--master=10.245.1.2:8080"],
                        "imagePullPolicy": "PullIfNotPresent"
                    }
                ],
                "restartPolicy": {
                    "always": {}
                }
            }
        }
    }


    $ export OPENSHIFT_DEV_CLUSTER=true
    $ vagrant up
    $ vagrant ssh master
    [vagrant@openshift-master ~]$ openshift kube create -c ~/pod.json pods
    [vagrant@openshift-master ~]$ openshift kube create -c ~/service.json services
    [vagrant@openshift-master ~]$ openshift kube create -c ~/route.json routes
    [vagrant@openshift-master ~]$ openshift kube create -c ~/router2.json pods  
    [vagrant@openshift-master ~]$ /vagrant/hack/install-router.sh 10.245.1.2
    [vagrant@openshift-master ~]$ openshift kube create -c /vagrant/images/dns/bind/pod.json
    [vagrant@openshift-master ~]$ openshift kube list pods

### Test the name resolution
    sudo yum -y install bind-utils
    
    
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

    
### Test the DNS Round Robin    
Ensure that you are seeing your two minion IPs resolve, in sequence to the domain name specified in `route.json`

    [vagrant@openshift-master ~]$ ping hello-openshift.shard1.v3.rhcloud.com
    PING hello-openshift.shard1.v3.rhcloud.com (10.245.2.3) 56(84) bytes of data.
    64 bytes from openshift-minion-2 (10.245.2.3): icmp_seq=1 ttl=63 time=0.874 ms
    64 bytes from openshift-minion-2 (10.245.2.3): icmp_seq=2 ttl=63 time=0.272 ms
    ^C
    --- hello-openshift.shard1.v3.rhcloud.com ping statistics ---
    2 packets transmitted, 2 received, 0% packet loss, time 1000ms
    rtt min/avg/max/mdev = 0.272/0.573/0.874/0.301 ms
    [vagrant@openshift-master ~]$ ping hello-openshift.shard1.v3.rhcloud.com
    PING hello-openshift.shard1.v3.rhcloud.com (10.245.2.2) 56(84) bytes of data.
    64 bytes from openshift-minion-1 (10.245.2.2): icmp_seq=1 ttl=63 time=0.372 ms
    64 bytes from openshift-minion-1 (10.245.2.2): icmp_seq=2 ttl=63 time=0.409 ms
    64 bytes from openshift-minion-1 (10.245.2.2): icmp_seq=3 ttl=63 time=0.423 ms


### Test the Route
    [vagrant@openshift-master ~]$ curl hello-openshift.shard1.v3.rhcloud.com
    Hello OpenShift!
