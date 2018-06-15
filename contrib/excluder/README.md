Excluder
========

Many times admins do not want certain packages updated 
when doing normal system updates.  The excluder scripts
exclude packages from yum or dnf so that the packages
are not updated.

The excluder scripts add and remove packages to the
exclude line of the yum or dnf config file.


excluder-template
-----------------

Create new excluder scripts by using the template.
Copy the template, then substitue values for the variables.

**Variables**

* @@CONF_FILE-VARIABLE@@
  * which config file to use (yum.conf or dnf.conf)
* @@PACKAGE_LIST-VARIABLE@@
  * Space seperated list of packages to exclude
  * The '*' wildcard is supported
  
**Example**

* sed "s|@@CONF_FILE-VARIABLE@@|yum.conf|" contrib/excluder/excluder-template > /usr/sbin/myspecial-excluder
* sed -i "s|@@PACKAGE_LIST-VARIABLE@@|zziplib docker*1.19* aalib|" /usr/sbin/myspecial-excluder


License
-------

Excluder is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).