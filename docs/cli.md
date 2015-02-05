osc command line interface
==============================

The `osc` command line tool is used to interact with the [OpenShift](http://openshift.github.io) and [Kubernetes](http://kubernetes.io/) HTTP API(s).

osc is *verb focused*. The base verbs are *get*, *create*, *delete*,
*update* and *describe*. These verbs can be used to manage both Kubernetes and
OpenShift resources.

Some verbs support `-f` flag, which accepts regular file path, URL or '-' for
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
deploymentConfigs, images, imageRepositories, routes, projects and others) and
all Kubernetes resources (pods, replicationControllers, services, minions,
events).

### Example Usage

```
$ osc get pods
$ osc get replicationController 1234-56-7890-234234-456456
$ osc get service database
$ osc get -f json pods
```

### Output formatting

You can control the output format by using the `-o format` flag for `get`.
By default, `cli` uses human-friendly printer format for console.

You can control what API version will be used to print the resource by using the
`--output-version` flag. By default it uses the latest API version.

Available formats include:

| Value        | Description                                           |
|:-------------|:------------------------------------------------------|
| json         | Pretty formated JSON format |
| yaml         | [YAML](http://www.yaml.org/) format |
| template     | User defined [Go template](http://golang.org/pkg/text/template) (combined with the `-t` flag |
| templatefile | Same as above, but use the template file instead of `-t` |

An example of using `-o template` to retrieve the *name* of the first build:

`$ osc get builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`

### Selectors

osc `get` provides also 'selectors' that you can use to filter the output
by applying key-value pairs that will be matched with the resource labels:

`$ osc get pods -s template=production`

This command will return only pods whose `labels` include `"template": "production"`

osc create
--------------

This command can be used to create resources. It does not require pointers about
what resource it should create because it reads it from the provided JSON/YAML.
After successful creation, the resource name will be printed to the console.

### Example Usage

```
$ osc create -f pod.json
$ cat pod.json | osc create -f -
$ osc create -f http://server/pod.json
```

osc update
---------------

This command can be used to update existing resources.

### Example Usage

```
$ osc update -f pod.json
$ cat pod.json | osc update -f -
$ osc update -f http://server/pod.json
```

osc delete
--------------

This command deletes the resource.

### Example Usage

```
$ osc delete -f pod.json
$ osc delete pod 1234-56-7890-234234-456456
```

osc describe
----------------

The `describe` command is a wordier version of `get` which also integrates other
information that's related to a given resource.

### Example Usage

`$ osc describe service frontend`

osc namespace
-----------------

You can use this command to set the default namespace used for all osc
commands.

### Example Usage

`$ osc namespace myuser`

osc log
------------

This command dumps the logs from a given Pod container. You can list the
containers from a Pod using the following command:

`$ osc get -o yaml pod POD_ID`

### Example Usage

`$ osc log frontend-pod mysql-container`

osc process
---------------

This command processes a Template into a valid Config resource. The processing
will take care of generating values for parameters specified in the Template and
substituting the values in the corresponding places. An example Template can be
found in [examples/sample-app/application-template-stibuild.json](https://github.com/openshift/origin/blob/master/examples/sample-app/application-template-stibuild.json).

### Example Usage

```
$ osc process -f examples/guestbook/template.json > config.json
$ osc process -f template.json | osc create -f -
```

osc build-logs
------------------

> **NOTE**: This command will be later replaced by upstream (see: [kubectl log](https://github.com/openshift/origin/blob/master/docs/cli.md#kubectl-log)).

This command will retrieve the logs from a Build container. It allows you to
debug broken Build. If the build is still running, this command can stream the
logs from the container to console. You can obtain a list of builds by using:

`$ osc get builds`

### Example Usage

`$ osc build-logs rubyapp-build`
