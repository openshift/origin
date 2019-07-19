
# Contributing Guide

Want to help the Heketi project? This document covers some of the
policies and preferences contributors to the project need to
know about.

* [The Basics](#the-basics)
* [Contributor's Workflow](#contributors-workflow)

## The Basics

### New to Go?

Heketi is primarily written in Go and if you are new to the language, it is *highly* encouraged you take [A Tour of Go](http://tour.golang.org/welcome/1).

### New to GitHub?

If you are new to the GitHub process, please see https://guides.github.com/introduction/flow/index.html.

### Getting Started

1. Fork the Heketi GitHub project
1. Download latest Go to your system
1. Setup your [GOPATH](http://www.g33knotes.org/2014/07/60-second-count-down-to-go.html) environment
1. Type: `mkdir -p $GOPATH/src/github.com/heketi`
1. Type: `cd $GOPATH/src/github.com/heketi`
1. Type: `git clone https://github.com/heketi/heketi.git`
1. Type: `cd heketi`

Now you need to setup your repo where you will be pushing your changes into:

1. `git remote add <rname> <your-github-fork>`
1. `git fetch <rname>`

Where `<rname>` is a remote name of your choosing and `<your-github-fork>`
is a git URL that you can both pull from and push to.

For example if you called your remote "github", you can verify your
configuration like so:

```sh
$ git remote -v
github  git@github.com:jdoe1234/heketi.git (fetch)
github  git@github.com:jdoe1234/heketi.git (push)
origin  https://github.com/heketi/heketi (fetch)
origin  https://github.com/heketi/heketi (push)
```

### Building and Testing

To build the Heketi server and command line tool, type `make`
from the top of the Heketi source tree.

Heketi comes with a suite of module and system tests. To run the suite of
code quality checks, unit and module tests run `make test` from the top
of the Heketi source tree.

To run the extensive suite of functional and system tests, change to the
tests/functional directory and run.sh. (Please note that this suite of
tests has additional requirements on your system, please see below.)

### Trying Out the Server

If you've already built the server and client you are ready to try
running Heketi locally. Otherwise, type `make` at the top of the
tree now.

```sh
$ cp etc/heketi.json heketi.json
$ vi heketi.json  # Change "db" to --> "db": "heketi.db",
$ ./heketi --config=heketi.json
Heketi v2.1.0-dev-2-gcb07059
[heketi] INFO 2016/09/02 11:52:09 Loaded mock executor
[heketi] INFO 2016/09/02 11:52:09 Loaded simple allocator
[heketi] INFO 2016/09/02 11:52:09 GlusterFS Application Loaded
Listening on port 8080
```

You can now either background the Heketi server process or switch to
another terminal, and run:

```
$ export HEKETI_CLI_SERVER=http://localhost:8080
$ cd client/cli/go
$ ./heketi-cli cluster list
...
```

Here the tool will print the cluster by querying the running server.


## Contributor's Workflow

Here is a guide on how to work on a new patch and get it included
in the official Heketi sources.

### Preparatory work

Before you start working on a change, you should check the existing
issues and pull requests for related content. Maybe someone has
already done some analysis or even started a patch for your topic...

### Working on the code and creating patches

In this example, we will work on a patch called *hellopatch*:

1. `git checkout master`
1. `git pull`
1. `git checkout -b hellopatch`

Do your work here and then commit it. For example, run `git commit -as`,
to automatically include all your outstanding changes into a new
patch.

#### Splitting your change into commits

Generally, you will not just commit all your changes into a single
patch but split them up into multiple commits. It is perfectly
okay to have multiple patches in one pull request to achieve
the higher level goal of the pull request. (For example one patch
fix a bug and one patch to add a regression test.)

You can use `git add -i` to select which hunks of your change to
commit. `git rebase -i` can be used to polish up a sequence of
work-in-progress patches into a sequence of patches of merge quality.

Heketi's guidelines for the contents of commits are:
- Commits should usually be as minimal and atomic as possible.
- I.e. a patch should only contain one logical change but achieve it completely.
- If the commit does X and Y, you should probably split it into two patches.
- Each patch should compile and pass `make test`

#### Good commit messages

Each commit has a commit message. The heketi project prefers
commit messages roughly of the following form:

```
component(or topic)[:component]: Short description of what the patch does

Optionally longer explanation of the why and how.

Signed-off-by: Author Name <author@email>
```

#### Linking to issues

If you are working on an existing issue you should make sure to use
the appropriate [keywords](https://help.github.com/articles/closing-issues-via-commit-messages/)
in your commit message (e.g. `Fixes #<issue-number>`).
Doing so will allow GitHub to automatically
create references between your changes and the issue.


### Testing the Change

Each pull request needs to pass the basic test suite in order
to qualify for merging. It is hence highly recommended that you
run at least the basic test suite on your branch, preferably
even on each individual commit and make sure it passes
before submitting your changes for review.

#### Basic Tests

As mentioned in the section above Heketi has a suite of quality checks and
unit tests that should always be run before submitting a change to the
project. The simplest way to run this suite is to run `make test` in the
top of the source tree.

Sometimes it may not make sense to run the entire suite, especially if you
are iterating on changes in a narrow area of the code. In this case, you
can execute the [Go language test tool](https://golang.org/cmd/go/#hdr-Test_packages)
directly. When using `go test` you can specify a package (sub-directory)
and the tool will only run tests in that directory. For example:
```
go test -v github.com/heketi/heketi/apps/glusterfs
```

You can also run an individual unit test by appending `-run <TestName>`
to the invocation of `go test`. In order for go to find the test, there are
two options: either call `go test` from the directory that contains the test,
or specify the path in the invocation of `go test` as above. For example:
```
go test -v -run TestVolumeEntryCreateFourBricks github.com/heketi/heketi/apps/glusterfs
```

#### Functional Tests

You should also execute the functional tests on your system when possible.
If you have around 16G or more system memory then you should be able to run
the entire functional test suite. If you have 8 G or less, only run the
TestSmokeTest.

The functional test suite has dependencies on Vagrant, Libvirt, and Ansible.
You will need these tools installed on your system prior to running the
test suite. In order to run the whole functional test suite, you can execute
`make test-functional`. Each functional test suite is a subirectory of the
[Functional Tests Directory](../tests/functional), and can be executed
separately by running the `run.sh` script in that directory.

Refer to the [README](../tests/functional/README.md) and the
test scripts within the
[Functional Tests Directory](https://github.com/heketi/heketi/tree/master/tests/functional)
in the Heketi repository for all the gory details.

### Pull Requests

Once you are satisfied with your changes you can push them to your Heketi fork
on GitHub. For example, `git push github hellopatch` will push the contents
of your hellopatch branch to your fork.

Now that the patch or patches are available on GitHub you can use the GitHub
interface to create a pull request (PR). If you are submitting a single patch
GitHub will automatically populate the PR description with the content of
the change's commit message. Otherwise provide a brief summary of your
changes and complete the PR.

Usually, a PR should concentrate on one topic like a fix for a
bug, or the implementation of a feature. This can be achieved
with multiple commits in the patchset for this PR, but if your
patchset accomplishes multiple independent things, you should
probably split it up and create multiple PRs.

*NOTE*: The PR description is not a replacement for writing good commit messages.
Remember that your commit messages may be needed by someone in the future
who is trying to learn why a particular change was made.

### Patch Review

Now other developers involved in the project will provide feedback on your
PR. If a maintainer decides the changes are good as-is they will merge
the PR into the main Heketi repository. However, it is likely that some
discussion will occur and changes may be requested.

### Iterating on Feedback

You will need to return to your local clone and work through the changes
as requested. You may end up with multiple changes across multiple commits.
The Heketi project developers prefer a linear history where each change is
a clear logical unit. This means you will generally expect to rebase
your changes.

Run `git rebase -i master` and you will be presented with output something
like the following:

```
pick e405b76 my first change
pick ac78522 my second change
...
pick bf34223 my twentythird change

# Rebase d03eaf9..bf34223 onto d03eaf9
#
# Commands:
#  p, pick = use commit
#  r, reword = use commit, but edit the commit message
#  e, edit = use commit, but stop for amending
#  s, squash = use commit, but meld into previous commit
#  f, fixup = like "squash", but discard this commit's log message
#  x, exec = run command (the rest of the line) using shell
...
```

What you do here is highly dependent on what changes were requested but let's
imagine you need to combine the change "my twentythird change" with
"my first change". In that case you could alter the file such that it
looks like:

```
pick e405b76 my first change
fixup bf34223 my twentythird change
pick ac78522 my second change
...
```

This will instruct git to combine the two changes, throwing away the
latter change's commit message.

Once done you need to re-submit your branch to GitHub. Since you've altered
the existing content of the branch you will likely need to force push,
like so: `git push -f github hellopatch`.

After iterating on the feedback you have got from other developers the
maintainers may accept your patch. It will be merged into the project
and you can delete your branch. Congratulations!
