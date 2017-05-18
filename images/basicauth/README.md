## README
This image provides an [OpenShift basicauthurl
server](http://docs.openshift.org/latest/architecture/authentication.html#BasicAuthPasswordIdentityProvider)
which an administrator can use to implement custom authentication for
OpenShift.

This directory contains the files necessary to build the image for
a secure httpd server running on RHEL 7 that can respond to basic
authentication requests as OpenShift requires.

The image must be deployed with server configuration that specifies the
authentication method to be used and the security configuration.

## Usage

This README assumes you are performing these commands on an installed
and running OpenShift Enterprise master host.  It may require adjustments
for other environments.

First, choose a project for the basicauthurl service to live in. It can
be the `default` project if you wish or another. This example assumes the
`basicauthurl` project.

    $ oc new-project basicauthurl

### Build and deploy the image

A Docker image may be provided in the future. For now, you will need to
build it from the source Dockerfile. You can directly use the OpenShift
origin repository, or clone it to your own OpenShift-accessible git
repository if you need to make any changes (e.g. to add an httpd module
or build for a Linux other than RHEL).

At the moment, there are some workarounds necessary for OpenShift
Enterprise. Some of the following should become unnecessary as features
and bug fixes are released.

    $ docker pull registry.access.redhat.com/library/rhel  # workaround for new-app
    $ oc new-app --name=basicauthurl --labels=name=basicauthurl \
                 --context-dir=images/basicauth \
                 'https://github.com/openshift/origin'
    W0713 10:54:28.014586  104323 pipeline.go:225] A service will not be generated for DeploymentConfig "basicauthurl" [...]
    imagestreams/rhel
    imagestreams/basicauthurl
    buildconfigs/basicauthurl
    deploymentconfigs/basicauthurl
    A build was created - you can run `oc start-build basicauthurl` to start it.

Since `oc new-app` does not currently do it, create a service for the deployment:

    $ oc expose dc basicauthurl --port=8443 --generator=service/v1
    $ oc get service basicauthurl

You can use the resulting service IP for the server certificate and
master config below.

If instead you would like the master to reach your authentication service
via a route (which may not be a good idea for security reasons), you
can create that route as follows:

    $ oc expose service/basicauthurl --hostname=<correctly resolving name>

You would then need to use `oc edit route/basicauthurl` to make the
resulting route use passthrough TLS.

### Specify the configuration

To configure this server on OpenShift, create a Kubernetes secret
containing the necessary configuration files as described below. Note
that the names below are how they are referred to in the secret. The
actual file names you build the secret from could be anything.

* conf

This file should contain the main httpd configuration directives that
you need for authentication. The `examples/` directory provides an
example `basicauth.conf` that would be backed by an htpasswd-generated
hash file. You could include directives for any [httpd authentication
module](http://httpd.apache.org/docs/2.4/howto/auth.html) that is
included in the image. (Note, the example ldap.conf will only work if
the unsupported `mod_ldap` RPM is added to the image.)

This file will be placed in the container as `/etc/authserver/include.conf`
and included by the secure server configuration.

* conf-dir

This file, if present, should be a tarball of files that will be placed
in the `/etc/authserver/conf/` directory on the container. This can include
anything needed outside the server configuration. For instance, it could
include an htpasswd-created file for use with the example above.

* cert, key, ca

In order to secure the connection between the master and the authentication
server, the server should present a valid TLS configuration.

These files should include the server certificate, server key, and any
necessary CA bundle required to secure the server with TLS.  In order to
create a properly secured server certificate, you will need to determine
the desired address(es) for the master to reach the server. This must
be either an IP for the service (created above) or some other address
that will resolve correctly on the master, such as an /etc/hosts entry
or DNS-resolved external route. Note that the master does not typically
use the cluster DNS resolver (`*.cluster.local`).

For testing purposes, you can likely use the OpenShift tools to create
a key and cert signed with the OpenShift CA, for example:

    oadm create-server-cert --signer-cert=/etc/openshift/master/ca.crt \
                            --signer-key=/etc/openshift/master/ca.key  \
                            --signer-serial=/etc/openshift/master/ca.serial.txt \
                            --cert=cert.crt --key=key.key \
                            --hostnames=basicauthurl.example.com,172.30.137.253

The CA certificate can just be a copy of the cert.crt if no CA is
needed. You could also use a self-signed certificate here and include
it as the CA cert in the master config (below). Only the master needs
to be able to validate the secure connection to the auth server.

**Creating the secret**

You can bundle up these files into a secret with the `oc secrets
new` command. Continuing with the htpasswd auth example, here is a
demonstration:

    $ mkdir secrets; cd secrets
    $ cp ../examples/basicauth.* .
    $ tar zfc conf.tgz basicauth.htpasswd
    [... create or copy the key/certs ...]
    $ oc secrets new httpd-auth conf=basicauth.conf conf-dir=conf.tgz \
                                 key=key.key cert=cert.crt ca=ca.crt

This secret becomes a volume that authorized pods can mount. The service
account, generally `default`, that deploys the server will need access to
the secret to enable mounting it:

    $ oc secrets add serviceaccount/default secrets/httpd-auth --for=mount

Next, add the secret volume to the server deployment configuration:

    $ oc volume  dc/basicauthurl --add --type=secret \
                                 --secret-name=httpd-auth \
                                 --mount-path=/etc/secret-volume

An automated build should already be running, and once it completes
successfully, it should be deployed with the content you specified in
the secret. If it was already deployed prior to the step above adding
the secret volume, it will be redeployed once the modification occurs.

### OpenShift Master setup:

Now to have the master use your authentication, specify
the following in the master's configuration file
(`/etc/openshift/master/master-config.yaml` for OpenShift Enterprise):

~~~
  identityProviders:
  - challenge: true
    login: true
    name: basicauthurl
    provider:
      apiVersion: v1
      kind: BasicAuthPasswordIdentityProvider
      url: https://<serviceIP or name>:8443/validate
      ca: /etc/openshift/master/ca.crt
~~~

The restart the master.

    # systemctl restart openshift-master

Following the htpasswd example, you should now be able to login as "bob"
with password "redhat".

### Modifying the configuration

With everything in place, reconfiguration just requires updating the
secret. To do so, delete and recreate the secret, add access for the
service account again, and redeploy.

    $ oc delete secrets/httpd-auth
    $ oc secrets new httpd-auth conf=basicauth.conf conf-dir=conf.tgz key=key.key cert=cert.crt ca=ca.crt
    $ oc secrets add serviceaccount/default secrets/httpd-auth --for=mount
    $ oc deploy basicauthurl --latest

For modifications to the image itself, update the git repository it is
based on and run the build:

    $ oc start-build basicauthurl

It will automatically build from source and re-deploy.

To scale up the service such that it will not be interrupted by the loss
of a node, just scale up the deployment config and redeploy it:

    $ oc scale dc/basicauthurl --replicas=2
    $ oc deploy basicauthurl --latest

(You can scale the existing deployment, but it is simplest to just
redeploy.)


## TODO:

* The intention is not to continue with a mod_php script. Right now
  this is primarily a prototype for hosting a basicauthurl server on OpenShift
  itself.
