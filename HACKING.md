## Installing Godep

OpenShift and Kubernetes use [Godep](https://github.com/tools/godep) for dependency management.  Godep allows versions of dependent packages to be locked at a specific commit by *vendoring* them (checking a copy of them into `Godeps/_workspace/`).  This means that everything you need for OpenShift is checked into this repository, and the `hack/config-go.sh` script will set your GOPATH appropriately.  To install `godep` locally run:

    $ go get github.com/tools/godep

If you are not updating packages you should not need godep installed.

## Updating Godeps from upstream

To update to a new version of a dependency that's not already included in Kubernetes, checkout the correct version in your GOPATH and then run `godep save <pkgname>`.  This should create a new version of `Godeps/Godeps.json`, and update `Godeps/workspace/src`.  Create a commit that includes both of these changes.

To update the Kubernetes version, checkout the new "master" branch from openshift/kubernetes (within your regular GOPATH directory for Kubernetes), and run `godep restore ./...` from the Kubernetes dir.  Then switch to the OpenShift directory and run `godep save ./...` 
