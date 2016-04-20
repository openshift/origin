This set of files will allow you to deploy a Jenkins server that is capable of executing Jenkins pipelines and
utilize pods run on OpenShift as Jenkins slaves.

To walk through the example:

1) Stand up an openshift cluster from origin master, installing the standard imagestreams to the openshift namespace:

$ oc cluster up

If you do not use oc cluster up, ensure the imagestreams are registered in the openshift namespace, as well as the
jenkins template represented by jenkinstemplate.json by running these commands as a cluster admin:

$ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/image-streams/image-streams-centos7.json -n openshift
$ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/pipeline/jenkinstemplate.json -n openshift

2) login as a normal user (any username is fine)

$ oc login

3) create a project for your user named "pipelineproject"

$ oc new-project pipelineproject

4) run this command to instantiate the template which will create a pipeline buildconfig and 
some other resources in your project:

$ oc new-app -f pipelinetemplate.json

At this point if you run "oc get pods" you should see a jenkins pod, or at least a jenkins-deploy pod. (along with other items
in your project)  This pod was created as a result of the new pipeline buildconfig being defined.

Note: this template grants the edit role to the default service account for your project so jenkins can manipulate your project.  
If you do not use this template to create your pipeline buildconfig, you need to grant edit permission to the default service
account:

$ oc policy add-role-to-user edit -z default -n pipelineproject

5) View/Manage jenkins
If you have a router running (oc cluster up provides one), run:

$ oc get route

and access the host for the jenkins route.

If you do not have a router, you can access jenkins directly via the service ip.  Determine the jenkins service ip ("oc get svc") 
and go to it in your browser on port 80.  Do not confuse it with the jenkins-jnlp service.

The login/password are admin/password.

Go to Manage Jenkins -> Configure System

Under the kubernetes plugin in the Cloud section, change the Kubernetes Namespace to "pipelineproject".

This is also a good time to approve the Jenkinsfile that will be executed:

Go to Manage Jenkins -> In-process Script Approval

Click approve on the waiting approval.

We're working on the script approval requirement:
https://wiki.jenkins-ci.org/display/JENKINS/Script+Security+Plugin
An administrator may now go to Manage Jenkins » In-process Script Approval where a list of scripts pending approval will be shown. Assuming nothing dangerous-looking is being requested, just click Approve to let the script be run henceforth.

6) Launch a new build

$ oc start-build sample-pipeline

Jenkins will: create an instance of the sample-pipeline job, launch a slave, trigger a build in openshift, trigger a
deployment in openshift, and tear the slave down.

(If you monitor the pods in your default project, you will also see the slave pod get created and deleted).
