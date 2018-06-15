# s2i command line interface

This document describes thoroughly all `s2i` subcommands and flags with explanation
of their purpose as well as an example usage.

Currently `s2i` has five subcommands, each of which will be described in the
following sections of this document:

* [create](#s2i-create)
* [build](#s2i-build)
* [rebuild](#s2i-rebuild)
* [usage](#s2i-usage)
* [version](#s2i-version)
* [help](#s2i-help)

Before diving into each of the aforementioned commands, let's have a closer look
at common flags that can be used with all of the subcommands.

| Name                       | Description                                             |
|:-------------------------- |:--------------------------------------------------------|
| `-h (--help)`              | Display help for the specified command |
| `--loglevel`               | Set the level of log output (0-5) (see [Log levels](#log-levels))|
| `-U (--url)`               | URL of the Docker socket to use (default: `unix:///var/run/docker.sock`) |

#### Log levels

There are six log levels:
* Level `0` - produces output from containers running `assemble` and `assemble-runtime` scripts and all encountered errors
* Level `1` - produces basic information about the executed process
* Level `2` - produces very detailed information about the executed process
* Level `3` - produces very detailed information about the executed process, along with listing tar contents
* Level `4` - currently produces same information as level `3`
* Level `5` - produces very detailed information about the executed process, lists tar contents, Docker Registry credentials, and copied source files

**NOTE**: All of the commands and flags are case sensitive!

# s2i create

The `s2i create` command is responsible for bootstrapping a new S2I enabled
image repository. This command will generate a skeleton `.s2i` directory and
populate it with sample S2I scripts you can start hacking on.

Usage:

```
$ s2i create <image name> <destination directory>
```

# s2i build

The `s2i build` command is responsible for building the Docker image by combining
the specified builder image and sources. The resulting image will be named according
to the tag parameter.

Usage:
```
$ s2i build <source location> <builder image> [<tag>] [flags]
```
The build command parameters are defined as follows:

1. `source location` - the URL of a Git repository or a local path to the source code
1. `builder image` - the Docker image to be used in building the final image
1. `tag` - the name of the final Docker image (if provided)

If the build image is compatible with incremental builds, `s2i build` will look for
an image tagged with the same name. If an image is present with that tag and a
`save-artifacts` script is present in the scripts directory, `s2i build` will save the build artifacts from
that image and add them to the tar streamed to the container into `/artifacts`.

#### Build flags

| Name                       | Description                                             |
|:-------------------------- |:--------------------------------------------------------|
| `--callback-url`           | URL to be invoked after a build (see [Callback URL](#callback-url)) |
| `-c (--copy)`              | Use local file system copy instead of git cloning the source url (allows for inclusion of empty directories and uncommitted files) |
| `-d (--destination)`       | Location where the scripts and sources will be placed prior doing build (see [S2I Scripts](https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md#s2i-scripts)) |
| `--dockercfg-path`         | The path to the Docker configuration file |
| `--incremental`            | Try to perform an incremental build |
| `-e (--env)`               | Environment variable to be passed to the builder eg. `NAME=VALUE` |
| `-E (--environment-file)`  | Specify the path to the file with environment |
| `--exclude`  | Regular expression for selecting files from the source tree to exclude from the build, where the default excludes the '.git' directory (see https://golang.org/pkg/regexp for syntax, but note that \"\" will be interpreted as allow all files and exclude no files) |
| `-p (--pull-policy)`       | Specify when to pull the builder image (`always`, `never` or `if-not-present`. Defaults to `if-not-present`) |
| `--run`                    | Launch the resulting image after a successful build. All output from the image is being printed to help determine image's validity. In case of a long running image you will have to Ctrl-C to exit both s2i and the running container.  (defaults to false) |
| `-r (--ref)`               | A branch/tag that the build should use instead of MASTER (applies only to Git source) |
| `--rm`                     | Remove the previous image during incremental builds |
| `--save-temp-dir`          | Save the working directory used for fetching scripts and sources |
| `--context-dir`            | Specify the directory containing your application (if not located within the root path) |
| `-s (--scripts-url)`       | URL of S2I scripts (see [S2I Scripts](https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md#s2i-scripts)) |
| `--ignore-submodules`      | Ignore all git submodules when cloning application repository. (defaults to false)|
| `-q (--quiet)`             | Operate quietly, suppressing all non-error output |
| `-i (--inject)`            | Inject the content of the specified directory into the path in the container that runs the assemble script |
| `-v (--volume)`            | Bind mounts a local directory into the container that runs the assemble script|
| `--runtime-image`          | Image that will be used as the base for the runtime image (see [How to use a non-builder image for the final application image](https://github.com/openshift/source-to-image/blob/master/docs/runtime_image.md)) |
| `-a (--runtime-artifact)`  | Specify a file or directory to be copied from the builder to the runtime image  (see [How to use a non-builder image for the final application image](https://github.com/openshift/source-to-image/blob/master/docs/runtime_image.md)) |

#### Context directory

In the case where your application resides in a directory other than your repository root
folder, you can specify that directory using the `--context-dir` parameter. The
specified directory will be used as your application root folder.

#### Injecting directories to build

If you want to inject files that should only be available during the build (ie
when the assemble script is invoked), you can specify the directories from which
the files will be copied into the container that runs the assemble script. To do
so you can invoke S2I as follows:

```console
$ s2i build --inject /mydir:/container/dir file://source builder-image output-image
```

In this case the content of the `/mydir` directory will get copied into
`/container/dir` inside the container which runs the assemble script.
After the `assemble` script finishes, all files copied will be truncated and thus
not available in the output image. The files are truncated instead of deleted
because the user under which we run the container with the assemble script might not
have permissions to delete files in the destination directory (eg. `/etc/ssl`).

You can also specify multiple directories, for example: `--inject /dir1:/container/dir1 --inject /dir2:container/dir2`.

You can use this feature to provide SSL certificates, private configuration
files which contains credentials, etc.

#### Callback URL

Upon completion (or failure) of a build, `s2i` can execute a HTTP POST to a URL with information
about the build:

* `success` - flag indicating the result of the build process (`true` or `false`)
* `labels`  - labels of the resulting image

Example: data posted will be in the form:
```
{
    "success": true,
    "labels": {
        "io.k8s.display-name": "my-app",
        "io.openshift.s2i.build.image": "builder-image:latest",
        ...
    }
}
```

#### Example Usage

Build a Ruby application from a Git source, using the official `ruby-23-centos7` builder
image, the resulting image will be named `ruby-app`:

```
$ s2i build https://github.com/openshift/ruby-hello-world openshift/ruby-23-centos7 ruby-app
```

Build a Node.js application from a local directory, using a local image, the resulting
image will be named `nodejs-app`:

```
$ s2i build /home/user/nodejs-app local-nodejs-builder nodejs-app
```

In case of building from the local directory, the sources will be copied into
the builder images using plain filesystem copy if the Git binary is not
available. In that case the output image will not have the Git specific labels.
Use this method only for development or local testing.

**NOTE**: All your changes have to be committed by `git` in order to build them with S2I.

Build a Java application from a Git source, using the official `openshift/wildfly-101-centos7`
builder image but overriding the scripts URL from local directory.  The resulting
image will be named `java-app`:

```
$ s2i build --scripts-url=file://s2iscripts --ref=7.1.x --context-dir=kitchensink https://github.com/jboss-developer/jboss-eap-quickstarts openshift/wildfly-101-centos7 java-app
```

Build a Ruby application from a Git source, specifying `ref`, and using the official
`ruby-23-centos7` builder image.  The resulting image will be named `ruby-app`:

```
$ s2i build --ref=my-branch https://github.com/openshift/ruby-hello-world openshift/ruby-23-centos7 ruby-app
```

***NOTE:*** If the ref is invalid or not present in the source repository then the build will fail.

Build a Ruby application from a Git source, overriding the scripts URL from a local directory,
and specifying the scripts and sources be placed in `/opt` directory:

```
$ s2i build --scripts-url=file://s2iscripts --destination=/opt https://github.com/openshift/ruby-hello-world openshift/ruby-23-centos7 ruby-app
```

# s2i rebuild

The `s2i rebuild` command is used to rebuild an image already built using S2I,
or the image that contains the required S2I labels.
The rebuild will read the S2I labels and automatically set the builder image,
source repository and other configuration options used to build the previous
image according to the stored labels values.

Optionally, you can set the new image name as a second argument to the rebuild
command.

Usage:

```
$ s2i rebuild <image name> [<new-tag-name>]
```


# s2i usage

The `s2i usage` command starts a container and runs the `usage` script which prints
information about the builder image. This command expects `builder image` name as
the only parameter.

Usage:
```
$ s2i usage <builder image> [flags]
```

#### Usage flags

| Name                       | Description                                             |
|:-------------------------- |:--------------------------------------------------------|
| `-d (--destination)`       | Location where the scripts and sources will be placed prior invoking usage (see [S2I Scripts](https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md#s2i-scripts))|
| `-e (--env)`               | Environment variable passed to the builder eg. `NAME=VALUE`) |
| `-p (--pull-policy)`       | Specify when to pull the builder image (`always`, `never` or `if-not-present`) |
| `--save-temp-dir`          | Save the working directory used for fetching scripts and sources |
| `-s (--scripts-url)`       | URL of S2I scripts (see [Scripts URL](https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md#s2i-scripts))|

#### Example Usage

Print the official `ruby-23-centos7` builder image usage:
```
$ s2i usage openshift/ruby-23-centos7
```


# s2i version

The `s2i version` command prints the version of S2I currently installed.


# s2i help

The `s2i help` command prints help either for the `s2i` itself or for the specified
subcommand.

### Example Usage

Print the help page for the build command:
```
$ s2i help build
```

***Note:*** You can also accomplish this with:
```
$ s2i build --help
```
