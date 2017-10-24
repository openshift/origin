# Kubernetes Code Copy Area

This directory is the copy of original types from the main 
[kubernetes/kubernetes repo](https://github.com/kubernetes/kubernetes) with 
maintaining original packages structure. The purpose of this package is to
avoid depending on the entire main repo and copy only necessary types as needed.

The types in this directory should be treated as read-only, and any necessary
changes should be addressed in the original repository. If there are some 
specific changes needed to be made for Service Catalog, the type should be moved
out of this directory to the appropriate one first.

## Updating dependencies to the latest Kubernetes release

When updating dependencies on 
[k8s.io/apimachinery](https://github.com/kubernetes/apimachinery),
[k8s.io/client-go](https://github.com/kubernetes/client-go), 
[k8s.io/apiserver](https://github.com/kubernetes/apiserver)
and other top-level Kubernetes repositories, we need to overwrite the types in
this directory with the latest version from the corresponding release of
[k8s.io/kubernetes](https://github.com/kubernetes/kubernetes) to keep the files
in sync.

## Long-term plan

In the long term the packages used by Kubernetes-based projects will be moved
from the main repo to separate top-level repositories, [k8s.io/common](https://github.com/kubernetes/common)
and [k8s.io/utils](https://github.com/kubernetes/utils). Once it is done, we can
eventually switch to those packages and drop this directory.
