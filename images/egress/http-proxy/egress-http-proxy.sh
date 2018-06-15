#!/bin/bash

# OpenShift egress HTTP proxy setup script

set -o errexit
set -o nounset
set -o pipefail

function die() {
    echo "$*" 1>&2
    exit 1
}

if [[ -z "${EGRESS_HTTP_PROXY_DESTINATION}" ]]; then
    die "No EGRESS_HTTP_PROXY_DESTINATION specified"
fi

IPADDR_REGEX="[[:xdigit:].:]*[.:][[:xdigit:].:]+"
OPT_CIDR_MASK_REGEX="(/[[:digit:]]+)?"
HOSTNAME_REGEX="[[:alnum:]][[:alnum:].-]+"
DOMAIN_REGEX="\*\.${HOSTNAME_REGEX}"

function generate_acls() {
    n=0
    saw_wildcard=
    while read dest; do
	if [[ "${dest}" =~ ^\w*$ || "${dest}" =~ ^# ]]; then
	    # comment or blank line
	    continue
	fi
	n=$(($n + 1))

	if [[ "${dest}" == "*" ]]; then
	    saw_wildcard=1
	    continue
	elif [[ -n "${saw_wildcard}" ]]; then
	    die "Wildcard must be last rule, if present"
	fi

	if [[ "${dest}" =~ ^! ]]; then
	    rule=deny
	    dest="${dest#!}"
	else
	    rule=allow
	fi

	echo ""
	if [[ "${dest}" =~ ^${IPADDR_REGEX}${OPT_CIDR_MASK_REGEX}$ ]]; then
	    echo acl dest$n dst "${dest}"
	    echo http_access "${rule}" dest$n
	elif [[ "${dest}" =~ ^${DOMAIN_REGEX}$ ]]; then
	    echo acl dest$n dstdomain "${dest#\*}"
	    echo http_access "${rule}" dest$n
	elif [[ "${dest}" =~ ^${HOSTNAME_REGEX}$ ]]; then
	    echo acl dest$n dstdomain "${dest}"
	    echo http_access "${rule}" dest$n
	else
	    die "Bad destination '${dest}'"
	fi
    done <<< "${EGRESS_HTTP_PROXY_DESTINATION}"

    echo ""
    if [[ -n "${saw_wildcard}" ]]; then
	echo "http_access allow all"
    else
	echo "http_access deny all"
    fi
}

if [[ "${EGRESS_HTTP_PROXY_MODE:-}" == "unit-test" ]]; then
    generate_acls
    exit 0
fi

CONF=/etc/squid/squid.conf
rm -f ${CONF}

cat > ${CONF} <<EOF
http_port 8080
cache deny all
access_log none all
debug_options ALL,0
shutdown_lifetime 0
EOF

generate_acls >> ${CONF}

echo "Running squid with config:"
sed -e 's/^/  /' ${CONF}
echo ""
echo ""

exec squid -N
