# apiserver-network-proxy

Created due to https://github.com/kubernetes/org/issues/715.

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack channel](https://kubernetes.slack.com/messages/sig-cloud-provider)
- [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-sig-cloud-provider)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## Build

Please make sure you have the REGISTRY and PROJECT_ID environment variables set.
For local builds these can be set to anything.
For image builds these determine the location of your image.
For GCE the registry should be gcr.io and PROJECT_ID should be the project you
want to use the images in.

### Local builds

```console
make clean
make certs
make build
```

### Build images

```console
build docker/proxy-server
build docker/proxy-agent
```

## Examples

The current examples run two actual services as well as a sample client on one end and a sample destination for
requests on the other.
- *Proxy service:* The proxy service takes the API server requests and forwards them appropriately.
- *Agent service:* The agent service connects to the proxy and then allows traffic to be forwarded to it.

### GRPC Client using mTLS Proxy with dial back Agent

```
client =HTTP over GRPC=> (:8090) proxy (:8091) <=GRPC= agent =HTTP=> SimpleHTTPServer(:8000)
  |                                                    ^
  |                          Tunnel                    |
  +----------------------------------------------------+
```

- Start SimpleHTTPServer (Sample destination)
```console
python -m SimpleHTTPServer
```

- Start proxy service
```console
./bin/proxy-server --server-ca-cert=certs/master/issued/ca.crt --server-cert=certs/master/issued/proxy-master.crt --server-key=certs/master/private/proxy-master.key --cluster-ca-cert=certs/agent/issued/ca.crt --cluster-cert=certs/agent/issued/proxy-master.crt --cluster-key=certs/agent/private/proxy-master.key
```

- Start agent service
```console
./bin/proxy-agent --ca-cert=certs/agent/issued/ca.crt --agent-cert=certs/agent/issued/proxy-agent.crt --agent-key=certs/agent/private/proxy-agent.key
```

- Run client (mTLS enabled sample client)
```console
./bin/proxy-test-client --ca-cert=certs/master/issued/ca.crt --client-cert=certs/master/issued/proxy-client.crt --client-key=certs/master/private/proxy-client.key
```

### HTTP-Connect Client using mTLS Proxy with dial back Agent (Either curl OR test client)

```
client =HTTP-CONNECT=> (:8090) proxy (:8091) <=GRPC= agent =HTTP=> SimpleHTTPServer(:8000)
  |                                                    ^
  |                          Tunnel                    |
  +----------------------------------------------------+
```

- Start SimpleHTTPServer (Sample destination)
```console
python -m SimpleHTTPServer
```

- Start proxy service
```console
./bin/proxy-server --mode=http-connect --server-ca-cert=certs/master/issued/ca.crt --server-cert=certs/master/issued/proxy-master.crt --server-key=certs/master/private/proxy-master.key --cluster-ca-cert=certs/agent/issued/ca.crt --cluster-cert=certs/agent/issued/proxy-master.crt --cluster-key=certs/agent/private/proxy-master.key
```

- Start agent service
```console
./bin/proxy-agent --ca-cert=certs/agent/issued/ca.crt --agent-cert=certs/agent/issued/proxy-agent.crt --agent-key=certs/agent/private/proxy-agent.key
```

- Run client (mTLS & http-connect enabled sample client)
```console
./bin/proxy-test-client --mode=http-connect  --proxy-host=127.0.0.1 --ca-cert=certs/master/issued/ca.crt --client-cert=certs/master/issued/proxy-client.crt --client-key=certs/master/private/proxy-client.key
```

- Run curl client (curl using a mTLS http-connect proxy)
```console
curl -v -p --proxy-key certs/master/private/proxy-client.key --proxy-cert certs/master/issued/proxy-client.crt --proxy-cacert certs/master/issued/ca.crt --proxy-cert-type PEM -x https://127.0.0.1:8090  http://localhost:8000```
```

### Running on kubernetes
See following [README.md](examples/kubernetes/README.md)

### Clients

`apiserver-network-proxy` components are intended to run as standalone binaries and should not be imported as a library. Clients communicating with the network proxy can import the `konnectivity-client` module.

## Troubleshoot

### Undefined ProtoPackageIsVersion3
As explained in https://github.com/golang/protobuf/issues/763#issuecomment-442767135,
protoc-gen-go binary has to be built from the vendored version:

```console
go install ./vendor/github.com/golang/protobuf/protoc-gen-go
make gen
```
