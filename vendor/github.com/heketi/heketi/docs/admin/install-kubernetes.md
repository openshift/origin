
For cluster administrators, the preferred method of deploying Heketi on
Kubernetes is the [gluster-kubernetes project](https://github.com/gluster/gluster-kubernetes).

This script handles both the Heketi components and the GlusterFS components
and provides a streamlined tool to accomplish the task.

It includes a [setup guide](https://github.com/gluster/gluster-kubernetes/blob/master/docs/setup-guide.md).
It also includes a [Hello World](https://github.com/gluster/gluster-kubernetes/tree/master/docs/examples/hello_world)
featuring an example web server pod using a dynamically-provisioned
GlusterFS volume for storage. For those looking to test or learn more about
this topic, follow the Quick-start instructions found in the main
[README](https://github.com/gluster/gluster-kubernetes) for gluster-kubernetes


If you are a developer or other interested party who wants to know more
about how Heketi integrates with Kubernetes on a component level,
please refer to the [Kubernetes integration document](../design/kubernetes-integration.md).
