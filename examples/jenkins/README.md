OpenShift 3 Jenkins Example
=========================

This sample walks through the process of starting up an OpenShift cluster and deploying a Jenkins Pod in it.
It also configures a simple application and then creates a Jenkins job to trigger a build of that application.

The Jenkins job will trigger OpenShift to build+deploy a test version of the application, validate that
the deployment works, and then tag the test version into production.

Steps
-----

1. Start OpenShift
    
        $ sudo openshift start &> logs/openshift.log &

2. Start the docker registry services

        $ osc create -f docker-registry-config.json

3. Start the Jenkins services

        $ osc create -f jenkins-config.json

4. Determine the IP address of the docker-registry service:

        $ osc get services docker-registry -o template --template="{{ .portalIP }}"
        
5. Edit the application-template.json file by replacing all occurences of `172.30.17.3` with the IP address from the previous step.

5. Create the application configuration

        $ osc process -f application-template.json | osc create -f -
 
6. Locate the Jenkins service endpoint and go to it in your browser:

        $ osc get services | grep jenkins | awk '{print $4":"$5}'

    Once it is available, proceed to the next step.
    
7. Create the Jenkins job named rubyJob:

        $ JENKINS_ENDPOINT=`osc get services | grep jenkins | awk '{print $4":"$5}'`
        $ cat job.xml | curl -X POST -H "Content-Type: application/xml" -H "Expect: " --data-binary @- http://$JENKINS_ENDPOINT/createItem?name=rubyJob

8. Run the Jenkins build
   
    Go back to your browser, refresh and select the rubyJob build job and choose `Build with parameters`. 
    You should not need to modify the `OPENSHIFT_HOST`.

9. Watch the job output

   It will trigger an OpenShift build of the application, wait for the build to result in a deployment,
   confirm the new deployment works, and re-tag the image for production.  This re-tagging will trigger
   another deployment, this time creating/updated the production service.

10. Confirm both the test and production services are available by browsing to both services:

        $ osc get services | grep frontend
   
