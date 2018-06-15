# Best practices for instrumenting infrastructure components in OpenShift

All OpenShift components MUST expose the following endpoints on either their public HTTPS port, or a public HTTP port if none exists

* `/healthz` - returns 200 and the text `ok` if the instance is alive
* `/metrics` - returns a reasonable set of metrics in the Prometheus format for capturing the health and performance of the server in question

Components MUST expose the following endpoints if they are exposed via a service or route

* `/healthz/ready` - returns a 200 and the text `ok` if the instance is ready to begin receiving network traffic

Components that only offer TCP endpoints should listen on a separate port and expose the appropriate checks above.

Components that have equivalents to these endpoints already that are taken "as-is" from the upstream community MAY
use those instead.

Components that expose tenanted info that use authentication MUST use HTTPS to serve metrics - this is a general
infrastructure requirement that all user info flow over TLS.


## Metrics

We use the Prometheus format to be consistent with the upstream Kubernetes community and because it is a natural fit
for the Go ecosystem. COTS software that does not offer a native Prometheus endpoint should use an adapter to expose
prometheus metrics, either in their process or as a sidecar container. Java components may use JMX, but we still
recommend adapting to the prometheus format.

The metrics to expose [should represent business measurements of the service in question](https://prometheus.io/docs/practices/instrumentation/) - if the point of the component
is to process requests, capture the count, duration, and types of requests. If the component has a queue, report the
queue depth and throughput of the queue. Follow Prometheus best practices for [labelling and naming](https://prometheus.io/docs/practices/naming/).

If metrics values or labels contain sensitive information, consider anonymizing them or categorizing them. If
the information is fundamentally multi-tenant, read the next section on security

Golang programs should have easy prometheus integration via the existing libraries. Use a Prometheus exporter
if available and if the cost of adapting metrics is higher in a particular framework or not possible to get
upstreamed.


## Profiling

Components that are high traffic or are known bottlenecks in performance and written in Go SHOULD expose
the `/debug/pprof/*` endpoints. These endpoints MUST be secured by authentication and authorization because
profiling can lead to information disclosure.


## Security

Health checks must be accessible on the pod or local networks so that remote agents can access them.  More
sophisticated checks MAY require authentication and authorization if the data can disclose sensitive info
from the component like passwords, paths on disk, user identifying information.

Metrics endpoints SHOULD be protected by authentication if they disclose tenant information (names of pods,
services, namespaces, or user identifying info). Use BASIC authentication with a password provided via
environment variable or secret (see the router for an example). You may optionally leverage the innate
authorization of the cluster if you are a system component like the controller manager, scheduler, kubelet,
or router.


## Push Metrics

Some components may need to send larger amounts of metrics to a remote agent. To preserve flexibility, this
is possible, but SHOULD be a last resort. Well designed prometheus endpoints should scale to ~1k-10k tenants
cleanly which is within our cluster size bounds today. Pushing metrics requires an exception.


## Exceptions

Exceptions may be granted to these requirements, but components that can't expose this info can't be monitored.
Consider lack of this info a bug.