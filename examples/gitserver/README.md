Configurable Git Server
=======================

Overview
--------

This example git server is intended for use within a container or Kubernetes pod.
It can clone repositories from remote systems on startup as well as automatically
launch builds in an OpenShift project. It can automatically initialize and receive
Git directories on push.

Hooks can be customized to perform actions on push such as invoking `oc new-app` on a
repository's source.

The Dockerfile built by this example is published as openshift/origin-gitserver

Persistent and ephemeral templates are provided. For OpenShift Online you need to use
the persistent one.


Quick Start
-----------

Prerequisites:

* You have an OpenShift v3 server running
* You are logged in as a standard user (not as system:admin) and have access to a project
* You have the `gitserver-ephemeral.yaml` or `gitserver-persistent.yaml` from this directory
* You can create externally accessible routes on your server

### Deploy the Git Server

1. Create the Git Server

    ```sh
    $ oc create -f gitserver-ephemeral.yaml
    ```

    OR 

    ```sh
    $ oc create -f gitserver-persistent.yaml
    ```

2. Grant `edit` access to the `git` service account

    ```sh
    $ oc policy add-role-to-user edit -z git
    ```


### Push code to the Git Server

Code repositories can be initialized on the Git Server by either doing a 'git push' or
a 'git clone' of the repository.


1. Find your git server's URL

   If you have a working router, determine the host name of your git server by getting its route:

   ```sh
   $ GITSERVER=http://$(oc get route git -o template --template '{{.spec.host}}')
   ```

   In this case, the URL of your git server will be the host name used by the route; something like: 
   
   ```sh
   $ echo $GITSERVER
   http://git-myproject.router.default.svc.cluster.local
   ```
  
   Alternatively, if your router is not functional, you can port-forward the git-server pod to your local machine.
   In a separate shell window, execute the following: (You must leave the port-forward command running
   for the port to be forwarded to your machine)

   ```
   $ oc get pods | grep git           ## get the git server pod
   $ oc port-forward -p PODNAME 8080  ## start port-forward where PODNAME is the git server pod
   ```

   In this case, the URL of your git server will be your local host:

   ```sh
   $ GITSERVER=http://localhost:8080
   ```


2. Setup your credentials

   By default the Git Server will allow users or service accounts that can create pods in
   the project namespace to create and push code to the Git Server. The easiest way to 
   provide credentials to the Git Server is by using a custom credential helper that will 
   send your OpenShift token by default to the server.

   **NOTE:** the config key is `credential.[git server URL].helper`

   ```sh
   $ git config --global credential.$GITSERVER.helper \
         '!f() { echo "username=$(oc whoami)"; echo "password=$(oc whoami -t)"; }; f'
   ```

3. Push a repository to your git server

   In an existing repository, add a remote that points to the git server and push to it
   
   ```sh
   # clone a public repository
   $ git clone https://github.com/openshift/ruby-hello-world.git

   # add a remote for your git server
   $ cd ruby-hello-world
   $ git remote add openshift $GITSERVER/ruby-hello-world.git

   # push the code to the git server
   $ git push openshift master
   ```

   **NOTE:** the ruby-hello-world.git repository does not exist before running these commands. 
   By pushing to it, you are creating it in the Git Server.

   On push the git server will invoke new-app on the code and create artifacts for it in 
   OpenShift.


### Secure the Git Server

Beyond demo uses, it is recommended that communication with the Git server be encrypted with the TLS
protocol to avoid transmission of source in plain text.

1. Modify your route to use edge termination:

   ```sh
   $ oc edit route git
   ```

   Add tls -> termination -> edge to the route specification:

   ```yaml
   apiVersion: v1
   kind: Route
   metadata:
     name: git
   spec:
     host: git-myproject.router.default.svc.cluster.local
     tls:
       termination: edge
     to:
       kind: Service
       name: git
   ```

2. If using a private certificate authority, configure your git client to use the private ca.crt file:

   ```sh
   $ git config --global http.https://git-myproject.router.default.svc.cluster.local.sslCAInfo /path/to/ca.crt
   ```

   where the key is http.[git server URL].sslCAInfo

3. Disable anonymous cloning. By default the git server will allow anonymous cloning to make it easier to
   run builds without having to specify a secret. You can disable this by setting the `ALLOW_ANON_GIT_PULL`
   environment variable to `false`.

   ```sh
   $ oc set env dc/git ALLOW_ANON_GIT_PULL=false
   ```

   **NOTE:** changing environment variables on the git server will cause a redeploy of the git server. If using
   the default ephemeral storage for it, all repositories that have been pushed previously will be wiped out.
   They will need to be pushed to the server again to restore.


Authentication
--------------

By default, the git server will authenticate using OpenShift user or service account credentials. For a user,
the credentials are the user name, and the user's token (from `oc whoami -t`). For a service account, the user
name is the service account name and the password is the service account token. The token can
be one of the 2 tokens created with the service account and stored in the service account secrets. These can 
be obtained by looking at the secrets that correspond to the service account (`oc get secrets`) and displaying
the token in one of them: `oc describe secret NAME`.


Authorization
-------------

Users or service accounts must be able to read pods in the current namespace in order to fetch repositories from
the Git Server. Specifying ALLOW_ANON_GIT_PULL=true as an environment variable for the Git Server will allow anyone
to fetch/clone content from the Git Server. To create/push content, users or service accounts must have the right 
to create pods in the namespace.


Automatically Cloning Public Repositories
-----------------------------------------

The Git Server can automatically clone a set of repositories on startup if it is going to be used for mirroring
purposes. Repositories to be cloned can be specified using a set of environment variables that match
`GIT_INITIAL_CLONE_[name]` where [name] must be unique. The value of the variable must be a URL to the remote
repository to clone and an optional name for the clone.

The following command will add an environment variable to clone the openshift/ruby-hello-world repository and give
it a name of `helloworld` inside the Git Server.

```sh
$ oc set env dc/git GIT_INITIAL_CLONE_1="https://github.com/openshift/ruby-hello-world.git;helloworld"
```

Automatically Starting Builds
-----------------------------

By default, whenever the git server receives a commit, it will look for a BuildConfig in the same namespace as the
git server and the same name as the repository where the commit is being pushed.  If it finds a BuildConfig with 
the same name, it will start a build for that BuildConfig. Alternatively, an annotation may be added to a 
BuildConfig to explicitly link it to a repository on the git server:

```yaml
apiVersion: v1
kind: BuildConfig
metadata:
  annotations:
      openshift.io/git-repository: myrepo
```

**NOTE**: A build will be started for the BuildConfig matching the name of the repository and for any BuildConfig 
that has an annotation pointing to the source repository. If there is a BuildConfig that has a matching name but
has an annotation pointing to a different repository, a build will not be invoked for it.


Build Strategy
--------------

When automatically starting a build, the git server will create a Docker type build if a Dockerfile is present
in the repository. Otherwise, it will attempt a source type build. To force the git server to always use one
strategy, set the BUILD_STRATEGY environment variable.

Setting the BUILD_STRATEGY to `docker` will force new builds to be created with the Docker strategy:

```sh
oc set env dc/git BUILD_STRATEGY=docker
```

For OpenShift online which does not allow Docker type builds, you will need to set the strategy to `source` 
if your repository contains a `Dockerfile`:

```sh
oc set env dc/git BUILD_STRATEGY=source
```

Valid values for BUILD_STRATEGY are "" (empty string), `source`, and `docker`.
