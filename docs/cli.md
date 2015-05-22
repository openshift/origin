osc command line interface
==============================

The `osc` command line tool is used to interact with the [OpenShift](http://openshift.github.io) and [Kubernetes](http://kubernetes.io/) HTTP API(s). `osc` is an alias for `openshift cli`.

`osc` is *verb focused*. The base verbs are *[get](#osc-get)*, *[create](#osc-create)*, *[delete](#osc-delete)*,
*[update](#osc-update)*, and *[describe](#osc-describe)*. These verbs can be used to manage both Kubernetes and
OpenShift resources.

Some verbs support the `-f` flag, which accepts regular file path, URL or '-' for
the standard input. For most actions, both JSON and YAML file formats are
supported.

Common Flags
-------------

| Name                       | Description                                             |
|:-------------------------- |:--------------------------------------------------------|
| --namespace (-n)           | Namespace scope for the CLI request (default 'default') |
| --ns-path                  | Path to a file with the default namespace |
| --match-server-version     | Require server version to match client version        |
| --loglevel                 | Set the log verbosity level between 0-5 (default '0') |
| --server (-s)              | API server to connect to                              |
| --client-certificate       | Path to a client certificate for TLS |
| --client-key               | Path to a client key file for TLS |
| --certificate-authority    | Path to a CA certificate file |
| --auth-path                | Path to the auth info file (used for HTTPS) |
| --api-version              | The version of the API to use against the server |
| --insecure-skip-tls-verify | Skip SSL certificate validation (will make HTTPS insecure) |
| --help (-h)                | Display help for specified command |

osc get
-----------

This command can be used for displaying one or many resources. Possible
resources are all OpenShift resources (builds, buildConfigs, deployments,
deploymentConfigs, images, imageRepositories, routes, projects, and others) and
all Kubernetes resources (pods, replicationControllers, services, minions,
events).

#### Examples

```
$ osc get pods
$ osc get replicationController 1234-56-7890-234234-456456
$ osc get service database
$ osc get -f json pods
```

#### Output formatting

You can control the output format by using the `-o format` flag. By default, 
`osc` uses human-friendly printer format for console. You can also 
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

```
$ osc get builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"
```

#### Selectors

`osc get` provides also *selectors* that you can use to filter the output
by applying key-value pairs that will be matched with the resource labels:

```
$ osc get pods -s template=production
```

This command will return only pods whose `labels` include `"template": "production"`

osc create
--------------

This command can be used to create resources. It does not require pointers about
what resource it should create because it reads it from the provided JSON/YAML.
After successful creation, the resource name will be printed to the console.

#### Examples

```
$ osc create -f pod.json
$ cat pod.json | osc create -f -
$ osc create -f http://server/pod.json
```

osc update
---------------

This command can be used to update existing resources.

#### Examples

```
$ osc update -f pod.json
$ cat pod.json | osc update -f -
$ osc update -f http://server/pod.json
```

osc delete
--------------

This command deletes a specified resource.

#### Examples

```
$ osc delete -f pod.json
$ osc delete pod 1234-56-7890-234234-456456
```

osc describe
----------------

This command is a wordier version of `osc get` which also integrates other
information that's related to a given resource.

#### Examples

```
$ osc describe service frontend
```

osc label
----------------

This command adds labels to a provided resource. 
It can also overwrite the existing labels by using the `--overwrite` flag.

#### Examples

```
$ osc label service frontend foo=bar
```

osc stop
----------------

This command gracefully shuts down a resource by id or filename. 
It attempts to shut down and delete a resource that supports graceful termination. 
If the resource is resizable, it will be resized to 0 before deletion.

#### Examples

```
$ osc stop service frontend
```

osc namespace
-----------------

This command sets the default namespace used for all `osc` commands.

#### Examples

```
$ osc namespace myuser
```

osc log
------------

This command dumps the logs from a given Pod container. You can list the
containers from a Pod using the following command:

```
$ osc get -o yaml pod POD_ID
```

#### Examples

```
$ osc log frontend-pod mysql-container
```

osc expose
------------

This command looks up a service and exposes it as a route. There is also
the ability to expose a deployment config, replication controller, service, 
or pod as a new service on a specified port. If no labels are specified, 
the new object will re-use the labels from the object it exposes.


#### Examples

```bash
# Expose a service as a route
$ osc expose service frontend
# Expose a deployment config as a service and use the specified port and name
$ osc expose dc ruby-hello-world --port=8080 --name=myservice --generator=services/v1
```

osc process
---------------

This command processes a Template into a valid Config resource. The processing
will take care of generating values for parameters specified in the Template and
substituting the values in the corresponding places. An example Template can be
found in [examples/sample-app/application-template-stibuild.json](https://github.com/openshift/origin/blob/master/examples/sample-app/application-template-stibuild.json).

#### Examples

```
$ osc process -f examples/sample-app/application-template-stibuild.json > config.json
$ osc process -f template.json | osc create -f -
```

osc start-build
------------------

This command will manually start a build by specifying either a buildConfig or
a build name with the `--from-build` flag. There is also the option of streaming 
the logs of the build if the `--follow` flag is specified.

#### Examples

```
$ osc start-build ruby-sample-build
$ osc start-build --from-build=ruby-sample-build-275d3373-c252-11e4-bc79-080027c5bfa9
$ osc start-build --from-build=ruby-sample-build-275d3373-c252-11e4-bc79-080027c5bfa9 --follow 
```

osc resize
------------------

This command sets a new size for a Replication Controller either directly or via its Deployment Configuration.

#### Examples

```bash
# n is the highest deployment number for the dc ruby-hello-world
$ osc resize rc ruby-hello-world-n  --replicas=3
$ osc resize dc ruby-hello-world --current-replicas=3 --replicas=5
```

osc build-logs
------------------

> **NOTE**: This command will be later replaced by upstream (see: [kubectl log](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/kubectl-log.md#kubectl-log)).

This command will retrieve the logs from a Build container. It allows you to
debug broken Build. If the build is still running, this command can stream the
logs from the container to console. You can obtain a list of builds by using:

```
$ osc get builds
```

#### Examples

```
$ osc build-logs rubyapp-build
```
