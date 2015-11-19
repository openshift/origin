Generate a config with:

    $ openshift start master --write-config=./cluster-config

Update the config:

    # edit ./cluster-config/master-config.yaml
    # set masterIPEnvVarName to "POD_IP"
    # set the etcd client URL to http://etcd:2379
    # TODO: setup client certs
    # clear the etcd config section

Create a secret for the master config and start the config

    $ oc secrets new master-config ./cluster-config
    $ oc secrets add sa/default secrets/master-config
    $ oc create -f examples/ha/openshift-ha.yaml