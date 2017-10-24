

kubernetes genericapiserver base

CURRENTLY TRACKING MASTER in k8s. Earliest possible release to
directly have the required binary compatibility may be k8s v1.6. K8s
changes on a daily basis so thing may break w/o being updated as K8s
changes.


Invoking `make service-catalog` in the root directory will result in a
`service-catalog` binary in the `bin/` directory.

When the API server starts up, it will generate a certificate will be
generated in `/var/run/kubernetes-service-catalog/` so that directory must be
creatable & writable by the running user.  Make sure you have that set up
before you move on to starting the server.  The following command checks
access.

```
$ bash -c 'if [ -w /var/run/kubernetes-service-catalog/ ] ; then echo "OK" ; else echo "FAIL: /var/run/kubernetes-service-catalog/ not writeable" ;fi'
```
If it fails, create the directory with the user that will run the apiserver.


To run it locally, start with:

```
# run etcd locally on the default port
$ etcd 
# switch to another shell and run
$ ./bin/service-catalog apiserver -v 10 --etcd-servers http://localhost:2379
```

Alternatively, you can run the apiserver and etcd as a pod:
```
# run the apiserver & etcd as a pod
$ kubectl create -f contrib/examples/apiserver/apiserver.yaml
# enable port forwarding from localhost:6443 to the apiserver
$ kubectl port-forward service-catalog-apiserver 6443:6443
# get apiserver certificate from the pod
$ kubectl cp service-catalog-apiserver:/var/run/kubernetes-service-catalog/apiserver.crt /var/run/kubernetes-service-catalog/apiserver.crt
```

In another term check for response from curl.
```
$ curl --cacert /var/run/kubernetes-service-catalog/apiserver.crt https://localhost:6443
{
  "paths": [
    "/apis",
    "/apis/servicecatalog.k8s.io",
    "/apis/servicecatalog.k8s.io/v1beta1",
    "/healthz",
    "/healthz/ping",
    "/swaggerapi/",
    "/version"
  ]
}
```


Let's take a look at apis

```
# curl --cacert /var/run/kubernetes-service-catalog/apiserver.crt https://localhost:6443/apis
{
  "kind": "APIGroupList",
  "groups": [
    {
      "name": "servicecatalog.k8s.io",
      "versions": [],
      "preferredVersion": {
        "groupVersion": "servicecatalog.k8s.io/v1beta1",
        "version": "v1beta1"
      },
      "serverAddressByClientCIDRs": [
        {
          "clientCIDR": "0.0.0.0/0",
          "serverAddress": "9.52.233.169:6443"
        }
      ]
    }
  ]
}
```

And some of ours:
```
# curl --cacert /var/run/kubernetes-service-catalog/apiserver.crt https://localhost:6443/apis/servicecatalog.k8s.io
{
  "kind": "APIGroup",
  "apiVersion": "v1",
  "name": "servicecatalog.k8s.io",
  "versions": [],
  "preferredVersion": {
    "groupVersion": "servicecatalog.k8s.io/v1beta1",
    "version": "v1beta1"
  },
  "serverAddressByClientCIDRs": null
}
```

```
# curl --cacert /var/run/kubernetes-service-catalog/apiserver.crt https://localhost:6443/apis/servicecatalog.k8s.io/v1beta1
{
  "kind": "APIResourceList",
  "apiVersion": "v1",
  "groupVersion": "servicecatalog.k8s.io/v1beta1",
  "resources": null
}
```

kubectl needs a basic kubeconfig setup. A kubeconfig consists of three
sections that can be set with the following three commands:

1. `kubectl config set-credentials service-catalog-creds --username=admin --password=admin`
1. `kubectl config set-cluster service-catalog-cluster --server=https://localhost:6443 --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt`
1. `kubectl config set-context service-catalog-ctx --cluster=service-catalog-cluster --user=service-catalog-creds`
1. `kubectl config use-context service-catalog-ctx`

kubectl seems happy enough. We know there are problems talking to api
groups that are not installed. This error is innocuous, but does
indicate that our apigroup is installed correctly.

```
$ kubectl --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt --server=https://localhost:6443 version
Client Version: version.Info{Major:"1", Minor:"4", GitVersion:"v1.4.6+e569a27", GitCommit:"e569a27d02001e343cb68086bc06d47804f62af6", GitTreeState:"not a git tree", BuildDate:"2016-11-12T09:26:56Z", GoVersion:"go1.7.3", Compiler:"gc", Platform:"darwin/amd64"}
error: failed to negotiate an api version; server supports: map[servicecatalog.k8s.io/v1beta1:{}], client supports: map[componentconfig/v1beta1:{} batch/v1:{} batch/v2alpha1:{} apps/v1beta1:{} autoscaling/v1:{} rbac.authorization.k8s.io/v1beta1:{} policy/v1beta1:{} extensions/v1beta1:{} rbac.authorization.k8s.io/v1beta1:{} storage.k8s.io/v1beta1:{} certificates.k8s.io/v1beta1:{} v1:{} authorization.k8s.io/v1beta1:{} federation/v1beta1:{} authentication.k8s.io/v1beta1:{}]
```
no version resource exists so this is to be expected.

```
$ kubectl --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt --server=https://localhost:6443 get foo
the server doesn't have a resource type "foo"
```
no foo resource exists either.

```
$ kubectl --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt --server=https://localhost:6443 api-versions
```
blank response. apiserver has no public apis. no errors either.



```
# kubectl --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt --server=https://localhost:6443 create -f instance.yaml
instance "test-instance" created
```
query
```
kubectl --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt --server=https://localhost:6443 get instance test-instance -o yaml
apiVersion: servicecatalog.k8s.io/v1beta1
kind: ServiceInstance
metadata:
  creationTimestamp: 2017-01-25T21:57:48Z
  name: test-instance
  resourceVersion: "9"
  selfLink: /apis/servicecatalog.k8s.io/v1beta1/namespaces//instances/test-instance
  uid: 4f88bd75-e349-11e6-8096-fa163e9a471d
spec:
  osbCredentials: ""
  externalID: ""
  osbInternalID: ""
  osbLastOperation: ""
  osbPlanID: ""
  osbServiceID: ""
  osbSpaceGUID: ""
  osbType: ""
  parameters: null
  planName: ""
  serviceClassName: test-serviceclass
status:
  conditions: []
```

cleanup
```
 kubectl --certificate-authority=/var/run/kubernetes-service-catalog/apiserver.crt --server=https://localhost:6443 delete instance test-instance
instance "test-instance" deleted
```



