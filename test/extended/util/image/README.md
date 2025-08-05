# Images used by e2e tests

We limit the set of images used by conformance e2e to reduce duplication and to allow us to provide offline mirroring of images for customers and restricted test environments. Every image used in e2e must be part of this utility package or referenced by the upstream `k8s.io/kubernetes/test/utils/image` package.

All images used by e2e tests that are part of the `conformance` suite are mirrored to:

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
   * Reference your image inside your tests tests using `github.com/openshift/origin/test/extended/util/image.LocationFor("my.source/image/location:versioned_tag")`
   * Regenerate the verify output with `make update` and compare the diff to see your source image located
   * Ensure your tests fail with "image cannot be pulled" errors.
   * Have the reviewer promote the image to the quay mirror location.
   * The reviewer should approve the PR and merge.

When adding a new image, first make the code changes and compile the `openshift-tests` binary. Then run `hack/update-generated-bindata.sh` to update `test/extended/util/image/zz_generated.txt`. Contact one of the OWNERS of this directory and have them review the image for inclusion into our suite (usually granted in the process above). Before merge and after review they will run the following command to mirror the content to quay:

    OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests images --upstream --to-repository quay.io/openshift/community-e2e-images | oc image mirror -f - --filter-by-os=.*

Note: The `registry.k8s.io/pause:3.9` image (and possibly others) contains uncompressed layers which quay.io does not allow.  The `oc image mirror` command always
mirrors the layers as is and thus fails to mirror that image.  You can use skopeo instead which will successfully mirror the image, but changes the 
digests due to switching the layer format.  This command will mirror the `registry.k8s.io/pause:3.9` image and can be adapted for other images as needed:

    skopeo copy --all --format oci docker://registry.k8s.io/pause:3.9 docker://quay.io/openshift/community-e2e-images:e2e-27-registry-k8s-io-pause-3-9-p9APyPDU5GsW02Rk

To become an OWNER in this directory you must be given permission to push to this repo by another OWNER.

### Kube exceptions:

* `webserver:404` - used to access an image that does not exist
* `gcr.io/google_samples/gb-redisslave:nonexistent` - used to access an auth protected upstream image
* `gcr.io/authenticated-image-pulling/alpine:3.7` - used to access an image that is authenticated and verify that pulls fail or succeed, cannot be mirrored
* `invalid.com/invalid/alpine:3.1`

### OpenShift exceptions

* Temporary
  * `docker.io/summerwind/h2spec:2.4.0` - for HTTP2 testing, waiting for mirror

## When rebasing

When a new version of Kubernetes is introduced new images will likely need to be mirrored. The failure will be pods that fail to start, usually with an ImagePullBackoff error (the image isn't available at `quay.io/openshift/community-e2e-images:*`).

1. Perform a rebase of openshift/origin and open a pull request
2. Observe whether any tests fail due to missing images
3. Notify an OWNER in this file, who will run:

        OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests images --upstream --to-repository quay.io/openshift/community-e2e-images | oc image mirror -f - --filter-by-os=.*

  Note: see above information about using skopeo to mirror images that contain uncompressed layers, such as the `pause` image.

4. Retest the PR, which should pass or identify new failures
5. If an upstream image is removed that OpenShift tests depend on, those tests should be refactored to use the appropriate equivalent.

Step 3 only has to be run once per new image version introduced in a test.


## When reviewing

We control images so that we are confident that if a user ran the tests binary in a controlled and protected offline environment that we are not introducing excessive risk for the user by running the tests (which run privileged). That means:

* Using images that are reproducible - can be updated if a security vulnerability is found
* Using images that are published to a secured location - a malicious third party shouldn't be able to trivially take over the location the image is published to to inject an invalid tag
* Using images that are versioned - `latest` or rolling tags where the API of the image can be broken MUST NOT be allowed, because then a future mirror might regress old tests in old versions

Kubernetes has a working process that we consider acceptable for upstream images documented at https://github.com/kubernetes/kubernetes/blob/master/test/images/README.md - images maintained by other communities likely do not satisfy these criteria and must be reviewed with these criteria in mind.

OpenShift test images must be built via CI and published to quay in a versioned fashion (no regressions).

New images should be added when:

1. An upstream component refactors to use a different image
  1. Ask whether the upstream image is a better image (i.e. is it better managed, more generic, well built, kept up to date by some process)
2. A new test is added and needs an image AND none of the existing images are sufficient AND none of the existing images can be extended to solve it
  1. I.e. agnhost is a generic tool for simulating clients inside a pod, and so it is better to use that function OR extend it than adding a separate test simulation
  2. The shell image is the ultimate catch all - ANY bash code that isn't wierd should use that.  If the bash code needs a novel new command we should add it to the `tools` image (which shell image points to) if it matches the criteria for tools (small Linux utilities that are useful for debugging an openshift cluster / node that are likely to be useful in a wide range of areas)
  3. Don't introduce new versions of an existing image unless there is no choice - i.e. if you need `redis` and are not testing a specific version of redis, just use the existing image

### Mirroring images for approved changes before the PR is merged

In order to merge the PR, the tests have to pass, which means the new image has to be mirrored prior to merge.

When mirroring from a PR (granting access), you should check out the PR in question and build locally. You should probably rebase the local PR to ensure you don't stomp changes in master (checking out a PR doesn't exactly match what is tested).

Then run

    OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests images --upstream --to-repository quay.io/openshift/community-e2e-images

to verify that all things check out. If everything looks good, run

    OPENSHIFT_SKIP_EXTERNAL_TESTS=1 ./openshift-tests images --upstream --to-repository quay.io/openshift/community-e2e-images | oc image mirror -f - --filter-by-os=.*

You must be logged in (to docker, using `oc registry login --registry=quay.io` or `skopeo login` or `docker login`) to a quay account that has write permission to `quay.io/openshift/community-e2e-images` which every OWNER should have.
