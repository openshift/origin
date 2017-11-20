# How to use a non-builder image for the final application image

## Overview

For dynamic languages like PHP, Python, or Ruby, the build-time and run-time environments are the same. In this case using the builder as a base image for a resulting application image is natural.

For compiled languages like C, C++, Go, or Java, the dependencies necessary for compilation might dramatically outweigh the size of the actual runtime artifacts, or provide attack surface areas that are undesirable in an application image. To keep runtime images slim, S2I enables a multiple-step build processes, where a binary artifact such as an executable or Java WAR file is created in the first builder image, extracted, and injected into a second image that simply places the executable in the correct location for execution. To give you even more abilities to customize a resulting image, S2I is also executing an `assemble-runtime` script inside of this (runtime) image. In this way you may do final adjustments by modifying files before an image will be committed.

The following diagram illustrates the build workflow:

![s2i workflow](./runtime-image-flow.png "s2i workflow")


## How it works

To make this work S2I needs to know the following:

* builder image
* mapping of the artifacts
* runtime image

This information is specified during S2I invocation:

    s2i build <repo> <builder-image> <my-app> --runtime-image <runtime-image> --runtime-artifact </path/to/artifact>

The only arguments here are the `--runtime-image` and `--runtime-artifact` options. The first option specifies the image that will be used as the base image for the final application image. The second option specifies a full path to a file (or directory) that will exist in the builder container after assembly and will be copied into the `WORKDIR` of the runtime container.

For our example S2I will do the following steps:

1. run a builder container and invoke the assemble script (as usual)
1. after the builder finishes but before stopping the builder container, download the requested artifacts from the builder and place them in a temporary directory on the host
1. start a container using the specified runtime image
1. upload the scripts and build artifacts from the temporary directory into `WORKDIR` of the runtime container
1. run the `assemble-runtime` script in the runtime container
1. commit the runtime container and tag it as the new application image

## Details

### Format and behavior of the `--runtime-artifact` option

`--runtime-artifact` option (or its short equivalent `-a`) have the format `<source>[:<destination>]`. Here are some example values:

| Value                   | Meaning |
|-------------------------|---------|
| `/tmp/app.war`          | `/tmp/app.war` file will be extracted from the builder container and uploaded into the `WORKDIR` of the runtime container |
| `/tmp/app.war:.`        | the same as above |
| `/tmp/app.war:app/dist` | `/tmp/app.war` file will be copied from the builder container into the `app/dist` subdirectory of the `WORKDIR` of the runtime container |
| `/tmp/app-0.1.war:app.war` | `/tmp/app-0.1.war` file will be uploaded into the *`app.war` subdirectory* of the `WORKDIR` of the runtime container |
| `/opt/data`             | `/opt/data` directory will be copied from the builder container into the `WORKDIR` of the runtime container |
| `/opt/data/`            | the same as above |
| `/opt/data/*.jar`       | invalid mapping because wildcards are not supported. The build will fail |

You can specify this option multiple times (for example, `-a /first/artifact -a /second/artifact`).

The `source` must be an absolute path. The `destination` must be a relative path and it must not start with `..` Because `destination` is always a **path to a directory**, it is impossible to rename artifacts during copying, you only able to choose where S2I will create this file.

When copying the artifacts, S2I will modify their permissions. All directories and files with executable bit will be uploaded with `0755` mode. Other files will have `0644` mode.

### `assemble-runtime` script requirements

`assemble-runtime` can be any executable script or binary. S2I searches the following locations for this script in the following order:

1. A script found at the `--scripts-url` URL
1. A script found in the application source `.s2i/bin` directory
1. A script found at the default image URL (`io.openshift.s2i.scripts-url` label)

The `assemble-runtime` script is always executed as the runtime image `USER`.

### Runtime image requirements

In most cases you can use any image as a runtime image. However, if you are using the `--allowed-uids` option then the image must have a numeric `USER` specified and the value must be within the range of allowed user ids.

To simplify the build workflow and provide some reasonable defaults, the author of the runtime image can use the following techniques:

* `run` and `assemble-runtime` scripts can be placed inside of the runtime image. Scripts from the image will be used as a fallback when user does not provide them in the `.s2i/bin` directory of the source repository and an alternative location is not specified with the `--scripts-url` option. The location of the scripts is defined by the value of the `io.openshift.s2i.scripts-url` label that should be presented on the image. For example, you can set it to `image:///usr/libexec/s2i`
* default mapping for the files can be specified by adding the `io.openshift.s2i.assemble-input-files` label to the runtime image. This mapping will be used as a fallback when the user does not specify the artifacts explicitly with the `--runtime-artifact` option.

To specify mappings for multiple files, separate them with a semicolon. For example: `/tmp/app.war:app;/opt/data`

### Build and runtime environments

Builder and runtime containers have the same environment. In other words `assemble` and `assemble-runtime` scripts are able to use environment variables defined with `--env` and `--environment-file` options along with the values from `.s2i/environment` file in the source repository.

### Extended build and incremental build

In the current implementation it is not possible to do an extended incremental build. This combination is invalid and the build will fail.
