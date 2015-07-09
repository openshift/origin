#
# This creates an OpenShift-compatible basicauthurl server base image,
# which can be further configured with the addition of a secret.
#
# The standard name for this image is openshift3/basicauthurl

FROM registry.access.redhat.com/library/rhel

RUN yum install -y --enablerepo=rhel-7-server-rpms  tar httpd mod_ssl php   mod_auth_kerb mod_auth_mellon mod_authnz_pam   && yum clean all
# replace the line above to pull in any needed modules, e.g. for LDAP:
#RUN yum install -y --enablerepo=rhel-7-server-rpms --enablerepo=rhel-7-server-optional-rpms   mod_ldap   tar httpd mod_ssl php   && yum clean all
# minimal install for htpasswd auth
#RUN yum install -y --enablerepo=rhel-7-server-rpms  tar httpd mod_ssl php    && yum clean all

ADD ssl.conf /etc/httpd/conf.d/ssl.conf
ADD basicauthurl.php /var/www/html/validate
ADD run-httpd /usr/bin/run-httpd

# Fix up some things so this will run as ordinary user:
RUN mkdir -p /etc/authserver && chmod 777 /etc/authserver
# /run/ is not wiped at start, and httpd by default expects access to things in /run/httpd
RUN chmod 777 /run/httpd
# adjust /etc/httpd/conf/httpd.conf not to change users, run at port 80, or log to files
RUN sed -i -e '/^\s*\(Listen\|User\|Group\|CustomLog\)/ s/^/#/' /etc/httpd/conf/httpd.conf

# really only want to listen at secure port
EXPOSE 8443

# expect essential conf files to land in a secret here:
VOLUME /etc/secret-volume
# cert: TLS certificate file
# key:  TLS key file
# ca:   TLS CA certificate(s) file
# conf: httpd configuration file to include
# conf-dir: optional tgz file including anything else needed, e.g. an htpasswd file.
#           This will be unzipped as /etc/authserver/conf/ when the container runs.

ENTRYPOINT /usr/bin/run-httpd
