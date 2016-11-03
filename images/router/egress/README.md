# OpenShift Egress Router

The OpenShift egress router runs a service that redirects traffic to a
specified remote server, using a private source IP address which is
not used for anything else. This can be used to allow pods to talk to
servers that are set up to only allow access from whitelisted IP
addresses.

## Deploying the egress router pod

The Pod definition for an egress router will look something like this:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: egress-1
  labels:
    name: egress-1
  annotations:
    pod.network.openshift.io/assign-macvlan: "true"
spec:
  containers:
  - name: egress-router
    image: openshift/origin-egress-router
    securityContext:
      privileged: true
    env:
    - name: EGRESS_SOURCE
      value: 192.168.12.99
    - name: EGRESS_GATEWAY
      value: 192.168.12.1
    - name: EGRESS_DESTINATION
      value: 203.0.113.25
  nodeSelector:
    site: springfield-1
```

The `pod.network.openshift.io/assign-macvlan` annotation tells
OpenShift to create a macvlan network interface on the primary network
interface, and then move it into the pod's network namespace before
starting the egress-router container. (Note the quotes around "true";
you'll get errors if you forget them.)

The pod contains a single container, using the
`openshift/origin-egress-router` image, and that container is run
privileged so that it can configure the macvlan interface and set up
iptables rules.

The environment variables tell the egress-router image what addresses
to use; it will configure the macvlan interface to use `EGRESS_SOURCE`
as its IP address, with `EGRESS_GATEWAY` as its gateway. (The
EGRESS_SOURCE is an IP address on the node subnet reserved by the
cluster administrator for use by this pod. The EGRESS_GATEWAY is the
same as the default gateway used by the node itself.). It will then
set up NAT rules so that connections to any TCP or UDP port on the
pod's cluster IP address will be redirected to the same port on
`EGRESS_DESTINATION`. In this example, connections to the pod will be
redirected to **203.0.113.25**, with a source IP address of
**192.168.12.99**.

If only some of the nodes in your cluster are capable of claiming the
specified source IP address and using the specified gateway, you can
specify a `nodeName` or `nodeSelector` indicating which nodes are
acceptable. In this example, the pod will only be deployed to nodes
with the label **site: springfield-1**.

## Deploying an egress router service

Though not strictly necessary, you will normally want to create a
service pointing to the egress router:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: egress-1
spec:
  ports:
  - name: http
    port: 80
  - name: https
    port: 443
  type: ClusterIP
  selector:
    name: egress-1
```

Your pods can now connect to this service, and their connections will
be redirected to the corresponding ports on the external server, using
the reserved egress IP address.
