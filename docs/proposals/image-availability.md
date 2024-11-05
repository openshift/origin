# Image Availability in OpenShift

Production applications and development pipelines depend on fast, reliable, and secure access to images at all times. The OpenShift runtime environment must work in concert with either the integrated registry or external registries to guarantee that availability.

This proposal covers the areas where availability can become compromised and proposes specific solutions to improve overall reliability of a cluster.

## Background

As a dynamic system, Kubernetes and OpenShift depend on the availibilty of images from all machines in order to move, scale, or update workloads. Thus the availability of applications on the cluster is gated by the uptime of the image registries that provide:

- Application images - the images used by running workloads
- Component images - the images used by parts of the OpenShift infrastructure (builders, deployments, task images)
- Build images - the images used as inputs to build processes to generate new source updates

There are many different services that provide image content:

- Content provider registry - a registry that holds images produced by a third party, like the DockerHub, the Red Hat Container Registry, quay.io, or GCR, that may directly serve images running in production
- Centralized private registry - a private hosted registry that is content distribution hub for an enterprise, and serves multiple clusters
- OpenShift integrated registry - holds images produced in local development loops and mirrors other content

The OpenShift integrated registry is intended to perform two core functions:

- Provide easy abstraction of image locations and image security from application configuration
- Ensure high availability and guaranteed deployment of images in production coming from multiple sources

It offers transparent proxying and mirroring, instant tagging, deployment triggering, and simple policy management per cluster specifically to assist other, more canonical registries. Like all OpenShift features it is optional and many production environments use a central private registry directly.


## Common problems

The existing container runtimes in Kubernetes (Docker) assume a global flat namespace of images that can be accessed via tag or image, and may be protected by some form of authentication. 

For running workloads (from the node) there are many reasons an image cannot be accessed:

* Registry is unavailable, misconfigured, or has been corrupted / lost content
* Authentication misconfigured on the client side
* Slow network links, partition, or deliberate disruption of an image source
* Container runtime has run out of storage due to poor scheduling or sudden surge in use

We seek solutions that allow a cluster to continue serving workloads in the face of disruption of the canonical registries hosting core content.


### Availability vs security

The standard image registry is designed to be a content addressable store, where images are assigned identifiers that uniquely identify the content they carry. While it is reasonable to protect the content of images from unauthorized access, many deployments would prefer to favor availability of their production applications over a failure that leaves the images unavailable. 

Checking image access early in the deployment pipeline and resolving to image digests (such as via the OpenShift deployment triggers or via image policy) would allow nodes to trust the master to control image access and thus open the door for optimizations around how images are retrieved on the node (via an additional source of image content that is highly available and optimized for performance).

In high and single tenant environments, the cluster administrator can likely rely on this security to properly isolate tenants. In medium density tenant environments, it's likely that separate nodes are used to isolate workloads, and so a per node configuration would prevent cross tenant security. 

For those still concerned with security of particular image content, it should be possible to opt out of this mechanism or at least ensure that certain content is not shared. Generally though, those concerned with this attack vector would also prefer dedicated nodes, and thus fall into a solution like the above.


### Failure of external registries

Centralized and content provider registries are usually hosted outside of the cluster and thus are susceptible to traffic interruptions as well as outages. This could be a specific outage (the DockerHub is down) or a general issue (there is no connection to the external network).

For the former, mirroring image content (on demand, or automatically) onto a secondary and high availability store and allowing node fallback to that secondary location as necessary would be sufficient. For the latter, it is incredibly likely that any running workload has an image available on at least one node of the cluster, and thus a mechanism that allows images to be sourced from other nodes as an ultimate fallback would succeed with high likelihood, albeit at a performance impact to that node.

It seems reasonable to assume that nodes should have some flexibility to attempt image pulls from another source in parallel to or in preference to the specified image string, as long as other guarantees are preserved.


### Local inability to create images

In an extreme, a local node may be unable to make space to host a new image. It is expected that the eviction and node GC processes will eventually make space available, thus this is a transient error. 


### Transient errors

It is important to identify the timescales under which image access retries will be possible - some workloads may be start time sensitive or have deadlines (a deployment config pod is one such example) and so a fixed number of retries may be insufficient.

We should audit the internal cluster processes that have limited retry windows and assess whether they are sufficient for common failure modes on the node. We should ensure that any fallback mitigations can survive at least one restart interval.


### Similarities with image promotion

Image promotion attempts to ensure multiple clusters have access to the exact content needed to redeploy an application. Features proposed here should be assessed for their ability to improve / optimize image promotion.


### Use of the integrated registry for availability

The integrated registry can be configured to both intercept and mirror content accessed via the cluster, turning it into a local mirror of remote content. As a centralized infrastructure component, its availability is gated by the network connection between a node and the integrated registry, by the master APIs that implement the core image API, and by the underlying etcd store. A failure in those components should not prevent nodes from being able to restart pods or to execute newly scheduled pods (for instance when a partition affects the infrastructure nodes running the integrated registry, but not the masters or the workload nodes).

In addition, the integrated registry cannot easily serve the infrastructure images used by the cluster during setup today, which should be addressed. It should be easy to configure the integrated registry to automatically mirror remote images, and to layer that mechanism on top of other, more fundamental fallbacks at the node level.


### Highly available registries

It is expected that content registries be optimized for the highest possible availability if their content will be used in production environments. However in practice the features expected of a dynamic registry may complicate or reduce overall availability. Where possible we should propose mechanisms that image registries can implement to become more available, and look to evolve the image specification to improve that availability.

Ultimately, the best availability can be found by leveraging CDN and object registries in the public cloud, minimizing critical path dependencies, reducing the amount of dynamic logic present in the serving path, avoiding complex authentication schemes (certificates or short lived tokens tend to cause outages if other rotation systems are inadequate), and in general falling back to principles used by the largest websites for managing their static content. Content addressible images offer a unique advantage in that they can be append only stores and tend to expire over very long periods.


## Designs

... TODO ...

Tools that we can consider

* Pullthrough / mirror to mitigate remote registries being down
  * Automatic mirroring for running pods
  * Better support for mirroring - controller that ensures registry has all content up to date
  * Mirroring registry content to S3 or other object store (see image mirror cli PR)
* Local node support for image fallback (via CRI, cri-o, skopeo, docker) to a different registry
  * Alternate: Let nodes pull images by digest from a completely different source (like mirror above)
* Consider letting nodes pull from each other to mitigate registry being down
  * See quay.io bittorrent support
  * See kubernetes addon for local registry proxy - we could do this easily to form a mesh?
* Think about how to make registries more available
* Think about how to make integrated registry more available
  * Public images support - can allow pull without auth
  * Local caching of image data or auth data - can we survive temporary auth/api downtime
  * Can we fail open for certain images (allow pull no matter what)
* Sharing underlying object store across multiple clusters (part of promotion discussions)
  * Need to make it easier to import export imagestreams as admins see them
  * Mirroring jobs (above) could make this easier


