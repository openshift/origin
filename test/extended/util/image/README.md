# Images used by e2e tests

We limit the set of images used by e2e to reduce duplication and to allow us to provide offline mirroring of images for customers and restricted test environments. Every image used in e2e must be part of this utility package or referenced by the upstream `k8s.io/kubernetes/test/utils/image` package.

All images used by e2e are mirrored to:

  quay.io/openshift/community-e2e-images:TAG

for easy mirroring by the `openshift-tests images` command.

## To add a new image:

1. Identify whether your use case can be solved by an existing image:
   * The standard "shell" image provided by `ShellImage()` that is available on all clusters and has bash + a set of standard tools available. Use this by default.
   * An upstream image that is provided by `test/utils/image`.
2. If your use case is novel, you must:
   * Describe your use case in a PR to this file and have it approved
   * Define a standard CI build and publish the image to quay.io in the openshift namespace
   * Add the new reference to the `init()` method in this package
   * Have the automation promote the image to the quay mirror location.

When adding a new image, first make the code changes and compile the `openshift-tests` binary. Then run `hack/update-generated-bindata.sh` to update `test/extended/util/image/zz_generated.txt`. Contact one of the OWNERS of this directory and have them review the image for inclusion into our suite (usually granted in the process above). Before merge and after review they will run the following command to mirror the content to quay:

    openshift-tests images --upstream --to-repository quay.io/openshift/community-e2e-images | oc image mirror -f - --filter-by-os=.*

To become an OWNER in this directory you must be given permission to push to this repo by another OWNER.

### Kube exceptions:

* `webserver:404` - used to access an image that does not exist
* `gcr.io/google_samples/gb-redisslave:nonexistent` - used to access an auth protected upstream image
* `gcr.io/authenticated-image-pulling/alpine:3.7` - used to access an image that is authenticated and verify that pulls fail or succeed, cannot be mirrored
* `invalid.com/invalid/alpine:3.1`

### OpenShift exceptions

* `docker.io/library/registry:2.7.1` - used to imitate 3rd-party registries
* Temporary
  * `docker.io/summerwind/h2spec:2.4.0` - for HTTP2 testing, waiting for mirror

## When rebasing

When a new version of Kubernetes is introduced new images will likely need to be mirrored. The failure will be pods that fail to start, usually with an ImagePullBackoff error (the image isn't available at `quay.io/openshift/community-e2e-images:*`).

1. Perform a rebase of openshift/origin and open a pull request
2. Observe whether any tests fail due to missing images
3. Notify an OWNER in this file, who will run:

        openshift-tests images --upstream --to-repository quay.io/openshift/community-e2e-images | oc image mirror -f - --filter-by-os=.*

4. Retest the PR, which should pass or identify new failures
5. If an upstream image is removed that OpenShift tests depend on, those tests should be refactored to use the appropriate equivalent.

Step 3 only has to be run once per new image version introduced in a test.
