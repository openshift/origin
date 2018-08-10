% OCI(1) OCI-RUNTIME-TOOL User Manuals
% OCI Community
% APRIL 2016
# NAME
oci-runtime-tool-validate - Validate an OCI runtime bundle

# SYNOPSIS
**oci-runtime-tool validate**  *[OPTIONS]*

# DESCRIPTION

Validate an OCI bundle

# OPTIONS
**--help**
  Print usage statement

**--path**=PATH
  Path to bundle. The default is current working directory.

**--platform**=PLATFORM
  Platform of the target bundle. (linux, windows, solaris) The default is host platform.
  It will be overwritten by the host platform if the global option '--host-specific' was set.

# SEE ALSO
**oci-runtime-tool**(1)

# HISTORY
April 2016, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
