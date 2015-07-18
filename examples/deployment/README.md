Deploying in OpenShift
======================

This guide demonstrates different types of deployments in OpenShift. The scenarios described below cover the main ways that applications can be updated. See the [OpenShift docs](https://docs.openshift.org/latest/dev_guide/deployments.html) for more details on managing deployments in OpenShift.

The examples below assume you have cloned the OpenShift Git repository, have installed the OpenShift client tools, an OpenShift server, the OpenShift router, and have created a project. See [the Getting Started guide](https://docs.openshift.org/latest/getting_started/administrators.html) for help installing OpenShift or [CONTRIBUTING.adoc](https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc) for a more general development guide.

OpenShift Deployments
---------------------

In OpenShift v3, deployments are described using three separate API objects:

* A deployment config, which describes the desired state of a particular component of the application as a pod template
* One or more replication controllers, which contain a point in time record of the state of a deployment config as a pod template
* One or more pods, which represent an instance of a particular version of an application.

When a user creates a deployment config, a replication controller is created representing the deployment config's pod template. If the deployment config changes, a new replication controller is created with the latest pod template, and a deployment process runs to scale down the old replication controller and scale up the new replication controller. This process is run from within a pod. At specific points in the process, optional *hooks* may be run to handle custom logic.

Errors or timeouts during the deployment process may result in the deployment being *failed*, and the old replication controller will be restored to its previous scale.

Instances of your application are automatically added and removed from both service load balancers and routers as they are created. As long as your application supports graceful shutdown when it receives the TERM signal, you can ensure that running user connections are given a chance to complete normally. See the section below on graceful termination.

Each deployment config has a *strategy* describing which fundamental deployment type to use. Some deployment types are implemented with multiple deployment configs - these are known as *composite deployment types*.

Deployment Types
----------------

### Rolling Deployments

A rolling deployment slowly replaces instances of the previous version of an application (in OpenShift and Kubernetes, pods) with instances of the new version of the application. A rolling deployment typically waits for new pods to become *ready* via a *readiness check* before scaling down the old components. If a significant issue occurs, the rolling deployment can be aborted.


#### When to use a rolling deployment?

* When you want to take no downtime during an application update
* When your application supports having old code and new code running at the same time

A rolling deployment means you to have both old and new versions of your code running at the same time. This typically requires that your application handle **N-1** compatibility - that data stored by the new version can be read and handled (or gracefully ignored) by the old version of the code. This can take many forms - data stored on disk, in a database, in a temporary cache, or that is part of a user's browser session. While most web applications can support rolling deployments, it's important to test and design your application to handle it.


#### Example

Rolling deployments are the default in OpenShift. To see a rolling update, follow these steps:

1.  Create a application based on the example deployment images:

        $ oc new-app openshift/deployment-example

    If you have the router installed, make the application available via a route (or use the service IP directly)

        $ oc expose svc/deployment-example

    Browse to the application at `deployment-example.<project>.<router_domain>` to verify you see the 'v1' image.

2.  Scale the deployment config up to three instances:

        $ oc scale dc/deployment-example --replicas=3

3.  Trigger a new deployment automatically by tagging a new version of the example as the `latest` tag:

        $ oc tag deployment-example:v2 deployment-example:latest

4.  In your browser, refresh the page until you see the 'v2' image.

5.  If you are using the CLI, the `oc deploy deployment-example` command will show you how many pods are on version 1 and how many are on version 2. In the web console, you should see the pods slowly being added to v2 and removed from v1.

During the deployment process, the new replication controller is incrementally scaled up. Once the new pods are marked as *ready* (because they pass their readiness check), the deployment process will continue. If the pods do not become ready, the process will abort, and the deployment config will be rolled back to its previous version.


#### Rolling deployment variants

Coming soon!


### Recreate Deployment

A recreate deployment removes all pods from the previous deployment before creating new pods in the new deployment.


#### When to use a recreate deployment?

* When you must run migrations or other data transformations before your new code starts
* When you don't support having new and old versions of your application code running at the same time

A recreate deployment incurs downtime, because for a brief period no instances of your application are running. However, your old code and new code do not run at the same time.


#### Example

You can configure a recreate deployment by updating a deployment config. The `recreate-example.yaml` file in this directory contains the same scenario we tried above, but configured to recreate. The `strategy` `type` field is set to `Recreate`.

1.  Create the example:

        $ oc create -f examples/deployment/recreate-example.yaml

    Browse to the application at `recreate-example.<project>.<router_domain>` to verify you see the 'v1' image.

2.  Trigger a new deployment automatically by tagging a new version of the example as the `latest` tag:

        $ oc tag recreate-example:v2 recreate-example:latest

3.  In your browser, refresh the page until you see the 'v2' image.

4.  If you are using the CLI, the `oc deploy recreate-example` command will show you how many pods are on version 1 and how many are on version 2. In the web console, you should see all old pods removed, and then all new pods created.


### Custom Deployment

Not all deployments are simple. OpenShift allows you to run your own custom deployment logic inside a pod (with an image of your choosing) and manipulate the replication controllers yourself to scale up or down. Since the previous two deployment types are implemented in a pod and invoke the API as well, your custom deployment can also perform the same actions as well as run custom code in between phases of your deployment.

Coming soon: examples!


### Blue-Green Deployment

Blue-Green deployments involve running two versions of an application at the same time and moving production traffic from the old version to the new version ([more about blue-green deployments](http://martinfowler.com/bliki/BlueGreenDeployment.html)). There are several ways to implement a blue-green deployment in OpenShift.


#### When to use a blue-green deployment?

* When you want to test a new version of your application in a production environment before moving traffic to it

Blue-Green deployments make switching between two different versions of your application easy. However, since many applications depend on persistent data you'll need to have an application that supports **N-1** compatibility if you share a database, or implement a live data migration between your database, store, or disk if you choose to create two copies of your data layer.


#### Examples

In order to maintain control over two distinct groups of instances (old and new versions of the code) the blue-green deployment is best represented with multiple deployment configs.

##### Using a route and two services

A route points to a service, and be changed to point to a different service at any time. As a developer, you may test the new version of your code by connecting to the new service before your production traffic is routed to it. Routes are intended for web (HTTP and HTTPS) traffic and so this technique is best suited for web applications.

1.  Create two copies of the example application

        $ oc new-app openshift/deployment-example:v1 --name=bluegreen-example-old
        $ oc new-app openshift/deployment-example:v2 --name=bluegreen-example-new

    This will create two independent application components - one running the `v1` image under the `bluegreen-example-old` service, and one using the `v2` image under the `bluegreen-example-new` service.

2.  Create a route that points to the old service:

         $ oc expose svc/bluegreen-example-old --name=bluegreen-example

    Browse to the application at `bluegreen-example.<project>.<router_domain>` to verify you see the `v1` image.

    Note: On versions of OpenShift older than v1.0.3 (OSE v3.0.1), this command will generate a route at `bluegreen-example-old.<project>.<router_domain>`, not the above location.

3.  Edit the route and change the service to `bluegreen-example-new`:

        $ oc edit route/bluegreen-example

    Change `spec.to.name` to `bluegreen-example-new` and save and exit the editor.

4.  In your browser, refresh the page until you see the 'v2' image.


##### Use a single service and change the label selector

TBD


### A/B Deployment

A/B deployments generally imply running two (or more) versions of the application code or application configuration at the same time for testing or experimentation purposes. The simplest form of an A/B deployment is to divide production traffic between two or more distinct *shards* -- a single group of instances with homogenous configuration and code. More complicated A/B deployments may involve a specialized proxy or load balancer that assigns traffic to specific shards based on information about the user or application (all "test" users get sent to the B shard, but regular users get sent to the A shard). A/B deployments can be considered similar to A/B testing, although an A/B deployment implies multiple versions of code and configuration, where as A/B testing often uses one codebase with application specific checks.


#### When to use an A/B deployment?

* When you want to test multiple versions of code or configuration, but are not planning to roll one out in preference to the other
* When you want to have different configuration in different regions

An A/B deployment groups different configuration and code -- multiple shards -- together under a single logical endpoint. Generally, these deployments, if they access persistent data, should properly deal with N-1 compatibility (the more shards you have, the more possible versions you have running). Use this pattern when you need separate internal configuration and code, but end users should not be aware of the changes


#### Examples

All A/B deployments are composite deployment types consisting of multiple deployment configs.

##### One service, multiple deployment configs

OpenShift, through labels and deployment configurations, can support multiple simultaneous shards being exposed through the same service. To the consuming user, the shards are invisible. An example of the simplest possible sharding is described below:

1.  Create the first shard of the application based on the example deployment images:

        $ oc new-app openshift/deployment-example --name=ab-example-a --labels=ab-example=true SUBTITLE="shard A"

2.  Edit the newly created shard to set a label `ab-example=true` that will be common to all shards:

        $ oc edit dc/ab-example-a

    In the editor, add the line `ab-example: "true"` underneath `spec.selector` and `spec.template.metadata.labels` alongside the existing `deploymentconfig=ab-example-a` label. Save and exit the editor.

3.  Trigger a re-deployment of the first shard to pick up the new labels:

        $ oc deploy ab-example-a --latest

4.  Create a service that uses the common label:

        $ oc expose dc/ab-example-a --name=ab-example --selector=ab-example=true --generator=service/v1

    If you have the router installed, make the application available via a route (or use the service IP directly)

        $ oc expose svc/ab-example

    Browse to the application at `ab-example.<project>.<router_domain>` to verify you see the 'v1' image.

5.  Create a second shard based on the same source image as the first shard, and set a unique value:

        $ oc new-app deployment-example --name=ab-example-b --labels=ab-example=true SUBTITLE="shard B"

6.  Edit the newly created shard to set a label `ab-example=true` that will be common to all shards:

        $ oc edit dc/ab-example-b

    In the editor, add the line `ab-example: "true"` underneath `spec.selector` and `spec.template.metadata.labels` alongside the existing `deploymentconfig=ab-example-b` label. Save and exit the editor.

7.  Trigger a re-deployment of the second shard to pick up the new labels:

        $ oc deploy ab-example-b --latest

8.  At this point, both sets of pods are being served under the route. However, since both browsers (by leaving a connection open) and the router (by default through a cookie) will attempt to preserve your connection to a backend server, you may not see both shards being returned to you. To force your browser to one or the other shard, use the scale command:

        $ oc scale dc/ab-example-a --replicas=0

    Refreshing your browser should show "v1" and "shard B"

        $ oc scale dc/ab-example-a --replicas=1; oc scale dc/ab-example-b --replicas=0

    Refreshing your browser should show "v1" and "shard A"

If you trigger a deployment on either shard, only the pods in that shard will be affected. You can easily trigger a deployment by changing the `SUBTITLE` environment variable in either deployment config `oc edit dc/ab-example-a` or `oc edit dc/ab-example-b`. You can add additional shards by repeating steps 5-7.

Note: these steps will be simplified in future versions of OpenShift.


Readiness Checks
----------------

A readiness check is a developer provided hook that lets the platform know when the application code is ready to serve traffic. Some application frameworks add this implicitly by rejecting traffic until all initialization is complete, but complex applications may have additional conditions that happen after the framework starts (like background caches being warmed up). If a developer or admin provides an endpoint that can be used to determine when the application is ready to serve traffic, the OpenShift infrastructure will ensure that load balancers and deployments do not send traffic to a particular instance until it is ready.

Each container in a pod or pod template can have its own `spec.containers[i].readinessProbe`. The probe can be a command run inside the container, a TCP endpoint, or an HTTP or HTTPS URL that returns 200 on GET. For frameworks that don't start listening until initialization is complete, the TCP connection check is sufficient. For complex applications, we recommend defining an HTTP endpoint within your application that can return a 200 once you are completely initialized.

The rolling deployment strategy will not accept new pods that don't pass their readiness check within a certain timeout (configurable). Pods will not be added to rotation under services or routers until they pass their readiness check.


N-1 Compatibility
-----------------

Applications that have new code and old code running at the same time must be careful to ensure that data written by the new code can be read by the old code. This is sometimes called *schema evolution* and is a complex problem. For some applications the period of time that old code and new code is running side by side is short and so bugs or some failed user transactions are acceptable. For others, the failure pattern may result in the entire application becoming non-functional.

One way to validate N-1 compatibility is to use an A/B deployment - run the old code and new code at the same time in a controlled fashion in a test environment, and verify that traffic that flows to the new deployment does not cause failures in the old deployment.


Graceful Termination
--------------------

OpenShift and Kubernetes give application instances time to shut down before removing them from load balancing rotations. However, applications must ensure they cleanly terminate user connections as well before they exit.

On shutdown, OpenShift will send a TERM signal to the processes in the container. Application code, on receiving SIGTERM, should stop accepting new connections. This will ensure that load balancers route traffic to other active instances. The application code should then wait until all open connections are closed (or gracefully terminate individual connections at the next opportunity) before exiting.

After the graceful termination period expires, a process that has not exited will be sent the KILL signal which immediately ends the process. The `terminationGracePeriodSeconds` attribute of a `Pod` or pod template controls the graceful termination period (default 30 seconds) and may be customized per application as necessary.