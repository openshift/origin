OpenShift 3 Jenkins Example
=========================
This sample walks through the process of login to an OpenShift cluster and deploying a Jenkins Pod in it.
It also configures a simple application and then creates a Jenkins job to trigger a build of that application.

The Jenkins job will trigger OpenShift to build+deploy a test version of the application, validate that
the deployment works, and then tag the test version into production.

Steps
-----

1. Unless you have built OpenShift locally, be sure to grab the [latest oc command](https://github.com/openshift/origin/releases/latest)

1. Login as a normal user

        $ oc login

1. Create a project  named "test"

        $ oc new-project test

1. Run this command to instantiate a Jenkins server and service account in your project:

    If your have persistent volumes available in your cluster:

        $ oc new-app jenkins-persistent

    Otherwise:

        $ oc new-app jenkins-ephemeral

    **Note**: This template uses an EmptyDir type volume.  If you want to ensure your jenkins configuration/job information is persisted through pod restarts and deployments, you can use the jenkins-persistent-template.json template file which uses a persistent volume but requires additional [PersistentVolume](https://docs.openshift.org/latest/install_config/persistent_storage/persistent_storage_nfs.html) setup.  
    
1. View/Manage Jenkins

    If you have a router running, run:

        $ oc get route

    and access the host for the Jenkins route.

    If you do not have a router or your host system does not support nip.io name resolution, you can access jenkins directly via the service ip.  Determine the jenkins service ip ("oc get svc") and go to it in your browser on port 80.  Do not confuse it with the jenkins-jnlp service.

    **Note**: The OpenShift Login plugin by default manages authentication into any Jenkins instance running in OpenShift.  When this is the case, and you do intend to access Jenkins via the Service IP and not the Route, then you will need to annotate the Jenkins service account with a redirect URL so that the OAuth server's whitelist is updated and allow the login to Jenkins to complete. 

        $ oc annotate sa/jenkins serviceaccounts.openshift.io/oauth-redirecturi.1=http://<jenkins_service_ip:jenkins_service_port>/securityRealm/finishLogin --overwrite
 
    Login with the user name you supplied to `oc login` and any non-empty password.


Advanced
-----

A set of example Jenkins pipelines that illustrate the various OpenShift/Jenkins integration features are available in the ['pipeline' subdirectory](pipeline).

Some references:

* [The list of OpenShift/Jenkins integration plugins](https://github.com/openshift/jenkins#plugins-focused-on-integration-with-openshift)

* [Pipeline Build Strategy introduction](https://docs.okd.io/latest/architecture/core_concepts/builds_and_image_streams.html#pipeline-build)

* [Pipeline Build Strategy options](https://docs.okd.io/latest/dev_guide/builds/build_strategies.html#pipeline-strategy-options)

* [How to create Jenkins agent images for OpenShift](https://docs.openshift.org/latest/using_images/other_images/jenkins.html#using-the-jenkins-kubernetes-plug-in-to-run-jobs).  


Troubleshooting
-----

If you run into difficulties running OpenShift or getting the `OpenShift Sample` job to complete successfully, start by reading through the [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md).

Updating
-----

The jenkins-ephemeral and jenkins-persistent templates are sourced from the [jenkins image repository](https://github.com/openshift/jenkins) via the [OpenShift Library](https://github.com/openshift/library), so they should not be directly updated here.  Make changes upstream and then run `make update-examples` to pull in changes.
