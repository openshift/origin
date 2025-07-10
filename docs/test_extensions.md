# OpenShift Test Extensions

`openshift-tests` can be extended using the [openshift-tests extension
interface](https://github.com/openshift/enhancements/pull/1676).

There is a registry of binaries defined in
`pkg/test/extensions/binary.go`, that lists the release image tag, and
path to each external test binary.  These binaries should implement the
OTE interface defined in the enhancement, and implemented by the
vendorable [openshift-tests-extension](https://github.com/openshift-eng/openshift-tests-extension)
framework.

## Local Development

If the architecture of your local system where `openshift-tests` will run
differs from the cluster under test, you should override the release payload
with a payload of the architecture of your own system, as it is where the
binaries will execute. Note, your OS must still be Linux to run any extracted images
from the payload.

Alternatively, you can point origin to locally-built binaries.  An
example workflow for Mac would be to override both the payload and
specific image binaries:

```
export EXTENSIONS_PAYLOAD_OVERRIDE=registry.ci.openshift.org/ocp-arm64/release-arm64:4.18.0-0.nightly-arm64-2024-11-15-135718
export EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS=tests,hyperkube
export EXTENSION_BINARY_OVERRIDE_HYPERKUBE=$HOME/go/src/github.com/kubernetes/kubernetes/_output/bin/k8s-tests-ext"
```

## Environment Variables

A number of environment variables for overriding the behavior of external
binaries are available, but in general this should "just work". A complex set
of logic for determining the optimal release payload, and which pull
credentials to use are found in this code, and extensively documented in code
comments.  The following environment variables are available to force certain
behaviors:

### Extension Binary Filtering

Filter which extension binaries are extracted by image tag:

```bash
# Exclude specific extensions
export EXTENSION_BINARY_OVERRIDE_EXCLUDE_TAGS="hyperkube,machine-api-operator"

# Include only specific extensions
export EXTENSION_BINARY_OVERRIDE_INCLUDE_TAGS="tests,hyperkube"
```

### Extension Binary Override

When developing locally, you may want to use a locally built extension
binary. You can override the binary from the registry by setting:

```
export EXTENSION_BINARY_OVERRIDE_HYPERKUBE="/home/sally/git/kubernetes/_output/bin/k8s-tests-ext"
```

This overrides all extension binaries registered for that image tag
(i.e. hyperkube). In the uncommon situation where an image tag is
providing multiple test binaries, you can more specifically override one
like this:

```
export EXTENSION_BINARY_OVERRIDE_HYPERKUBE_USR_BIN_K8S_TESTS_EXT_GZ="/home/sally/git/kubernetes/_output/bin/k8s-tests-ext"
```

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
