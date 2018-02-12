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

* [CakePHP](https://raw.githubusercontent.com/openshift/library/master/official/cakephp/templates/cakephp-mysql-example.json) - Provides a basic CakePHP application with a MySQL database. For more information see the [source repository](https://github.com/openshift/cakephp-ex).
* [CakePHP persistent](https://raw.githubusercontent.com/openshift/library/master/official/cakephp/templates/cakephp-mysql-persistent.json) - Provides a basic CakePHP application with a persistent MySQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/cakephp-ex).

* [Dancer](https://raw.githubusercontent.com/openshift/library/master/official/dancer/templates/dancer-mysql-example.json) - Provides a basic Dancer (Perl) application with a MySQL database. For more information see the [source repository](https://github.com/openshift/dancer-ex).
* [Dancer persistent](https://raw.githubusercontent.com/openshift/library/master/official/dancer/templates/dancer-mysql-persistent.json) - Provides a basic Dancer (Perl) application with a persistent MySQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/dancer-ex).

* [Django](https://raw.githubusercontent.com/openshift/library/master/official/django/templates/django-psql-example.json) - Provides a basic Django (Python) application with a PostgreSQL database. For more information see the [source repository](https://github.com/openshift/django-ex).
* [Django persistent](https://raw.githubusercontent.com/openshift/library/master/official/django/templates/django-psql-persistent.json) - Provides a basic Django (Python) application with a persistent PostgreSQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/django-ex).

* [Httpd](https://raw.githubusercontent.com/openshift/library/master/official/httpd/templates/httpd-example.json) - Provides a basic Httpd static content application. For more information see the [source repository](https://github.com/openshift/httpd-ex).

* [Nginx](https://raw.githubusercontent.com/openshift/library/master/official/nginx/templates/nginx-example.json) - Provides a basic Nginx static content application. For more information see the [source repository](https://github.com/sclorg/nginx-ex).

* [NodeJS](https://raw.githubusercontent.com/openshift/library/master/official/nodejs/templates/nodejs-mongodb-example.json) - Provides a basic NodeJS application with a MongoDB database. For more information see the [source repository](https://github.com/openshift/nodejs-ex).
* [NodeJS persistent](https://raw.githubusercontent.com/openshift/library/master/official/nodejs/templates/nodejs-mongo-persistent.json) - Provides a basic NodeJS application with a persistent MongoDB database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/nodejs-ex).

* [Rails](https://raw.githubusercontent.com/openshift/library/master/official/rails/templates/rails-postgresql-example.json) - Provides a basic Rails (Ruby) application with a PostgreSQL database. For more information see the [source repository](https://github.com/openshift/rails-ex).
* [Rails persistent](https://raw.githubusercontent.com/openshift/library/master/official/rails/templates/rails-pgsql-persistent.json) - Provides a basic Rails (Ruby) application with a persistent PostgreSQL database. Note: requires available persistent volumes.  For more information see the [source repository](https://github.com/openshift/rails-ex).

Note: This file is processed by `hack/update-external-examples.sh`. New examples
must follow the exact syntax of the existing entries. Files in this directory
are automatically pulled down, do not modify/add files to this directory.
