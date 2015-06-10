#!/usr/bin/env bash

# patch incompatible with fail-over DNS setup
SCRIPT='/etc/NetworkManager/dispatcher.d/fix-slow-dns'
if [[ -f "${SCRIPT}" ]]; then
    echo "Removing ${SCRIPT}..."
    rm "${SCRIPT}"
    sed -i -e '/^options.*$/d' /etc/resolv.conf
fi
unset SCRIPT
