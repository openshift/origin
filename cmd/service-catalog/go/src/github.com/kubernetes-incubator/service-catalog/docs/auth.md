# Serving Certificates, Authentication, and Authorization

This document outlines how the service catalog API server handles
authentication and authorization.

The service catalog Helm chart's defaults paired with most Kubernetes
distributions will automatically set up all authentication and authorization
details correctly. This documentation, therefore, exists for the benefit of
those who wish to develop and advanced understanding of this topic and those
who have a need to address various outlying scenarios.

## Certificates Overview

Certificates are used for parties to identify themselves to one another.

CA (Certificate Authority) certificates are used to delegate trust.
Whenever something trusts the CA, it can trust any certificates *signed*
by the CA.

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

   The service catalog Helm chart automatically generates this CA. There is
   generally no need to override this.

2. a client CA: this CA signs client certificates, and is used by the main
   Kubernetes API server to authenticate users based on the client certificates
   they submit.  The same client CA may also be used for the service catalog
   API server, but a different CA may also be used.  Using the same CA
   ensures that identity trust works consistently for both the main Kubernetes
   API server and the service catalog API server.

   As an example, the default cluster admin user generated in many
   Kubernetes distributions uses client certificate authentication.
   Additionally, controllers or non-human clients running outside the cluster
   often use certificate-based authentication.

3. a RequestHeader client CA: this special CA signs proxy client
   certificates. Clients presenting these certificates are effectively
   trusted to masquerade as any other identity.  When running behind the
   API aggregator, this *must* be the same CA used to sign the
   aggregator's proxy client certificate.

   On many Kubernetes distributions, this CA is provided through a flag on the
   main Kubernetes API server. The main Kubernetes API server inserts the CA
   certificate into a config map and the API server for all aggregated APIs
   will, by default, inherit the CA from that config map. For Kubernetes
   distributions that do not handle this automatically, the RequestHeader client
   CA can be set manually on the service catalog API server.

### Generating certificates

In the common case, all three CA certificates referenced above already
exist as part of the main Kubernetes cluster setup.

In case you need to generate any of the CA certificate pairs mentioned
above yourself, the Kubernetes documentation has [detailed
instructions](https://kubernetes.io/docs/admin/authentication/#creating-certificates)
on how to create certificates several different ways.

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
