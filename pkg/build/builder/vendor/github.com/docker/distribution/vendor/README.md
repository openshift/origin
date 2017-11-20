The packages in this directory represent dependencies that differ
between Kubernetes (which uses the Google Cloud APIs for the
CloudProvider functionality) and Docker (which uses only Google Cloud
Storage).

To recreate this vendor directory:

1. Checkout the appropriate tag for `github.com/docker/distribution` as
   vendored by OpenShift
2. Copy the `vendor` directories into
   `vendor/github.com/docker/distribution/vendor`
3. Remove all dependencies that are the same (or still compile) from the
   local copy - this typically includes:

   * gopkg.in/*
   * github.com/docker/*
   * golang.org/x/net/*
   * golang.org/x/crypto/*

At build item, Go 1.6 selects dependencies for
`github.com/docker/distribution/...` first from this directory, and then
from the normal GOPATH.
