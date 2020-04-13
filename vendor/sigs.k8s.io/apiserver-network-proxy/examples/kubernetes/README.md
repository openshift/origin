# HTTP-Connect Client using UDS Proxy with dial back Agent runs on kubernetes cluster

# Push all images to container registry
```console
TAG=$(git rev-parse HEAD)
make docker-push TAG=${TAG}
```

# Start a test web-server as a kubernetes pod & service
```bash
kubectl apply -f examples/kubernetes/kubia.yaml
```

# Initialize environment variables
*CLUSTER_CERT* and *CLUSTER_KEY* are certificates used for starting [kubernetes API server](https://kubernetes.io/docs/concepts/cluster-administration/certificates/)

```bash
CLUSTER_IP=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed -n "s/https\:\/\/\(\S*\).*$/\1/p")
KUBIA_IP=$(kubectl get svc kubia -o=jsonpath='{.spec.clusterIP}')
PROXY_IMAGE=$(docker images | grep "proxy-server-" -m1 | awk '{print $1}')
AGENT_IMAGE=$(docker images | grep "proxy-agent-" -m1 | awk '{print $1}')
TEST_CLIENT_IMAGE=$(docker images | grep "proxy-test-client-" -m1 | awk '{print $1}')
SERVER_TOKEN=$(./examples/kubernetes/token_generation.sh 32)
CLUSTER_CERT=<yourdirectory/server.crt>
CLUSTER_KEY=</yourdirectory/server.key>
```

#### GCE sample configuration
```bash
CLUSTER_CERT=/etc/srv/kubernetes/pki/apiserver.crt
CLUSTER_KEY=/etc/srv/kubernetes/pki/apiserver.key
```

# Register SERVER_TOKEN in [static-token-file](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#static-token-file)
Append the output of the following line to the [static-token-file](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#static-token-file) and restart **kube-apiserver** on the master
```bash
echo "${SERVER_TOKEN},system:konnectivity-server,uid:system:konnectivity-server"
```

#### GCE sample configuration
1. [static-token-file](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#static-token-file) location is: **/etc/srv/kubernetes/known_tokens.csv**

1. Restart kube-apiserver
```bash
K8S_API_PID=$(sudo crictl ps | grep kube-apiserver | awk '{ print $1; }') 
sudo crictl stop ${K8S_API_PID}
```

# Save following config at /etc/srv/kubernetes/konnectivity-server/kubeconfig on master VM
```bash
SERVER_TOKEN=${SERVER_TOKEN} envsubst < examples/kubernetes/kubeconfig
```

# Create a clusterrolebinding allowing proxy-server authenticate proxy-client
```bash
kubectl create clusterrolebinding --user system:konnectivity-server --clusterrole system:auth-delegator system:konnectivity-server
```

# Start **proxy-server** as a [static pod](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/) with following configuration
```bash
TAG=${TAG} PROXY_IMAGE=${PROXY_IMAGE} CLUSTER_CERT=${CLUSTER_CERT} CLUSTER_KEY=${CLUSTER_KEY} envsubst <  examples/kubernetes/konnectivity-server.yaml
```
#### GKE specific configuration
*/etc/kubernetes/manifests* is a folder where .yaml file needs to be created for static pod

# Start **proxy-agent** as a kubernetes pod
```bash
TAG=${TAG} AGENT_IMAGE=${AGENT_IMAGE} CLUSTER_IP=${CLUSTER_IP}  envsubst < examples/kubernetes/konnectivity-agent.yaml | kubectl apply -f -
```

# Run **test-client** as a [static pod](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/) with following configuration on same machine where **proxy-server** runs
```bash
TAG=${TAG} KUBIA_IP=${KUBIA_IP} TEST_CLIENT_IMAGE=${TEST_CLIENT_IMAGE} envsubst < examples/kubernetes/konnectivity-test-client.yaml
```

Last row in the following log file **/var/log/konnectivity-test-client.log** supposed to be: **You've hit kubia**

#### GKE specific configuration
*/etc/kubernetes/manifests* is a folder where .yaml file needs to be created for static pod
