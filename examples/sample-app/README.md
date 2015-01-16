OpenShift 3 Application Lifecycle Sample
========================================

This is a set of configuration files and scripts which work with OpenShift 3 to create a new application and perform application builds.

This example assumes you have successfully built the `openshift` binary executable and have Docker installed/working.  See https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc.

Alternatively, if you are using the openshift/origin Docker container, please
make sure you follow these instructions first:
https://github.com/openshift/origin/blob/master/examples/sample-app/container-setup.md

Setup
-----
At this stage of OpenShift 3 development, there are a few things that you will need to configure on the host where OpenShift is running in order for things to work.

- - -

**NOTE:** You do not need to do this is you are using [Vagrant](https://vagrantup.com/) to work with OpenShift (see the [Vagrantfile](https://github.com/openshift/origin/blob/master/Vagrantfile) for more info). These changes are only necessary when you have set up the host system yourself. If you are using Vagrant, [jump to the next section](#application-build-deploy-and-update-flow).

- - -

### Docker Changes ###

First, you'll need to configure the docker daemon on your host to trust the docker registry service you'll be starting.

To do this, you need to add "--insecure-registry 172.30.17.0/24" to the docker daemon invocation, eg:

    $ docker -d --insecure-registry 172.30.17.0/24

If you are running docker as a service via `systemd`, you can add this argument to the options value in `/etc/sysconfig/docker`

This will instruct the docker daemon to trust any docker registry on the 172.30.17.0/24 subnet, rather than requiring a certificate.

These instructions assume you have not changed the kubernetes/openshift service subnet configuration from the default value of 172.30.17.0/24.

### SELinux Changes ###

Presently the OpenShift 3 policies for SELinux are a work in progress. For the time being, to play around with the OpenShift system, it is easiest to temporarily disable SELinux:

    $ sudo setenforce 0

This can be re-enabled after you are done with the sample app:

    $ sudo setenforce 1

### FirewallD Changes ###

Similar to our work on SELinux policies, the OpenShift firewalld rules are also a work in progress. For now it is easiest to disable firewalld altogether:

    $ sudo systemctl stop firewalld

Firewalld will start again on your next reboot, but you can manually restart it with this command when you are done with the sample app:

    $ sudo systemctl start firewalld

### Still Having Trouble? ###

If you hit any snags while taking the sample app for a spin, check out the [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md).

Application Build, Deploy, and Update Flow
------------------------------------------

This section covers how to perform all the steps of building, deploying, and updating an application on the OpenShift platform.

All commands assume the `openshift` binary is in your path (normally located under origin/_output/local/go/bin):

1. Pre-pull the docker images used in this sample.  This is not strictly necessary as OpenShift will pull the images as it needs them, but by doing it up front it will prevent lengthy operations during build and deployment which might otherwise lead you to believe the process has failed or hung.

        $ ./pullimages.sh

2. Launch `openshift`

        $ sudo openshift start --cors-allowed-origins=[host machine ip] &> logs/openshift.log &

    Note: sudo is required so the kubernetenes proxy can manipulate iptables rules to expose service ports.

3. Deploy the private docker registry within OpenShift:

        $ openshift cli apply -f docker-registry-config.json

4. Confirm the registry is started (this can take a few mins):

        $ openshift cli get pods

    You should see:

        Name                                   Image(s)                    Host                     Labels                                                                                                   Status
        ----------                             ----------                  ----------               ----------                                                                                               ----------
        94679170-54dc-11e4-88cc-3c970e3bf0b7   openshift/docker-registry   localhost.localdomain/   deployment=registry-config,name=registrypod,replicationController=946583f6-54dc-11e4-88cc-3c970e3bf0b7   Running

5. Confirm the registry service is running.  Note that the actual IP address may vary.

        $ openshift cli get services

    You should see:

        Name                Labels              Selector            IP                  Port
        ----------          ----------          ----------          ----------          ----------
        docker-registry                         name=registrypod    172.30.17.3        5001

6. Confirm the registry is accessible (you may need to run this more than once):

        $ curl `openshift cli get services docker-registry -o template --template="{{ .portalIP}}:{{ .port }}"`

    You should see:

        "docker-registry server (dev) (v0.9.0)"


7. Create a new project in OpenShift

        $ openshift cli create Project -f project.json

8. *Optional:* View the OpenShift web console in your browser by browsing to `http://[host machine ip]:8081`
    If you click the `Hello OpenShift` project and leave the tab open, you'll see the page update as you deploy objects into the project and run builds.

9. Fork the [ruby sample repository](https://github.com/openshift/ruby-hello-world)

10. *Optional:* Add the following webhook to your new github repository:

        $ http://<host>:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/github?namespace=hello-openshift-project
  * Note: Using the webhook requires your OpenShift server be publicly accessible so github can reach it to invoke the hook.

11. Edit application-template-stibuild.json
 * Update the BuildConfig's sourceURI (git://github.com/openshift/ruby-hello-world.git) to point to your forked repository.
 * Replace occurences of `172.30.17.3` with the IP address of the docker-registry service as seen in step 5.

 *Note:* You can skip this step if your registry service ip is 172.30.17.3, which should normally be the case.

12. Submit the application template for processing and create the application using the processed template:

        $ openshift cli process -f application-template-stibuild.json | openshift cli apply --namespace=hello-openshift-project -f -

13. Trigger an initial build of your application
 * If you setup the github webhook in step 10, push a change to app.rb in your ruby sample repository from step 9.
 * Otherwise you can simulate the webhook invocation by running:

            $ curl -X POST http://localhost:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=hello-openshift-project

14. Monitor the builds and wait for the status to go to "complete" (this can take a few mins):

        $ openshift cli get builds --namespace=hello-openshift-project

    Sample output:

        Name                                   Status              Pod Name
        ----------                             ----------          ----------
        20f54507-3dcd-11e4-984b-3c970e3bf0b7   complete            build-docker-20f54507-3dcd-11e4-984b-3c970e3bf0b7

     The build will be automatically pushed to the private docker registry running in OpenShift and tagged with the imageTag listed
     in the buildcfg.json.  Note that the private docker registry is using ephemeral storage, so when it is stopped, the image will
     be lost.  An external volume can be used for storage, but is beyond the scope of this tutorial.

     If you want to see the build logs of a complete build, use this command (substituting your build id from the "openshift cli get builds" output):

         $ openshift cli build-logs 20f54507-3dcd-11e4-984b-3c970e3bf0b7 --namespace=hello-openshift-project

    The creation of the new image will automatically trigger a deployment of the application.

15. Wait for the application's frontend pod and database pods to be started (this can take a few mins):

        $ openshift cli get pods --namespace=hello-openshift-project

    Sample output:

        Name                                                Image(s)                                                                                                          Host                     Labels                                                                                                                                                       Status
        ----------                                          ----------                                                                                                        ----------               ----------                                                                                                                                                   ----------
        1b978f62-605f-11e4-b0db-3c970e3bf0b7                mysql                                                                                                             localhost.localdomain/   deploymentConfig=,deploymentID=database,name=database,replicationController=1b960e56-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample             Running
        4a792f55-605f-11e4-b0db-3c970e3bf0b7                172.30.17.3:5001/openshift/origin-ruby-sample:9477bdb99a409b9c747e699361ae7934fd83bb4092627e2ee35f9f0b0869885b   localhost.localdomain/   deploymentConfig=frontend,deploymentID=frontend-1,name=frontend,replicationController=4a749831-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample   Running

16. Determine the IP for the frontend service:

        $ openshift cli get services --namespace=hello-openshift-project

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434
        docker-registry                                       name=registrypod    172.30.17.3        5001
        frontend            template=ruby-helloworld-sample   name=frontend       172.30.17.4        5432


    In this case, the IP for frontend is 172.30.17.4 and it is on port 5432.

    *Note:* you can also get this information from the web console if you launched it in step 8.

17. Confirm the application is now accessible via the frontend service on port 5432.  Go to http://172.30.17.4:5432 (or whatever IP address was reported above) in your browser.

    You should see a welcome page and a form that allows you to query and update key/value pairs.  The keys are stored in the database container running in the database pod.

18. Make a change to your ruby sample main.html file and push it.

 * If you do not have the webhook enabled, you'll have to manually trigger another build:

            $ curl -X POST http://localhost:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=hello-openshift-project


19. Repeat step 14 (waiting for the build to complete).  Once the build is complete, refreshing your browser should show your changes.

Congratulations, you've successfully deployed and updated an application on OpenShift.  

In addition to creating resources, you can delete resources based on IDs. For example, if you want to remove only the containers or services created during the demo:

  - List the existing services:

        $ openshift cli get services --namespace=hello-openshift-project

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        docker-registry                                       name=registrypod    172.30.17.3        5001
        frontend            template=ruby-helloworld-sample   name=frontend       172.30.17.4        5432
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434


  - To remove the **frontend** service use the command:

        $ openshift cli delete service frontend --namespace=hello-openshift-project

    Sample output:

        Status
        ----------
        Success

  - Check the service was removed:

        $ openshift cli get services --namespace=hello-openshift-project

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

        $ openshift cli get pods --namespace=hello-openshift-project

    Sample output:

        Name                                                Image(s)                                                                                                          Host                     Labels                                                                                                                                                       Status
        ----------                                          ----------                                                                                                        ----------               ----------                                                                                                                                                   ----------
        b8f087b7-605e-11e4-b0db-3c970e3bf0b7                openshift/docker-registry                                                                                         localhost.localdomain/   name=registrypod,replicationController=docker-registry                                                                                                       Running
        1b978f62-605f-11e4-b0db-3c970e3bf0b7                mysql                                                                                                             localhost.localdomain/   deploymentConfig=,deploymentID=database,name=database,replicationController=1b960e56-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample             Running
        4a792f55-605f-11e4-b0db-3c970e3bf0b7                172.30.17.3:5001/openshift/origin-ruby-sample:9477bdb99a409b9c747e699361ae7934fd83bb4092627e2ee35f9f0b0869885b   localhost.localdomain/   deploymentConfig=frontend,deploymentID=frontend-1,name=frontend,replicationController=4a749831-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample   Running

  - Delete the **frontend** pod by specifying its ID:

        $ openshift cli delete pod 4a792f55-605f-11e4-b0db-3c970e3bf0b7 --namespace=hello-openshift-project

  - Verify that the pod has been removed by listing the available pods. This also stopped the associated Docker container, you can check using the command:

        $ docker ps -a

    Sample output:

        CONTAINER ID        IMAGE                                                COMMAND                CREATED              STATUS                          PORTS               NAMES
        068ffffa9624        127.0.0.1:5001/openshift/origin-ruby-sample:latest   "ruby /tmp/app.rb"     3 minutes ago        Exited (0) About a minute ago                       k8s_ruby-helloworld


To clean up all of your environment, you can run the script:

        $ sudo ./cleanup.sh

This will stop the `openshift` process, remove the etcd storage, and kill all Docker containers running on your host system.  The cleanup script needs root privileges to be able to remove all the directories OpenShift created.  (**Use with caution!**   Docker containers unrelated to OpenShift will also be killed by this script)
