Jenkins Master->Slave Example
=============================
This sample walks through the process of starting up an OpenShift cluster and deploying a Jenkins Pod in it.
It also leverage the [kubernetes-plugin](https://wiki.jenkins-ci.org/display/JENKINS/Kubernetes+Plugin) to manage
the Jenkins slaves that run as on-demand Pods.

The sample job that this Jenkins example contains will execute an unit test for
a simple Ruby application. The unit test will be executed in a pod managed by
the kubernetes plugin.

Because the Jenkins Slave pod needs a Docker image that connects to Jenkins
when it starts, this sample also provides a template that allows you to convert
any OpenShift S2I image to a valid Jenkins Slave image.

This template is not required if you already have Docker image that has the
script that launches the [slave
agent](https://wiki.jenkins-ci.org/display/JENKINS/Distributed+builds#Distributedbuilds-Launchslaveagentheadlessly)
as an entrypoint. In that case, you have to create the image stream for this
image and set the image stream label `role` to `jenkins-slave`.

The [Jenkins image](https://github.com/openshift/jenkins) includes the
Kubernetes plugin, so it can manage slave pods by default. This tutorial
includes a template which further customizes the Jenkins image by adding the Git
plugin and define the sample job used by the tutorial.

Steps
------
Before you begin, ensure you have created the [default imagestreams](https://docs.openshift.org/latest/install_config/imagestreams_templates.html#creating-image-streams-for-openshift-images) in the openshift namespace.

1. Create new OpenShift project, where the Jenkins server will run:
  ```
  $ oc new-project ci
  ```

2. Give the Jenkins Pod [service account](https://docs.openshift.org/latest/admin_guide/service_accounts.html)
   rights to do API calls to OpenShift.  This allows us to do the Jenkins Slave
   image discovery automatically.
  ```
  $ oc policy add-role-to-user edit system:serviceaccount:ci:default -n ci
  ```

3. Create the sample application:
  ```bash
  # Ruby application template
  $ oc new-app https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/application-template.json
  ```
  
4. Install the provided OpenShift templates:
  ```
  # Slave converter (optional):
  $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/master-slave/jenkins-slave-template.json
  
  # Jenkins master template:
  $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/master-slave/jenkins-master-template.json
  ```

5. Now navigate to the OpenShift Web Console and switch to the `ci` project.

6. Click on *Add to project* a select the `jenkins-slave-builder` template. Here
   you can customize the image stream name for the builder image or use
   alternative repository with your own custom entry point script.

7. Click *Create* and navigate to *Browse/Builds*. You should see the build
   running. Once this build finishes, you should have the Jenkins Slave image
   ready to be used.

8. Now click on *Add to project* again and select the `jenkins-master` template.
   Here you can pick an alternative Docker image (by default the OpenShift Jenkins
   Image will be used) or the custom S2I repository with your configuration. For this
   sample, you don't need to change these values.

9. Click *Create* and navigate to the *Overview* page. You should see the
   Jenkins route and the link to a `jenkins-master #1 build`. Once the build
   finish, you can follow the link and navigate to your new Jenkins server.

10. In the Jenkins, you should see the `ruby-hello-world-test` job. When you
   start this job, the [Kubernetes plugin](https://wiki.jenkins-ci.org/display/JENKINS/Kubernetes+Plugin)
   will provision a new Pod and attach it to Jenkins as a slave machine. The
   `ruby-hello-world-test` job has the field *Restrict where this project can be
   run* set to `ruby-22-centos7` which is the label of the pre-configured slave
   image.
```

More details
------------

* To manually tweak the Kubernetes Plugin configuration, go to 'Manage Jenkins'
  screen and then 'Configure System'. The Kubernetes settings should be under
  'Cloud/Kubernetes'.

* **NOTE**: This sample uses ephemeral storage. For production use, consider
  adding a persistent volume for the `/var/lib/jenkins` folder.

* If you already have a Jenkins Slave image, you can simply push the image to
  the OpenShift internal registry, under the `ci/` repository and create the image
  stream for it. Then you just need to set `"role": "jenkins-slave"` label for
  the image stream and re-deploy the Jenkins server.
