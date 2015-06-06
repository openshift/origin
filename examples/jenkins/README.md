OpenShift 3 Jenkins Example
=========================

This sample walks through the process of starting up an OpenShift cluster and deploying a Jenkins Pod in it.
It also configures a simple application and then creates a Jenkins job to trigger a build of that application.

The Jenkins job will trigger OpenShift to build+deploy a test version of the application, validate that
the deployment works, and then tag the test version into production.

Steps
-----

1. Follow steps 1-10 from the [sample-app](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md)

2. Start the Jenkins services

        $ oc create -n test -f jenkins-config.json
        
3. Create the application configuration

        $ oc process -n test -f application-template.json | oc create -n test -f -
 
4. Locate the Jenkins service endpoint and go to it in your browser:

        $ oc get services -n test jenkins --template="{{ .spec.portalIP }}:{{ with index .spec.ports 0 }}{{ .port }}{{ end }}"

    Once it is available, proceed to the next step.
    
5. Create the Jenkins job named rubyJob:

        $ JENKINS_ENDPOINT=`oc get services -n test jenkins --template="{{ .spec.portalIP }}:{{ with index .spec.ports 0 }}{{ .port }}{{ end }}"`
        $ cat job.xml | curl -X POST -H "Content-Type: application/xml" -H "Expect: " --data-binary @- http://$JENKINS_ENDPOINT/createItem?name=rubyJob

6. Add API credentials to the Jenkins build:

    1. Go back to your browser, refresh and select the rubyJob build job.
    2. Choose `Configure`.
    3. Locate the KUBECONFIG_CREDENTIALS parameter, and replace the default value with the contents of the `openshift.local.config/master/openshift-client.kubeconfig` file.
    4. Click `Save`.

6. Run the Jenkins build
   
    1. In the browser, select the rubyJob build job and choose `Build with parameters`.
    2. Click `Build` (you should not need to modify any parameters).

7. Watch the job output

   It will trigger an OpenShift build of the application, wait for the build to result in a deployment,
   confirm the new deployment works, and re-tag the image for production.  This re-tagging will trigger
   another deployment, this time creating/updated the production service.

8. Confirm both the test and production services are available by browsing to both services:

        $ oc get services -n test | grep frontend
   
