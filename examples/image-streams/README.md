Imagestreams
===========

Imagestreams provide an abstraction for images located in a registry.  By referencing an imagestream (or a tag within an imagestream) instead
of referencing a image registry/repository:tag directly, your resources can be triggered when the underlying image changes, as well as control
when image updates are rolled out.

* [.NET Centos7](https://raw.githubusercontent.com/openshift/library/master/community/dotnet/imagestreams/dotnet-centos7.json)

* [HTTPD Centos7](https://raw.githubusercontent.com/openshift/library/master/community/httpd/imagestreams/httpd-centos7.json)
* [HTTPD RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/httpd/imagestreams/httpd-rhel7.json)

* [Jenkins Centos7](https://raw.githubusercontent.com/openshift/library/master/community/jenkins/imagestreams/jenkins-centos7.json)
* [Jenkins RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/jenkins/imagestreams/jenkins-rhel7.json)

* [MariaDB Centos7](https://raw.githubusercontent.com/openshift/library/master/community/mariadb/imagestreams/mariadb-centos7.json)
* [MariaDB RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/mariadb/imagestreams/mariadb-rhel7.json)

* [MongoDB Centos7](https://raw.githubusercontent.com/openshift/library/master/community/mongodb/imagestreams/mongodb-centos7.json)
* [MongoDB RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/mongodb/imagestreams/mongodb-rhel7.json)

* [MySQL Centos7](https://raw.githubusercontent.com/openshift/library/master/community/mysql/imagestreams/mysql-centos7.json)
* [MySQL RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/mysql/imagestreams/mysql-rhel7.json)

* [Nginx Centos7](https://raw.githubusercontent.com/openshift/library/master/community/nginx/imagestreams/nginx-centos7.json)
* [Nginx RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/nginx/imagestreams/nginx-rhel7.json)

* [NodeJS Centos7](https://raw.githubusercontent.com/openshift/library/master/community/nodejs/imagestreams/nodejs-centos7.json)
* [NodeJS RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/nodejs/imagestreams/nodejs-rhel7.json)

* [Perl Centos7](https://raw.githubusercontent.com/openshift/library/master/community/perl/imagestreams/perl-centos7.json)
* [Perl RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/perl/imagestreams/perl-rhel7.json)

* [PHP Centos7](https://raw.githubusercontent.com/openshift/library/master/community/php/imagestreams/php-centos7.json)
* [PHP RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/php/imagestreams/php-rhel7.json)

* [Python Centos7](https://raw.githubusercontent.com/openshift/library/master/community/python/imagestreams/python-centos7.json)
* [Python RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/python/imagestreams/python-rhel7.json)

* [PostgreSQL Centos7](https://raw.githubusercontent.com/openshift/library/master/community/postgresql/imagestreams/postgresql-centos7.json)
* [PostgreSQL RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/postgresql/imagestreams/postgresql-rhel7.json)

* [Redis Centos7](https://raw.githubusercontent.com/openshift/library/master/community/redis/imagestreams/redis-centos7.json)
* [Redis RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/redis/imagestreams/redis-rhel7.json)

* [Ruby Centos7](https://raw.githubusercontent.com/openshift/library/master/community/ruby/imagestreams/ruby-centos7.json)
* [Ruby RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/ruby/imagestreams/ruby-rhel7.json)

* [Wildfly Centos7](https://raw.githubusercontent.com/openshift/library/master/community/wildfly/imagestreams/wildfly-centos7.json)


Note: This file is processed by `hack/update-external-examples.sh`. New examples
must follow the exact syntax of the existing entries. Files in this directory
are automatically pulled down, do not modify/add files to this directory.
