
### 1. Build the image
In nodes,build origin openshif/nginx-router image:
 - git clone https://github.com/openshift/origin
 - cd origin/images/router/nginx/
 - docker build -t openshift/nginx-router .

### 2. Run the nginx router
In master, deploy and edit nginx-router so that working directory can be fixed
 - oadm policy add-scc-to-user hostnetwork -z router
 - oadm router nginx-router –images=openshift/nginx-router
 - oc edit deploymentconfigs/nginx-router -o json
```
# edit the json and insert the 'command' line next to image key. As below:
..
"image": "openshift/nginx-router",
"command": ["/usr/bin/openshift-router", "--loglevel=4", "--working-dir=/var/lib/nginx/router" ],
"imagePullPolicy": "IfNotPresent",
..
```

### 3. Test
Test no security route in nginx-router
 - curl https://raw.githubusercontent.com/openshift/origin/master/examples/hello-openshift/hello-pod.json | oc create -f -
 - oc expose pod hello-openshift
 - oc expose svc hello-openshift
 - [root@qe-weliang-37master-etcd-nfs-1 ~]# oc get pod -o wide
```
NAME READY STATUS RESTARTS AGE IP NODE
hello-openshift 1/1 Running 0 5m 172.21.0.7 host-8-241-64.host.centralci.eng.rdu2.redhat.com
nginx-router-2-g200w 1/1 Running 0 9m 10.8.241.27 host-8-241-27.host.centralci.eng.rdu2.redhat.com
```
 - oc get route
```
NAME HOST/PORT PATH SERVICES PORT TERMINATION WILDCARD
hello-openshift hello-openshift-p1.apps.1002-3uz.qe.rhcloud.com hello-openshift 8080 None
```

 - curl --resolve hello-openshift-p1.apps.1002-3uz.qe.rhcloud.com:80:10.8.241.27 http://hello-openshift-p1.apps.1002-3uz.qe.rhcloud.com
```
Hello OpenShift!
```
