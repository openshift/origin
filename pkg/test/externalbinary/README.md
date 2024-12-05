# External Binaries

This package includes the code used for working with external test binaries.
It's intended to house the implementation of the openshift-tests side of the
[openshift-tests extension interface](https://github.com/openshift/enhancements/pull/1676), which is only
partially implemented here for the moment.

There is a registry defined in binary.go, that lists the release image tag, and
path to each external test binary.  These binaries should implement the OTE
interface defined in the enhancement, and implemented by the vendorable
[openshift-tests-extension](https://github.com/openshift-eng/openshift-tests-extension).

## Requirements

If the architecture of your local system where `openshift-tests` will run
differs from the cluster under test, you should override the release payload
with a payload of the architecture of your own system, as it is where the
binaries will execute. Note, your OS must still be Linux. That means on Apple
Silicon, you'll still need to run this in a Linux environment, such as a
virtual machine, or x86 podman container.

## Overrides

A number of environment variables for overriding the behavior of external
binaries are available, but in general this should "just work". A complex set
of logic for determining the optimal release payload, and which pull
credentials to use are found in this code, and extensively documented in code
comments.  The following environment variables are available to force certain
behaviors:

### Caching

By default, binaries will be cached in `$XDG_CACHE_HOME/openshift-tests`
(typically: `$HOME/.cache/openshift-tests`). Upon invocation, older binaries
than 7 days will be cleaned up. To disable this feature:

```bash
export OPENSHIFT_TESTS_DISABLE_CACHE=1
```

### Registry Auth Credentials

To change the pull secrets used for extracting the external binaries, set:

```bash
export REGISTRY_AUTH_FILE=$HOME/pull.json
```

### Release Payload

To change the payload used for extracting the external binaries, set:

```bash
export EXTENSIONS_PAYLOAD_OVERRIDE=registry.ci.openshift.org/ocp-arm64/release-arm64:4.18.0-0.nightly-arm64-2024-11-15-135718
```
