# Capabilities
A capability is something that a user interface may change its behavior based on.

### Examples of capabilities:

In v2 of OpenShift we tracked:
* user capabilities
  * max teams
  * view global teams
  * plan upgrade enabled
  * subaccounts allowed
  * max domains
* domain capabilities
  * can I use custom SSL certs
  * what gear sizes can be created
  * max gears
  * max storage per gear

In v3 of OpenShift we may have:
* global configuration
  * view global teams
  * min allocatable amount of resource X per container (cpu / mem / disk /etc)
    * this is to prevent people from creating containers with unusably small amounts of cpu / memory
* project capabilities (note: capabilities related to max resource limits may exist on the account and then be subdivided/allocated to different projects)
  * can I use custom SSL certs / how many custom ssl certs can i have
  * max total amount of system resource X (cpu / mem / disk / etc)
  * max number of a given resource type (services, git repos, image streams, builds, etc)
* account capabilities
  * max teams
  * plan change enabled

## Global configuration
Capabilities triggered from global configuration will consist of two types:

1. Features that are turned on/off for the whole openshift deployment
2. Global resource constraints, ex: setting a minimum allocatable size per container for a given system resource (mem / cpu / disk)

Features that are turned on/off globally will result in changes to policy objects.  There will be an API for requesting the effective policy for a set of actions for a given user.

Global resource constraints will change the effective resource constraints for a project if the global constraints are more restrictive.

## Project (namespace) level capabilities
Capabilities for the project will consist of two main types:

1. Features that are turned on/off for the project
2. Resource constraints

Resource constraints for the project will be stored on the “ResourceController” associated with the namespace.  The ResourceController is the same object used by the admission control system, both to check allowed usage and to store actual usage.

Features that are turned on/off for a project are persisted as changes to policy objects.  There will be an API for requesting the effective policy for a set of actions for a given user.

## Account level capabilities
Account is a separate entity from user, many users might be associated with a single account, and users will have a role on the account.

Once we have accounts, they will have a ResourceController associated with them.

## Policy vs. Admission Control
Policy should never check capabilities on a project or account.  If something needs to be checked against the capabilities on the project or account then the check should be in admission control.

Both policy and admission control will check globally configured capabilities.

## Other types of config
Global configuration for blacklists / whitelists (blacklisting namespaces / docker registries / etc) will not be considered capabilities and will not be surfaced through the API.  It will be the job of the admission controller to check these configurations and throw reasonable errors when something is rejected due to blacklisting/whitelisting.

## APIs to retrieve capabilities

### Resource constraints
To retrieve a project's (or account's) effective resource constraints and current usage, you will GET the ResourceController for the project (account).  The effective resource constraint for a given resource type is either the global resource constraint or the project's resource constraint, whichever is more restrictive.

When getting a ResourceController the relevant subobjects are:
* ResourceController.Spec.Allowed.somekey = the project's resource constraint for "somekey"
* ResourceController.Status.Allowed.somekey = the effective resource constraint for "somekey"
* ResourceController.Status.Allocated.somekey = the current usage for "somekey"
