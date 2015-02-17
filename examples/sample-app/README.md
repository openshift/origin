OpenShift 3 Application Lifecycle Sample
========================================

This is a set of configuration files and scripts which work with OpenShift 3 to create a new application and perform application builds.

This example assumes you have successfully built the `openshift`
binary executable (normally located under origin/\_output/local/go/bin),
you have that and its symlink/copy `osc` in your `PATH` and root's,
and Docker is installed and working.  See
https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc.

Alternatively, if you are using the openshift/origin Docker container, please
make sure you follow these instructions first:
https://github.com/openshift/origin/blob/master/examples/sample-app/container-setup.md

Security Warning
----------------
OpenShift no longer requires SElinux to be disabled, however OpenShift is a system which runs Docker containers on your system.  In some cases (build operations and the registry service) it does so using privileged containers.  Furthermore those containers access your host's Docker daemon and perform `docker build` and `docker push` operations.  As such, you should be aware of the inherent security risks associated with performing `docker run` operations on arbitrary images as they effectively have root access.  This is particularly relevant when running the OpenShift nodes directly on your host system.

For more information, see these articles:

* http://opensource.com/business/14/7/docker-security-selinux
* https://docs.docker.com/articles/security/

The OpenShift security model will continue to evolve and tighten as we head towards production ready code.

Setup
-----
At this stage of OpenShift 3 development, there are a few things that you will need to configure on the host where OpenShift is running in order for things to work.

- - -

**NOTE:** You do not need to do this if you are using [Vagrant](https://vagrantup.com/) to work with OpenShift (see the [Vagrantfile](https://github.com/openshift/origin/blob/master/Vagrantfile) for more info). These changes are only necessary when you have set up the host system yourself. If you are using Vagrant, [jump to the next section](#application-build-deploy-and-update-flow).

- - -

### Docker Changes ###

First, you'll need to configure the docker daemon on your host to trust the docker registry service you'll be starting.

To do this, you need to add "--insecure-registry 172.30.17.0/24" to the docker daemon invocation, eg:

    $ docker -d --insecure-registry 172.30.17.0/24

Note that you need to have installed Docker 1.3.2 or higher in order to use the `--insecure-registry` flag.

If you are running docker as a service via `systemd`, you can add this argument to the options value in `/etc/sysconfig/docker`

This will instruct the docker daemon to trust any docker registry on the 172.30.17.0/24 subnet,
rather than requiring the registry to have a verifiable certificate.

These instructions assume you have not changed the kubernetes/openshift service subnet configuration from the default value of 172.30.17.0/24.

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

NOTE: All commands assume the `osc` binary/symlink is in your path and
the present working directory is the same directory as this README.

1. *Optional*: Pre-pull the docker images used in this sample.  This is
    not strictly necessary as OpenShift will pull the images as it needs them,
    but by doing it up front it will prevent lengthy operations during build
    and deployment which might otherwise lead you to believe the process
    has failed or hung.

        $ ./pullimages.sh

2. Launch an all-in-one `openshift` instance

        $ sudo openshift start &> logs/openshift.log &

    Note: sudo is required so the kubernetes proxy can manipulate iptables rules to expose service ports.

3. Set up your client to reach the OpenShift master now running.

    Since OpenShift services are secured by TLS, your client will
    need to accept the server certificates and present its own client
    certificate. These are generated as part of the `openshift start`
    command in whatever the current directory is at the time. You will
    need to point osc and curl at the appropriate CA bundle and client
    key/cert in order to connect to OpenShift. Assuming you are running as
    a user other than root, you will also need to make the private client
    key readable by that user. (Note: this is just for example purposes;
    in a real installation, users would generate their own keys and not
    have access to the system keys.)

        $ export KUBECONFIG=`pwd`/openshift.local.certificates/admin/.kubeconfig
        $ export CURL_CA_BUNDLE=`pwd`/openshift.local.certificates/admin/root.crt
        $ sudo chmod +r `pwd`/openshift.local.certificates/admin/key.key

4. Deploy a private docker registry within OpenShift with the certs necessary for access to master:

        $ sudo chmod +r ./openshift.local.certificates/master/key.key
        $ pushd ../..
        $ CERT_DIR=examples/sample-app/openshift.local.certificates/master hack/install-registry.sh
        $ popd

    Note that the private docker registry is using ephemeral storage,
    so when it is stopped, the image will be lost. An external volume
    could be used for persistent storage, but that is beyond the scope
    of this tutorial.

5. Confirm the registry is started (this can take a few minutes):

        $ osc get pods

    You should see:

        Name                                   Image(s)                    Host                     Labels                                                                                                   Status
        ----------                             ----------                  ----------               ----------                                                                                               ----------
        94679170-54dc-11e4-88cc-3c970e3bf0b7   openshift/docker-registry   localhost.localdomain/   deployment=registry-config,name=registrypod,replicationController=946583f6-54dc-11e4-88cc-3c970e3bf0b7   Running

6. Confirm the registry service is running.  Note that the actual IP address may vary.

        $ osc get services

    You should see:

        Name                Labels              Selector            IP                  Port
        ----------          ----------          ----------          ----------          ----------
        docker-registry                         name=registrypod    172.30.17.3        5001

7. Confirm the registry is accessible (you may need to run this more than once):

        $ curl `osc get services docker-registry -o template --template="{{ .portalIP}}:{{ .port }}"`

    You should see:

        "docker-registry server (dev) (v0.9.0)"


8. Create a new project in OpenShift. This creates a namespace `test` to contain the builds and app that we will generate below.

        $ osc create -f project.json

9. *Optional:* View the OpenShift web console in your browser by browsing to `https://<host>:8444`

    * You will need to have the browser accept the certificate at
      `https://<host>:8443` before the console can consult the OpenShift
      API. Of course this would not be necessary with a legitimate
      certificate.
    * If you click the `Hello OpenShift` project and leave the tab open,
      you'll see the page update as you deploy objects into the project
      and run builds.

10. *Optional:* Fork the [ruby sample repository](https://github.com/openshift/ruby-hello-world)
    to an OpenShift-visible git account that you control, preferably
    somewhere that can also reach your OpenShift server with a webhook.
    A github.com account is an obvious place for this, but an in-house
    git hosting site may work better for reaching your OpenShift server.

    We will demonstrate building from a repository and then triggering
    a new build from changing that repository. If you do not have an
    account that will work for this purpose, that is fine; just use
    a GitHub account and simulate the webhook (demonstrated below).
    Without your own fork, you can still run the initial build from
    OpenShift's public repository, just not a changed build.

11. *Optional:* Add the following webhook under the settings in your new GitHub repository:

        $ https://<host>:8443/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/github?namespace=test

  * Note: Using the webhook requires that your OpenShift server be
    publicly accessible so GitHub can reach it to invoke the hook. You
    will almost certainly need to "Disable SSL Verification" for your test
    instance as the certificate chain generated is not publicly verified.

12. Edit application-template-stibuild.json which will define the sample application

 * Update the BuildConfig's sourceURI (git://github.com/openshift/ruby-hello-world.git) to point to your forked repository.
   *Note:* You can skip this step if you did not create a forked repository.

13. Submit the application template for processing (generating shared parameters requested in the template)
    and then request creation of the processed template:

        $ osc process -n test -f application-template-stibuild.json | osc create -n test -f -

    This will define a number of related OpenShift entities in the project:

    * A BuildConfig (ruby-sample-build) to specify a build that uses
      your ruby-hello-world fork as the input for a source-to-image (STI) build
    * ImageRepositories for the images used and created in the build:
      * The ruby-20-centos STI builder will build an image from your source
      * The output image will be called origin-ruby-sample
    * DeploymentConfigs (frontend, backend) for defining Deployments once the images are available
    * Services (routable endpoints) for the ruby frontend and database backend deployments
      that will deployed as output of the build

    Note that no build has actually occurred yet, so at this time there
    is no image to deploy and no application to visit.

14. Trigger an initial build of your application
 * If you setup the GitHub webhook, push a change to app.rb in your ruby sample repository from step 10.
 * Otherwise you can request a new build by running:

        $ osc start-build -n test ruby-sample-build

15. Monitor the builds and wait for the status to go to "complete" (this can take a few minutes):

        $ osc get -n test builds

    You can add the --watch flag to wait for updates until the build completes:

        $ osc get -n test builds --watch

    Sample output:

        Name                                   Status              Pod Name
        ----------                             ----------          ----------
        20f54507-3dcd-11e4-984b-3c970e3bf0b7   complete            build-docker-20f54507-3dcd-11e4-984b-3c970e3bf0b7

     The built image will be named with the ImageRepository
     (origin-ruby-sample) named in the BuildConfig and pushed to the
     private Docker registry running in OpenShift.  (Note that the private
     docker registry is using ephemeral storage, so when it is stopped,
     the image will be lost.)

     If you want to see the build logs of a complete build, use this
     command (substituting your build name from the "osc get builds"
     output):

         $ osc build-logs -n test 20f54507-3dcd-11e4-984b-3c970e3bf0b7

    The creation of the new image in the Docker registry will
    automatically trigger a deployment of the application, creating a
    pod each for the frontend (your Ruby code) and backend.

16. Wait for the application's frontend pod and database pods to be started (this can take a few minutes):

        $ osc get -n test pods

    Sample output:

        Name                                                Image(s)                                                                                                          Host                     Labels                                                                                                                                                       Status
        ----------                                          ----------                                                                                                        ----------               ----------                                                                                                                                                   ----------
        1b978f62-605f-11e4-b0db-3c970e3bf0b7                mysql                                                                                                             localhost.localdomain/   deploymentConfig=,deploymentID=database,name=database,replicationController=1b960e56-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample             Running
        4a792f55-605f-11e4-b0db-3c970e3bf0b7                172.30.17.3:5001/openshift/origin-ruby-sample:9477bdb99a409b9c747e699361ae7934fd83bb4092627e2ee35f9f0b0869885b   localhost.localdomain/   deploymentConfig=frontend,deploymentID=frontend-1,name=frontend,replicationController=4a749831-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample   Running

17. Determine the IP for the frontend service:

        $ osc get -n test services

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434
        frontend            template=ruby-helloworld-sample   name=frontend       172.30.17.4        5432


    In this case, the IP for frontend is 172.30.17.4 and it is on port 5432.

    *Note:* you can also get this information from the web console if you launched it in step 9.

18. Confirm the application is now accessible via the frontend service on port 5432.  Go to http://172.30.17.4:5432 (or whatever IP address was reported above) in your browser if you're running this locally; otherwise you can use curl to see the HTML, or port forward the address to your local workstation to visit it.

    You should see a welcome page and a form that allows you to query and update key/value pairs.  The keys are stored in the database container running in the database pod.

19. Make a change to your ruby sample main.html file, commit, and push it via git.

 * If you do not have the webhook enabled, you'll have to manually trigger another build:

            $ osc start-build -n test ruby-sample-build


20. Repeat step 15 (waiting for the build to complete).  Once the build is complete, refreshing your browser should show your changes.

Congratulations, you've successfully deployed and updated an application on OpenShift.

Advanced
---------
OpenShift also provides features that live outside the deployment life cycle like routing.  

1.  Your sample app has been created with a secure route which can be viewed by performing a `GET` on the route api object.

            $ osc get -n test routes
            NAME                HOST/PORT           PATH                SERVICE             LABELS
            route-edge          www.example.com                         frontend            template=ruby-helloworld-sample


2.  To use the route you must first install a router.  OpenShift provides an HAProxy router implementation that we'll use.
To install the router you must know the ip address of the host the router will be deployed on (used later) and the api 
url the master is listening on.  The api url can be found in the logs, your ip address can be determined with `ip a`.  Replace
the ip address shown below with the correct one for your environment.

            # Optional: pre-pull the router image.  This will be pulled automatically when the pod is created but will 
            # take some time.  Your pod will stay in Pending state while the pull is completed 
            $ docker pull openshift/origin-haproxy-router
            
            $ export OPENSHIFT_CA_DATA=$(<openshift.local.certificates/master/root.crt)
            $ pushd ../..
            $ hack/install-router.sh router https://10.0.2.15:8443
              Creating router file and starting pod...
              router
            $ popd


3.  Wait for the router to start.


            $ osc get pods
            POD                       IP                  CONTAINER(S)                   IMAGE(S)                          HOST                           LABELS                                                                                                             STATUS
            docker-registry-1-fnd84   172.17.0.3          registry-container             openshift/docker-registry         openshiftdev.local/127.0.0.1   deployment=docker-registry-1,deploymentconfig=docker-registry,name=registrypod,template=docker-registry-template   Running
            router                    172.17.0.10         origin-haproxy-router-router   openshift/origin-haproxy-router   openshiftdev.local/127.0.0.1   <none>                                                                                                             Running


4.  *Optional:* View the logs of the router.
 

            $ osc log router


5.  Curl the url, substituting the ip address shown for the correct value in your environment.

            $ curl -s -k --resolve www.example.com:443:10.0.2.15 https://www.example.com 
                ... removed for readability ... 
                <title>Hello from OpenShift v3!</title>
                ... removed for readability ...
            
7. *Optional*: View the certificate being used for the secure route.
            
            $ openssl s_client -servername www.example.com -connect 10.0.2.15:443
            ... removed for readability ...
            subject=/CN=www.example.com/ST=SC/C=US/emailAddress=example@example.com/O=Example/OU=Example
            issuer=/C=US/ST=SC/L=Default City/O=Default Company Ltd/OU=Test CA/CN=www.exampleca.com/emailAddress=example@example.com
            ... removed for readability ...
            ^C
            


Additional Operations
---------------------

In addition to creating resources, you can delete resources based on IDs. For example, if you want to remove only the containers or services created during the demo:

  - List the existing services:

        $ osc get -n test services

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        frontend            template=ruby-helloworld-sample   name=frontend       172.30.17.4        5432
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434


  - To remove the **frontend** service use the command:

        $ osc delete service -n test frontend

    Sample output:

        Status
        ----------
        Success

  - Check the service was removed:

        $ osc get -n test services

    Sample output:

        Name                Labels                            Selector            IP                  Port
        ----------          ----------                        ----------          ----------          ----------
        database            template=ruby-helloworld-sample   name=database       172.30.17.5        5434


  - You can also curl the application to check the service has terminated:

        $ curl http://172.17.17.4:5432

    Sample output:

        curl: (7) Failed connect to 172.17.17.4:5432; No route to host

Another interesting example is deleting a pod.

  - List available pods:

        $ osc get -n test pods

    Sample output:

        Name                                                Image(s)                                                                                                          Host                     Labels                                                                                                                                                       Status
        ----------                                          ----------                                                                                                        ----------               ----------                                                                                                                                                   ----------
        1b978f62-605f-11e4-b0db-3c970e3bf0b7                mysql                                                                                                             localhost.localdomain/   deploymentConfig=,deploymentID=database,name=database,replicationController=1b960e56-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample             Running
        4a792f55-605f-11e4-b0db-3c970e3bf0b7                172.30.17.3:5001/openshift/origin-ruby-sample:9477bdb99a409b9c747e699361ae7934fd83bb4092627e2ee35f9f0b0869885b   localhost.localdomain/   deploymentConfig=frontend,deploymentID=frontend-1,name=frontend,replicationController=4a749831-605f-11e4-b0db-3c970e3bf0b7,template=ruby-helloworld-sample   Running

  - Delete the **frontend** pod by specifying its ID:

        $ osc delete pod -n test 4a792f55-605f-11e4-b0db-3c970e3bf0b7

  - Verify that the pod has been removed by listing the available pods. This also stopped the associated Docker container, you can check using the command:

        $ docker ps -a

    Sample output:

        CONTAINER ID        IMAGE                                                COMMAND                CREATED              STATUS                          PORTS               NAMES
        068ffffa9624        127.0.0.1:5001/openshift/origin-ruby-sample:latest   "ruby /tmp/app.rb"     3 minutes ago        Exited (0) About a minute ago                       k8s_ruby-helloworld

Cleaning Up
-----------
To clean up all of your environment, you can run the script:

        $ sudo ./cleanup.sh

This will stop the `openshift` process, remove the etcd storage, and kill all Docker containers running on your host system.  The cleanup script needs root privileges to be able to remove all the directories OpenShift created.  (**Use with caution!**   Any Docker prefixed with "k8s_" will be killed by this script)
