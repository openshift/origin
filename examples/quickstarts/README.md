QuickStarts
===========

QuickStarts provide the basic skeleton of an application. Generally they
reference a repository containing very simple source code that implements a
trivial application using a particular framework. In addition they define any
components needed for the application including a Build configuration,
supporting services such as Databases, etc.

You can instantiate these templates as is, or fork the source repository they
reference and supply your forked repository as the source-repository when
instantiating them.

* [CakePHP](https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql.json) - Provides a basic CakePHP application with a MySQL database. For more information see the [source repository](https://github.com/openshift/cakephp-ex).
* [CakePHP persistent](https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql-persistent.json) - Provides a basic CakePHP application with a persistent MySQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/cakephp-ex).
* [Dancer](https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json) - Provides a basic Dancer (Perl) application with a MySQL database. For more information see the [source repository](https://github.com/openshift/dancer-ex).
* [Dancer persistent](https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql-persistent.json) - Provides a basic Dancer (Perl) application with a persistent MySQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/dancer-ex).
* [Django](https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django-postgresql.json) - Provides a basic Django (Python) application with a PostgreSQL database. For more information see the [source repository](https://github.com/openshift/django-ex).
* [Django persistent](https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django-postgresql-persistent.json) - Provides a basic Django (Python) application with a persistent PostgreSQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/django-ex).
* [Httpd](https://raw.githubusercontent.com/openshift/httpd-ex/master/openshift/templates/httpd.json) - Provides a basic Httpd static content application. For more information see the [source repository](https://github.com/openshift/httpd-ex).
* [NodeJS](https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb.json) - Provides a basic NodeJS application with a MongoDB database. For more information see the [source repository](https://github.com/openshift/nodejs-ex).
* [NodeJS persistent](https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb-persistent.json) - Provides a basic NodeJS application with a persistent MongoDB database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/nodejs-ex).
* [Rails](https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json) - Provides a basic Rails (Ruby) application with a PostgreSQL database. For more information see the [source repository](https://github.com/openshift/rails-ex).
* [Rails persistent](https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql-persistent.json) - Provides a basic Rails (Ruby) application with a persistent PostgreSQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/rails-ex).

Note: This file is processed by `hack/update-external-examples.sh`. New examples
must follow the exact syntax of the existing entries. Files in this directory
are automatically pulled down, do not modify/add files to this directory.
