# Container Setup for the Sample Application
OpenShift Origin is available as a [Docker](https://www.docker.io) container. It
has all of the software prebuilt and pre-installed, but you do need to do a few
things to get it going.

## Download and Run OpenShift Origin
If you have not already, perform the following to (download and) run the Origin
Docker container:

    $ docker run -d --name "openshift-origin" --net=host --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    openshift/origin start

Note that this won't hold any data after a restart, so you'll need to use a data
container or mount a volume at `/var/lib/openshift` to preserve that data. For
example, create a `/var/lib/openshift` folder on your Docker host, and then
start origin with the following:

    $ docker run -d --name "openshift-origin" --net=host --privileged \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/lib/openshift:/var/lib/openshift \
    openshift/origin start

## Preparing the Docker Host
On your **Docker host** you will need to fetch some images. You can do so by
running the pullimages.sh script like so:

    $ sh <(curl \
    https://raw.githubusercontent.com/openshift/origin/master/examples/sample-app/pullimages.sh)

This will fetch several Docker images that are used as part of the Sample
Application.

Next, be sure to follow the **Setup** instructions for the Sample Application
regarding an "insecure" Docker registry.

## Connect to the OpenShift Container
Once the container is started, you need to attach to it in order to execute
commands:

    $ docker exec -it openshift-origin bash

You may or may not want to change the bash prompt inside this container so that
you know where you are:

    $ PS1="openshift-dock: [\u@\h \W]\$ "

## Get the Sample Application Code
Inside the OpenShift Docker container, you'll need to fetch some of the code
bits that are used in the sample app.

    $ cd /var/lib/openshift
    $ mkdir -p examples/sample-app
    $ wget \
    https://raw.githubusercontent.com/openshift/origin/master/examples/sample-app/application-template-stibuild.json \
    -O examples/sample-app/application-template-stibuild.json

## Configure client security

    $ export CURL_CA_BUNDLE=`pwd`/openshift.local.config/master/ca.crt

For more information on this step, see [Application Build, Deploy, and Update
Flow](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md#application-build-deploy-and-update-flow),
step #3.

## Deploy the private docker registry

    $ oadm registry --create --credentials="${OPENSHIFTCONFIG}"
    $ cd examples/sample-app

For more information on this step, see [Application Build, Deploy, and Update
Flow](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md#application-build-deploy-and-update-flow),
step #4.

## Continue With Sample Application
At this point you can continue with the steps in the [Sample
Application](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md),
starting from [Application Build, Deploy, and Update
Flow](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md#application-build-deploy-and-update-flow),
step #5.

You can watch the OpenShift logs by issuing the following on your **Docker
host**:

    $ docker attach openshift-origin
