# OpenShift Command-Line Interface

The `oc` command line tool is used to interact with the [OpenShift](http://openshift.github.io) and [Kubernetes](http://kubernetes.io/) HTTP API(s). `oc` is an alias for `openshift cli`.

`oc` is *verb focused*.
The base verbs are *[get](#oc-get)*, *[create](#oc-create)*, *[delete](#oc-delete)*, *[replace](#oc-replace)*, and *[describe](#oc-describe)*.
These verbs can be used to manage both Kubernetes and OpenShift resources.
Overall, there are six command groups:
[basic](#basic-commands),
[build and deploy](#build-and-deploy-commands),
[application modification](#application-modification-commands),
[troubleshooting and debugging](#troubleshooting-and-debugging-commands),
[advanced](#advanced-commands),
and [settings](#settings-commands).

Some verbs support the `-f` flag, which accepts regular file path, URL or `-` for
the standard input. For most actions, both JSON and YAML file formats are
supported.

Use `oc --help` for a full list of the verbs and subcommands available. A detailed list of examples for the most common verbs and subcommands is documented in the [oc by example](./generated/oc_by_example_content.adoc) and [oadm by example](./generated/oc_by_example_content.adoc) documents.

## Common Flags

CLI commands support both local (specific to the given command) and global (works for every command available) flags. Some of the most common global flags are:

| Name                       | Description                                             |
|:---------------------------|:--------------------------------------------------------|
|`--namespace` (`-n`) *ns*   | Use *ns* as the namespace scope for the CLI request (default `default`). |
|`--ns-path` *filename*      | Look in *filename* for the default namespace. |
|`--match-server-version`    | Require server version to match client version. |
|`--loglevel` *n*            | Set the log verbosity level to *n* (between 0-5, default 0). |
|`--server` (`-s`) *host*    | Connect to *host* for API service. |
|`--client-certificate` *filename* | Look in *filename* for the TLS client certificate. |
|`--client-key` *filename*   | Look in *filename* for the TLS client key. |
|`--certificate-authority` *filename* | Look in *filename* for the CA certificate. |
|`--auth-path` *filename*    | Look in *filename* for the auth info (for HTTPS). |
|`--api-version` *version*   | Specify API version *version* to use against the server. |
|`--insecure-skip-tls-verify` | Skip SSL certificate validation (makes HTTPS insecure). |
|`--help` (`-h`)             | Display help for the specified command. |

Use `oc options` for a full list of all global flags available.

## Basic Commands

### oc

This displays the list of available commands.

### oc login

This retrieves a session token that allows you to act as a user.
Invoked without arguments, `oc login` prompts for a username and a password.
For example, these two invocations are identical:

```bash
$ oc login
Username: test
Password: test

$ oc login -u test -p test
```

See also [`oc logout`](#oc-logout) and [`oc whoami`](#oc-whoami).

### oc new-project

This creates a new project, with the currently logged-in user as the project admin.
Option `--display-name` specifies the user-facing name of the project.
Option `--description` specifies its description.

For example:

```bash
$ oc new-project web-team-dev \
  --display-name="Web Team Development" \
  --description="Development project for the web team."
```

Note that we use double-quotes around the option arguments.

### oc new-app

This creates a new application in OpenShift with the specified source code, templates, and images.
It builds up the components of an application using images, templates, or code that has a public repository.
It looks up images on the local Docker installation (if available), a Docker registry, or an OpenShift image stream.
If you specify a source code URL, it sets up a build that takes the source code and converts it into an image that can run in a pod.
Local source must be in a git repository that has a remote repository that the OpenShift instance can see.
The images will be deployed via a deployment configuration, and a service will be connected to the first public port of the app.
You may either specify components using the various existing flags or let new-app autodetect what kind of components you have provided.

If you provide source code, you may need to run a build with `oc start-build` after the application is created.

The general form is:

```bash
$ oc new-app <component> [options]
```

where *component* is the same as for [`oc new-build`](#oc-new-build).
The options are:

| Option                       | Description                                        |
|:-----------------------------|:---------------------------------------------------|
|`--code` *dir*                | Use source code in *dir*                           |
|`--context-dir` *dir*         | Use *dir* as context dir in the build              |
|`--docker-image` *image*      | Include Docker image *image* in the app            |
|`--env` (`-e`) *k1=v1,...*    | Set env vars *k1...* to values *v1...*             |
|`--file` *filename*           | Use template in *filename*                         |
|`--group` *comp1*`+`*comp2*   | Group together components *comp1* and *comp2*      |
|`--image-stream` (`-i`) *is*  | Use imagestream *is* in the app                    |
|`--insecure-registry`         | Bypass cert checks for referenced Docker images    |
|`--labels` (`-l`) *k1=v1,...* | Label all resources with *k1=v1,...*               |
|`--name` *name*               | Give *name* to all generated app artifacts         |
|`--no-headers`                | For default output, don't print headers            |
|`--output-template` *s*       | Template string (`-o template`) or path (`-o templatefile`) |
|`--output-version` *version*  | Output with *version* (default api-version)        |
|`--output` (`-o`) *format*    | *format* is one of: `json`, `yaml`, `template`, `templatefile` |
|`--param` (`-p`) *k1=v1,...*  | Set/override parameters *k1...* with *v1...*       |
|`--strategy` *s*              | Use build strategy *s*, one of: `docker`, `source` |
|`--template` *t*              | Use OpenShift stored template *t* in the app       |

The template format is [golang templates](http://golang.org/pkg/text/template/#pkg-overview).
The following example uses a MySQL image in a private registry to create an app and override application artifacts' names.

```bash
$ oc new-app \
  --docker-image=myregistry.com/mycompany/mysql \
  --name=private
```

See also [`oc start-build`](#oc-start-build).

### oc status

This shows a high level overview of the current project.

See also [`oc describe`](#oc-describe) and [`oc get`](#oc-get).

### oc project

This displays the current project, or switches to another one.

For example:

```bash
# Switch to the myapp project
$ oc project myapp

# Display the project currently in use
$ oc project
```

## Build and Deploy Commands

### oc start-build

This manually starts a build, using either the specified buildConfig or a build name with the `--from-build` option.
There is also the option of streaming
the logs of the build if the `--follow` flag is specified.

```bash
$ oc start-build ruby-sample-build
$ oc start-build --from-build=ruby-sample-build-1
$ oc start-build --from-build=ruby-sample-build-1 --follow
```

See also [`oc new-build`](#oc-new-build) and [`oc new-app`](#oc-new-app).

### oc build-logs

This retrieves the logs from a Build container.
It allows you to
debug broken Build.
If the build is still running, this streams the logs from the container to console.
Use `oc get builds` to obtain a list of builds.

```bash
$ oc build-logs rubyapp-build
```

### oc deploy

This views, starts, cancels or retries deployments.
The general form is:

```bash
$ oc deploy <deployment-config> [options]
```

If invoked without options, `oc deploy` displays the latest deployment for the specified *deployment-config*.
For example:

```bash
$ oc deploy database
```

| Option    | Description |
|:----------|-------------|
|`--latest` | Start a deployment. |
|`--retry`  | Retry the latest failed deployment. |
|`--cancel` | Cancel the in-progress deployment. |

The following example shows how to cancel the `database` deployment:

```bash
$ oc deploy database --cancel
```

### oc rollback

This reverts the pod and container configuration back to a previous deployment.
Scaling and trigger settings are normally left as-is.
The general form is:

```bash
$ oc rollback <deployment> [options]
```

The options are:

| Option                      | Description |
|:----------------------------|:------------|
|`--dry-run`                  | Display what the rollback *would do* but do not perform the rollback. |
|`--change-triggers`          | Include the previous deployment's triggers in the rollback. |
|`--change-strategy`          | Include the previous deployment's strategies in the rollback. |
|`--change-scaling-settings`  | Include the previous deployment's replication controller replica count and selector in the rollback. |
|`--output` *format*          | Do not roll back; instead, display updated deployment configuration in the specified *format*, one of: `json`, `yaml`, `template`, `templatefile`. |
|`-t` *template-string*       | Use *template-string* (with `--output template`). |
|`-t` *filename*              | Write to *filename* (with `--output templatefile`). |

The *template-string* is in [golang template](http://golang.org/pkg/text/template) format.
Here are two examples:

```bash
# Perform a rollback.
$ oc rollback deployment-1

# Perform the rollback "manually" by piping back to "oc replace".
$ oc rollback deployment-1 --output=json | oc replace dc deployment -f -
```

See also [`oc replace`](#oc-replace).

### oc new-build

This creates a new build with the specified source code.
It creates a build configuration for your application using images and code that has a public repository.
It looks up the images on the local Docker installation (if available), a Docker registry, or an OpenShift image stream.
If you specify a source code URL, it sets up a build that takes the source code and converts it into an image that can run inside a pod.
Local source must be in a git repository that has a remote repository that the OpenShift instance can see.

Once the build configuration is created you may need to run a build with `oc start-build`.

The general form is:

```bash
$ oc new-build <component> [options]
```

where *component* has one of the forms:

| Form            | Description           |
|:----------------|:----------------------|
| *image*         | Use *image* directly. |
| *imagestream*   | Use the latest image in *imagestream*. |
| *path*          | Use source code found at *path*. |
| *url*           | Use source code found at *url*. |
| *image*~*url*   | Note the tilde `~` between *image* and *url*.  In this case the component is actually made of two sub-components, the *image* and the source code found at *url*.  Use the image as the base and arrange to build the source code on it. |

The options are:

| Option                             | Description                                          |
|:-----------------------------------|:-----------------------------------------------------|
|`--code`                            |                                                      |
|`--image` (`-i`) *repository*       | Find the specified image in *repository*.            |
|`--labels` (`-l`) *k1=v1,k2=v2,...* | Add labels *k1=v1,k2=v2,...* to all created objects. |
|`--strategy` *s*                    | Use strategy *s* (one of: `docker`, `source`).       |
|`--to-docker`                       | Force the build output to be `DockerImage`.          |
|`--name` *name*                     | Give generated build artifacts the name *name*.      |

The following example creates a NodeJS buildConfig based on the provided image / source code combination:

```bash
$ oc new-build openshift/nodejs-010-centos7~https://bitbucket.com/user/nodejs-app
```

See also [`oc start-build`](#oc-start-build) and [`oc new-app`](#oc-new-app).

### oc cancel-build

This cancels a pending or running build.
The general form is:

```bash
$ oc cancel-build <build> [options]
```

The options are:

| Option       | Description |
|:-------------|:------------|
|`--dump-logs` | Display the build logs for the cancelled build. |
|`--restart`   | Create a new build after the current build is cancelled. |

The following example cancels the build named `1da32cvq` and restarts it.

```bash
$ oc cancel-build 1da32cvq --restart
```

See also [`oc new-build`](#oc-new-build).

### oc import-image

This imports tag and image information from an external Docker image registry.
For example, the following command imports from the `mystream` registry.

```bash
$ oc import-image mystream
```

### oc scale

This sets a new size for a Replication Controller either directly or via its Deployment Configuration.

```bash
# n is the highest deployment number for the dc ruby-hello-world
$ oc scale rc ruby-hello-world-n  --replicas=3
$ oc scale dc ruby-hello-world --current-replicas=3 --replicas=5
```

### oc tag

This tags existing images into one or more image streams.
The option `--source` is a hint for the source type; its value is one of: `imagestreamtag`, `istag`, `imagestreamimage`, `isimage`, and `docker`.
The general form is:

```bash
$ oc tag [--source=<sourcetype>] <source> <dest> [<dest> ...]
```

For example:

```bash
$ oc tag --source=docker openshift/origin:latest myproject/ruby:tip
```

## Application Modification Commands

### oc get

This displays one or many resources.
Possible
resources are all OpenShift resources (builds, buildConfigs, deployments,
deploymentConfigs, images, imageRepositories, routes, projects, and others) and
all Kubernetes resources (pods, replicationControllers, services, minions,
events).

```bash
$ oc get pods
$ oc get replicationController 1234-56-7890-234234-456456
$ oc get service database
$ oc get -f json pods
```

#### Output formatting

You can control the output format by using the `-o format` flag. By default,
`oc` uses human-friendly printer format for console. You can also
control what API version will be used to print the resource by using the
`--output-version` flag. By default, it uses the latest API version.

Available formats include:

| Value        | Description                                           |
|:-------------|:------------------------------------------------------|
| json         | Pretty formated JSON format |
| yaml         | [YAML](http://www.yaml.org/) format |
| template     | User defined [Go template](http://golang.org/pkg/text/template) (combined with the `-t` flag) |
| templatefile | Same as above, but use the template file instead of `-t` |

An example of using `-o template` to retrieve the *name* of the first build:

```bash
$ oc get builds -o template --template="{{with index .items 0}}{{.metadata.name}}{{end}}"
```

#### Selectors

`oc get` provides also *selectors* that you can use to filter the output
by applying key-value pairs that will be matched with the resource labels:

```bash
$ oc get pods -s template=production
```

This command will return only pods whose `labels` include `"template": "production"`.

See also [`oc describe`](#oc-describe).

### oc describe

This functions similar to [`oc get`](#oc-get), but also includes other information related to the specified resource.

```bash
$ oc describe service frontend
```

### oc edit

This starts an editor opened to the YAML representation of the specified object.
When the editor exits, the object is updated.
You can specify the editor through environment variables `OC_EDITOR`, `GIT_EDITOR`, or `EDITOR`.
If none of those are set, `oc edit` uses the `vi` program.
The general form is:

```bash
$ oc edit <resource-type>/<name> [options]
```

The options are:

| Option                      | Description                                       |
|:----------------------------|:--------------------------------------------------|
|`--output` (`-o`) *format*   | Edit object as *format*, one of: `json`, `yaml`.  |
|`--output-version` *version* | Use API version *version*.                        |

For example, to edit the service `docker-registry` in JSON using the `v1` API format:

```bash
$ oc edit svc/docker-registry --output-version=v1 -o json
```

### oc env

This updates the environment on a resource with a pod template.
The general form is:

```bash
$ oc env <resource-type>/<name> <k1>=<v1> <k2>=<v2>
```

For example:

```bash
$ oc env dc/app DB_USER=user DB_PASS=pass
```

### oc volume

This controls the storage associated with various resources.
The general form is:

```bash
$ oc volume <resource-type>/<name> --<action> <options>
```

where *action* is one of `add`, `remove`, `list` and *options* depends on *action*.
For example, to arrange for the deployment configuration `registry` to access the host *_/opt_* directory, use:

```bash
$ oc volume dc/registry --add --mount-path=/opt
```

### oc label

This adds labels to a provided resource.
It can also overwrite the existing labels by using the `--overwrite` flag.

```bash
$ oc label service frontend foo=bar
```

### oc expose

This exposes containers internally as services or externally via routes. There is also
the ability to expose a deployment config, replication controller, service,
or pod as a new service on a specified port. If no labels are specified,
the new object will re-use the labels from the object it exposes.

```bash
# Expose a service as a route
$ oc expose service frontend
# Expose a deployment config as a service and use the specified port and name
$ oc expose dc ruby-hello-world --port=8080 --name=myservice --generator=service/v1
```

### oc delete

This deletes a specified resource.

```bash
$ oc delete -f pod.json
$ oc delete pod 1234-56-7890-234234-456456
```

## Troubleshooting and Debugging Commands

### oc logs

This dumps the logs from a given Pod container.
Use `oc get pod <pod-id>` to list the containers from a Pod.

```bash
$ oc logs frontend-pod -c mysql-container
```

### oc exec

This executes a command in a container.
The general form is one of:

```bash
$ oc exec -p <pod> [-c <container>] <command>
$ oc exec -p <pod> [-c <container>] [-i] [-t] -- <command> [args...]
```

If `-c <container>` is omitted, OpenShift chooses the first container in the pod.
The `-i` (or `--stdin`) flag passes local stdin to the container.
The `-t` (or `--tty`) flag arranges for stdin to be a TTY.

Some examples are:

```bash
# Get output from running 'date' in 'ruby-container' from pod 123456-7890.
$ oc exec -p 123456-7890 -c ruby-container date

# Switch to raw terminal mode, attach stdin to 'bash' in 'ruby-container'
# from pod 123456-780, and stdout/stderr from 'bash' back to the client.
$ oc exec -p 123456-7890 -c ruby-container -i -t -- bash -il
```

### oc port-forward

This forwards one or more local ports to a pod.
The general form is:

```bash
$ oc port-forward -p <pod> <forwarding-spec> [...]
```

where *forwarding-spec* is either a single port (integer), or a pair of ports separated by a colon `<outside>:<inside>`.
If *outside* is omitted or zero, OpenShift chooses a random port as the effective listening port.

Some examples are:

```bash
# Listen on ports 5000 and 6000 locally, forwarding data
# to/from ports 5000 and 6000 in the pod.
$ oc port-forward -p mypod 5000 6000

# Listen on 8888 locally, forwarding to 5000 in the pod.
$ oc port-forward -p mypod 8888:5000

# Listen on a random port locally, forwarding to 5000 in the pod.
# (These invocations are equivalent.)
$ oc port-forward -p mypod :5000
$ oc port-forward -p mypod 0:5000
```

### oc proxy

This runs a proxy to the Kubernetes API server.
By default, the proxy listens on port 8001.
API endpoints are served under directory `/api/` and any static files are served under directory `/static/`.
The general form is:

```bash
$ oc proxy [options]
```

The options are:

| Option                     | Description |
|:---------------------------|:------------|
|`--port` (`-p`) *n*         | Listen on port *n*. |
|`--api-prefix` *dir*        | Serve the proxied API under *dir*. |
|`--www` (`-w`)              | Enable serving static files. |
|`--www-prefix` (`-P`) *dir* | Serve static files under *dir*. |
|`--disable-filter`          | Disable request filtering. |
|`--accept-hosts` *rx*       | Accept requests from hosts matching regular expression *rx*. |
|`--accept-paths` *rx*       | Accept paths matching regular expression *rx*. |
|`--reject-paths` *rx*       | Reject paths matching regular expression *rx*. |

**WARNING**:
The `--disable-filter` flag is dangerous and can leave you vulnerable to XSRF attacks.
Use with caution.

The following example runs a proxy on port 8011 with API prefix `k8s-api`.

```bash
$ oc proxy -p 8011 --api-prefix k8s-api
```

This makes, for instance, the pods API (version 1) available at `localhost:8011/k8s-api/v1/pods/`.

## Advanced Commands

### oc create

This creates resources. It does not require pointers about
what resource it should create because it reads it from the provided JSON/YAML.
After successful creation, the resource name will be printed to the console.

```bash
$ oc create -f pod.json
$ cat pod.json | oc create -f -
$ oc create -f http://server/pod.json
```

### oc replace

This replaces existing resources.

```bash
$ oc replace -f pod.json
$ cat pod.json | oc replace -f -
$ oc replace -f http://server/pod.json
```

### oc patch

This updates one or more fields of a resource using strategic merge patch.
The general form is:

```bash
$ oc patch <resource-type> <name> -p <patch>
```

where *patch* is a JSON or YAML map expression that names one or more fields and their new values.
The following example sets the `spec.unschedulable` field of the `app` node to the value `true`:

```bash
$ oc patch node app -p '{"spec":{"unschedulable":true}}'
```

The equivalent operation with YAML is:

```bash
$ oc patch node app -p '
spec:
  unschedulable: true
'
```

In both cases, the top-level field is `spec` and its value is another map expression whose sole key is `unschedulable`.

### oc process

This processes a Template into a valid Config resource. The processing
will take care of generating values for parameters specified in the Template and
substituting the values in the corresponding places. An example Template can be
found in [examples/sample-app/application-template-stibuild.json](https://github.com/openshift/origin/blob/master/examples/sample-app/application-template-stibuild.json).

```bash
$ oc process -f examples/sample-app/application-template-stibuild.json > config.json
$ oc process -f template.json | oc create -f -
```

### oc export

This displays to standard output the specified resource(s) in YAML format.
The general form is:

```bash
$ oc export <resource-type>/<name> [options]
```

The options are:

| Option                | Description                                      |
|:----------------------|:-------------------------------------------------|
|`-f` *filename*        | Write to *filename* instead of standard output.  |
|`--as-template` *name* | Output in template format with name *name*.      |
|`--all`                | Select all objects with given *resource-type*.   |
|`--exact`              | Preserve fields that may be cluster specific, such as service `portalIP`s or generated names. |
|`--raw`                | Do not alter the resources in any way after they are loaded. |

The following example exports all services to a template with name `test`.

```bash
$ oc export service --all --as-template=test
```

## Settings Commands

### oc logout

This destroys the session token, preventing further access until next login (with [`oc login`](#oc-login)).

### oc config

This manages the OpenShift [kubeconfig files](https://github.com/kubernetes/kubernetes/blob/master/docs/user-guide/kubeconfig-file.md).
The general form is:

```bash
$ oc config <subcommand> [<arg> ...]
```

The subcommands are:

| Subcommand       | Description                                                        |
|:-----------------|:-------------------------------------------------------------------|
|`set`             | Set an individual value in a kubeconfig file.                      |
|`set-cluster`     | Set a cluster entry in kubeconfig.                                 |
|`set-context`     | Set a context entry in kubeconfig.                                 |
|`set-credentials` | Sets a user entry in kubeconfig.                                   |
|`unset`           | Unset an individual value in a kubeconfig file.                    |
|`use-context`     | Set the current-context in a kubeconfig file.                      |
|`view`	           | Display merged kubeconfig settings or a specified kubeconfig file. |

The following example changes the config context to use:

```bash
$ oc config use-context my-context
```

### oc whoami

This displays information about the current session.
If invoked without arguments, `oc whoami` displays the currently authenticated username.
Flag `-t` (or `--token`) means to instead display the session token.
Flag `-c` (or `--context`) means to instead display the user context name.

```bash
$ oc whoami -t
<token>
```

See also [`oc login`](#oc-login).
