depcheck
========

Dependency analysis tool for Go repositories.

TL;DR traverse a Go repository, creating a dependency graph which can be used to
visualize the directory structure, or to generate an analysis report. Skip to the
[Quick setup for Origin](#origin) section if running against `openshift/origin`.

__Table of Contents__
- [Why](#why)
- [Getting started](#started)
  - [Compiling](#compiling)
  - [Running](#running)
  - [Limitations](#limitations)
  - [Quick setup for Origin](#origin)
- [Advanced Usage](#advanced)
  - [Analyze](#analyze)
  - [Trace](#trace)
  - [Filters](#filters)
  - [Excludes](#excludes)
  
<a name="why"></a>
## Why

In the case of OpenShift, having a flat `vendor` dependency tree that brings in a
large enough dependency, such as `kubernetes`, creates a problem with conflicting
dependencies. It may be the case that we depend on a specific version of package `A`,
while `kubernetes` (or one of its dependencies) depends on that same package `A`, 
but a different version of it.
Because only one version of package `A` may exist under the `vendor` directory, it
might be useful to know:
  - Which parts of our codebase depend on package `A`, and are responsible for the conflict?
  - Would those parts of our codebase work with the version of package `A` that is brought
  in by `kubernetes` (or one of its dependencies)?
  - If not, can we break our dependency on package `A`?
 
Both the `analyze` and `trace` commands included in this tool are meant to aid in 
answering the above questions.

<a name="started"></a>
## Getting started

<a name="compiling"></a>
### Compiling

To compile the `depcheck` binary, simply use `go build` and point to the tool's entrypoint:

```bash
$ go build /path/to/openshift/origin/tools/depcheck/depcheck.go
```

After this step, you should have a `depcheck` binary in your current working directory.


<a name="running"></a>
### Running

The `depcheck` tool can be used to obtain `dot` output for a given repository.
This output may be fed into tools like [dot](https://www.systutorials.com/docs/linux/man/1-dot/)
or [d3-graphviz](https://github.com/magjac/d3-graphviz) in order to obtain a visual dependency graph.

Example:

Invoke the `trace` command using `openshift`-specific defaults, generating DOT output which can
be used to render a graph of the repository:

```bash
$ depcheck trace --root=github.com/openshift/origin --entry=cmd/... --openshift > graph.dot
```

*Notes*

- The `--root` flag contains the fully-qualified Go import-path of the target repository.

Additionally, this tool can be used to obtain an analysis report of your repository's vendored packages
against those of a vendored dependency. This output lists which packages you and them have in common, as
well as which ones are unique to either of you.

Example:

Invoke the `analyze` command using `openshift`-specific defaults, comparing dependencies against those of `kubernetes`:

```bash
$ depcheck analyze --root=github.com/openshift/origin --entry=cmd/... --entry=pkg/... --entry=tools/... --entry=test/... --openshift --dep=github.com/openshift/origin/vendor/k8s.io/kubernetes
```

*Notes*

- The ellipsis (`...`) after each `--entry` flag value indicate that you wish to recurse through that directory and include those packages.
  - It is possible to omit the ellipsis if you wish to do a "shallow" analysis of only that directory, as long as the directory path provided contains at least a single Go file under it.
- Directory names provided via `--entry` may be specified relative to the value of `--root`, or as fully qualified Go import-paths.


The `depcheck` tool can further be used to `pin` dependencies from a `glide.lock` file onto a `glide.yaml` file.
*Pinning* these dependencies means that you are taking all dependencies listed in a lock file that are not currently present
in the given `glide.yaml` file, and generating a new `glide.yaml` file that additionally contains this new set of dependencies. Each newly added dependency
is pinned to the same commit hash that was specified for it in the lock file.

Example:

```bash
$ depcheck pin --glide.yaml=glide.yaml --glide.lock=glide.lock > newglide.yaml
```

<a name="limitations"></a>
### Limitations

Before beginning to use this tool, it is important to note that because both the `trace`
and `analyze` commands make use of [go list](https://golang.org/cmd/go/#hdr-List_packages)
in order to obtain information about packages in a Go repository, there are limitations
that exist for these two commands:  

  - No symlinked directories will be traversed in a given repo. Any resulting graph information generated
by these commands will simply not include these directories or their subtrees. (Since Origin heavily relies on symlinks,
jump to the [solution for it here](#symlink-solution))

  - A `GOPATH` must be set in the environment where this tool will be used.

As a result, of these limitations, it is recommended that any symlinks in a repo be "expanded" before using these
two commands.

If you are dealing with a repository that is free of symlinks, feel free to skip to the
next section `Compiling`.

<a name="symlink-solution"></a>
A straightforward way of expanding symlinks for a given repository is to use `cp -L` to copy the original repository
to a temporary location, which replaces symbolic links with the data that each symlink points to:

```bash
# setup the new temporary location
$ mkdir -p /tmp/depcheck/src/github.com/openshift
$ cp -rL /path/to/openshift/origin /tmp/depcheck/src/github.com/openshift/origin

# set the GOPATH to look in the new temporary location, in order
# to ensure we traverse packages of this particular directory, and
# not the original "origin" directory
$ export GOPATH=/tmp/depcheck
```

<a name="origin"></a>
### Quick Setup for Origin

Taking the above steps and limitations into account, here is a quick setup guide
that is specific to the `openshift/origin` repository:

```bash
# create a temporary directory where we will copy the Origin repo.
mkdir -p /tmp/depcheck/src/github.com/openshift
```

```bash
# copy an existing copy of the Origin repo to the temp location we just created.
# the purpose behind this is to expand all of the repo's symlinks (we have a few,
# so this might take a minute).
cp -rL /path/to/openshift/origin /tmp/depcheck/src/github.com/openshift/origin
```

```bash
# ensure your current working directory is the new temp location
cd /tmp/depcheck/src/github.com/openshift/origin
```

```bash
# set the GOPATH to look in the new temporary location, in order
# to ensure we traverse packages of this particular directory, and
# not the original "origin" directory
export GOPATH=/tmp/depcheck
```

```bash
# compile the depcheck binary
go build tools/depcheck/depcheck.go
```

```bash
# with the depcheck binary in your current working directory, you
# are now ready to run any command. Use the --openshift flag for
# automatically using default package excludes and package filters
# specific to the Origin repo. You can read more about excludes and
# filters in the "Advanced Usage" section.

# the command below analyzes first-level dependencies for the Origin repo,
# and outputs a list of dependencies shared between Origin and our vendored
# version of kubernetes.
./depcheck analyze --root=github.com/openshift/origin --entry=cmd/... --entry=pkg/... --entry=tools/... --entry=test/... --openshift --dep=github.com/openshift/origin/vendor/k8s.io/kubernetes
```

```bash
# the command below creates a dependency graph (in "dot" format)
# for the origin repo, given a series of entrypoints into the repo.
./depcheck trace --root=github.com/openshift/origin --entry=cmd/... --entry=pkg/... --entry=tools/... --entry=test/... --openshift > graph.dot

# the resulting output in the ./graph.dot file can then be fed into a graph rendering
# tool, such as "dot" to create a "svg" image, for example, which can be viewed in a browser.
dot -T svg graph.dot > graph.svg
```

<a name="advanced"></a>
## Advanced Usage

<a name="analyze"></a>
### Analyze

The `analyze` command requires the following flags:
  - `--root=`  The `Go` import path of the repository root (e.g. `github.com/foo/bar`)
  - `--entry=` The (relative) path for each of the entry-point directories to be used to begin traversing the repository's directory structure. (e.g. `./pkg`)
  - `--dep=`   The full `Go` import path of the vendored dependency whose dependencies we are analyzing against our own (e.g. `github.com/foo/bar/vendor/k8s.io/kubernetes`).
 
Optionally, the following flags may be specified as well:
  - `--filter=`   The path to the `filters.json` file containing package paths to "squash".
  - `--exclude=`  The path to the `excludes.json` file containing package paths to exclude from the internal graph and analysis.
  - `--openshift` Mutually exclusive with `--filter` and `--exclude`. Uses `openshift` specific repository information to generate an internal list of package paths to exclude and filter (requires that the `origin` repository exist within your `GOPATH`).

Example:

Invoke the `analyze` command, comparing dependencies against those of `kubernetes`:

```bash
$ depcheck analyze --root=github.com/openshift/origin --entry=cmd/... --entry=pkg/... --entry=tools/... --entry=test/... --exclude=tools/depcheck/examples/origin_excludes.json --filters=tools/depcheck/examples/origin_filters.json --dep=github.com/openshift/origin/vendor/k8s.io/kubernetes
```

*Notes*
- Unlike the example listed in the [Running](#running) section, the command above does not make use of `--openshift` specific defaults.
Instead, it provides a path to a file containing a JSON array of Go import-paths to exclude, as well as paths to filter on.
See the [Excludes](#excludes) and [Filters](#filters) sections for more details on these files.

The above command will use the `./cmd`, `./pkg`, `./tools`, and `./test` directories as points of origin, to begin traversing the repo.
It will generate an output similar to the one below:

```
"Yours": 39 dependencies exclusive to ["github.com/openshift/origin/vendor/k8s.io/kubernetes"]
    - github.com/clusterhq/flocker-go
    - github.com/quobyte/api
    - github.com/vmware/govmomi
    - github.com/xanzy/go-cloudstack
    - github.com/rancher/go-rancher
    ...
"Mine": 4 direct (first-level) dependencies exclusive to the origin repo
    - github.com/vjeantet/ldapserver
    - github.com/openshift/imagebuilder
    - github.com/gonum/graph
    - github.com/joho/godotenv

"Ours": 79 shared dependencies between the origin repo and [github.com/openshift/origin/vendor/k8s.io/kubernetes]
    - github.com/ghodss/yaml
    - github.com/RangelReale/osincli
    - github.com/aws/aws-sdk-go
    ... 
```

<a name="trace"></a>
### Trace

The `trace` command requires the following flags:
  - `--root=`  The `Go` import path of the repository root (e.g. `github.com/foo/bar`)
  - `--entry=` The (relative) path for each of the entry-point directories to be used to begin traversing the repository's directory structure. (e.g. `./pkg`)
 
Optionally, the following flags may be specified as well:
  - `--filter=`   The path to the `filters.json` file containing package paths to "squash".
  - `--exclude=`  The path to the `excludes.json` file containing package paths to exclude from the internal graph and analysis.
  - `--openshift` Mutually exclusive with `--filter` and `--exclude`. Uses `openshift` specific repository information to generate an internal list of package paths to exclude and filter (requires that the `origin` repository exist within your `GOPATH`).
  - `--output` The particular type of output to use when printing graph information to standard output (defaults to `dot`).

Example:

Generate `dot` output for packages in the openshift repo, reachable from the `./pkg` and `./vendor` directories, and render a graph using the graphviz `dot` utility:

```bash
$ depcheck trace --root=github.com/openshift/origin --entry=./pkg/... | dot -T png > graph.png
```

*Notes*
- Although we meant to also include the `./vendor` subtree as part of the command's traversal,
it is not necessary to explicitly provide it. The `vendor` directory is implicitly traversed every time the `trace` or `analyze` commands are run.

<a name="filters"></a>
### Filters

Commands like `analyze` and `trace` build an internal graph of the given repository, starting
at user-specified entry-points. The graph that is internally generated may be "squashed" or
filtered depending on the level of detail that is needed. Filtered paths are defined in
a json file, whose path can be specified via the `--filter` flag.

Example:

Given the following file...
```
$ cat filters.json
[
    "github.com/openshift/origin/pkg/api",
    "github.com/openshift/origin/pkg/auth"
]
```

...the internal graph that is built will still contain nodes for every Go package traversed in
the repo, as well as the `api` and `auth` packages, but the sub-trees of these two packages
will be "squashed" into `api` and `auth`. This means that while the child nodes of these two
packages are deleted, the `edges` of those nodes to other sub-trees are preserved.

For example:

Given the following set of package dependencies (assume directed edges from parent to child, starting with `A` and `D`):

```
      A                D
     / \              / \
(A/B)   (A/C) -> (D/E)   (D/F)
     \
      (A/B/C)
```

Since the set of packages contained within repository `A` brings in a package `A/C` that depends on a package `D/E`, which is brought in by repository `D`,
"squashing" these two sets of packages into their roots `A` and `D` results in the following:

```
      A -> D
```

[See the `depcheck` directory for filter examples](https://github.com/openshift/origin/tree/master/tools/depcheck/examples)

<a name="excludes"></a>
### Excludes

Just as it is possible to squash sub-packages into a parent package within a repository, it 
is also possible to exclude entire directory sub-trees from a repository.

Example:

Given the following file...
```
$ cat excludes.json
[
    "github.com/openshift/origin/images",
    "github.com/openshift/origin/pkg/build/builder"
    "github.com/openshift/origin/cmd/service-catalog"
]
```

...the internal graph that is built will not contain nodes for any directory sub-trees under
the specified set of paths in `excludes.json`.

*Notes*

- Keep in mind that `depcheck` will still use `go list` internally to traverse the entire tree,
including paths that are supposed to be excluded. However, once all Go package information has
been collected, the nodes themselves will not be created for those packages while the graph is
being built.
