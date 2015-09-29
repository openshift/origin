OpenShift 3 Application Lifecycle Sample
========================================

This is a set of configuration files and scripts which work with OpenShift 3 to create a new application and perform application builds.

This example assumes you have successfully built the `openshift` binary
executable (normally located under origin/\_output/local/bin/linux/amd64, or the
equivalent for your host platform/architecture), you have that and its
symlink/copy `oc` in your `PATH` and root's, and Docker is installed and
working. See https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc.

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

**NOTE:** You do not need to do this if you are using [Vagrant](https://vagrantup.com/) to work with OpenShift.  Refer to the "VAGRANT USERS" callouts throughout this document for modifications specific to Vagrant users.

- - -
**VAGRANT USERS**:
If you haven't already, fire up a Vagrant instance, where since a OpenShift compile is occurring in a subsequent step below, you need to override the default amount of memory assigned to the VM.

	$ export OPENSHIFT_MEMORY=2096
	$ vagrant up
	$ vagrant ssh

Inside of your Vagrant instance, the path to the origin directory is `/data/src/github.com/openshift/origin`.

	$ cd /data/src/github.com/openshift/origin

Run an advance build of the OpenShift binaries before continuing:

	$ make clean build

This will set up a go workspace locally and will build all go components.  It is not necessary to make the docker and firewall changes, instead [jump to the next section](#application-build-deploy-and-update-flow).

- - -

### Docker Changes ###

**VAGRANT USERS**:
If you are using the OpenShift Vagrant image you can skip this step.

First, you'll need to configure the Docker daemon on your host to trust the Docker registry service you'll be starting.

To do this, you need to add "--insecure-registry 172.30.0.0/16" to the Docker daemon invocation, eg:

    $ docker -d --insecure-registry 172.30.0.0/16

Note that you need to have installed Docker 1.3.2 or higher in order to use the `--insecure-registry` flag.

If you are running Docker as a service via `systemd`, you can add this argument to the options value in `/etc/sysconfig/docker`

This will instruct the Docker daemon to trust any Docker registry on the 172.30.0.0/16 subnet,
rather than requiring the registry to have a verifiable certificate.

These instructions assume you have not changed the kubernetes/openshift service subnet configuration from the default value of 172.30.0.0/16.

### FirewallD Changes ###

**VAGRANT USERS**:
If you are using the OpenShift Vagrant image you can skip this step.

Similar to our work on SELinux policies, the OpenShift firewalld rules are also a work in progress. For now it is easiest to disable firewalld altogether:

    $ sudo systemctl stop firewalld

Firewalld will start again on your next reboot, but you can manually restart it with this command when you are done with the sample app:

    $ sudo systemctl start firewalld

### Still Having Trouble? ###

If you hit any snags while taking the sample app for a spin, check out the [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md).

Application Build, Deploy, and Update Flow
------------------------------------------

This section covers how to perform all the steps of building, deploying, and updating an application on the OpenShift platform.

- - -
**NOTE**

* All commands assume the `oc` binary/symlink is in your path.
* All commands assume that you are working from the `sample-app` directory in your local environment.
    * If you are working from a local git repo, this might be `$GOPATH/src/github.com/<username>/origin/examples/sample-app`
    * **VAGRANT USERS**: `cd /data/src/github.com/openshift/origin/examples/sample-app`

- - -


1. *Optional*: Pre-pull the Docker images used in this sample.  This is
    not strictly necessary as OpenShift will pull the images as it needs them,
    but by doing it up front it will prevent lengthy operations during build
    and deployment which might otherwise lead you to believe the process
    has failed or hung.

        $ ./pullimages.sh


2. Launch an all-in-one `openshift` instance

        $ sudo openshift start &> openshift.log &

       **VAGRANT USERS**: Instead of the above command, use

        $ sudo /data/src/github.com/openshift/origin/_output/local/bin/linux/amd64/openshift start --public-master=localhost --volume-dir=</absolute/path> &> openshift.log &

    Note: sudo is required so the kubernetes proxy can manipulate iptables rules to expose service ports.

    Note: when using vagrant synced folder it is advised to use a different directory for volume storage than the one in the synced folder. This can be achieved by passing `--volume-dir=/absolute/path` to `openshift start` command.


3. Set up your client to reach the OpenShift master now running.

    Since OpenShift services are secured by TLS, your client will
    need to accept the server certificates and present its own client
    certificate. These are generated as part of the `openshift start`
    command in whatever the current directory is at the time. You will
    need to point oc and curl at the appropriate certificates in order
    to connect to OpenShift. Assuming you are running as a user other
    than root, you will also need to make the .kubeconfig readable by
    that user. (Note: this is just for example purposes; in a real
    installation, users would generate their own keys and not have access
    to the system keys.)

        $ export CURL_CA_BUNDLE=`pwd`/openshift.local.config/master/ca.crt
        $ sudo chmod a+rwX openshift.local.config/master/admin.kubeconfig

4. Deploy a private docker registry within OpenShift with the certs necessary for access to master:

        $ sudo chmod +r openshift.local.config/master/openshift-registry.kubeconfig
        $ oadm registry --create --credentials=openshift.local.config/master/openshift-registry.kubeconfig --config=openshift.local.config/master/admin.kubeconfig
          deploymentconfigs/docker-registry
          services/docker-registry

    Note that the private Docker registry is using ephemeral storage,
    so when it is stopped, the image will be lost. An external volume
    could be used for persistent storage, but that is beyond the scope
    of this tutorial.


5. Confirm the registry is started (this can take a few minutes):

        $ oc describe service docker-registry --config=openshift.local.config/master/admin.kubeconfig

    You should see:

        Name:       docker-registry
        Labels:     docker-registry=default
        Selector:   docker-registry=default
        Port:       5000
        Endpoints:  172.17.0.60:5000
        No events.

    If "Endpoints" is listed as `<none>`, your registry hasn't started yet.  You can run `oc get pods` to
    see the registry pod and if there are any issues. Once the pod has started, the IP of the pod will
    be added to the docker-registry service list so that it's reachable from other places.


6. Login as `test-admin` using any password

        $ oc login --certificate-authority=openshift.local.config/master/ca.crt

       **VAGRANT USERS**: If subsequent commands fail because of a config validation error, log out, unset the $KUBECONFIG environment variable (if it is set) and then log in again.



7. Create a new project in OpenShift. This creates a namespace `test` to contain the builds and app that we will generate below.

        $ oc new-project test --display-name="OpenShift 3 Sample" --description="This is an example project to demonstrate OpenShift v3"


8. *Optional:* View the OpenShift web console in your browser by browsing to `https://<host>:8443/console`.  Login using the user `test-admin` and any password.

    * You will need to have the browser accept the certificate at
      `https://<host>:8443` before the console can consult the OpenShift
      API. Of course this would not be necessary with a legitimate
      certificate.
    * If you click the `OpenShift 3 Sample` project and leave the tab open,
      you'll see the page update as you deploy objects into the project
      and run builds.


9. *Optional:* Fork the [ruby sample repository](https://github.com/openshift/ruby-hello-world)
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


10. *Optional:* Add the following webhook under the settings in your new GitHub repository:

        $ https://<host>:8443/osapi/v1beta3/namespaces/test/buildconfigs/ruby-sample-build/webhooks/secret101/github

  * Note: Using the webhook requires that your OpenShift server be
    publicly accessible so GitHub can reach it to invoke the hook. You
    will almost certainly need to "Disable SSL Verification" for your test
    instance as the certificate chain generated is not publicly verified.


11. Edit application-template-stibuild.json which will define the sample application

 * Update the BuildConfig's sourceURI (git://github.com/openshift/ruby-hello-world.git) to point to your forked repository.
   *Note:* You can skip this step if you did not create a forked repository.


12. Submit the application template for processing (generating shared parameters requested in the template)
    and then request creation of the processed template:

        $ oc new-app application-template-stibuild.json

    This will define a number of related OpenShift entities in the project:

    * A BuildConfig (ruby-sample-build) to specify a build that uses
      your ruby-hello-world fork as the input for a source-to-image (STI) build
    * ImageStreams for the images used and created in the build:
      * The ruby-20-centos7 STI builder will build an image from your source
      * The output image will be called origin-ruby-sample
    * DeploymentConfigs (frontend, backend) for defining Deployments once the images are available
    * Services (routable endpoints) for the ruby frontend and database backend deployments
      that will deployed as output of the build

    Note that no build has actually occurred yet, so at this time there
    is no image to deploy and no application to visit. But since we've defined
    ImageChange trigger inside of BuildConfig object a new Build will be started
    immediately.


13. Monitor the builds and wait for the status to go to "complete" (this can take a few minutes):

        $ oc get builds

    You can add the --watch flag to wait for updates until the build completes:

        $ oc get builds --watch

    Sample output:

        NAME                  TYPE                STATUS              POD
        ruby-sample-build-1   STI                 Complete            ruby-sample-build-1

     The built image will be named with the ImageStream
     (origin-ruby-sample) named in the BuildConfig and pushed to the
     private Docker registry running in OpenShift.  (Note that the private
     docker registry is using ephemeral storage, so when it is stopped,
     the image will be lost.)

     If you want to see the build logs of a complete build, use the
     command below (substituting your build name from the "oc get builds"
     output).

         $ oc build-logs ruby-sample-build-1 -n test

    The creation of the new image in the Docker registry will
    automatically trigger a deployment of the application, creating a
    pod each for the frontend (your Ruby code) and backend.


14. Wait for the application's frontend pod and database pods to be started (this can take a few minutes):

        $ oc get pods

    Sample output:

        POD                   IP                  CONTAINER(S)               IMAGE(S)                                                                                                      HOST                           LABELS                                                                                                                  STATUS              CREATED
        database-1-zhomu      172.17.0.16         ruby-helloworld-database   openshift/mysql-55-centos7                                                                                    openshiftdev.local/127.0.0.1   deployment=database-1,deploymentconfig=database,name=database,template=application-template-stibuild                    Running             3 minutes
        frontend-1-lb4c4      172.17.0.20         ruby-helloworld            172.30.17.113:5000/test/origin-ruby-sample:029721e6cb1b4a4f6b52ccac0abfbf2a3be1a344fb1355a75bed29ccca0d1ba2   openshiftdev.local/127.0.0.1   deployment=frontend-1,deploymentconfig=frontend,name=frontend,template=application-template-stibuild                    Running             About a minute
        ruby-sample-build-1                       sti-build                  openshift/origin-sti-builder:latest                                                                           openshiftdev.local/127.0.0.1   build=ruby-sample-build-1,buildconfig=ruby-sample-build,name=ruby-sample-build,template=application-template-stibuild   Succeeded           3 minutes


15. Determine the IP for the frontend service:

        $ oc get services

    Sample output:

        NAME                LABELS                                   SELECTOR            IP                PORT
        database            template=application-template-stibuild   name=database       172.30.17.5       5434
        frontend            template=application-template-stibuild   name=frontend       172.30.17.4       5432

    In this case, the IP for frontend is 172.30.17.4 and it is on port 5432.

    *Note:* you can also get this information from the web console.


16. Confirm the application is now accessible via the frontend service on port 5432.  Go to http://172.30.17.4:5432 (or whatever IP address was reported above) in your browser if you're running this locally; otherwise you can use curl to see the HTML, or port forward the address to your local workstation to visit it.

	- - -
	**VAGRANT USERS:**
	Open a new terminal and enter this command to forward the application port to a port on your workstation:

		$ vagrant ssh -- -L 9999:172.30.17.4:5432 (or 9999:whatever IP address was reported above)

	You can now confirm the application is accessible on port 5432 by going to `http://<host>:9999`.  Note that port 9999 is arbitrary.
	- - -

    You should see a welcome page and a form that allows you to query and update key/value pairs.  The keys are stored in the database container running in the database pod.


17. Make a change to your ruby sample main.html file, commit, and push it via git.

 * If you do not have the webhook enabled, you'll have to manually trigger another build:

            $ oc start-build ruby-sample-build


18. Repeat step 13 (waiting for the build to complete).  Once the build is complete, refreshing your browser should show your changes.

Congratulations, you've successfully deployed and updated an application on OpenShift.


Advanced
---------
OpenShift also provides features that live outside the deployment life cycle like routing.

1.  Your sample app has been created with a secure route which can be viewed by performing a `GET` on the route api object.

            $ oc get routes
            NAME                HOST/PORT           PATH                SERVICE             LABELS
            route-edge          www.example.com                         frontend            template=application-template-stibuild


2.  To use the route you must first install a router.  OpenShift provides an HAProxy router implementation that we'll use.
To install the router you must know the ip address of the host the router will be deployed on (used later) and the api
url the master is listening on.  The api url can be found in the logs, your ip address can be determined with `ip a`.  Replace
the ip address shown below with the correct one for your environment.

            # Optional: pre-pull the router image.  This will be pulled automatically when the pod is created but will
            # take some time.  Your pod will stay in Pending state while the pull is completed
            $ docker pull openshift/origin-haproxy-router

            # Create a service account that the router will use.  This service account must have access to use a
            # security context constraint that allows host ports
            $ echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f -

            # You may either create a new SCC or use an existing SCC.  The following command will
            # display existing SCCs and if they support host network and host ports.
            $ oc get scc -t "{{range .items}}{{.metadata.name}}: n={{.allowHostNetwork}},p={{.allowHostPorts}}; {{end}}"
            privileged: n=true,p=true; restricted: n=false,p=false;

            # Edit your security context constraint to add the new service account in the users section
            # in the form of system:serviceaccount:<namespace>:<name>.  In the above example the full
            # name would be system:serviceaccount:default:router if you are creating the router in the default namespace.
            $ oc edit scc <name>

            $ sudo chmod +r openshift.local.config/master/openshift-router.kubeconfig
            # The router by default uses the host network. If you wish to
            # use the container network stack and expose ports, add the
            # --host-network=false option to the oadm router command.
            $ oadm router --credentials=openshift.local.config/master/openshift-router.kubeconfig --config=openshift.local.config/master/admin.kubeconfig --service-account=router
              router # the service
              router # the deployment config


3.  Switch to the `default` project to watch for router to start

            $ oc project default --config=openshift.local.config/master/admin.kubeconfig

4.  Wait for the router to start.

            $ oc describe dc router --config=openshift.local.config/master/admin.kubeconfig
            # watch for the number of deployed pods to go to 1


5.  *Optional:* View the logs of the router.  First though, you need to get random suffix that Kubernetes includes as part of the name it generates.

	    $ oc get pods --config=openshift.local.config/master/admin.kubeconfig
            # Look for the pod name starting with "router-1-"

6. *Optional:* With that precise pod name, you can view its logs.

            $ oc logs router-1-<podrandom-suffix> --config=openshift.local.config/master/admin.kubeconfig


7.  Curl the url, substituting the ip address shown for the correct value in your environment.  The easiest way to get the IP is to do a ifconfig from where you have been running the oc command.

            $ curl -s -k --resolve www.example.com:443:10.0.2.15 https://www.example.com
                ... removed for readability ...
                <title>Hello from OpenShift v3!</title>
                ... removed for readability ...


8. *Optional*: View the certificate being used for the secure route.

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

        $ oc get services

    Sample output:

        NAME                LABELS                                   SELECTOR            IP                PORT
        database            template=application-template-stibuild   name=database       172.30.17.5       5434
        frontend            template=application-template-stibuild   name=frontend       172.30.17.4       5432


  - To remove the **frontend** service use the command:

        $ oc delete service frontend

    Sample output:

        services/frontend

  - Check the service was removed:

        $ oc get services

    Sample output:

        NAME                LABELS                                   SELECTOR            IP                PORT
        database            template=application-template-stibuild   name=database       172.30.17.5       5434

  - You can also curl the application to check the service has terminated:

        $ curl http://172.17.17.4:5432

    Sample output:

        curl: (7) Failed connect to 172.17.17.4:5432; No route to host

Another interesting example is deleting a pod.

  - List available pods:

        $ oc get pods

    Sample output:

        POD                   IP                  CONTAINER(S)               IMAGE(S)                                                                                                      HOST                           LABELS                                                                                                                  STATUS              CREATED
        database-1-zhomu      172.17.0.16         ruby-helloworld-database   openshift/mysql-55-centos7                                                                                    openshiftdev.local/127.0.0.1   deployment=database-1,deploymentconfig=database,name=database,template=application-template-stibuild                    Running             9 minutes
        frontend-1-lb4c4      172.17.0.20         ruby-helloworld            172.30.17.113:5000/test/origin-ruby-sample:029721e6cb1b4a4f6b52ccac0abfbf2a3be1a344fb1355a75bed29ccca0d1ba2   openshiftdev.local/127.0.0.1   deployment=frontend-1,deploymentconfig=frontend,name=frontend,template=application-template-stibuild                    Running             7 minutes
        ruby-sample-build-1                       sti-build                  openshift/origin-sti-builder:latest                                                                           openshiftdev.local/127.0.0.1   build=ruby-sample-build-1,buildconfig=ruby-sample-build,name=ruby-sample-build,template=application-template-stibuild   Succeeded           9 minutes

  - Delete the **frontend** pod by specifying its ID:

        $ oc delete pod frontend-1-lb4c4

  - Verify that the pod has been removed by listing the available pods. This also stopped the associated Docker container, you can check using the command:

        $ docker ps -a

    Sample output:

        CONTAINER ID        IMAGE                                                COMMAND                CREATED              STATUS                          PORTS               NAMES
        068ffffa9624        127.0.0.1:5001/openshift/origin-ruby-sample:latest   "ruby /tmp/app.rb"     3 minutes ago        Exited (0) About a minute ago                       k8s_ruby-helloworld

Cleaning Up
-----------
To clean up all of your environment, you can run the script:

        $ sudo ./cleanup.sh

This will stop the `openshift` process, remove files created by OpenShift and kill all Docker containers created by Kubernetes in your host system.  The cleanup script needs root privileges to be able to remove all the directories OpenShift created.

**Use with caution!** Any Docker container prefixed with "k8s_" will be killed by this script.
