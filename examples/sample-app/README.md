OpenShift 3 Application Lifecycle Sample
========================================

This is a set of configuration files and scripts which work with OpenShift 3 to create a new application and perform application builds.

This example assumes you have successfully built the `openshift` binary executable and have Docker installed/working.  See https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc.


Application Build, Deploy, and Update Flow
------------------------------------------

This section covers how to perform all the steps of building, deploying, and updating an application on the OpenShift platform.

All commands assume the `openshift` binary is in your path (normally located under origin/_output/local/go/bin):

1. Pre-pull the docker images used in this sample.  This is not strictly necessary as OpenShift will pull the images as it needs them, but by doing it up front it will prevent lengthy operations during build and deployment which might otherwise lead you to believe the process has failed or hung.

        $ ./pullimages.sh

2. Launch `openshift`

        $ sudo openshift start &> logs/openshift.log &

    Note: sudo is required so the kubernetenes proxy can manipulate iptables rules to expose service ports.

3. Deploy the private docker registry within OpenShift:

        $ openshift kubectl apply -f docker-registry-config.json

4. Confirm the registry is started (this can take a few mins):

        $ openshift kubectl get pods

    You should see:

        Name                                   Image(s)                    Host                     Labels                                                                                                   Status
        ----------                             ----------                  ----------               ----------                                                                                               ----------
        94679170-54dc-11e4-88cc-3c970e3bf0b7   openshift/docker-registry   localhost.localdomain/   deployment=registry-config,name=registrypod,replicationController=946583f6-54dc-11e4-88cc-3c970e3bf0b7   Running

5. Confirm the registry service is running.  Note that the actual IP address will vary.

        $ openshift kubectl get services

    You should see:

        Name                Labels              Selector            IP                  Port
        ----------          ----------          ----------          ----------          ----------
        docker-registry                         name=registrypod    172.30.17.3        5001

6. Confirm the registry is accessible (you may need to run this more than once):

        $ curl `openshift kubectl get services docker-registry -o template --template="{{ .portalIP}}:{{ .port }}"`

    You should see:

        "docker-registry server (dev) (v0.9.0)"


7. Fork the [ruby sample repository](https://github.com/openshift/ruby-hello-world)

8. *Optional:* Add the following webhook to your new github repository:

        $ http://<host>:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/github
  * Note: Using the webhook requires your OpenShift server be publicly accessible so github can reach it to invoke the hook.

9. Edit application-template-stibuild.json
 * Update the BuildConfig's sourceURI (git://github.com/openshift/ruby-hello-world.git) to point to your forked repository.
 * Replace occurences of `172.30.17.3` with the IP address of the docker-registry service as seen in step 5.

10. Submit the application template for processing and create the application using the processed template:

        $ openshift kubectl process -f application-template-stibuild.json | openshift kubectl apply -f -

11. Trigger an initial build of your application
 * If you setup the github webhook in step 8, push a change to app.rb in your ruby sample repository from step 7.
 * Otherwise you can simulate the webhook invocation by running:

            $ curl -X POST http://localhost:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic

12. Monitor the builds and wait for the status to go to "complete" (this can take a few mins):

        $ openshift kubectl get builds

    Sample output:

        Name                                   Status              Pod Name
        ----------                             ----------          ----------
        20f54507-3dcd-11e4-984b-3c970e3bf0b7   complete            build-docker-20f54507-3dcd-11e4-984b-3c970e3bf0b7

     The build will be automatically pushed to the private docker registry running in OpenShift and tagged with the imageTag listed
     in the buildcfg.json.  Note that the private docker registry is using ephemeral storage, so when it is stopped, the image will
     be lost.  An external volume can be used for storage, but is beyond the scope of this tutorial.

     If you want to see the build logs of a complete build, use this command (substituting your build id from the "openshift kubectl get builds" output):

         $ openshift kubectl build-logs 20f54507-3dcd-11e4-984b-3c970e3bf0b7

    The creation of the new image will automatically trigger a deployment of the application.

13. Wait for the application's frontend pod and database pods to be started (this can take a few mins):

        $ openshift kubectl get pods

    Sample output:

        Name                                                Image(s)                                                                                                          Host                     Labels                                                                                                                                                       Status
        ----------                                          ----------                                                                                                        ----------               ----------                                                                                                                                                   ----------
        b8f087b7-605e-11e4-b0db-3c970e3bf0b7                openshift/docker-registry                                                                                         localhost.localdomain/   name=registrypod,replicationController=docker-registry                                                                                                       Running
        1b978f62-605f-11e4-b0db-3c970e3bf0b7                mysql                                                                                                             localhost.localdomain/   deploymentConfig=,deploymentID=database,name=database,replicationController=1b960e56-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample             Running
        4a792f55-605f-11e4-b0db-3c970e3bf0b7                172.30.17.3:5001/openshift/origin-ruby-sample:9477bdb99a409b9c747e699361ae7934fd83bb4092627e2ee35f9f0b0869885b   localhost.localdomain/   deploymentConfig=frontend,deploymentID=frontend-1,name=frontend,replicationController=4a749831-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample   Running

14. Determine the IP for the frontend service:

        $ openshift kubectl get services

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434
        docker-registry                                       name=registrypod    172.30.17.3        5001
        frontend            template=ruby-helloworld-sample   name=frontend       172.30.17.4        5432


    In this case, the IP for frontend is 172.30.17.4 and it is on port 5432.

15. Confirm the application is now accessible via the frontend service on port 5432.  Go to http://172.30.17.4:5432 (or whatever IP address was reported above) in your browser.

You should see a welcome page and a form that allows you to query and update key/value pairs.  The keys are stored in the database container running in the database pod.

16. Make a change to your ruby sample main.html file and push it.
 * If you do not have the webhook enabled, you'll have to manually trigger another build:

            $ curl -X POST http://localhost:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic

17. Repeat step 12 (waiting for the build to complete).  Once the build is complete, refreshing your browser should show your changes.

Congratulations, you've successfully deployed and updated an application on OpenShift.  

In addition to creating resources, you can delete resources based on IDs. For example, if you want to remove only the containers or services created during the demo:

  - List the existing services:

        $ openshift kubectl get services

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        docker-registry                                       name=registrypod    172.30.17.3        5001
        frontend            template=ruby-helloworld-sample   name=frontend       172.30.17.4        5432
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434


  - To remove the **frontend** service use the command:

        $ openshift kubectl delete service frontend

    Sample output:

        Status
        ----------
        Success

  - Check the service was removed:

        $ openshift kubectl get services

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        docker-registry                                       name=registrypod    172.30.17.3        5001
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434


  - You can also curl the application to check the service has terminated:

        $ curl http://172.17.17.4:5432

    Sample output:

        curl: (7) Failed connect to 172.17.17.4:5432; No route to host

Another interesting example is deleting a pod.

  - List available pods:

        $ openshift kubectl get pods

    Sample output:

        Name                                                Image(s)                                                                                                          Host                     Labels                                                                                                                                                       Status
        ----------                                          ----------                                                                                                        ----------               ----------                                                                                                                                                   ----------
        b8f087b7-605e-11e4-b0db-3c970e3bf0b7                openshift/docker-registry                                                                                         localhost.localdomain/   name=registrypod,replicationController=docker-registry                                                                                                       Running
        1b978f62-605f-11e4-b0db-3c970e3bf0b7                mysql                                                                                                             localhost.localdomain/   deploymentConfig=,deploymentID=database,name=database,replicationController=1b960e56-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample             Running
        4a792f55-605f-11e4-b0db-3c970e3bf0b7                172.30.17.3:5001/openshift/origin-ruby-sample:9477bdb99a409b9c747e699361ae7934fd83bb4092627e2ee35f9f0b0869885b   localhost.localdomain/   deploymentConfig=frontend,deploymentID=frontend-1,name=frontend,replicationController=4a749831-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample   Running

  - Delete the **frontend** pod by specifying its ID:

        $ openshift kubectl delete pod 4a792f55-605f-11e4-b0db-3c970e3bf0b7

  - Verify that the pod has been removed by listing the available pods. This also stopped the associated Docker container, you can check using the command:

        $ docker ps -a

    Sample output:

        CONTAINER ID        IMAGE                                                COMMAND                CREATED              STATUS                          PORTS               NAMES
        068ffffa9624        127.0.0.1:5001/openshift/origin-ruby-sample:latest   "ruby /tmp/app.rb"     3 minutes ago        Exited (0) About a minute ago                       k8s_ruby-helloworld


To clean up all of your environment, you can run the script:

        $ sudo ./cleanup.sh

This will stop the `openshift` process, remove the etcd storage, and kill all Docker containers running on your host system.  The cleanup script needs root privileges to be able to remove all the directories OpenShift created.  (**Use with caution!**   Docker containers unrelated to OpenShift will also be killed by this script)
