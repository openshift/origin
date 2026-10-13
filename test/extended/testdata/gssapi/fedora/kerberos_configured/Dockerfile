FROM gssapiproxy/fedora-gssapi-kerberos

ENV CLIENT CLIENT_HAS_LIBS_IS_CONFIGURED

RUN sed -i.bak1 -e "s/\[realms\]/\[realms\]\n${REALM} = {\n kdc = ${HOST}\n admin_server = ${HOST}\n default_domain = ${HOST}\n}/" /etc/krb5.conf && \
    sed -i.bak2 -e "s/\[domain_realm\]/\[domain_realm\]\n.${HOST} = ${REALM}\n${HOST} = ${REALM}/" /etc/krb5.conf && \
    sed -i.bak3 -e "s!# default_realm = ! default_realm = ${REALM}\n#!" /etc/krb5.conf
