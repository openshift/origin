OpenShift 3 Database Examples
=============================

This directory contains example JSON templates to deploy databases in OpenShift.
They can be used to immediately instantiate a database and expose it as a
service in the current project, or to add a template that can be later used from
the Web Console or the CLI.

The examples can also be tweaked to create new templates.


## Ephemeral vs. Persistent

For each supported database, there are two template files.

Files named `*-ephemeral-template.json` use
"[emptyDir](https://docs.openshift.org/latest/dev_guide/volumes.html)" volumes
for data storage, which means that data is lost after a pod restart.
This is tolerable for experimenting, but not suitable for production use.

The other templates, named `*-persistent-template.json`, use [persistent volume
claims](https://docs.openshift.org/latest/architecture/additional_concepts/storage.html#persistent-volume-claims)
to request persistent storage provided by [persistent
volumes](https://docs.openshift.org/latest/architecture/additional_concepts/storage.html#persistent-volumes),
that must have been created upfront.


## Usage

### Instantiating a new database service

Use these instructions if you want to quickly deploy a new database service in
your current project. Instantiate a new database service with this command:

    $ oc new-app /path/to/template.json

Replace `/path/to/template.json` with an appropriate path, that can be either a
local path or an URL. Example:

    $ oc new-app https://raw.githubusercontent.com/openshift/origin/master/examples/db-templates/mongodb-ephemeral-template.json

The parameters listed in the output above can be tweaked by specifying values in
the command line with the `-p` option:

    $ oc new-app examples/db-templates/mongodb-ephemeral-template.json -p DATABASE_SERVICE_NAME=mydb -p MONGODB_USER=default

Note that the persistent template requires an existing persistent volume,
otherwise the deployment won't ever succeed.


### Adding a database as a template

Use these instructions if, instead of instantiating a service right away, you
want to load the template into an OpenShift project so that it can be used
later. Create the template with this command:

    $ oc create -f /path/to/template.json

Replace `/path/to/template.json` with an appropriate path, that can be either a
local path or an URL. Example:

    $ oc create -f https://raw.githubusercontent.com/openshift/origin/master/examples/db-templates/mongodb-ephemeral-template.json
    template "mongodb-ephemeral" created

The new template is now available to use in the Web Console or with `oc
new-app`.


## Available database example templates

* [MariaDB](https://raw.githubusercontent.com/openshift/library/master/official/mariadb/templates/mariadb-ephemeral.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/mariadb.html).
* [MariaDB Persistent](https://raw.githubusercontent.com/openshift/library/master/official/mariadb/templates/mariadb-persistent.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/mariadb.html).
* [MongoDB](https://raw.githubusercontent.com/openshift/library/master/official/mongodb/templates/mongodb-ephemeral.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/mongodb.html).
* [MongoDB Persistent](https://raw.githubusercontent.com/openshift/library/master/official/mongodb/templates/mongodb-persistent.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/mongodb.html).
* [MySQL](https://raw.githubusercontent.com/openshift/library/master/official/mysql/templates/mysql-ephemeral.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/mysql.html).
* [MySQL Persistent](https://raw.githubusercontent.com/openshift/library/master/official/mysql/templates/mysql-persistent.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/mysql.html).
* [PostgreSQL](https://raw.githubusercontent.com/openshift/library/master/official/postgresql/templates/postgresql-ephemeral.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/postgresql.html).
* [PostgreSQL Persistent](https://raw.githubusercontent.com/openshift/library/master/official/postgresql/templates/postgresql-persistent.json) - For more information see the [product documentation](https://docs.openshift.org/latest/using_images/db_images/postgresql.html).
* [Redis](https://raw.githubusercontent.com/openshift/library/master/official/redis/templates/redis-ephemeral.json) - For more information see the [image documentation](https://github.com/sclorg/redis-container/blob/master/README.md).
* [Redis Persistent](https://raw.githubusercontent.com/openshift/library/master/official/redis/templates/redis-persistent.json) - For more information see the [image documentation](https://github.com/sclorg/redis-container/blob/master/README.md).

Note: This file is processed by `hack/update-external-examples.sh`. New examples
must follow the exact syntax of the existing entries. Files in this directory
are automatically pulled down, do not modify/add files to this directory.
