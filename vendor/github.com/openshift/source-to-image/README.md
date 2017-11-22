# Source-To-Image (S2I)

## Overview

[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/source-to-image)](https://goreportcard.com/report/github.com/openshift/source-to-image)
[![GoDoc](https://godoc.org/github.com/openshift/source-to-image?status.png)](https://godoc.org/github.com/openshift/source-to-image)
[![Travis](https://travis-ci.org/openshift/source-to-image.svg?branch=master)](https://travis-ci.org/openshift/source-to-image)
[![License](https://img.shields.io/github/license/openshift/source-to-image.svg)](https://www.apache.org/licenses/LICENSE-2.0.html)

Source-to-Image (S2I) is a toolkit and workflow for building reproducible Docker images from source code. S2I produces
ready-to-run images by injecting source code into a Docker container and letting the container prepare that source code for execution. By creating self-assembling **builder images**, you can version and control your build environments exactly like you use Docker images to version your runtime environments.

For a deep dive on S2I you can view [this presentation](https://www.youtube.com/watch?v=flI6zx9wH6M).

Want to try it right now?  Download the [latest release](https://github.com/openshift/source-to-image/releases/latest) and run:

	$ s2i build https://github.com/openshift/django-ex centos/python-35-centos7 hello-python
	$ docker run -p 8080:8080 hello-python

Now browse to http://localhost:8080 to see the running application.

You've just built and run a new Docker application image from source code in a git repository, no Dockerfile necessary.

### How Source-to-Image works

For a dynamic language like Ruby, the build-time and run-time environments are typically the same. Starting with a **builder image** that describes this environment - with Ruby, Bundler, Rake, Apache, GCC, and other packages needed to set up and run a Ruby application installed - source-to-image performs the following steps:

1. Start a container from the builder image with the application source injected into a known directory
1. The container process transforms that source code into the appropriate runnable setup - in this case, by installing dependencies with Bundler and moving the source code into a directory where Apache has been preconfigured to look for the Ruby `config.ru` file.
1. Commit the new container and set the image entrypoint to be a script (provided by the builder image) that will start Apache to host the Ruby application.

For compiled languages like C, C++, Go, or Java, the dependencies necessary for compilation might dramatically outweigh the size of the actual runtime artifacts. To keep runtime images slim, S2I enables a multiple-step build processes, where a binary artifact such as an executable or Java WAR file is created in the first builder image, extracted, and injected into a second runtime image that simply places the executable in the correct location for execution.

For example, to create a reproducible build pipeline for Tomcat (the popular Java webserver) and Maven:

1. Create a builder image containing OpenJDK and Tomcat that expects to have a WAR file injected
2. Create a second image that layers on top of the first image Maven and any other standard dependencies, and expects to have a Maven project injected
3. Invoke source-to-image using the Java application source and the Maven image to create the desired application WAR
4. Invoke source-to-image a second time using the WAR file from the previous step and the initial Tomcat image to create the runtime image

By placing our build logic inside of images, and by combining the images into multiple steps, we can keep our runtime environment close to our build environment (same JDK, same Tomcat JARs) without requiring build tools to be deployed to production.

## Goals

### Reproducibility
Allow build environments to be tightly versioned by encapsulating them within a Docker image and defining a simple interface (injected source code) for callers. Reproducible builds are a key requirement to enabling security updates and continuous integration in containerized infrastructure, and builder images help ensure repeatability as well as the ability to swap runtimes.

### Flexibility
Any existing build system that can run on Linux can be run inside of a container, and each individual builder can also be part of a larger pipeline. In addition, the scripts that process the application source code can be injected into the builder image, allowing authors to adapt existing images to enable source handling.

### Speed
Instead of building multiple layers in a single Dockerfile, S2I encourages authors to represent an application in a single image layer. This saves time during creation and deployment, and allows for better control over the output of the final image.

### Security
Dockerfiles are run without many of the normal operational controls of containers, usually running as root and having access to the container network. S2I can be used to control what permissions and privileges are available to the builder image since the build is launched in a single container. In concert with platforms like OpenShift, source-to-image can enable admins to tightly control what privileges developers have at build time.


## Anatomy of a builder image

Creating builder images is easy. `s2i` looks for you to supply the following scripts to use with an
image:

1. `assemble` - builds and/or deploys the source
1. `run`- runs the assembled artifacts
1. `save-artifacts` (optional) - captures the artifacts from a previous build into the next incremental build
1. `usage` (optional) - displays builder image usage information

Additionally for the best user experience and optimized `s2i` operation we suggest images
to have `/bin/sh` and `tar` commands available.

See [a practical tutorial on how to create a builder image](examples/nginx-centos7/README.md) and read [a detailed description of the requirements and scripts along with examples of builder images](docs/builder_image.md).

## Build workflow

The `s2i build` workflow is:

1. `s2i` creates a container based on the build image and passes it a tar file that contains:
    1. The application source in `src`, excluding any files selected by `.s2iignore`
    1. The build artifacts in `artifacts` (if applicable - see [incremental builds](#incremental-builds))
1. `s2i` sets the environment variables from `.s2i/environment` (optional)
1. `s2i` starts the container and runs its `assemble` script
1. `s2i` waits for the container to finish
1. `s2i` commits the container, setting the CMD for the output image to be the `run` script and tagging the image with the name provided.

Filtering the contents of the source tree is possible if the user supplies a
`.s2iignore` file in the root directory of the source repository, where `.s2iignore` contains regular
expressions that capture the set of files and directories you want filtered from the image s2i produces.

Specifically:

1. Specify one rule per line, with each line terminating in `\n`.
1. Filepaths are appended to the absolute path of the  root of the source tree (either the local directory supplied, or the target destination of the clone of the remote source repository s2i creates).
1. Wildcards and globbing (file name expansion) leverage Go's `filepath.Match` and `filepath.Glob` functions.
1. Search is not recursive.  Subdirectory paths must be specified (though wildcards and regular expressions can be used in the subdirectory specifications).
1. If the first character is the `#` character, the line is treated as a comment.
1. If the first character is the `!`, the rule is an exception rule, and can undo candidates selected for filtering by prior rules (but only prior rules).

Here are some examples to help illustrate:

With specifying subdirectories, the `*/temp*` rule prevents the filtering of any files starting with `temp` that are in any subdirectory that is immediately (or one level) below the root directory.
And the `*/*/temp*` rule prevents the filtering of any files starting with `temp` that are in any subdirectory that is two levels below the root directory.

Next, to illustrate exception rules, first consider the following example snippet of a `.s2iignore` file:


```
*.md
!README.md
```


With this exception rule example, README.md will not be filtered, and remain in the image s2i produces.  However, with this snippet:


```
!README.md
*.md
```


`README.md`, if filtered by any prior rules, but then put back in by `!README.md`, would be filtered, and not part of the resulting image s2i produces.  Since `*.md` follows `!README.md`, `*.md` takes precedence.

Users can also set extra environment variables in the application source code.
They are passed to the build, and the `assemble` script consumes them. All
environment variables are also present in the output application image. These
variables are defined in the `.s2i/environment` file inside the application sources.
The format of this file is a simple key-value, for example:

```
FOO=bar
```

In this case, the value of `FOO` environment variable will be set to `bar`.

## Using ONBUILD images

In case you want to use one of the official Docker language stack images for
your build you don't have do anything extra. S2I is capable of recognizing the
Docker image with [ONBUILD](https://docs.docker.com/engine/reference/builder/#/onbuild) instructions and choosing the OnBuild strategy. This
strategy will trigger all ONBUILD instructions and execute the assemble script
(if it exists) as the last instruction.

Since the ONBUILD images usually don't provide any entrypoint, in order to use
this build strategy you will have to provide one. You can either include the 'run',
'start' or 'execute' script in your application source root folder or you can
specify a valid S2I script URL and the 'run' script will be fetched and set as
an entrypoint in that case.

### Incremental builds

`s2i` automatically detects:

* Whether a builder image is compatible with incremental building
* Whether a previous image exists, with the same name as the output name for this build

If a `save-artifacts` script exists, a prior image already exists, and the `--incremental=true` option is used, the workflow is as follows:

1. `s2i` creates a new Docker container from the prior build image
1. `s2i` runs `save-artifacts` in this container - this script is responsible for streaming out
   a tar of the artifacts to stdout
1. `s2i` builds the new output image:
    1. The artifacts from the previous build will be in the `artifacts` directory of the tar
       passed to the build
    1. The build image's `assemble` script is responsible for detecting and using the build
       artifacts

**NOTE**: The `save-artifacts` script is responsible for streaming out dependencies in a tar file.


## Dependencies

1. [docker](https://docker.com) >= 1.6
1. [Go](https://golang.org/dl/) >= 1.7.1
1. (optional) [Git](https://git-scm.com/)

## Installation

##### Using `go get`

You can install the s2i binary using `go get` which will download the source-to-image code into your `$GOPATH`, build the s2i binary, and install it into your `$GOPATH/bin`.

```$ go get github.com/openshift/source-to-image/cmd/s2i```

##### For Mac

You can either follow the installation instructions for Linux (and use the darwin-amd64 link) or you can just install source-to-image with Homebrew:

```$ brew install source-to-image```

##### For Linux

Go to the [releases](https://github.com/openshift/source-to-image/releases/latest) page and download the correct distribution for your machine. Choose either the linux-386 or the linux-amd64 links for 32 and 64-bit, respectively.

Unpack the downloaded tar with

```$ tar -xvf release.tar.gz```.

You should now see an executable called s2i.  Either add the location of s2i to your PATH environment variable, or move it to a pre-existing directory in your PATH.
For example,

```# cp /path/to/s2i /usr/local/bin```

will work with most setups.

##### For Windows

Download the latest [64-bit Windows release](https://github.com/openshift/source-to-image/releases/latest).
Extract the zip file through a file browser.  Add the extracted directory to your PATH.  You can now use
s2i from the command line.

##### From source

Assuming Go and Docker are installed and configured, execute the following commands:

```
$ go get github.com/openshift/source-to-image
$ cd ${GOPATH}/src/github.com/openshift/source-to-image
$ export PATH=$PATH:${GOPATH}/src/github.com/openshift/source-to-image/_output/local/bin/linux/amd64/
$ hack/build-go.sh
```

## Security

Since the `s2i` command uses the Docker client library, it has to run in the same
security context as the `docker` command. For some systems, it is enough to add
yourself into the 'docker' group to be able to work with Docker as 'non-root'.
In the latest versions of Fedora/RHEL, it is recommended to use the `sudo` command
as this way is more auditable and secure.

If you are using the `sudo docker` command already, then you will have to also use
`sudo s2i` to give S2I permission to work with Docker directly.

Be aware that being a member of the 'docker' group effectively grants root access,
as described [here](https://github.com/docker/docker/issues/9976).

## Getting Started

You can start using `s2i` right away (see [releases](https://github.com/openshift/source-to-image/releases))
with the following test sources and publicly available images:

```
$ s2i build https://github.com/openshift/ruby-hello-world centos/ruby-23-centos7 test-ruby-app
$ docker run --rm -i -p :8080 -t test-ruby-app
```

```
$ s2i build --ref=10.x --context-dir=helloworld https://github.com/wildfly/quickstart openshift/wildfly-101-centos7 test-jee-app
$ docker run --rm -i -p 8080:8080 -t test-jee-app
```

Want to know more? Read the following resources:

* [Descriptions and examples of the S2I commands](docs/cli.md)
* [Instructions for using builder images](docs/user_guide.md)
* [Guidance for S2I builder image creators](docs/builder_image.md)
* [Using a non-builder image for the base of the application image](docs/runtime_image.md)
* [Troubleshooting and debugging S2I problems](docs/debugging-s2i.md)
