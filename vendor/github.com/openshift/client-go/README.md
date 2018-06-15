OpenShift Client in Go
==============================


Go clients for speaking to an OpenShift cluster.

Versions track OpenShift releases.

See [INSTALL.md](/INSTALL.md) for detailed installation instructions.


## Table of Contents

- [How to use it](#how-to-use-it)


### How to use it

See [examples](/examples).

### Compatibility

openshift/client-go is backwards compatible with prior server versions back to
v3.6 when we switched to API groups.  It is not compatible before that.

Keep in mind that using a newer client is generally safe, but the server will
strip newer fields it doesn't understand from objects.  That means that if you're
trying to use a new feature on an old server, the server will do the best it can,
but you still won't have the new feature.

Using an older client can be risky if you issue updates (not patches) to existing
resources.  The older client will remove newer fields on updates.  You will not have
this problem if you issue patches instead of updates.