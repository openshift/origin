#!/bin/bash

set -e

if [[ "" == "$@" ]]; then
    echo "usage: start-dns.sh <ip> ...."
    echo "    where <ip> is a space separated list of routers that bind will"
    echo "    use as a round robin for wild card dns forwarding of the v3.rhcloud.com domain"
    exit 1
fi

TEMP_FILE="/tmp/v3.rhcloud.com.zone"

cp /var/named/template.zone ${TEMP_FILE}

echo -n "*" >> ${TEMP_FILE}

for i in $@; do
    echo "  IN A  ${i}" >> ${TEMP_FILE}
done

mv ${TEMP_FILE} /var/named/v3.rhcloud.com.zone

/usr/sbin/named -fg -unamed
