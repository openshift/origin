openshift/sti-image-builder
============================

This image is used as a builder image for all STI images. It is used as part of
a [CustomBuild](https://github.com/openshift/origin/blob/master/docs/builds.md#custom-builds).

The following list of environment variables is used in the STI build:

| Name        | Description                  | Default  |
| ----------- |:----------------------------:|----------|
| IMAGE_NAME  | The output Docker image name | required |
| CONTEXT_DIR | Relative path to Dockerfile  | "."      |

The sample custom BuildConfig definition might look as following:

```json
{
  "kind": "BuildConfig",
  "apiVersion": "v1beta1",
  "metadata":{
    "name": "ruby-20-centos7-build"
  },
  "triggers": [
    {
      "type": "github",
      "github": {
        "secret": "secret101"
      }
    }
  ],
  "parameters": {
    "source" : {
      "type" : "Git",
      "git" : {
        "uri": "git://github.com/openshift/sti-ruby.git"
      }
    },
    "strategy": {
      "type": "Custom",
      "customStrategy": {
        "image": "openshift/sti-image-builder",
        "exposeDockerSocket": true,
        "env": [
          { "name": "IMAGE_NAME", "value": "openshift/ruby-20-centos7"}
          { "name": "CONTEXT_DIR", "value": "/2.0/"}
        ]
      }
    },
    "output": {
      "to": "ruby-20-centos7-repository",
      "tag": "latest",
    },
  },
  "labels": {
    "name": "ruby-20-centos7-build"
  }
}

```

This example will trigger a build for the 'ruby-20-centos7' every time there is a
push into its GitHub repository. It will set the name of the resulting image to
"openshift/ruby-20-centos7" and it expects the Dockerfile to be present in the
root directory of the GIT repository.

After a successful build, the 'openshift/ruby-20-centos7' image will be pushed
into "ruby-20-centos7-repository" ImageRepository and tagged as 'latest'.
