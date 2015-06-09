OpenShift 3: Project Quota and Resource Limits
========================================
This example will demonstrate how quota and resource limits can be applied to resources in a project.

This example assumes you have completed the sample-app example, and have a functional OpenShift setup.

Resources
-----------------------------------------
By default, a pod in your project runs with unbounded CPU and memory constraints.  This means that
any pod in the system will be able to consume as much CPU and memory on the node that runs the pod.

An author of a pod may set explicit resource limits per container in the pod in order to control
memory usage dedicated to its containers on a node.

The following is an example of a pod that has a single container.  This container sets it's resource
limit for cpu to 100m and memory as 6Mi.  This means that the container will get 100 millicores of
a core on the Node.  In effect, if the node had a single core, this pod could be scheduled 10 times
at most to a single host.

```shell
$ cat pod-with-resources.yaml
apiVersion: v1beta3
kind: Pod
metadata:
  name: pod-with-resources
spec:
  containers:
  - capabilities: {}
    image: gcr.io/google_containers/serve_hostname
    imagePullPolicy: IfNotPresent
    name: kubernetes-serve-hostname
    resources:
      limits:
        cpu: 100m
        memory: 6Mi
    securityContext:
      capabilities: {}
      privileged: false
  dnsPolicy: ClusterFirst
  restartPolicy: Always
```

Applying a Project Quota
-----------------------------------------
Administrators may want to restrict how much of the cluster resources a given project may consume across
all of its pods.  To do this, an administrator applies a quota to a project.  A quota lets the
administrator set hard limits on the total amount of node resources (cpu, memory) and
API resources (pods, services, etc) that a project may require.

Let's create a simple project that applies a basic quota where the total cpu usage across all pods cannot exceed 1 core and may not consume more than 750Mi of memory.

```shell
$ oadm new-project quota-demo --admin=test-admin
$ oc project quota-demo
$ cat quota.yaml
kind: ResourceQuota
metadata:
  name: quota
spec:
  hard:
    cpu: "1"
    memory: "750Mi"
    pods: "10"
    replicationcontrollers: "10"
    resourcequotas: "1"
    services: "10"
$ oc create -f quota.yaml
```

A few moments after the quota is created, the current usage in the project is calculated.

You can view the current usage by doing the following:

```
$ oc describe project quota-demo
Name:   quota-demo
Created:  4 hours ago
Labels:   <none>
Annotations:  displayName=,openshift.io/node-selector=
Display Name: <none>
Description:  <none>
Status:   Active
Node Selector:  <none>

Quota:
  Name:     quota
  Resource    Used  Hard
  --------    ----  ----
  cpu     0m  1
  memory      0m  750Mi
  pods      0m  10
  replicationcontrollers  0m  10
  resourcequotas    1 1
  services    0m  10

Resource limits:  <none>
```

Applying default resource limits
-----------------------------------------
Pod authors rarely specify resource limits for their pods.  As noted earlier, this is problematic because
it means a pod can consume as much resource on a node as is available.

Since we applied a quota to our project, let's see what happens when an end-user creates a pod that has unbounded
cpu and memory.

```shell
$ cat pod-without-resources.yaml
apiVersion: v1beta3
kind: Pod
metadata:
  name: pod-without-resources
spec:
  containers:
  - capabilities: {}
    image: gcr.io/google_containers/serve_hostname
    imagePullPolicy: IfNotPresent
    name: kubernetes-serve-hostname
    securityContext:
      capabilities: {}
      privileged: false
  dnsPolicy: ClusterFirst
  restartPolicy: Always
$ oc create -f pod-without-resources.yaml
Error from server: Pod "pod-without-resources" is forbidden: Limited to 750Mi memory, but pod has no specified memory limit
```

The administrator is happy because end-users need to specify resource limits.

The end-user is miserable because they now need to set explicit resource values, and this is more work.

To make things better, the administrator can set some project wide defaults for resource constraints.

In addition, the administrator can set some limits to the basic shape of a pod and its container to ensure
pods can best fit the available node profile.  For example, while a project may be allowed to request 750Mi of
memory across all containers, the administrator may want to limit the amount of memory a single pod can consume
to 500Mi.  This type of flexibility allows administrators to set min/max limits for cpu and memory constraints
at a pod or container level to fit the nodes that are in the cluster.  After all, if a user can use 20 cpus,
but the largest node in your cluster is 4 cpus, you don't really want user's to build pods that request 8 cpus.

It's best to reject those types of pods up front.

So let's set some default limits for this project:

```shell
$ cat limits.yaml
apiVersion: v1beta3
kind: LimitRange
metadata:
  name: limits
spec:
  limits:
  - max:
      cpu: 500m
      memory: 750Mi
    min:
      cpu: 10m
      memory: 5Mi
    type: Pod
  - default:
      cpu: 100m
      memory: 100Mi
    max:
      cpu: 500m
      memory: 750Mi
    min:
      cpu: 10m
      memory: 5Mi
    type: Container
$ oc create -f limits.yaml
$ oc describe project quota-demo
Name:   quota-demo
Created:  4 hours ago
Labels:   <none>
Annotations:  displayName=,openshift.io/node-selector=
Display Name: <none>
Description:  <none>
Status:   Active
Node Selector:  <none>

Quota:
  Name:     quota
  Resource    Used  Hard
  --------    ----  ----
  cpu     0m  1
  memory      0m  750Mi
  pods      0m  10
  replicationcontrollers  0m  10
  resourcequotas    1 1
  services    0m  10

Resource limits:
  Name:   limits
  Type    Resource  Min Max Default
  ----    --------  --- --- ---
  Pod   memory    5Mi 750Mi -
  Pod   cpu   10m 500m  -
  Container cpu   10m 500m  100m
  Container memory    5Mi 750Mi 100Mi
```

You can now see that the project has set min/max limits at the pod and container scopes.

If a pod is created that has no cpu resource limit set, the default (100m) will be set as an explicit limit.  Similarly, if a pod is created that has no memory resource limit set, the default (256Mi) will be set as an explicit limit.

To demonstrate this, let's try to create the pod that failed previously:

```shell
$ oc create -f pod-without-resources.yaml
$ oc describe project quota-demo
Name:   quota-demo
Created:  4 hours ago
Labels:   <none>
Annotations:  displayName=,openshift.io/node-selector=
Display Name: <none>
Description:  <none>
Status:   Active
Node Selector:  <none>

Quota:
  Name:     quota
  Resource    Used    Hard
  --------    ----    ----
  cpu     100m    1
  memory      104857600 750Mi
  pods      1   10
  replicationcontrollers  0m    10
  resourcequotas    1   1
  services    0m    10

Resource limits:
  Name:   limits
  Type    Resource  Min Max Default
  ----    --------  --- --- ---
  Pod   memory    5Mi 750Mi -
  Pod   cpu   10m 500m  -
  Container cpu   10m 500m  100m
  Container memory    5Mi 750Mi 100Mi
```

As you can see, we now have a single pod in our project, and that pod is consuming the default amount of resources.

Templates: Parameterized resources
-----------------------------------------
Templates allow project editors to quickly add content to the project from pre-defined content.

Pods that are created from template content will use any of the specified resource defaults that we had
previously defined in our project, but as a template author, it is possible to actually expose memory
and cpu consumption as parameters in your template.

To demonstrate this, let's provision a custom template that enumerates resources:

```shell
$ oc create -f application-template-with-resources.json
$ oc describe template templates ruby-helloworld-sample-with-resources
Name:   ruby-helloworld-sample-with-resources
Created:  12 minutes ago
Labels:   <none>
Description:  This example shows how to create a simple ruby application in openshift origin v3
Annotations:  iconClass=icon-ruby,tags=instant-app,ruby,mysql

Parameters:    
    Name:   ADMIN_USERNAME
    Description:  administrator username
    Generated:    expression
    From:   admin[A-Z0-9]{3}

    Name:   ADMIN_PASSWORD
    Description:  administrator password
    Generated:    expression
    From:   [a-zA-Z0-9]{8}

    Name:   MYSQL_USER
    Description:  database username
    Generated:    expression
    From:   user[A-Z0-9]{3}

    Name:   MYSQL_PASSWORD
    Description:  database password
    Generated:    expression
    From:   [a-zA-Z0-9]{8}

    Name:   MYSQL_DATABASE
    Description:  database name
    Value:    root
    Name:   MYSQL_RESOURCES_LIMITS_MEMORY
    Description:  database memory limit
    Value:    200Mi
    Name:   MYSQL_RESOURCES_LIMITS_CPU
    Description:  database cpu limit
    Value:    400m
    Name:   DEPLOY_MYSQL_RESOURCES_LIMITS_MEMORY
    Description:  deploy database memory limit
    Value:    50Mi
    Name:   DEPLOY_MYSQL_RESOURCES_LIMITS_CPU
    Description:  deploy database cpu limit
    Value:    150m
    Name:   FRONTEND_RESOURCES_LIMITS_MEMORY
    Description:  frontend memory limit
    Value:    200Mi
    Name:   FRONTEND_RESOURCES_LIMITS_CPU
    Description:  frontend cpu limit
    Value:    200m
    Name:   DEPLOY_FRONTEND_RESOURCES_LIMITS_MEMORY
    Description:  deploy frontend memory limit
    Value:    50Mi
    Name:   DEPLOY_FRONTEND_RESOURCES_LIMITS_CPU
    Description:  deploy frontend cpu limit
    Value:    150m
    Name:   BUILD_RUBY_RESOURCES_LIMITS_MEMORY
    Description:  build ruby memory limit
    Value:    50Mi
    Name:   BUILD_RUBY_RESOURCES_LIMITS_CPU
    Description:  build ruby cpu limit
    Value:    150m

Object Labels:  template=application-template-stibuild

Objects:   
    Service frontend
    Route route-edge
    ImageStream origin-ruby-sample
    ImageStream ruby-20-centos7
        
        
    Service database
```

Notice that the template exposes parameters to limit the amount of memory and cpu used by the pods in your project.

* MYSQL_RESOURCES_LIMITS_CPU - the amount of cpu for your mysql containers
* MYSQL_RESOURCES_LIMITS_MEMORY - the amount of memory for your mysql containers
* FRONTEND_RESOURCES_LIMITS_CPU - the amount of cpu for your mysql containers
* FRONTEND_RESOURCES_LIMITS_MEMORY - the amount of memory for your mysql containers

When you build your source code, OpenShift will create pods to execute the build in your project.  Those pods consume
node resources, so they are subject to quota.  It is possible to customize the amount of cpu and memory used by
a build.  Notice that the template exposes the following parameters to tailor the amount of resources per build.

* BUILD_RUBY_RESOURCES_LIMITS_MEMORY - the amount of memory used when running builds of your ruby code
* BUILD_RUBY_RESOURCES_LIMITS_CPU - the amount of cpu used when running builds of your ruby code

Finally, when you deploy new versions of your code, OpenShift will create pods to execute the deployment.  Those
pods consume node resources, so they are subject to quota.  Like builds, you can customize the amount of resources
you give to a deployment task:

* DEPLOY_FRONTEND_RESOURCES_LIMITS_MEMORY - the amount of memory used when deploying new versions of your frontend
* DEPLOY_FRONTEND_RESOURCES_LIMITS_CPU - the amount of cpu used when deploying new versions of your frontend
* DEPLOY_MYSQL_RESOURCES_LIMITS_MEMORY - the amount of memory used when deploying new versions of your database
* DEPLOY_MYSQL_RESOURCES_LIMITS_CPU - the amount of cpu used when deploying new versions of your database

Putting it all together
---------------------------------------
Now that we have created our template, let's create the content within it.

```shell
$ oc process ruby-helloworld-sample-with-resources | openshift cli create -f -
services/frontend
routes/route-edge
imageStreams/origin-ruby-sample
imageStreams/ruby-20-centos7
buildConfigs/ruby-sample-build
deploymentConfigs/frontend
services/database
deploymentConfigs/database
```

Every action in the project that consumes node level cpu or memory resources has defined limits.

If you kick off builds, or execute deployments, you will see that those pods have defined resource limits that are derived from their associated configuration.  All of these actions are therefore
tracked in the project quota.

To demonstrate this, let's show what happens when you run at the limit of quota.

Let's kick off a number of builds to see what happens when we exceed quota.

```shell
$ oc start-build ruby-sample-build
$ ... [repeat until exceeded quota] ...
```

Let's assume our 5th build exceeded quota:

```
$ oc describe builds ruby-sample-build-5
Name:     ruby-sample-build-5
Created:    2 minutes ago
Labels:     buildconfig=ruby-sample-build,name=ruby-sample-build,template=application-template-stibuild
BuildConfig:    ruby-sample-build
Status:     New
Duration:   waiting for 2m13s
Build Pod:    ruby-sample-build-5
Strategy:   Source
Image Reference:  DockerImage openshift/ruby-20-centos7:latest
Incremental Build:  yes
Source Type:    Git
URL:      git://github.com/openshift/ruby-hello-world.git
Output to:    origin-ruby-sample:latest
Output Spec:    <none>
Events:
  FirstSeen       LastSeen      Count From      SubobjectPath Reason    Message
  Tue, 19 May 2015 20:55:47 +0000 Tue, 19 May 2015 20:56:01 +0000 2 {build-controller }     failedCreate  Error creating: Pod "ruby-sample-build-5" is forbidden: Limited to 750Mi memory 
```

Note the event that was published from the build controller to denote that there is no more quota
available in the project to execute the build.

Once the other builds complete, and build pods complete, quota will be released, and eventually your
build will schedule a pod and complete.

Summary
----------------------------
Actions that consume node resources for cpu and memory can be subject to hard quota limits defined
by the administrator.

Any action that consumes those resources can be tweaked, or can pick up project level defaults to
meet your end goal.