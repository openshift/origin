#!/bin/bash

# mock passwd and group files
(
  exec 2>/dev/null
  username="${NSS_USERNAME:-$(id -un)}"
  uid="${NSS_UID:-$(id -u)}"

  groupname="${NSS_GROUPNAME:-$(id -gn)}"
  gid="${NSS_GID:-$(id -g)}"

  echo "${username}:x:${uid}:${uid}:gecos:${HOME}:/bin/bash" > "${NSS_WRAPPER_PASSWD}"
  echo "${groupname}:x:${gid}:" > "${NSS_WRAPPER_GROUP}"
)

# wrap command
export LD_PRELOAD=/usr/lib64/libnss_wrapper.so
exec "$@"
