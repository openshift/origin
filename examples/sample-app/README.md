OpenShift 3 Application Lifecycle Sample
========================================

This is a set of configuration files and scripts which work with OpenShift 3 to create a new application and perform application builds.

This example assumes you have successfully built the `openshift` binary
executable (normally located under origin/\_output/local/bin/linux/amd64, or the
equivalent for your host platform/architecture), you have that and its
symlink/copy `oc` in your `PATH` and root's, and Docker is installed and
working. See https://github.com/openshift/origin/blob/master/CONTRIBUTING.adoc.

Alternatively, if you are using the openshift/origin container, please
make sure you follow these instructions first:
https://github.com/openshift/origin/blob/master/examples/sample-app/container-setup.md

Security Warning
----------------
OpenShift no longer requires SElinux to be disabled, however OpenShift is a system which runs containers on your system.  In some cases (build operations and the registry service) it does so using privileged containers.  Furthermore those containers access your host's Docker daemon and perform `docker build` and `docker push` operations.  As such, you should be aware of the inherent security risks associated with performing `docker run` operations on arbitrary images as they effectively have root access.  This is particularly relevant when running the OpenShift nodes directly on your host system.

For more information, see these articles:

* http://opensource.com/business/14/7/docker-security-selinux
* https://docs.docker.com/engine/security/security/

The OpenShift security model will continue to evolve and tighten going forward.

Setup
-----
At this stage of OpenShift 3 development, there are a few things that you will need to configure on the host where OpenShift is running in order for things to work.

**NOTE:** You do not need to do this if you are using [Vagrant](https://vagrantup.com/) to work with OpenShift.  Refer to the "VAGRANT USERS" callouts throughout this document for modifications specific to Vagrant users.

- - -
**VAGRANT USERS**:
If you haven't already, fire up a Vagrant instance, where since a OpenShift compile is occurring in a subsequent step below, you need to override the default amount of memory assigned to the VM.

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

First, you'll need to configure the Docker daemon on your host to trust the container image registry service you'll be starting.

To do this, you need to add "--insecure-registry 172.30.0.0/16" to the Docker daemon invocation, eg:

    $ docker daemon --insecure-registry 172.30.0.0/16

Note that you need to have installed Docker 1.3.2 or higher in order to use the `--insecure-registry` flag.

If you are running Docker as a service via `systemd`, you can add this argument to the options value in `/etc/sysconfig/docker`

This will instruct the Docker daemon to trust any container image registry on the 172.30.0.0/16 subnet,
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

* All commands assume the `oc` binaries are in your path.
* All commands assume that you are working from the `sample-app` directory in your local environment.
    * If you are working from a local git repo, this might be `$GOPATH/src/github.com/<username>/origin/examples/sample-app`
- - -


1. For the sake of this demo, grant a `cluster-admin` role to the `test-admin` user and login as that user using any password you want (note that in a real world scenario, as an OpenShift user you would be granted roles from a cluster admin and you might not be able to do most of the following things - depending on your granted roles).

        $ oc adm policy add-cluster-role-to-user cluster-admin test-admin --kubeconfig=openshift.local.config/master/admin.kubeconfig
        $ oc login --certificate-authority=openshift.local.config/master/ca.crt -u test-admin


2. Create a new project in OpenShift. This creates a namespace `test` to contain the builds and app that we will generate below.

        $ oc new-project test --display-name="OpenShift 3 Sample" --description="This is an example project to demonstrate OpenShift v3"


3. *Optional:* View the OpenShift web console in your browser by browsing to `https://<host>:8443/console`.  Login using the user `test-admin` and any password.

    * You will need to have the browser accept the certificate at
      `https://<host>:8443` before the console can consult the OpenShift
      API. Of course this would not be necessary with a legitimate
      certificate.
    * If you click the `OpenShift 3 Sample` project and leave the tab open,
      you'll see the page update as you deploy objects into the project
      and run builds.


4. *Optional:* Fork the [ruby sample repository](https://github.com/openshift/ruby-hello-world)
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


5. *Optional:* Add the following webhook under the settings in your new GitHub repository:

        $ https://<host>:8443/osapi/v1/namespaces/test/buildconfigs/ruby-sample-build/webhooks/secret101/github

  * Note: Using the webhook requires that your OpenShift server be
    publicly accessible so GitHub can reach it to invoke the hook. You
    will almost certainly need to "Disable SSL Verification" for your test
    instance as the certificate chain generated is not publicly verified.


6. Edit application-template-stibuild.json which will define the sample application

 * Update the BuildConfig's sourceURI (https://github.com/openshift/ruby-hello-world.git) to point to your forked repository.
   *Note:* You can skip this step if you did not create a forked repository.


7. Submit the application template for processing (generating shared parameters requested in the template)
    and then request creation of the processed template:

        $ oc new-app application-template-stibuild.json
        --> Deploying template ruby-helloworld-sample for "application-template-stibuild.json"

             ruby-helloworld-sample
             ---------
             This example shows how to create a simple ruby application in openshift origin v3

             * With parameters:
                * MYSQL_USER=userPJJ # generated
                * MYSQL_PASSWORD=cJHNK3se # generated
                * MYSQL_DATABASE=root

        --> Creating resources with label app=ruby-helloworld-sample ...
            service "frontend" created
            route "route-edge" created
            imagestream "origin-ruby-sample" created
            imagestream "ruby-27-centos7" created
            buildconfig "ruby-sample-build" created
            deploymentconfig "frontend" created
            service "database" created
            deploymentconfig "database" created
        --> Success
            Build scheduled, use 'oc logs -f bc/ruby-sample-build' to track its progress.
            Run 'oc status' to view your app.

    Note that no build has actually occurred yet, so at this time there
    is no image to deploy and no application to visit. But since we've defined
    ImageChange trigger inside of BuildConfig object a new Build will be started
    immediately.


8. Monitor the progress of the build (this can take a few minutes):

        $ oc get builds
        NAME                  TYPE      FROM          STATUS    STARTED         DURATION
        ruby-sample-build-1   Source    Git@bd94cbb   Running   7 seconds ago   7s


    The built image will be named with the ImageStream
    (origin-ruby-sample) named in the BuildConfig and pushed to the
    private container image registry running in OpenShift.  (Note that the private
    container image registry is using ephemeral storage, so when it is stopped,
    the image will be lost.)

    Stream the build logs:

        $ oc logs -f bc/ruby-sample-build
        ... removed for readability ...
        ---> Installing application source ...
        ---> Building your Ruby application from source ...
        ---> Running 'bundle install --deployment' ...
        Fetching gem metadata from https://rubygems.org/..........
        Installing rake 10.3.2
        Installing i18n 0.6.11
        Installing json 1.8.3
        Installing minitest 5.4.2
        ... removed for readability ...
        I0310 12:54:07.595390       1 sti.go:250] Using provided push secret for pushing 172.30.163.205:5000/test/origin-ruby-sample:latest image
        I0310 12:54:07.596061       1 sti.go:254] Pushing 172.30.163.205:5000/test/origin-ruby-sample:latest image ...
        I0310 12:54:10.286740       1 sti.go:270] Successfully pushed 172.30.163.205:5000/test/origin-ruby-sample:latest


    The creation of the new image in the container image registry will
    automatically trigger a deployment of the application, creating a
    pod each for the frontend (your Ruby code) and backend.


9. Wait for the application's frontend pod and database pods to be started. By the time your build completes, the database pod will most probably have been deployed. Since your frontend depends on your build and once your build is complete, you can monitor your frontend deployment:

        $ oc logs -f dc/frontend
        I0310 12:36:46.976047       1 deployer.go:199] Deploying test/frontend-1 for the first time (replicas: 2)
        I0310 12:36:47.025372       1 lifecycle.go:168] Created lifecycle pod test/frontend-1-hook-pre for deployment test/frontend-1
        I0310 12:36:47.025942       1 lifecycle.go:181] Watching logs for hook pod test/frontend-1-hook-pre while awaiting completion
        I0310 12:36:50.194431       1 lifecycle.go:221] Finished reading logs for hook pod test/frontend-1-hook-pre
        I0310 12:36:50.195868       1 rolling.go:120] Pre hook finished
        I0310 12:36:50.197033       1 recreate.go:126] Scaling test/frontend-1 to 1 before performing acceptance check
        I0310 12:36:52.247222       1 recreate.go:131] Performing acceptance check of test/frontend-1
        I0310 12:36:52.249717       1 lifecycle.go:445] Waiting 120 seconds for pods owned by deployment "test/frontend-1" to become ready (checking every 1 seconds; 0 pods previously accepted)
        I0310 12:36:54.289925       1 lifecycle.go:466] All pods ready for test/frontend-1
        I0310 12:36:54.290422       1 recreate.go:139] Scaling test/frontend-1 to 2
        I0310 12:36:56.360679       1 recreate.go:156] Deployment frontend-1 successfully made active
        I0310 12:36:56.371762       1 lifecycle.go:168] Created lifecycle pod test/frontend-1-hook-post for deployment test/frontend-2
        I0310 12:36:56.371825       1 lifecycle.go:181] Watching logs for hook pod test/frontend-1-hook-post while awaiting completion
        I0310 12:37:00.209644       1 lifecycle.go:221] Finished reading logs for hook pod test/frontend-1-hook-post
        I0310 12:37:00.236213       1 lifecycle.go:87] Hook failed, ignoring:
        I0310 12:37:00.236387       1 rolling.go:134] Post hook finished


    *Note:* If the deployment finishes before you try to get its logs, `oc logs -f dc/frontend` will start serving logs from the application pods.


        $ oc get pods
        NAME                        READY     STATUS      RESTARTS   AGE
        database-1-le4wx            1/1       Running     0          1m
        frontend-1-e572n            1/1       Running     0          27s
        frontend-1-votq4            1/1       Running     0          31s
        ruby-sample-build-1-build   0/1       Completed   0          1m



10. Determine the IP for the frontend service:

        $ oc get services
        NAME       CLUSTER-IP      EXTERNAL-IP   PORT(S)    SELECTOR        AGE
        database   172.30.80.39    <none>        5434/TCP   name=database   1m
        frontend   172.30.17.4     <none>        5432/TCP   name=frontend   1m

    In this case, the IP for frontend is 172.30.17.4 and it is on port 5432.

    *Note:* you can also get this information from the web console.


11. Confirm the application is now accessible via the frontend service on port 5432.  Go to http://172.30.17.4:5432 (or whatever IP address was reported above) in your browser if you're running this locally; otherwise you can use curl to see the HTML, or port forward the address to your local workstation to visit it.

	- - -
	**VAGRANT USERS:**
	Open a new terminal and enter this command to forward the application port to a port on your workstation:

		$ vagrant ssh -- -L 9999:172.30.17.4:5432 (or 9999:whatever IP address was reported above)

	You can now confirm the application is accessible on port 5432 by going to http://127.0.0.1:9999.  Note that port 9999 is arbitrary.
	- - -

    You should see a welcome page and a form that allows you to query and update key/value pairs.  The keys are stored in the database container running in the database pod.


12. Make a change to your ruby sample main.html file, commit, and push it via git. If you do not have the webhook enabled, you'll have to manually trigger another build:

        $ oc start-build ruby-sample-build


13. Repeat step 13 (waiting for the build to complete).  Once the build is complete, refreshing your browser should show your changes.

Congratulations, you've successfully deployed and updated an application on OpenShift!


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


    Optional: pre-pull the router image.  This will be pulled automatically when the pod is created but will take some time.  Your pod will stay in Pending state while the pull is completed


        $ docker pull openshift/origin-haproxy-router


    Create a service account that the router will use.


        $ echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f -


    Give the following permissions to your router service account. It needs to be able to use host network and host ports, and it also needs to be able to list endpoints in all namespaces, that's why you need to grant it the `system:router` cluster role.


        $ oc adm policy add-scc-to-user hostnetwork -z router
        $ oc adm policy add-cluster-role-to-user system:router system:serviceaccount:default:router


    The router by default uses the host network. If you wish to use the container network stack and expose ports, add the --host-network=false option to the oc adm router command.


        $ oc adm router --service-account=router
        DeploymentConfig "router" created
        Service "router" created


3.  Switch to the `default` project to watch for router to start

        $ oc project default

4.  Wait for the router to start.

        $ oc describe dc router
        # watch for the number of deployed pods to go to 1


5.  *Optional:* View the logs of the router.

        $ oc logs dc/router
        I0310 13:08:53.095282       1 router.go:161] Router is including routes in all namespaces


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
        NAME       CLUSTER-IP      EXTERNAL-IP   PORT(S)    SELECTOR        AGE
        database   172.30.80.39    <none>        5434/TCP   name=database   6m
        frontend   172.30.17.4     <none>        5432/TCP   name=frontend   6m


  - To remove the **frontend** service use the command:

        $ oc delete service frontend
        Service "frontend" deleted

  - Check the service was removed:

        $ oc get services
        NAME       CLUSTER-IP      EXTERNAL-IP   PORT(S)    SELECTOR        AGE
        database   172.30.80.39    <none>        5434/TCP   name=database   6m

  - You can also curl the application to check the service has terminated:

        $ curl http://172.30.17.4:5432
        curl: (7) Failed connect to 172.30.17.4:5432; No route to host

Another interesting example is deleting a pod.

  - List available pods:

        $ oc get pods
        NAME                        READY     STATUS      RESTARTS   AGE
        database-1-le4wx            1/1       Running     0          7m
        frontend-1-e572n            1/1       Running     0          6m
        frontend-1-votq4            1/1       Running     0          6m
        ruby-sample-build-1-build   0/1       Completed   0          7m


  - Delete the **frontend** pod by specifying its ID:

        $ oc delete pod frontend-1-votq4

  - Verify that the pod has been removed by listing the available pods. This also stopped the associated container, you can check using the command:

        $ docker ps -a
        CONTAINER ID        IMAGE                                                COMMAND                CREATED              STATUS                          PORTS               NAMES
        [ ... ]
        068ffffa9624        127.0.0.1:5001/openshift/origin-ruby-sample:latest   "ruby /tmp/app.rb"     3 minutes ago        Exited (0) About a minute ago                       k8s_ruby-helloworld
        [ ... ]


Cleaning Up
-----------
To clean up all of your environment, you can run the script:

        $ sudo ./cleanup.sh

This will stop the `openshift` process, remove files created by OpenShift and kill all containers created by Kubernetes in your host system.  The cleanup script needs root privileges to be able to remove all the directories OpenShift created.

**Use with caution!** Any container prefixed with "k8s_" will be killed by this script.
