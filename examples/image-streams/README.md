Imagestreams
===========

Imagestreams provide an abstraction for images located in a registry.  By referencing an imagestream (or a tag within an imagestream) instead
of referencing a image registry/repository:tag directly, your resources can be triggered when the underlying image changes, as well as control
when image updates are rolled out.

* [.NET Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/dotnet/imagestreams/dotnet-centos.json)
* [.NET RHEL7](https://raw.githubusercontent.com/openshift/library/master/official/dotnet/imagestreams/dotnet-rhel.json)

* [HTTPD Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/httpd/imagestreams/httpd-centos.json)
* [HTTPD RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/httpd/imagestreams/httpd-rhel.json)

* [Jenkins Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/jenkins/imagestreams/jenkins-centos.json)
* [Jenkins RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/jenkins/imagestreams/jenkins-rhel.json)

* [MariaDB Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/mariadb/imagestreams/mariadb-centos.json)
* [MariaDB RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/mariadb/imagestreams/mariadb-rhel.json)

* [MySQL Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/mysql/imagestreams/mysql-centos.json)
* [MySQL RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/mysql/imagestreams/mysql-rhel.json)

* [Nginx Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/nginx/imagestreams/nginx-centos.json)
* [Nginx RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/nginx/imagestreams/nginx-rhel.json)

* [NodeJS Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/nodejs/imagestreams/nodejs-centos.json)
* [NodeJS RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/nodejs/imagestreams/nodejs-rhel.json)

* [Perl Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/perl/imagestreams/perl-centos.json)
* [Perl RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/perl/imagestreams/perl-rhel.json)

* [PHP Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/php/imagestreams/php-centos.json)
* [PHP RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/php/imagestreams/php-rhel.json)

* [Python Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/python/imagestreams/python-centos.json)
* [Python RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/python/imagestreams/python-rhel.json)

* [PostgreSQL Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/postgresql/imagestreams/postgresql-centos.json)
* [PostgreSQL RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/postgresql/imagestreams/postgresql-rhel.json)

* [Redis Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/redis/imagestreams/redis-centos.json)
* [Redis RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/redis/imagestreams/redis-rhel.json)

* [Ruby Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/ruby/imagestreams/ruby-centos.json)
* [Ruby RHEL7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/official/ruby/imagestreams/ruby-rhel.json)

* [Wildfly Centos7](https://raw.githubusercontent.com/openshift/library/master/arch/x86_64/community/wildfly/imagestreams/wildfly-centos7.json)


Note: This file is processed by `hack/update-external-examples.sh`. New examples
must follow the exact syntax of the existing entries. Files in this directory
are automatically pulled down, do not modify/add files to this directory.
