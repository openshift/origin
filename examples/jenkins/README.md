OpenShift 3 Jenkins Example
=========================
This sample walks through the process of starting up an OpenShift cluster and deploying a Jenkins Pod in it.
It also configures a simple application and then creates a Jenkins job to trigger a build of that application.

The Jenkins job will trigger OpenShift to build+deploy a test version of the application, validate that
the deployment works, and then tag the test version into production.

Steps
-----

1. Unless you have built OpenShift locally, be sure  to grab the [latest oc command](https://github.com/openshift/origin/releases/latest)

1. Stand up an openshift cluster from origin master, installing the standard imagestreams to the openshift namespace:

        $ oc cluster up

1. Login as a normal user (any non-empty user name and password is fine)

        $ oc login

1. Create a project  named "test"

        $ oc new-project test

1. Run this command to instantiate a Jenkins server and service account in your project:

    If your have persistent volumes available in your cluster:

        $ oc new-app jenkins-persistent

    Otherwise:

        $ oc new-app jenkins-ephemeral

    **Note**: This template uses an EmptyDir type volume.  If you want to ensure your jenkins configuration/job information is persisted through pod restarts and deployments, you can use the jenkins-persistent-template.json template file which uses a persistent volume but requires additional [PersistentVolume](https://docs.openshift.org/latest/admin_guide/persistent_storage_nfs.html) setup.  
    
1. Create the sample application configuration

        $ oc new-app -f https://raw.githubusercontent.com/openshift/origin/master/examples/jenkins/application-template.json

1. View/Manage Jenkins

    If you have a router running (`oc cluster up` provides one), run:

        $ oc get route

    and access the host for the Jenkins route.

    If you do not have a router or your host system does not support xip.io name resolution, you can access jenkins directly via the service ip.  Determine the jenkins service ip ("oc get svc") and go to it in your browser on port 80.  Do not confuse it with the jenkins-jnlp service.

    **Note**: The OpenShift Login plugin by default manages authentication into any Jenkins instance running in OpenShift.  When this is the case, and you do intend to access Jenkins via the Service IP and not the Route, then you will need to annotate the Jenkins service account with a redirect URL so that the OAuth server's whitelist is updated and allow the login to Jenkins to complete. 

        $ oc annotate sa/jenkins serviceaccounts.openshift.io/oauth-redirecturi.1=http://<jenkins_service_ip:jenkins_service_port>/securityRealm/finishLogin --overwrite
 
    Login with the user name you supplied to `oc login` and any non-empty password.

1. In the Jenkins console, select the the `OpenShift Sample` job and click `Configure`.  You'll see a series of Jenkins build steps defined.  These build steps are from the Jenkins plugin for V3 Openshift.  Read about the [OpenShift Jenkins plugin](https://github.com/openshift/jenkins-plugin) for details on the various functionality provided.  The default values for each of the various build steps listed for the sample job should work as is.  You can save your changes to the job, click `Build` and skip to the "Watch the job output" step.

1. Optional (if the default values are no longer applicable based on how your OpenShift environment was constructed): change the settings for each build step as needed.  For example, update the "URL of the OpenShift api endpoint" field with `https://hostname:port` where hostname/ip and port are for your OpenShift api endpoint, or update the "The authorization token for interacting with OpenShift" field with the token value retrieved in step 2.  You can save your changes to the job, click `Build` and skip to the "Watch the job output" step.

1. Optional (if you would like to set the build step fields via Jenkins build parameters): Set any given build step field with the name of the build parameter you will specify.  Then check `This build is parameterized` and add  String parameters, defining those build parameters.  The README for the [OpenShift Jenkins plugin](https://github.com/openshift/jenkins-plugin) has an example for doing this with screenshots.

1. Save your changes to the job and click `Build with Parameters` and then `Build`.

1. Watch the job output

   It will trigger an OpenShift build of the application, wait for the build to result in a deployment,
   confirm the new deployment works, and then tag the image for production.  This tagging will trigger
   another deployment, this time creating/updating the production service.

1. Confirm both the test and production services are available by browsing to both services:

        $ oc get services -n test | grep frontend

Troubleshooting
-----

If you run into difficulties running OpenShift or getting the `OpenShift Sample` job to complete successfully, start by reading through the [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md).

