Openshift 3 Application Docker Registry Sample
==============================================

This is a set of configuration files and scripts which create a docker registry
inside Openshift 3 and than perform application build and push to the registry.

This example assumes you have successfully built the openshift binary executable
and have docker installed/working. See https://github.com/openshift/origin.

Quickstart
----------
To quickly exercise the environment, you can run:

    $ ./run.sh

This will:

1) Launch the openshift server, logs are in `openshift.log`.
2) Submit the `registry/docker-registry.json` config to create a docker registry
that will use local `/tmp/docker` directory for storing its files.
3) Confirm the registry is running by listing its contents.
4) Create a build config.
5) Trigger a new build of the application.
7) Show the contents of the docker repository.

To reset your system after running this example, you can run:

    $ ./cleanup.sh

This will stop the openshift process, remove the etcd storage, and kill all docker
 containers running on your host system.  (**Use with caution!**   Docker
 containers unrelated to openshift will also be killed by this script)

