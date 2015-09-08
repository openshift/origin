FROM openshift/openldap-2441-centos7:latest

# OpenLDAP server image for OpenShift Origin testing based on OpenLDAP 2.4.41
#
# Volumes:
# * /var/lib/openldap/data - Datastore for OpenLDAP
# * /etc/openldap/slapd.d  - Config directory for slapd
# Environment:
# * $OPENLDAP_DEBUG_LEVEL (Optional) - OpenLDAP debugging level, defaults to 256

MAINTAINER Steve Kuznetsov <skuznets@redhat.com>

# Add LDAP test data and script
COPY *init.sh /usr/local/bin/
COPY contrib/init.ldif /usr/local/etc/openldap/

# Set OpenLDAP data and config directories in a data volume
VOLUME ["/var/lib/ldap", "/etc/openldap"]

# Expose default ports for ldap and ldaps
EXPOSE 389 636

CMD ["/usr/local/bin/init.sh"]
