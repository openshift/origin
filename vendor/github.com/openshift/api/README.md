# api
The canonical location of the OpenShift API definition.  This repo holds the API type definitions and serialization code used by [openshift/client-go](https://github.com/openshift/client-go)

## pull request process

Pull requests that change API types in this repo that have corresponding "internal" API objects in the 
[openshift/origin](https://github.com/openshift/origin) repo must be paired with a pull request to
[openshift/origin](https://github.com/openshift/origin).

To ensure the corresponding origin pull request is ready to merge as soon as the pull request to this repo is merged:
1. Base your pull request to this repo on latest [openshift/api#master](https://github.com/openshift/api/commits/master) and ensure CI is green
2. Base your pull request to openshift/origin on latest [openshift/origin#master](https://github.com/openshift/origin/commits/master)
3. In your openshift/origin pull request:
   1. Add a TMP commit that points [glide.yaml](https://github.com/openshift/origin/blob/master/glide.yaml#L39-L41) at your fork of openshift/api, and the branch of your pull request:

      ```
      - package: github.com/openshift/api
        repo:    https://github.com/<your-username>/api.git
        version: "<your-openshift-api-branch>"
      ```

    2. Update your `bump(*)` commit to include the result of running `hack/update-deps.sh`, which will pull in the changes from your openshift/api pull request
    3. Make sure CI is green on your openshift/origin pull request 
    4. Get LGTM on your openshift/api pull request (for API changes) and your openshift/origin pull request (for code changes)

Once both pull requests are ready, the openshift/api pull request can be merged.

Then do the following with your openshift/origin pull request:
1. Drop the TMP commit (pointing glide back at openshift/api#master)
2. Rerun `hack/update-deps.sh` and update your `bump(*)` commit
3. It can then be tagged and merged by CI
