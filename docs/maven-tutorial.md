# Maven + Nexus Tutorial

While developing your application with Java and Maven, you'll most likely be
building many times. In order to shorten the build times of your
pods, Maven dependencies can be cached in a local Nexus repository. This
tutorial will step you through creating a Nexus repository on your cluster.

|Content:|
|:--------|
|[Prerequisites](#prerequisites)|
|[Setting up Nexus](#setting-up-nexus)|
|[Connecting to Nexus](#connecting-to-nexus)|
|[See Also](#see-also)|

## Prerequisites

This tutorial assumes that you'll be working with a project that is already set
up for use with Maven. If you're interested in using Maven with your Java
project, we highly recommend looking at
[their guide](https://maven.apache.org/guides/getting-started/index.html).

In addition, be sure to check your application's image for Maven mirror
capabilities. Many images that use Maven have a 'MAVEN_MIRROR_URL' environment
variable that you can use to simplify this process. If it doesn't have this
capability, read
[the Nexus Maven documentation](https://books.sonatype.com/nexus-book/reference/config.html)
to configure your build properly.

## Setting up Nexus

Use `new-app` to download and deploy the official Nexus Docker image.
```bash
oc new-app sonatype/nexus
```

After the build has finished, run:
```bash
oc get svc
```

The output should resemble:
```bash
NAME      CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
nexus     172.30.170.249   <none>        8081/TCP   6s
```

Confirm that Nexus is running by navigating your browser to the new Cluster-IP
at port 8081. The server may take a bit of time to start, be patient!

Nexus comes pre-configured for the Central Repository, but you may need others
for your application.  For many Red Hat images it is recommended to add the
jboss-ga repository at `https://maven.repository.redhat.com/ga/`. Documentation
on how to add resositories can be found
[here](https://books.sonatype.com/nexus-book/reference/config-sect-custom.html).
The default admin username is 'admin', and the password is 'admin123'.

#### Using Probes to Check for Success

This is a good time to set up readiness and livenes probes. These will
periodically check to see that Nexus is running properly. More information on
probes can be found
[here](https://docs.openshift.org/latest/dev_guide/application_health.html).

```bash
oc set probe dc/nexus \
	--liveness \
	--failure-threshold 3 \
	--initial-delay-seconds 30 \
	-- echo ok
oc set probe dc/nexus \
	--readiness \
	--failure-threshold 3 \
	--initial-delay-seconds 30 \
	--get-url=http://nexus:8081/content/groups/public
```

#### Adding Persistence to Nexus

> NOTE: If you don't want persistent storage, continue to
> [Connecting to Nexus](#connecting-to-nexus). However, your cached
> dependencies will be lost if the pod is restarted for any reason.

Now we'll create a Persistent Volume Claim for Nexus, so that our dependencies
will not be lost when the Pod running the server terminates. If you do not have
admin access on your cluster, ask your system administrator to create a
Read/Write Persistent Volume for you.  Otherwise, read
[Managing Volumes in OpenShift](https://docs.openshift.org/latest/dev_guide/volumes.html)
for instructions on creating a persistent volume.

Add a Persistent Volume Claim to the Nexus Deployment Config.
```bash
oc volumes dc/nexus --remove --confirm
oc volumes dc/nexus --add \
	--name 'nexus-pvc' \
	--type 'pvc' \
	--mount-path '/sonatype-work/' \
	--claim-name 'nexus-pv' \
	--claim-size '1G'
```

This removes the previous ephemeral volume for the Deployment Config and adds a claim
for 1 gigabyte of persistent storage mounted at `/sonatype-work`, which is where our
dependencies will be stored. Due to the change in configuration, the Nexus pod will
be redeployed automatically.

Verify that Nexus is running by refreshing the Nexus page in your browser. You
can monitor the deployment's progress using
```bash
oc get pods -w
```

## Connecting to Nexus

The next steps demonstrate defining a build that will use the new Nexus
repository.  The rest of the tutorial will be using
[this example repository](https://github.com/openshift/jee-ex.git)
with `wildfly-100-centos7` as a builder, but these changes should work for any
project.

The example builder image, wildfly-100-centos7, supports 'MAVEN_MIRROR_URL' as
part of its environment, so we can use this to point our builder image to our
Nexus repository. If your image does not support consuming an environment
variable to configure a Maven mirror, you may need to modify the builder image
to provide the correct Maven settings to point to the Nexus mirror.

```bash
oc new-build openshift/wildfly-100-centos7:latest~https://github.com/openshift/jee-ex.git \
	-e MAVEN_MIRROR_URL='http://nexus.<Nexus_Project>:8081/content/groups/public/'
oc logs build/jee-ex-1 --follow
```

Replace <Nexus_Project> with the project name of the Nexus repository.  If it is
in the same project as the application that is using it, you can remove the
'<Nexus_Project>.'.  More information on DNS resolution in OpenShift can be
found
[here](https://docs.openshift.org/latest/architecture/additional_concepts/networking.html).

#### Confirming Success

In your web browser, navigate to
`http://<Nexus IP>:8081/content/groups/public` to confirm that it has stored
your application's dependencies. You can also check the build logs to see if
Maven is using the Nexus mirror. If successful, you should see output
referencing the URL `http://nexus:8081`.

## See Also 
* [Networking Concepts in OpenShift](https://docs.openshift.org/latest/architecture/additional_concepts/networking.html)
* [Managing Volumes in OpenShift](https://docs.openshift.org/latest/dev_guide/volumes.html)
* [Improving Build Time of Java Builds on OpenShift](https://blog.openshift.com/improving-build-time-java-builds-openshift/)
* [Application Health](https://docs.openshift.org/latest/dev_guide/application_health.html)
* [Maven "Getting Started"](https://maven.apache.org/guides/getting-started/index.html)
* [Nexus Repository Documentation](https://books.sonatype.com/nexus-book/reference/index.html)
* [Adding Repositories to Nexus](https://books.sonatype.com/nexus-book/reference/config-sect-new-repo.html)
