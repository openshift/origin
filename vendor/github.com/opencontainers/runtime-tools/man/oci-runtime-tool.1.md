% OCI(1) OCI-RUNTIME-TOOL User Manuals
% OCI Community
% APRIL 2016
# NAME
oci-runtime-tool \- OCI (Open Container Initiative) runtime tools

# SYNOPSIS
**oci-runtime-tool** [OPTIONS] COMMAND [arg...]

**oci-runtime-tool** [--help|-v|--version]

# DESCRIPTION
oci-runtime-tool is a collection of tools for working with the [OCI runtime specification](https://github.com/opencontainers/runtime-spec).


# OPTIONS
**--help**
  Print usage statement.

**--host-specific**
  Generate host-specific configs or do host-specific validations.

  By default, generator generates configs without checking whether they are
  supported on the current host. With this flag, generator will first check
  whether each config is supported on the current host, and only add it into
  the config file if it passes the checking.

  By default, validation only tests for compatibility with a hypothetical host.
  With this flag, validation will also run more specific tests to see whether
  the current host is capable of launching a container from the configuration.

**--log-level**=LEVEL
  Log level (panic, fatal, error, warn, info, or debug) (default: "error").

**--compliance-level**=LEVEL
  Compliance level (`may`, `should`, or `must`) (default: `must`).
  For example, a SHOULD-level violation is fatal if `--compliance-level` is `may` or `should` but non-fatal if `--compliance-level` is `must`.

**-v**, **--version**
  Print version information.

# COMMANDS
**validate**
  Validating OCI bundle
  See **oci-runtime-tool-validate**(1) for full documentation on the **validate** command.

**generate**
  Generating OCI runtime spec configuration files
  See **oci-runtime-tool-generate**(1) for full documentation on the **generate** command.

# SEE ALSO
**oci-runtime-tool-validate**(1), **oci-runtime-tool-generate**(1)

# HISTORY
April 2016, Originally compiled by Daniel Walsh (dwalsh at redhat dot com)
