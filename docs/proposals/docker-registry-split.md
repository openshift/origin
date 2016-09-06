# Move docker-registry to separate repository

## Problem

The `docker/distribution` has a lot of dependencies. Since `kubernetes` began to use docker/distribution
it has become very difficult to update `docker/distribution` in the part where you need to combine the
dependencies of these projects. There is no guarantee that the merge of the dependencies would not make
regression in these projects.

## Constraints and Assumptions

The docker-registry is highly integrated in a common code-base so it is very difficult to describe
all the difficulties of separation.

## Investigation

### etcd

The docker-registry does not have its own `etcd`. All metadata are stored to the `origin` database using
client API.

### code-base

The `origin` interacts with the docker-registry only through client library (no internal calls),
but this cannot be said about docker-registry.

1. docker-registry should use [clientsets](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/generating-clientset.md)
of `origin`.
2. docker-registry should use [clientsets](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/generating-clientset.md)
of `kubernetes`.
3. We need to create versioned client (clientsets?) for docker-registry as well.
4. We need to split some utilities to reduce the number of dependencies to `origin`.
5. We need to make common framework for tests. Currently docker-registry uses a common test utilities and clients.

### testing

Use versioning clients will allow us maintain compatibility between repositories. In this case,
in the jenkins job for `origin` we can build the image from the separate repository before build and
test `origin` itself. Right now we make docker-registry image from the `origin` repository.

### release scripts

/TODO/

## Dependencies

`cmd/dockerregistry` uses
```
github.com/openshift/origin/pkg/api/install
github.com/openshift/origin/pkg/cmd/dockerregistry
github.com/openshift/origin/pkg/cmd/util                 (Env)
github.com/openshift/origin/pkg/cmd/util/serviceability  (BehaviorOnPanic, Profile)

k8s.io/kubernetes/pkg/api/install
k8s.io/kubernetes/pkg/apis/extensions/install
```

`pkg/cmd/dockerregistry` uses
```
github.com/openshift/origin/pkg/cmd/server/crypto
github.com/openshift/origin/pkg/dockerregistry/server
```

`pkg/dockerregistry/server` uses
```
github.com/openshift/origin/pkg/authorization/api
github.com/openshift/origin/pkg/client
github.com/openshift/origin/pkg/cmd/util/clientcmd
github.com/openshift/origin/pkg/image/admission          (AdmitImage)
github.com/openshift/origin/pkg/image/api
github.com/openshift/origin/pkg/image/importer
github.com/openshift/origin/pkg/quota/util               (IsErrorQuotaExceeded)
github.com/openshift/origin/pkg/util/httprequest         (SchemeHost)

k8s.io/kubernetes/pkg/api
k8s.io/kubernetes/pkg/api/errors
k8s.io/kubernetes/pkg/client/cache
k8s.io/kubernetes/pkg/client/restclient
k8s.io/kubernetes/pkg/client/unversioned
k8s.io/kubernetes/pkg/runtime
k8s.io/kubernetes/pkg/util
k8s.io/kubernetes/pkg/util/sets
```

`pkg/dockerregistry/server` tests uses
```
github.com/openshift/origin/pkg/api/install
github.com/openshift/origin/pkg/api/latest
github.com/openshift/origin/pkg/authorization/api
github.com/openshift/origin/pkg/client
github.com/openshift/origin/pkg/client/testclient
github.com/openshift/origin/pkg/cmd/util/clientcmd
github.com/openshift/origin/pkg/dockerregistry/testutil
github.com/openshift/origin/pkg/image/admission/testutil
github.com/openshift/origin/pkg/image/api
github.com/openshift/origin/pkg/user/api

k8s.io/kubernetes/pkg/api
k8s.io/kubernetes/pkg/apimachinery/registered
k8s.io/kubernetes/pkg/client/restclient
k8s.io/kubernetes/pkg/client/unversioned
k8s.io/kubernetes/pkg/client/unversioned/testclient
k8s.io/kubernetes/pkg/runtime
k8s.io/kubernetes/pkg/util
k8s.io/kubernetes/pkg/util/diff
```

### docker-registry -> k8s -> docker/distribution

Right now we have dependencies between kubernetes and docker/distribution:
```
cmd/dockerregistry/main.go:
`-> github.com/openshift/origin/pkg/api/install, k8s.io/kubernetes/pkg/api/install
 `-> k8s.io/kubernetes/pkg/api/v1
  `-> k8s.io/kubernetes/pkg/util/parsers
   `-> github.com/docker/distribution/reference

pkg/dockerregistry/server/repositorymiddleware.go:
`-> k8s.io/kubernetes/pkg/client/restclient
 `-> k8s.io/kubernetes/pkg/api/v1
  `-> k8s.io/kubernetes/pkg/util/parsers
   `-> github.com/docker/distribution/reference

pkg/dockerregistry/server/token.go:
`-> k8s.io/kubernetes/pkg/client/restclient
 `-> k8s.io/kubernetes/pkg/api/v1
  `-> k8s.io/kubernetes/pkg/util/parsers
   `-> github.com/docker/distribution/reference

pkg/dockerregistry/server/auth.go:
`-> k8s.io/kubernetes/pkg/client/restclient
 `-> k8s.io/kubernetes/pkg/api/v1
  `-> k8s.io/kubernetes/pkg/util/parsers
   `-> github.com/docker/distribution/reference

pkg/dockerregistry/testutil/util.go:
`-> k8s.io/kubernetes/pkg/client/unversioned/testclient
 `-> k8s.io/kubernetes/pkg/api/v1
  `-> k8s.io/kubernetes/pkg/util/parsers
   `-> github.com/docker/distribution/reference
```
