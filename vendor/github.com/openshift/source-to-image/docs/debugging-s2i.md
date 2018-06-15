
Troubleshooting
=================

This document contains some tips and suggestions for troubleshooting the use of source-to-image ("s2i") to build Docker images.

Can't Find The `s2i` Executable
--------------

Whether by  running `make build` or `hack/build-release.sh`, look under the `_output/local/go/bin` directory.

If you have downloaded one of the official releases (see [releases](https://github.com/openshift/source-to-image/releases)), the `s2i` executable is the only file in the tar archive.

Security And Docker
--------------

As noted [here](https://github.com/openshift/source-to-image/#security), there are considerations to allow S2I to interact with Docker, including possibly running `sudo s2i`.

Image Mechanics
--------------

By default, s2i will use the local builder image if available on the host from
which you are running the `s2i` command. However, if you want to always pull
the image from the remote Docker repository to be sure that you are not using
stale and out of date image you should provide `--pull-policy=always` option
when running the `s2i` command.

Permissions Needed During the S2I Process
--------------

The s2i process will leverage the `/tmp` directory at various stages, such as when cloning the source repository, when augmenting the Dockerfile for layered builds, during various `tar` operations that are performed, and when executing the scripts you have provided.  Invalid permissions will result in a message like this:

     E0720 21:07:17.145257 04202 main.go:328] An error occurred: creating temporary directory  failed

Hence, the user ID you execute the `s2i` command with must have the appropriate permission to read, create, and remove files under `/tmp`.

Passing In Environment Variables
--------------

If you are passing in environment variables with the -e option from the command line, and those environment variables have values with commas in them, Go command line processing will break.

       $ s2i build --loglevel=5 -e "MAVEN_ARGS='-P some-repo,some-other-repo'" file:///home/mfojtik/sandbox/ruby openshift/ruby-20-centos7 foo
       	 I0519 14:20:51.410089 17228 util.go:34] Using my@cred.sk credentials for pulling openshift/ruby-20-centos7
     	 I0519 14:20:51.410222 17228 main.go:239] An error occurred: malformed env string: some-other-repo'

To deal with this behavior, s2i by default will look for environment variables in the file `.s2i/environment` in your source repository.  You can also point to a separate file with environment variable settings with the -E option.
With both approaches, leveraging a file instead of the command line allows for the values of environment variable to contain commas.  Environment variable processing is described in the [README](https://github.com/openshift/source-to-image#anatomy-of-a-builder-image) as well.

With the above example, whichever file you leverage for environment variables would have this line:


```
MAVEN_ARGS='-P some-repo,some-other-repo'
```


Providing S2I Scripts
--------------

First, a basic reminder:  you should verify your scripts have executable permissions.

Then, a few of the trickier obstacles that have arisen for users in the past:

#### Interfacing With `tar`

As noted [here](https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md#save-artifacts), if you employ a `save-artifacts` script, that script is responsible for properly receiving tar streams.  Issues with `tar` stream processing could result in messages like:

	      W0720 21:07:52.145257 04204 sti.go:131 Clean build will be performed because of error saving previous build artifacts
	      E0720 21:07:52.145263 04204 sti.go:133] ERROR: timeout waiting for tar stream


Review the example [here](https://github.com/gabemontero/source-to-image/blob/master/docs/builder_image.md#save-artifacts), and perhaps revisit the tar man pages or the bash user manual, to help address any problems.


#### Dowloading / Finding The Scripts

Per this discussion point  [here](https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md#s2i-scripts), if your s2i related scripts are already placed in the image, but their locations are not indicated by one of the provided means, attempts to download the scripts will fail.

   	E0720 21:08:37.166063 04204 main.go:328] An error occurred: scripts inside the image:  image:///path/to/image

Properly reference the script location with the methods described in the above link.


#### ONBUILD

At various points when executing ONBUILD instructions (these are defined in the Dockerfile of the builder image you are using with s2i), if those instructions result in the need for root user access, but your builder image is not configured to run as root,
then attempts to reference that image in another Dockerfile will result in permission errors.

If you consider the following Dockerfile pseudo-example:

```
FROM foo
USER root
RUN some-root-operation
ONBUILD some-root-operation
USER joe
```

You will be able to Docker build the image, but when you then reference that image in another Dockerfile, things will fail because the ONBUILD commands will execute as "joe" and not "root".

Must Gather
-----------
If you find yourself still stuck, before seeking help in #openshift on freenode.net, please recreate your issue and gather the following:

1. s2i logs at level 5 (verbose logging):

        $ s2i < other options >  --loglevel=5 &> /tmp/s2i.log

2. Container logs

    The following bit of scripting will pull logs for **all** containers that have been run on your system.  This might be excessive if you don't keep a clean history, so consider manually grabbing logs for the relevant containers instead:

        for container in $(docker ps -aq); do
            docker logs $container >& $LOG_DIR/container-$container.log
        done

3. By default, the working directory under `/tmp` is removed when the `s2i` command completes.  The `--save-temp-dir=true` option will preserve the working directory under `/tmp`.  Those files can often provide useful diagnostics.



Debugging Integration Test Failures
--------------

For diagnosing failures when running hack/test-integration.sh, passing the -v option to the hack/test-integration.sh script will not only turn on verbose bash tracing, but will set --loglevel=5 tracing for the S2I internals, in
order to provide additional details that can help diagnose issues.
