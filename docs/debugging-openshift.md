Troubleshooting
=================

This document contains some tips and suggestions for troubleshooting an OpenShift v3 deployment.

System Environment
------------------

1. Run as root

   Currently OpenShift v3 must be started as root in order to manipulate your iptables configuration.  The openshift commands (e.g. `oc create`) do not need to be run as root.

1. Properly configure or disable firewalld

   On Fedora or other distributions using firewalld: Add docker0 to the public zone

        $ firewall-cmd --zone=trusted --change-interface=docker0
        $ systemctl restart firewalld

    Alternatively you can disable it via:

        $ systemctl stop firewalld

1. Setup your host DNS to an address that the containers can reach

  Containers need to be able to resolve hostnames, so if you run a local DNS server on your host, you should update your /etc/resolv.conf to instead use a DNS server that will be reachable from within running containers.  Google's "8.8.8.8" server is a popular choice.

1. Save iptables rules before restarting iptables and restore them afterwards. If iptables have to be restarted, then the iptables rules should be saved and restored, otherwise the docker inserted rules would get lost.


        $ iptables-save > /path/to/iptables.bkp
        $ systemctl restart iptables
        $ iptables-restore < /path/to/iptables.bkp



Build Failures
--------------

To investigate a build failure, first check the build logs.  You can view the build logs via

    $ oc logs build/[build_id]

and you can get the build id via:

    $ oc get builds

the build id is in the first column.

If you're unable to retrieve the logs in this way, you can also get them directly from docker.  First you need to find the docker container that ran your build:

    $ docker ps -a | grep builder

The most recent container in that list should be the one that ran your build.  The container id is the first column.  You can then run:

    $ docker logs [container id]

Hopefully the logs will provide some indication of what it failed (e.g. failure to find the source repository, an actual build issue, failure to push the resulting image to the docker registry, etc).

One issue seen sometimes is not being able to resolve any hostname (for example github.com) from within running containers:

    E0708 17:28:07.845231       1 git.go:102] fatal: unable to access 'https://github.com/gabemontero/cakephp-ex.git/': Could not resolve host: github.com; Unknown error

If this shows up in your build logs, restart docker and then resubmit a build:

    $ sudo systemctl restart docker
    $ oc start-build <your build identifier>

Another item seen stems from how OpenShift operates in a SELinux environment.  The SELinux policy requires that host directories that are bind mounted have the svirt_sandbox_file_t label.  Generally
this simply happens for you under the covers, but there is a growing list of user operations which hamper the registry deployment to the point where the svrt_sandbox_file_t label ends up missing, and you can see
various authentication or push failures.  One example, when initiating a build:

     Failed to push image: Error pushing to registry: Server error: unexpected 500 response status trying to initiate upload of test/origin-ruby-sample

And when inspecting the Docker registry, you will see messages like this:

    173.17.42.1 - - [03/Jun/2015:13:26:19 +0000] "POST /v2/test/origin-ruby-sample/blobs/uploads/ HTTP/1.1" 500 203 "" "docker/1.6.0 go/go1.4.2 kernel/3.17.4-301.fc21.x86_64 os/linux arch/amd64"

When this sequence occurs, without needing to restart Docker nor OpenShift, you can work around it by running the following command:

     $ sudo chcon -R -t svirt_sandbox_file_t < path to >/openshift.local.volumes

Docker Registry
---------------

Most of the v3 flows today assume you are running a docker registry pod.  You should ensure that this local registry is running:

    $ openshift admin registry

If it's running, you should see this:

    Docker registry "docker-registry" service exists

If it's not running, you will instead see:

    F0429 09:22:54.492990   25259 registry.go:154] Docker-registry "docker-registry" does not exist (no service). Pass --create to install.

If it's not running, you can launch it via:

    $ oadm registry --create --credentials="${KUBECONFIG}"

Probing Containers
------------------

In general you may want to investigate a particular container.  You can either gather the logs from a container via `docker logs [container id]` or use `docker exec -it [container id] /bin/sh` to enter the container's namespace and poke around.

Sometimes you'll hit a problem while developing an sti builder or Docker build where the image fails to start up.  Another scenario that is possible is that you're working on a liveness probe and it's failing and therefore killing the container before you have time to figure out what is happening.  Sometimes you can run `docker start <CONTAINER ID>` however if the pod has been destroyed and it was dependent on a volume it won't let you restart the container if the volume has been cleaned up.

If you simply want to take a container that OpenShift has created but debug it outside of the Master's knowledge you can run the following:

    $ docker commit <CONTAINER ID> <some new name>
    $ docker run -it <name from previous step> /bin/bash

Obviously this won't work if you don't have bash installed but you could always package it in for debugging purposes.

Name Resolution Within Containers
-------------------

DNS related services like `dnsmasq` can interfere with naming resolution in the Docker containers launched by OpenShift, including binding on the same port (53) that OpenShift will attempt to use.  To circumvent this conflict, disable
these services.  Using the `dnsmasq` example on Fedora, run all three of the following before starting OpenShift to ensure `dnsmasq` is not running, does not launch later on, and hence does not interfere with OpenShift:

    $ sudo systemctl stop dnsmasq
    $ sudo systemctl disable dnsmasq
    $ sudo killall dnsmasq


Benign Errors/Messages
----------------------

There are a number of suspicious looking messages that appear in the openshift log output which can normally be ignored:

1. Failed to find an IP for pod (benign as long as it does not continuously repeat)

        E1125 14:51:49.665095 04523 endpoints_controller.go:74] Failed to find an IP for pod: {{ } {7e5769d2-74dc-11e4-bc62-3c970e3bf0b7 default /api/v1beta1/pods/7e5769d2-74dc-11e4-bc62-3c970e3bf0b7  41 2014-11-25 14:51:48 -0500 EST map[template:ruby-helloworld-sample deployment:database-1 deploymentconfig:database name:database] map[]} {{v1beta1 7e5769d2-74dc-11e4-bc62-3c970e3bf0b7 7e5769d2-74dc-11e4-bc62-3c970e3bf0b7 [] [{ruby-helloworld-database mysql []  [{ 0 3306 TCP }] [{MYSQL_ROOT_PASSWORD rrKAcyW6} {MYSQL_DATABASE root}] 0 0 [] <nil> <nil>  false }] {0x1654910 <nil> <nil>}} Running localhost.localdomain   map[]} {{   [] [] {<nil> <nil> <nil>}} Pending localhost.localdomain   map[]} map[]}

1. Proxy connection reset

        E1125 14:52:36.605423 04523 proxier.go:131] I/O error: read tcp 10.192.208.170:57472: connection reset by peer

1. No network settings

        W1125 14:53:10.035539 04523 rest.go:231] No network settings: api.ContainerStatus{State:api.ContainerState{Waiting:(*api.ContainerStateWaiting)(0xc208b29b40), Running:(*api.ContainerStateRunning)(nil), Termination:(*api.ContainerStateTerminated)(nil)}, RestartCount:0, PodIP:"", Image:"kubernetes/pause:latest"}


Vagrant synced folder
----------------

When using [vagrant synced folder](http://docs.vagrantup.com/v2/synced-folders/), (by default your
origin directory is mounted using synced folder into `/data/src/github.com/openshift/origin`) you may encounter
following errors in OpenShift log:

        E0706 11:29:43.421460    3664 empty_dir.go:322] Expected directory "/data/src/github.com/openshift/origin/openshift.local.volumes/pods/4c390e43-23d2-11e5-b42d-080027c5bfa9/volumes/kubernetes.io~secret/deployer-token-f9mi7" permissions to be: -rwxrwxrwx; got: -rwxrwxr-x
        E0706 11:29:43.421741    3664 kubelet.go:1114] Unable to mount volumes for pod "docker-registry-1-deploy_default": operation not supported; skipping pod
        E0706 11:29:43.438449    3664 pod_workers.go:108] Error syncing pod 4c390e43-23d2-11e5-b42d-080027c5bfa9, skipping: operation not supported

This will happen when using our provided Vagrantfile to develop OpenShift with vagrant. One of the reasons
is that you can't use ACLs on shared directories. The solution to this problem is to use a different directory
for volume storage than the one in synced folder. This can be achieved by passing `--volume-dir=/absolute/path` to `openshift start` command.


Must Gather
-----------
If you find yourself still stuck, before seeking help in #openshift on freenode.net, please recreate your issue with verbose logging and gather the following:

1. OpenShift logs at level 4 (verbose logging):

        $ openshift start --loglevel=4 &> /tmp/openshift.log

1. Container logs

    The following bit of scripting will pull logs for **all** containers that have been run on your system.  This might be excessive if you don't keep a clean history, so consider manually grabbing logs for the relevant containers instead:

        for container in $(docker ps -aq); do
            docker logs $container >& $LOG_DIR/container-$container.log
        done

1. Authorization rules:

    If you are getting "forbidden" messages or 403 status codes that you aren't expecting, you should gather the policy bindings, roles, and rules being used for the namespace you are trying to access.  Run the following commands, substituting `<project-namespace>` with the namespace you're trying to access.

        $ oc describe policy default --namespace=master
        $ oc describe policybindings master --namespace=master
        $ oc describe policy default --namespace=<project-namespace>
        $ oc describe policybindings master --namespace=<project-namespace>
        $ oc describe policybindings <project-namespace> --namespace=<project-namespace>
