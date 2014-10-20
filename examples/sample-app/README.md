OpenShift 3 Application Lifecycle Sample
========================================

This is a set of configuration files and scripts which work with Openshift 3 to create a new application and perform application builds.

This example assumes you have successfully built the `openshift` binary executable and have Docker installed/working.  See https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc.


Application Build, Deploy, and Update Flow
------------------------------------------

This section covers how to perform all the steps of building, deploying, and updating an application on the OpenShift platform.

Note:  If you just want to quickly validate your environment or see the expected results, you can run the quickstart script described [here](QUICKSTART.md)

All commands assume the `openshift` binary is in your path:

1. Pre-pull the docker images used in this sample.  This is not strictly necessary as OpenShift will pull the images as it needs them, but by doing it up front it will prevent lengthy operations during build and deployment which might otherwise lead you to believe the process has failed or hung.

        $ ./pullimages.sh

2. Launch `openshift`

        $ openshift start &> logs/openshift.log &

3. Deploy the private docker registry within OpenShift:

        $ openshift kube apply -c registry-config.json

4. Confirm the registry is started (this can take a few mins):

        $ openshift kube list pods

    You should see:

        ID                                     Image(s)                    Host                     Labels                                                                                                   Status
        ----------                             ----------                  ----------               ----------                                                                                               ----------
        94679170-54dc-11e4-88cc-3c970e3bf0b7   openshift/docker-registry   localhost.localdomain/   deployment=registry-config,name=registryPod,replicationController=946583f6-54dc-11e4-88cc-3c970e3bf0b7   Running


5. Fork the [ruby sample repository](https://github.com/openshift/ruby-hello-world)

6. *Optional:* Add the following webhook to your new github repository:

        $ http://<host>:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github
  * Note: Using the webhook requires your OpenShift server be publicly accessible so github can reach it to invoke the hook.

7. Edit buildcfg/buildcfg.json
 * Update the sourceURI to point to your forked repository.

8. Create a build configuration for your application.  This configuration is used by OpenShift to rebuild your application's Docker image (e.g. when you push changes to the application source).

        $ openshift kube create buildConfigs -c buildcfg/buildcfg.json

    You should see the build configuration returned as output (SourceURI will depend on your repo name):

        ID                  Type                SourceURI
        ----------          ----------          ----------
        build100            docker              git://github.com/openshift/ruby-hello-world.git

9. Trigger an initial build of your application
 * If you setup the github webhook in step 6, push a change to app.rb in your ruby sample repository from step 5.
 * Otherwise you can simulate the webhook invocation by running:

            $ curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @buildinvoke/pushevent.json http://localhost:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

    In the OpenShift logs (logs/openshift.log) you should see something like:

        I0916 13:50:22.479529 21375 log.go:134] POST /osapi/v1beta1/buildConfigHooks/build100/secret101/github

    which confirms the webhook was triggered.

10. Monitor the builds and wait for the status to go to "complete" (this can take a few mins):

        $ openshift kube list builds

    Sample output:

        ID                                     Status              Pod ID
        ----------                             ----------          ----------
        20f54507-3dcd-11e4-984b-3c970e3bf0b7   complete            build-docker-20f54507-3dcd-11e4-984b-3c970e3bf0b7

     The build will be automatically pushed to the private docker registry running in OpenShift and tagged with the imageTag listed
     in the buildcfg.json.  Note that the private docker registry is using ephemeral storage, so when it is stopped, the image will
     be lost.  An external volume can be used for storage, but is beyond the scope of this tutorial.

11. Submit the application template for processing and create the application using the processed template:

        $ openshift kube process -c template/template.json | openshift kube apply -c -

12. Wait for the application's frontend pod to be started (this can take a few mins):

        $ openshift kube list pods

    Sample output:

        ID                                                  Image(s)                       Host                Labels                                                   Status
        ----------                                          ----------                     ----------          ----------                                               ----------
        fc66bffd-3dcc-11e4-984b-3c970e3bf0b7                openshift/origin-ruby-sample   127.0.0.1/          name=frontend,replicationController=frontendController   Running

13. Confirm the application is now accessible via the frontend service on port 5432:

        $ curl http://localhost:5432

    Sample output:

        Hello World!
        User is adminM5K
        Password is qgRpLNGO
        DB password is dQfUlnTG


14. Make an additional change to your ruby sample app.rb file and push it.
 * If you do not have the webhook enabled, you'll have to manually trigger another build:

            $ curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @buildinvoke/pushevent.json http://localhost:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

15. Repeat steps 9-10

16. Locate the container running the ruby application and kill it:

        $ docker kill `docker ps | grep origin-ruby-sample | awk '{print $1}'`

17. Use 'docker ps' to watch as OpenShift automatically recreates the killed container using the latest version of your image.  Once the container is recreated, curl the application to see the change you made in step 14.

        $ curl http://localhost:5432

Congratulations, you've successfully deployed and updated an application on OpenShift.  To clean up your environment, you can run:

        $ ./cleanup.sh

This will stop the `openshift` process, remove the etcd storage, and kill all Docker containers running on your host system.  (**Use with caution!**   Docker containers unrelated to OpenShift will also be killed by this script)
