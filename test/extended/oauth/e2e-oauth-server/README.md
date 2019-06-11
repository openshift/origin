## OpenShift extended test oauth server

This is what I have so far.

Prerequisites
-------------

* Have the environment variable `KUBECONFIG` set pointing to your cluster.


## Run the following from this directory

First, you'll need to edit the cliconfig-noidp.yaml with values from your cluster.

```console
$ oc create -f ns-svc-svcaccct.yaml
$ oc create -f cliconfig-noidp.yaml or cliconfig-htpasswd.yaml
$ oc create -f route.yaml
$ oc create -f serviceca-configmap.yaml
$ oc create -f session-secret.yaml
$ oc create -f htpasswd-secret.yaml
$ oc create -f pod.yaml or pod-htpasswd.yaml  (need a deployment)
```


What's created
------------------------

* service - e2e-oauth - with annotation for service-ca-op to create serving-certs secret
* configmap - e2e-oauth-cliconfig - holds oauth config, will edit/replace data w/ updated IDP configs eventually
* configmap - service-ca - service-ca-operator-managed service-ca.crt injected
* secret - serving-cert - service-ca-operator-managed serving-certs
* secret - session
* secret - v4-0-config-user-idp-0-file-data - required for htpasswd configuration (not sure this exact name is necessary)
* route - e2e-oauth
* pod - e2e-oauth 

Need to transform the above in a test/extended/oauth/something.go file
(also, whatever yaml we end up requiring will go in test/extended/testdata)


NOTE: oauthclients get created, need to be cleaned up, so if you have a test-oauth-server running,
      you'll need to `oc delete oauthclient openshift-browser-client` b4 re-creating a new one.  
