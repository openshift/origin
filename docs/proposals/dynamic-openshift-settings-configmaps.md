# Changing OpenShift settings dynamically through ConfigMaps

## Problem

Currently, there are settings contained within the master-config.yaml file that
are of value to an unpriviledged user. Specifically, the `autoProvisionEnabled`
boolean under `jenkinsPipelineConfig` is only available to users with the
ability to write to the master-config.

## Design proposal:

- In each project there can exist one or more ConfigMaps annotated with
  'project.openshift.io/configuration'

- The ConfigMap will contain the key/value pair of setting(s) to be overridden
  (key) and the value(s) to use.

- Some cluster settings will be overridable at the project level using this
  method, and other project specific settings can be introduced via the
  ConfigMap as well.

- For values with more than one entry, e.g. `corsAllowedOrigins`, yaml or json
  array notation could be used.

- (Optional) Have a utility function for consolidating settings over multiple
  ConfigMaps. Have documentation to alert users that overlapping settings causes
  nondeterministic or unsupported behavior.

## Issues:

- We will likely not be exposing all settings in this way, so there needs to be
  an easy way for affected users to discover these.

- Will admins be able to disable this feature, and if so, how?

## Other solutions considered

Handling settings like this through project annotations was explored, but
because unpriviledged users cannot annotate their own projects, it was
abandoned.

We could use a specific name of a ConfigMap to designate a project-level
configuration. This has the benefits of

 1.  Making the code to process the project settings simpler (do not need to
     retrieve all ConfigMaps and filter down to the ones w/ the annotation, and merge
     keys across ConfigMaps, deal w/ conflicts, etc).
 
However, this 

 1.  Would prevent the user from ever using that name for anything else
 1.  Would not allow for grouping different settings into multiple ConfigMaps

## Example

For the setting `autoProvisionEnabled` under jenkinsPipelineConfig:

```bash
$ oc create configmap jenkins-pipeline-config --from-literal=autoProvisionEnabled=false
configmap "jenkins-pipeline-config" created
$ oc annotate configmap jenkins-pipeline-config project.openshift.io/configuration=true
$ oc new-app jenkins-pipeline-example
--> Deploying template "openshift/jenkins-pipeline-example" to project myproject
...
--> Creating resources ...
    buildconfig "sample-pipeline" created
    service "nodejs-mongodb-example" created
    route "nodejs-mongodb-example" created
    imagestream "nodejs-mongodb-example" created
    buildconfig "nodejs-mongodb-example" created
    deploymentconfig "nodejs-mongodb-example" created
    service "mongodb" created
    deploymentconfig "mongodb" created
--> Success
$ oc get pods
NAME               READY     STATUS              RESTARTS   AGE
mongodb-1-deploy   1/1       Running             0          57s
mongodb-1-yasy3    0/1       ContainerCreating   0          55s
```
