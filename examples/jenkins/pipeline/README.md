# Using Jenkins Pipelines with OpenShift

This set of files will allow you to deploy a Jenkins server that is capable of executing Jenkins pipelines and
utilize pods run on OpenShift as Jenkins slaves.

## Basic Pipeline

To walk through the example:

0. If using `oc cluster up`, be sure to grab the [latest oc command](https://github.com/openshift/origin/releases/latest)

1. Stand up an openshift cluster from origin master, installing the standard imagestreams to the openshift namespace:

        $ oc cluster up

    If you do not use oc cluster up, ensure the imagestreams are registered in the openshift namespace, as well as the
jenkins template represented by jenkinstemplate.json by running these commands as a cluster admin:

        $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/image-streams/image-streams-centos7.json -n openshift
        $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/jenkins-ephemeral-template.json -n openshift

    Note: If you have persistent volumes available in your cluster and prefer to use persistent storage (recommended) for your Jenkins server, register the jenkins-persistent-template.json file as well:

        $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/jenkins-persistent-template.json -n openshift

2. Login as a normal user (any user name is fine)

        $ oc login

3. Create a project for your user named "pipelineproject"

        $ oc new-project pipelineproject

4. Run this command to instantiate the template which will create a pipeline buildconfig and some other resources in your project:

    If you used cluster up:

        $ oc new-app jenkins-pipeline-example

    Otherwise:

        $ oc new-app -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/pipeline/samplepipeline.yaml

    At this point if you run `oc get pods` you should see a jenkins pod, or at least a jenkins-deploy pod. (along with other items in your project)  This pod was created as a result of the new pipeline buildconfig being defined by the sample-pipeline template.

5. View/Manage Jenkins (optional)

    You should not need to access the jenkins console for anything, but if you want to configure settings or watch the execution,
    here are the steps to do so:

    If you have a router running (`oc cluster up` provides one), run:

        $ oc get route

    and access the host for the Jenkins route.

    If you do not have a router, or your host system does not support nip.io name resolution you can access jenkins directly via the service ip.  Determine the jenkins service ip ("oc get svc") and go to it in your browser on port 80.  Do not confuse it with the jenkins-jnlp service.
    If you take this approach, run the following command before attempting to log into Jenkins:

        $ oc annotate sa/jenkins serviceaccounts.openshift.io/oauth-redirecturi.1=http://<jenkins_service_ip:jenkins_service_port>/securityRealm/finishLogin --overwrite

    Only include the port in the uri if it is not port 80.

    Login with the user name used to create the "pipelineproject" and any non-empty password.

6. Launch a new build

        $ oc start-build sample-pipeline

    Jenkins will: create an instance of the sample-pipeline job, launch a slave, trigger a build in openshift, trigger a deployment in openshift, and tear the slave down.

    If you monitor the pods in your default project, you will also see the slave pod get created and deleted.

## Maven Slave Example

The `maven-pipeline.yaml` template contains a pipeline that uses a maven node to build and package a WAR.
It then builds an image with the WAR using a Docker-strategy OpenShift build.

To run this example:

1. Ensure that you have a running OpenShift environment as described in the basic example
2. Create a new project for your pipeline on the OpenShift web console:
   1. Login 
   2. Click on *New Project*
   3. Enter a project name
   4. Click *Create*
3. In the *Add to Project* page, click on *Import YAML/JSON*
4. In a separate browser tab, navigate to [maven-pipeline.yaml](https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/pipeline/maven-pipeline.yaml) and copy its content.
5. Paste the YAML text in the text box of the *Import YAML/JSON* tab.
6. Click on *Create*
7. Leave *Process the template* checked and click on *Continue*
8. Modify the URL and Reference of the code repository if you have created your own fork.
9. Click on *Create*
10. Navigate to *Builds* -> *Pipelines*
11. Click on *Start Pipeline* next to *openshift-jee-sample*

On the first pipeline run, there will be a delay as Jenkins is instantiated for the project. 
When the pipeline completes, the openshift-jee-sample application should be deployed and running.

## Orchestration Pipeline Example

The `mapsapp-pipeline.yaml` template contains a pipeline that instantiates other pipelines and runs them.
It shows how more than one pipeline can be launched in parallel and how a single Jenkins pipeline
can work with multiple source code repositories.

To run this example:

1. Ensure that you have a running OpenShift environment as described in the basic example
2. Create a new project for your pipeline on the OpenShift web console:
   1. Login 
   2. Click on *New Project*
   3. Enter a project name
   4. Click *Create*
3. In the *Add to Project* page, click on *Import YAML/JSON*
4. In a separate browser tab, navigate to [mapsapp-pipeline.yaml](https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/pipeline/mapsapp-pipeline.yaml) and copy its content.
5. Paste the YAML text in the text box of the *Import YAML/JSON* tab.
6. Click on *Create*
7. Leave *Process the template* checked and click on *Continue*
8. Modify the URLs and References of the sample repositories if you have created your own forks.
9. Click on *Create*
10. Navigate to *Builds* -> *Pipelines*
11. Click on *Start Pipeline* next to *mapsapp-pipeline*

On the first pipeline run, there will be a delay as Jenkins is instantiated for the project. The pipeline will instantiate 
other pipelines and those will in turn instantiate OpenShift objects. Once the pipeline has completed, a maps frontend 
should be running with 2 backends: nationalparks and mlbparks.


## Blue Green Deployment Example

The `bluegreen-pipeline.yaml` template contains a pipeline that demonstrates alternating blue/green 
deployments with a manual approval step. The template contains three routes, one main route, and 2
other routes; one prefixed by `blue` and the other one prefixed by `green`. Each time the pipeline
is run, it will alternate between building the green or the blue service. You can verify the running
code by browsing to the route that was just built. Once the deployment is approved, then the service
that was just built becomes the *active* one.

To run this example:

1. Create a fork of https://github.com/openshift/nodejs-ex.git
2. Create a new project for your pipeline on the OpenShift web console:
   1. Login 
   2. Click on *New Project*
   3. Enter a project name
   4. Click *Create*
3. In the *Add to Project* page, click on *Import YAML/JSON*
4. In a separate browser tab, navigate to [bluegreen-pipeline.yaml](https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/pipeline/bluegreen-pipeline.yaml) and copy its content.
5. Paste the YAML text in the text box of the *Import YAML/JSON* tab.
6. Click on *Create*
7. Leave *Process the template* checked and click on *Continue*
8. Modify the *Git Repository URL* to contain the URL of your fork
9. Click on *Create*
10. Navigate to *Builds* -> *Pipelines*
11. Click on *Start Pipeline* next to *bluegreen-pipeline*
12. Once the code has been deployed, the pipeline will pause for your approval. Click on the pause icon to approve the deployment of the changes.
13. Push a change to your fork of the nodejs-ex repository
14. Start the pipeline again. Go back to step 11 and repeat.

On the first pipeline run, there will be a delay as Jenkins is instantiated for the project. 

## OpenShift Client Plugin Example

The `openshift-client-plugin-pipeline.yaml` build config references a pipeline that showcases the fluent Jenkins pipeline syntax provided by the
OpenShift Client Plugin.  The DSL provided by this plugin allows for rich interactions with an OpenShift API Server from Jenkins pipelines.  At this
time, it is only available with the OpenShift Jenkins Images for Centos (docker.io/openshift/jenkins-1-centos7:latest and docker.io/openshift/jenkins-2-centos7:latest).

See [the plugin's README](https://github.com/openshift/jenkins-client-plugin) for details on the syntax and features.

This example leverages a [a sample Jenkins pipeline](https://github.com/openshift/jenkins-client-plugin/blob/master/examples/jenkins-image-sample.groovy) defined
in the plugin's source repository.

To run this example:

1. Ensure that you have a running OpenShift environment as described in the basic example

2. Run this command to create a pipeline buildconfig in your project:

        $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/pipeline/openshift-client-plugin-pipeline.yaml

    At this point if you run `oc get pods` you should see a jenkins pod, or at least a jenkins-deploy pod. This pod was created as a result of the new pipeline buildconfig being defined.

On the first pipeline run, there will be a delay as Jenkins is instantiated for the project. 

3. Launch a new build

        $ oc start-build sample-pipeline-openshift-client-plugin

    Jenkins will: create an instance of the sample-pipeline-openshift-client-plugin job, and trigger various builds and deployments in openshift.

