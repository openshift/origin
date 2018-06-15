# Clone from the Fedora 23 image
FROM fedora:23

# Install GSSAPI
RUN dnf install -y \
  apr-util-openssl \
  authconfig \
  httpd \
  krb5-libs \
  krb5-server \
  krb5-workstation \
  mod_auth_gssapi \
  mod_ssl \
  && dnf clean all

# Add conf files for Kerberos
ADD krb5.conf /etc/krb5.conf
ADD kdc.conf  /var/kerberos/krb5kdc/kdc.conf
ADD kadm5.acl /var/kerberos/krb5kdc/kadm5.acl

# Add conf file for Apache
ADD proxy.conf /etc/httpd/conf.d/proxy.conf

# Add health check file
ADD healthz /var/www/html/healthz

# 80  = http
# 443 = https
# 88  = kerberos
EXPOSE 80 443 88 88/udp

ADD configure.sh /usr/sbin/configure.sh
ENTRYPOINT /usr/sbin/configure.sh
