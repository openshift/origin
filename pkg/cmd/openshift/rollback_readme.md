## Steps

    #start kube
    sudo _output/local/bin/linux/amd64/openshift start --loglevel=4
    
    #deploy 1st config
    openshift kube create -c paul_temp/hello_deploy_1.json deploymentConfigs
    openshift kube list deployments
    Name                Status              Cause
    ----------          ----------          ----------
    frontend-1          Complete            ConfigChange

    #validate 1st config
    docker ps | grep hello-openshift | cut -d' ' -f1 | xargs docker inspect | grep TEST
                "TEST_VAL=1",

    
    #deploy 2nd config
    openshift kube create -c paul_temp/hello_deploy_2.json deploymentConfigs
    openshift kube list deployments
    Name                Status              Cause
    ----------          ----------          ----------
    frontend-1          Complete            ConfigChange
    frontend-2          Complete            ConfigChange
    
    #validate 2nd config
    docker ps | grep hello-openshift | cut -d' ' -f1 | xargs docker inspect | grep TEST
                "TEST_VAL=2",
                
    #rollback
    openshift-rollback --f=paul_temp/rollback.json
    I1203 11:56:27.408591 19973 rollback.go:43] ------------------ Executing a rollback ----------------------
    I1203 11:56:27.410176 19973 rollback.go:51] Reading rollback config
    I1203 11:56:27.412969 19973 rollback.go:58] Finding current deploy config named frontend
    I1203 11:56:27.432268 19973 rollback.go:66] Finding rollback deployment with name frontend-1
    I1203 11:56:27.454106 19973 rollback.go:78] -------------------  Rollback Complete  ----------------------

    openshift kube list deployments
    Name                Status              Cause
    ----------          ----------          ----------
    frontend-1          Complete            ConfigChange
    frontend-2          Complete            ConfigChange
    frontend-3          Complete            ConfigChange

    docker ps | grep hello-openshift | cut -d' ' -f1 | xargs docker inspect | grep TEST
                "TEST_VAL=1",


## Configs
hello_deploy_1.json

     {
         "metadata":{
             "name": "frontend"
         },
         "kind": "DeploymentConfig",
         "apiVersion": "v1beta1",
         "triggers": [
             {"type": "ConfigChange"}
         ],
         "template": {
             "strategy": {
                 "type":"Recreate"
             },
             "controllerTemplate": {
                 "replicas": 1,
                 "replicaSelector": {
                     "name": "frontend"
                 },
                 "podTemplate": {
                     "desiredState": {
                         "manifest": {
                             "version": "v1beta1",
                             "containers": [
                                 {
                                     "name": "hello-openshift",
                                     "image": "openshift/hello-openshift",
                                     "ports": [{
                                         "containerPort": 8080
                                     }],
                                     "env": [
                                         {
                                             "name": "TEST_VAL",
                                             "value": "1"
                                         }
                                     ]
                                 }
                             ]
     
                         }
                     },
                     "labels": {
                         "name": "frontend"
                     }
                 }
             }
         }
     }

hello_deploy_2.json: same as `hello_deploy_1.json` but with the value for `TEST_VAL` updated to 2

rollback.json:

     {
         "metadata": {"name":"frontend"},
         "kind":"DeploymentConfig",
         "rollback": { "to": "frontend-1" }
     }
