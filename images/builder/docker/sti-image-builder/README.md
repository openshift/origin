openshift/sti-image-builder
============================

This image is used as a builder image for all STI images. It is used as part of
a [CustomBuild](https://github.com/openshift/origin/blob/master/docs/builds.md#custom-builds).

The sample CustomBuild JSON might look as following:

```json
{
  "kind": "BuildConfig",
  "apiVersion": "v1beta1",
  "metadata":{
    "name": "ruby-20-centos-build"
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
        "uri": "git://github.com/openshift/ruby-20-centos.git"
      }
    },
    "strategy": {
      "type": "Custom",
      "customStrategy": {
        "image": "openshift/sti-image-builder",
        "exposeDockerSocket": true,
        "env": [
          { "name": "IMAGE_NAME", "value": "openshift/ruby-20-centos"}
        ]
      }
    }
  },
  "labels": {
    "name": "ruby-20-centos-build"
  }
}

```
