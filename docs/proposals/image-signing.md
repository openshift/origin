# Image Signing

## Abstract

This proposal describes the process of verifying the signatures of Docker Images and
enforce enforce policy that will prevent the image from running in the cluster if the
image signature is not trusted.

## Use-cases

* As an admin, I want to be able to add trusted GPG public keys to my cluster and determine through policy
  the keys that are required to run certain images
* As an user, I want to be able to see what images are signed and if the signature is trusted

## Overview

### Signing Images with Skopeo

The current signing mechanism for Docker image is to use the
[Skopeo](https://github.com/projectatomic/skopeo/blob/master/docs/skopeo.1.md) tool which
will sign the Docker image locally, using the GPG key and attach the signature to the
Docker Image. The Docker image is then pushed to a Docker Registry with enabled signature
storage.

Signature verification [policy
files](https://github.com/containers/image/blob/master/docs/policy.json.md) are used to
define policy, e.g. trusted keys, applicable when deciding whether to accept an image, or
individual signatures of that image, as valid:

```json
{
  "default": [{"type": "reject"}],
  "transports": {
    "atomic": {
      "172.30.62.231:5000": [
      {
        "type": "signedBy",
        "keyType": "GPGKeys",
        "keyPath": "/keys/public.gpg"
      }
      ]
    },
    "docker": {
      "docker.io/openshift": [{"type": "insecureAcceptAnything"}]
    }
  }
}
```

In this example, the default policy is set to reject all Docker images by default and
whitelist only images that are pushed in the OpenShift Integrated Registry and signed
using the public GPG key.
It also whitelists all Docker images from the OpenShift DockerHub repository.

To sign an Docker image, you have to execute the following command:

```console
$ skopeo --tls-verify=false copy --sign-by user@example.com docker://docker.io/myrepo/image:latest \
  atomic:172.30.74.246:5000/mynamespce/image:latest
```

When this command is executed, skopeo will ask you to enter the password the
*user@example.com* GPG key, sign the local image and push the signed image into OpenShift
Integrated Registry.

### Kubernetes `ImagePolicyWebhook` Admission Controller

Kubernetes provides the
[ImagePolicyWebhook](http://kubernetes.io/docs/admin/admission-controllers/#imagepolicywebhook)
to allow a backend webhook to make admission decisions about whether an image specified by
the container inside a Pod is allowed to run in the cluster or not.

To configure this you have to specify a backed HTTPS service that Kubernetes will send the
`ImageReview` objects as a payload, expecting the `ImageReview` in return with the
*status* field set to indicate allowance or rejection of the image(s).

## Assumptions and Constraints

The assumption is that we provide a HTTPS backend that implements the `ImagePolicyWebhook`
by making calls into Skopeo [go library](https://github.com/containers/image) to evaluate
the policy.json file provided by the cluster admin.

Kubernetes requirements for the ImagePolicyWebhook backend are:

* The admission controller config file (`--admission-controller-config-file`) to set configuration options for
  the behavior of the backend
* The admission config file must reference a kubeconfig formatted file which sets up the connection
  to the backend
* The backend must communicate over TLS

The image verification backend requirements are:

* Policy file must be provided
* All public GPG keys must be available for the the backend server
* The backend must have view access to all images in the registry specified in policy file

The assumption is then that we will run the backend HTTPS server as a part of OpenShift
infrastructure, similar to how we run Docker Registry or the HAProxy router.

### Policy

The policy must be provided in order for Skopeo to match the provided image with the rules
and public GPG keys. There are couple possibilities how the policy file might be provided
to the backend server:

* A `ConfigMap` provided by the cluster administrator with custom policies set based on
  the cluster configuration.
* A `ConfigMap` generated automatically based on the annotations in Image* objects (TBD)

The ConfigMap approach benefits are that we don't have 'standartize' the `policy.json`
file in any way in our API. The drawback is that there are no tools to automatically
generate this file and thus admins will have to write the JSON file manually.

### Public GPG Keys

The GPG keys can be provided in similar fashion using the `ConfigMap` that contains the
policy or a different `ConfigMap`. Since the GPG keys stored here are public, there is no
need to make them "secret".

Since the "policy.json" file specifies them using filesystem path, there are benefits in
having them in shared `ConfigMap` with the policy.json file as they can be specified
relatively.

### Registry Authentication

If the policy file specifies a rule that is scoped globally for the entire cluster, the
backend needs to run as a user that have "registry-viewer" role granted.
The `.docker/config.json` files has to be mounted into the Pod that runs the backend
server, in similar fashion we mount it for the builds.
