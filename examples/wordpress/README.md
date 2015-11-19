# How To Use Persistent Volumes

The purpose of this guide is to help you understand storage provisioning by creating a WordPress blog and MySQL database.
In this example, both the blog and database require persistent storage.  

This guide assumes knowledge of OpenShift fundamentals and that you have a cluster up and running.  Please review steps 1 - 10 in the
[sample-app](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md) to run an OpenShift cluster.

## Root access

The Wordpress Dockerhub image binds Apache to port 80 in the container, which requires root access.  We can allow that 
in this example, but those wishing to run a more secure cluster will want to ensure their images don't require root access (e.g, bind to high number ports, don't chown or chmod dirs, etc)

Allow Wordpress to bind to port 80 by editing the restricted security context restraint.  Change "runAsUser" from ```MustRunAsRange``` to ```RunAsAny```.


```
$ oc edit scc restricted

apiVersion: v1
groups:
- system:authenticated
kind: SecurityContextConstraints
metadata:
  creationTimestamp: 2015-06-26T14:01:03Z
  name: restricted
  resourceVersion: "59"
  selfLink: /api/v1/securitycontextconstraints/restricted
  uid: c827a79d-1c0b-11e5-9166-d4bed9b39058
runAsUser:
  type: MustRunAsRange  <-- change to RunAsAny
seLinuxContext:
  type: MustRunAs
```

Changing the restricted security context as shown above allows the Wordpress container to bind to port 80.  

## Storage Provisioning

OpenShift expects that storage volumes are provisioned by system administrator outside of OpenShift. As subsequent step, the system admin then tells OpenShift about these volumes by creating Persistent Volumes objects. Wearing your "system admin" hat, follow these guides to create Persistent Volumes named `pv001` and `pv002`.

* [NFS](nfs/README.md)
* [OpenStack Cinder](cinder/README.md)
- [Fibre Channel](fc/README.md)

## Persistent Volumes Claims
Now that the "system admin" guy has deployed some Persistent Volumes, you can continue as an application developer and actually use these volumes to store some MySQL and Wordpress data. From now on, the guide does not depend on the underlying storage technology!

```
# Create claims for storage.
# The claims in this example carefully match the volumes created above.
$ oc create -f examples/wordpress/pvc-wp.yaml 
$ oc create -f examples/wordpress/pvc-mysql.yaml
$ oc get pvc

NAME          LABELS    STATUS    VOLUME
claim-mysql   map[]     Bound     pv0002
claim-wp      map[]     Bound     pv0001
```

## MySQL 

Launch the MySQL pod.

```
oc create -f examples/wordpress/pod-mysql.yaml
```

After a few moments, MySQL will be running and accessible via the pod's IP address.  We don't know what the IP address
will be and we wouldn't want to hard-code that value in any pod that wants access to MySQL.  

Create a service in front of MySQL that allows other pods to connect to it by name.

```
# This allows the pod to access MySQL via a service name instead of hard-coded host address
oc create -f examples/wordpress/service-mysql.yaml 
```

## WordPress

We use the MySQL service defined above in our Wordpress pod.  The variable WORDPRESS_DB_HOST is set to the name
 of our MySQL service.
 
Because the Wordpress pod and MySQL service are running in the same namespace, we can reference the service by name.  We
can also access a service in another namespace by using the name and namespace: ```mysql.another_namespace```.  The fully qualified
name of the service would also work: ```mysql.<namespace>.svc.cluster.local```

```
- name: WORDPRESS_DB_HOST
  # this is the name of the mysql service fronting the mysql pod in the same namespace
  # expands to mysql.<namespace>.svc.cluster.local  - where <namespace> is the current namespace
  value: mysql
```

Launch the Wordpress pod and its corresponding service.

```
oc create -f examples/wordpress/pod-wordpress.yaml 
oc create -f examples/wordpress/service-wp.yaml 

oc get svc
NAME            LABELS                                    SELECTOR         IP(S)            PORT(S)
mysql           name=mysql                                name=mysql       172.30.115.137   3306/TCP
wpfrontend      name=wpfrontend                           name=wordpress   172.30.170.55    5055/TCP
```


## Start Blogging

In your browser, visit 172.30.170.55:5055 (your IP address will vary).  The Wordpress install process will lead you through setting up the blog.
