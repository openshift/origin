# Serving Certificates, Authentication, and Authorization

This document outlines how the service catalog API server handles
authentication and authorization, and the steps needed to set this up in
your cluster.

Additionally, it provides instructions for generating the serving
certificates used to securely serve the service catalog API.

**Note**: when launching from the service catalog API server from the Helm
charts, authentication and authorization is disabled by default.  The
`apiserver.auth.enabled` option can be set on the Helm chart to enable
authentication and authorization.

## Certificates Overview

CA (Certificate Authority) certificates are used to delegate trust.
Whenever something trusts the CA, it can trust any certificates *signed*
by the CA private key by verifying the signature using the CA certificate.

If a certificate is not signed by a separate CA, it is instead
*self-signed*. A self-signed certificate must either be trusted directly
(instead of being trusted indirectly by trusting a CA), or not trusted at
all.  Generally, our client CA certificates will be self-signed, since
they represent the "root" of our trust relationship: clients must
inherently trust the CA.

In a full setup of the service catalog API server, there are three
*different* CAs (and these really should be different):

1. a serving CA: this CA signs "serving" certificates, which are used to
   encrypt communication over HTTPS.  The same CA used to sign the main
   Kubernetes API server serving certificate pair may also be used to sign
   the service catalog serving certificates, but a different CA
   may also be used.

   By default, the service catalog API server automatically generates
   self-signed certificates if no serving certificates are passed in,
   making this CA optional.  However, in a real setup, you'll need this CA
   so that clients can easily trust the identity of the service catalog
   API server.

2. a client CA: this CA signs client certificates, and is used by the API
   server to authenticate users based on the client certificates they
   submit.  The same client CA may also be used for the service catalog
   API server, but a different CA may also be used.  Using the same CA
   ensures that identity trust works the same way between the main
   Kubernetes API server and the service catalog API server.

   As an example, the default cluster admin user generated in many
   Kubernetes setups uses client certificate authentication. Additionally,
   controllers or non-human clients running outside the cluster often use
   certificate-based authentication.

3. a RequestHeader client CA: this special CA signs proxy client
   certificates.  Clients presenting these certificates are effectively
   trusted to masquerade as any other identity.  When running behind the
   API aggregator, this *must* be the same CA used to sign the
   aggregator's proxy client certificate.  When not running with an
   aggregator (e.g. pre-Kubernetes-1.7, without a separate aggregator
   pod), this simply needs to exist.

### Quickstart

**Note**: options on the service catalog API server take paths to files.
Options passed to the Helm chart take the base64-encoded contents of the
files.

In order to run the service catalog API server with a full authentication
and authorization setup, refer to the following stesp:

1. Ensure that you have a ConfigMap in the `kube-system` namespace named
   `extensions-apiserver-authentication`.  If not, simply follow the "if
   not, ..." clauses in steps 2 and 3.

   ```shell
   $ kubectl get configmap extension-apiserver-authentication --namespace kube-system -o yaml
   ```

2. Ensure that there is a `client-ca-file` key in that ConfigMap.  If not,
   you'll need a client CA file (see [generating
   certificates](#generating-certificates) below).  This file can be
   passed to the service catalog API server using `--client-ca-file`.

3. Ensure that there is a `requestheader-client-ca-file` key in that
   ConfigMap. If not, you'll need a requestheader client CA file (see
   [generating certificates](#generating-certificates) below).  This file
   can be passed to the service catalog API server using
   `--requestheader-client-ca-file`, or the
   `apiserver.tls.requestHeaderCA` Helm chart option:

   ```shell
   helm install charts/catalog --name catalog --namespace catalog --set apiserver.auth.enabled=true --set apiserver.tls.requestHeaderCA=$(base64 --wrap 0 requestheader-client-ca.crt)
   ```

4. If running the service catalog API server outside of the corresponding
   Kubernetes cluster, ensure that you pass in a Kubeconfig file to
   authenticate with the main cluster using the
   `--authentication-kubeconfig` and `--authorization-kubeconfig` options
   on the service catalog API server.

### Generating certificates

The Kubernetes documentation has a [detailed
section](https://kubernetes.io/docs/admin/authentication/#creating-certificates)
on how to create certificates several different ways.  For convenience,
we'll reproduce the basics using the `openssl` and `cfssl` commands below
(you can install `cfssl` using `go get -u
github.com/cloudflare/cfssl/cmd/...`).

In the common case, all three CA certificates referenced above already
exist as part of the main Kubernetes cluster setup.

In case you need to generate any of the CA certificate pairs mentioned
above yourself, you can do so using the following command (see below for
appropriate values of `$PURPOSE`):

```shell
export PURPOSE=<purpose>
openssl req -x509 -sha256 -new -nodes -days 365 -newkey rsa:2048 -keyout ${PURPOSE}-ca.key -out ${PURPOSE}-ca.crt -subj "/CN=ca"
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","'${PURPOSE}'"]}}}' > "${PURPOSE}-ca-config.json"
```

This generates a certificate and private key for the CA, as well as
a signing configuration used by `cfssl` below. `$PURPOSE` should be set to
one of `serving`/`server`, `client`, or `requestheader-client`, as
detailed above in the [certificates overview](#certificates-overview).

These CA certificates are self-signed; no "higher-level" CAs are signing
these CA certificates, so they represent the "roots" of your trust
relationship.

To generate a serving certificate keypair (see the [serving
certificates](#serving-certificates) section for more details), you can
use the following commands:

```shell
export SERVICE_NAME=<service>
export ALT_NAMES="<service>.<namespace>,<service>.<namespace>.svc"
echo '{"CN":"'${SERVICE_NAME}'","hosts":['${ALT_NAMES}'],"key":{"algo":"rsa","size":2048}}' | cfssl gencert -ca=server-ca.crt -ca-key=server-ca.key -config=server-ca-config.json - | cfssljson -bare apiserver
```

`<service>` should be the name of the Service for service
catalog API server (e.g. `<release>-<chart>` when using Helm).

To base64 encode these files for passing to the Helm charts, run `base64
--wrap=0 <file>`.  The resulting output may be passed to the Helm charts
for the `apiserver.tls.*` series of options.

### Serving Certificates

In order to securely serve the service catalog API server over HTTPS,
you'll need serving certificates.  By default, a set of self-signed
certificates are generated by the service catalog API server. However,
clients have no way to trust these (since they are self-signed, there is
no separate CA), so production deployments, or deployments running behind
an API server aggregator, should use manually generated CA certificates.

By default, the service catalog API server looks for these certificates in
the `/var/run/kubernetes-service-catalog` directory, although this may be
overridden using the `--cert-dir` option on the API server.  The files
must be named `apiserver.crt` and `apiserver.key`.

When deploying using the Helm charts, you should pass the
`apiserver.tls.cert` and `apiserver.tls.key` options, populated with the
contents of the aforementioned files, base64 encoded.

## Authentication

The service catalog API server makes use of the standard Kubernetes add-on
API server delegated authentication setup.  There are several components
of the delegated authentication setup, detailed below.

### Client Certificate Authentication

Client certificate authentication authenticates clients who connect using
certificates signed by a given CA (as specified by the *client CA
certificate*).  This same mechanism is also generally used by the main
Kubernetes API server.

Generally, the default admin user in a cluster connects with client
certificate authentication.  Additionally, off-cluster non-human clients
often use client certificate authentication.

By default, a main Kubernetes API server configured with the
`--client-ca-file` option automatically creates a ConfigMap called
`extension-apiserver-authentication` in the `kube-system` namespace,
populated with the the client CA file.  The service catalog API server use
this CA certificate as the CA used to verify client authentication. This
way, client certificate users who can authenticate with the main
Kubernetes system can also authenticate with the service catalog API
server.

See the [delegated token authentication](#delegated-token-authentication)
section for more information about how the service catalog API server
contacts the main Kubernetes API server to access this ConfigMap.

If you wish to use a different client CA certificate to verify client
certificate authentication, you can manually pass the `--client-ca-file`
option to the service catalog API server.

See the [x509 client
certificates](https://kubernetes.io/docs/admin/authentication/#x509-client-certs)
section of the Kubernetes documentation for more information.

### Delegated Token Authentication

Delegated token authentication authenticates clients who pass in a token
using the `Authorization: Bearer $TOKEN` HTTP header.  This is the common
authentication method used by most human Kubernetes clients, as well as
in-cluster non-human clients.

In this case, the service catalog API server extracts the token from the
HTTP request, and verifies it against another API server. In common cases,
this is the main Kubernetes API server.  This allows users who are can
authentication with the main Kubernetes system to also authenticate with
the service catalog API server.

By default, the service catalog API server searches for the connection
information and credentials that are automatically injected into every pod
running on a Kubernetes cluster.

If you do not wish to have the service catalog API server authenticate
against the same cluster that it is running on, or if it is running
outside of a cluster, you can pass the `--authentication-kubeconfig`
option to the serice catalog API server to specify a different Kubeconfig
file to use to connect.

The [Webhook token
authentication](https://kubernetes.io/docs/admin/authentication/#webhook-token-authentication)
method described in the Kubernetes authentication documentation works
similarly in principal to delegated token authentication, except that we
use an existing Kubernetes cluster instead of an external webhook.

### RequestHeader Authentication

RequestHeader authentication authenticates connections from API server
proxies, which themselves have already authenticated the client.  It works
similarly to [client certificate
authentication](#client-certificate-authentication): it validates the
certificate of the proxy using a CA certificate.  However, it then allows
the proxy to masquerade as any other user, by reading a series of headers
set by the proxy. This allows the service catalog API server to run behind
the API server aggregator.

By default, the service catalog API server attempts to pull the
requestheader client CA certificate and appropriate header names from the
`extension-apiserver-authentication` ConfigMap mentioned above in
[client-certificate-authentication](#client-certificate-authentication).
The main Kubernetes API server populates this if it was configured with
the `--requestheader-client-ca-file` option (and optionally associated
`--requestheader-` options).

However, some API servers are not configured with the
`--requestheader-client-ca-file` option.  In these cases, you must pass
the `--requestheader-client-ca-file` option to the service catalog API
server. Any API server proxies need to have client certificates signed by
this CA certificate in order to properly pass their authentication
information through to the service catalog API server.

When using the Helm charts, you can pass the contents of the CA
certificate in the `apiserver.tls.requestHeaderCA` option (base64
encoded), similarly to how other certificates are passed to the Helm
charts.

Alternatively, you can pass the `--authentication-skip-lookup` flag to the
service catalog API server.  However, this will *also* disable client
certificate authentication unless you manually pass the corresponding
`--client-ca-file` flag.

In addition to the CA certificate, you can also configure a number of
additional options.  See the [authenticating
proxy](https://kubernetes.io/docs/admin/authentication/#authenticating-proxy)
section of the Kubernetes documentation for more information.

### Authorization

The service catalog API server uses delegated authorization.  This means
that it queries for authorization against the main Kubernetes API server.
This means that you can store our policy in the same place as the policy
used for the main Kubernetes API server, and in the same format (e.g.
Kubernetes RBAC).

By default, the service catalog API server searches for the connection
information and credentials that are automatically injected into every pod
running on a Kubernetes cluster.

If you do not wish to have the service catalog API server authenticate
against the same cluster that it is running on, or if it is running
outside of a cluster, you can pass the `--authorization-kubeconfig` option
to the serice catalog API server to specify a different Kubeconfig file to
use to connect.
