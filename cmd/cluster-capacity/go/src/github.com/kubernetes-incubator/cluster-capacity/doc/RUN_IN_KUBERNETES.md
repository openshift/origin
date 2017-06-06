# Deploying cluster-capacity on kubernetes

 - Have a running kubernetes environment - You can use e.g ``$ hack/local-up-cluster.sh`` in cloned local [kubernetes repository](https://github.com/kubernetes/kubernetes)

 - If your kubernetes cluster doesn't have limit ranges and requests specified you can do it by specifying limitrange object. If you just want to try cluster-capacity, you can use 
 [Example limit range file](https://github.com/kubernetes-incubator/cluster-capacity/blob/master/doc/example-limit-range.yaml)

 - Make sure the kubernetes default service has the correct port and the traffic from the VIP is forwarded to running Apiserver (e.g. ``kubernetes`` service on VIP ``10.0.0.1`` and port ``443`` with Apiserver running on ``127.0.0.1:6443``).
```
# change service port
$ kubectl patch svc kubernetes -p '[{"op": "replace", "path": "/spec/ports/0/port", "value":6443}]' --type="json"
# update iptables to forward the kubernetes service to the Apiserver
$ sudo iptables -t nat -A PREROUTING -d 10.0.0.1 -p tcp --dport 6443 -j DNAT --to-destination 127.0.0.1
```
 
 - Create pod object:
```sh
$ cat cluster-capacity-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: cluster-capacity
  labels:
    name: cluster-capacity
spec:
  containers:
  - name: cluster-capacity
    image: docker.io/gofed/cluster-capacity:latest
    command:
    - "/bin/sh"
    - "-ec"
    - |
      echo "Generating pod"
      /bin/genpod --namespace=cluster-capacity >> /pod.yaml
      cat /pod.yaml
      echo "Running cluster capacity framework"
      /bin/cluster-capacity --podspec=/pod.yaml --default-config /config/default-scheduler.yaml
    ports:
    - containerPort: 8081
$ kubectl create -f cluster-capacity-pod.yaml
```

 - We need to create proxy, so user can access server running in a pod. That can be done using [kubectl expose](http://kubernetes.io/docs/user-guide/kubectl/kubectl_expose/)

```sh
$ kubectl expose pod cluster-capacity --port=8081
```

 - Get endpoint URL
 
```sh
$ kubectl get endpoints cluster-capacity
```

 - Now you should be able to see cluster status: 

```sh
curl http://<endpoint>/capacity/status
```

 - For more information of how to access acquired data any see [API operations](https://github.com/kubernetes-incubator/cluster-capacity/blob/master/doc/api-operations.md)
 
