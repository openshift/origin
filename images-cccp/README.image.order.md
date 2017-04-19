= OpenShift Origin Image Build Order =

 * <Directory Name> {Docker Name}

== BUILDABLE ==

* rhel7 / centos7
    * base {openshift/origin-base}
        * builder/docker/custom-docker-builder {openshift/origin-custom-docker-builder}
        * dockerregistry {openshift/origin-docker-registry}
        * ipfailover/keepalived {openshift/origin-keepalived-ipfailover}
        * origin {openshift/origin}
            * builder/docker/docker-builder {openshift/origin-docker-builder}
            * builder/docker/sti-builder {openshift/origin-sti-builder}
            * deployer {openshift/origin-deployer}
            * node {openshift/node}
            * router/f5 {openshift/origin-f5-router}
        * router/egress {openshift/origin-egress-router}
        * router/haproxy-base {openshift/origin-haproxy-router-base}
            * router/haproxy {openshift/origin-haproxy-router}
        * recycler {openshift/origin-recycler}
        * release {openshift/origin-release}
    * dind {openshift/dind}
    * openvswitch {openshift/openvswitch}
    * pod {openshift/origin-pod}

=== BUILDING ===
```
git clone https://github.com/tdawson/origin.git
cd origin/images/
git checkout 2016-centos-images
HERE=`pwd`
cd ${HERE}/base
docker build -t openshift/origin-base .
cd ${HERE}/builder/docker/custom-docker-builder
docker build -t openshift/origin-custom-docker-builder .
cd ${HERE}/dockerregistry
docker build -t openshift/origin-docker-registry .
cd ${HERE}/ipfailover/keepalived
docker build -t openshift/origin-keepalived-ipfailover .
cd ${HERE}/origin
docker build -t openshift/origin .
cd ${HERE}/builder/docker/docker-builder
docker build -t openshift/openshift/origin-docker-builder .
cd ${HERE}/builder/docker/sti-builder
docker build -t openshift/openshift/origin-sti-builder .
cd ${HERE}/deployer
docker build -t openshift/origin-deployer .
cd ${HERE}/node
docker build -t openshift/node .
cd ${HERE}/router/f5
docker build -t openshift/origin-f5-router .
cd ${HERE}/router/egress
docker build -t openshift/origin-egress-router .
cd ${HERE}/router/haproxy-base
docker build -t openshift/openshift/origin-haproxy-router-base .
cd ${HERE}/router/haproxy
docker build -t openshift/openshift/origin-haproxy-router .
cd ${HERE}/recycler
docker build -t openshift/origin-recycler .
cd ${HERE}/release
docker build -t openshift/origin-release .
cd ${HERE}/dind
docker build -t openshift/openshift/dind .
cd ${HERE}/openvswitch
docker build -t openshift/openvswitch .
cd ${HERE}/pod
docker build -t openshift/origin-pod .
```

== NOT BUILDABLE ==
* openldap (?FROM openshift/openldap-2441-centos7:latest) {}
* simple-authenticated-registry (?registry:2?)  {}
