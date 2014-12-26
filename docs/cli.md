cli command line interface
==============================

The `cli` command line tool is used to interact with the [OpenShift](http://openshift.github.io) and [Kubernetes](http://kubernetes.io/) HTTP API(s).

cli is *verb focused*. The base verbs are *get*, *create*, *delete*,
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

cli get
-----------

This command can be used for displaying one or many resources. Possible
resources are all OpenShift resources (builds, buildConfigs, deployments,
deploymentConfigs, images, imageRepositories, routes, projects and others) and
all Kubernetes resources (pods, replicationControllers, services, minions,
events).

### Example Usage

```
$ openshift cli get pods
$ openshift cli get replicationController 1234-56-7890-234234-456456
$ openshift cli get service database
$ openshift cli get -f json pods
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

`$ openshift cli get builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`

### Selectors

cli `get` provides also 'selectors' that you can use to filter the output
by applying key-value pairs that will be matched with the resource labels:

`$ openshift clic get pods -s template=production`

This command will return only pods whose `labels` include `"template": "production"`

cli create
--------------

This command can be used to create resources. It does not require pointers about
what resource it should create because it reads it from the provided JSON/YAML.
After successful creation, the resource name will be printed to the console.

### Example Usage

```
$ openshift cli create -f pod.json
$ cat pod.json | openshift cli create -f -
$ openshift cli create -f http://server/pod.json
```

cli update
---------------

This command can be used to update existing resources.

### Example Usage

```
$ openshift cli update -f pod.json
$ cat pod.json | openshift cli update -f -
$ openshift cli update -f http://server/pod.json
```

cli delete
--------------

This command deletes the resource.

### Example Usage

```
$ openshift cli delete -f pod.json
$ openshift cli delete pod 1234-56-7890-234234-456456
```

cli describe
----------------

The `describe` command is a wordier version of `get` which also integrates other
information that's related to a given resource.

### Example Usage

`$ openshift cli describe service frontend`

cli createall
-----------------

This command creates multiple resources defined in JSON or YAML file provided
using the `-f` option. The list of resources is defined as an 'array'.

### Example Usage

```
$ openshift cli createall -f resources.json
$ cat resources.json | openshift cli createall -f -
```

cli namespace
-----------------

You can use this command to set the default namespace used for all cli
commands.

### Example Usage

`$ openshift cli namespace myuser`

cli log
------------

This command dumps the logs from a given Pod container. You can list the
containers from a Pod using the following command:

`$ openshift cli get -o yaml pod POD_ID`

### Example Usage

`$ openshift cli log frontend-pod mysql-container`

cli apply
-------------

> **NOTE**: This command will be later replaced by upstream (see: [kubectl createall](https://github.com/openshift/origin/blob/master/docs/cli.md#kubectl-createall)).

This command takes a Config resource that defines a list of resources and performs
create operation on them. Look at [examples/sample-app/docker-registry-config.json](https://github.com/openshift/origin/blob/master/examples/sample-app/docker-registry-config.json).


### Example Usage

```
$ openshift cli apply -f examples/sample-app/docker-registry-config.json
$ cat config.json | openshift cli apply -f -
```

cli process
---------------

This command processes a Template into a valid Config resource. The processing
will take care of generating values for parameters specified in the Template and
substituting the values in the corresponding places. An example Template can be
found in [examples/guestbook/template.json](https://github.com/openshift/origin/blob/master/examples/guestbook/template.json).

### Example Usage

```
$ openshift cli process -f examples/guestbook/template.json > config.json
$ openshift cli process -f template.json | openshift cli apply -f -
```

cli build-logs
------------------

> **NOTE**: This command will be later replaced by upstream (see: [kubectl log](https://github.com/openshift/origin/blob/master/docs/cli.md#kubectl-log)).

This command will retrieve the logs from a Build container. It allows you to
debug broken Build. If the build is still running, this command can stream the
logs from the container to console. You can obtain a list of builds by using:

`$ openshift cli get builds`

### Example Usage

`$ openshift cli build-logs rubyapp-build`
