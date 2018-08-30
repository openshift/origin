We should separate the config for our separate processes.  We have
processes for kube-apiserver, kube-controller-manager, kube-scheduler, openshift-apiserver,
and openshift-controller manager.  All of these are currently fed based on master-config.yaml,
but they have many discrete, non-overlapping values.

Coarse separation:
 1. openshift-apiserver - we will have a config for this for the forseeable future.
 It will be different than a kube-apiserver config.
 We should start this struct now.
 2. openshift-controllers - we will have a config for this for the forseeable future.
 It will be different than a kube-controller-manager config.
 We should start this struct now.
 3. kube-scheduler - We should not be special for this.
 We have no need for custom flags.
 We should use the flags as indicated by upstream so that we can take advantage of any migration they provide.
 4. kube-apiserver - our config will be different from the upstream for the forseeable future.
 We have many configuration options they do not have.
 If the kube-apiserver gets config upstream, we will either embed in our config (short term) or
 we will have two sources of config.
 In both cases, we have a transition.  In both cases we need a config file for things like our IDPs.
 We should create a new config file with this and handle a migration in a year when we need to.
 5. kube-controller-manager - We have config for service serving cert CAs and recycler image.
 We have to have an indication that we are running this for openshift. This *could* be a feature flag.
 Our config is not going to move upstream.
 Service serving cert CA will eventually be dropped for some other injection mechanism.
 Our config is distinct from any eventual upstream config.
 Our config will live for a long time.
 If we use our config for our things and kube's flags for their things, we can benefit 
 from any migration they provide.
 If we keep our current config and just keep the "flags here" parameter, we can do the same.
 In both cases, you end up having a separate config that live for a considerable time.
 