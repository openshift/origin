#!/usr/bin/env bash

# https://github.com/kubernetes/kubernetes/blob/84beab6f26609527752ff441c155813b783fe145/cluster/gce/gci/configure-helper.sh#L44
# secure_random generates a secure random string of bytes. This function accepts
# a number of secure bytes desired and returns a base64 encoded string with at
# least the requested entropy. Rather than directly reading from /dev/urandom,
# we use uuidgen which calls getrandom(2). getrandom(2) verifies that the
# entropy pool has been initialized sufficiently for the desired operation
# before reading from /dev/urandom.
#
# ARGS:
#   #1: number of secure bytes to generate. We round up to the nearest factor of 32.
function secure_random {
  local infobytes="${1}"
  if ((infobytes <= 0)); then
    echo "Invalid argument to secure_random: infobytes='${infobytes}'" 1>&2
    return 1
  fi

  local out=""
  for (( i = 0; i < "${infobytes}"; i += 32 )); do
    # uuids have 122 random bits, sha256 sums have 256 bits, so concatenate
    # three uuids and take their sum. The sum is encoded in ASCII hex, hence the
    # 64 character cut.
    out+="$(
     (
       uuidgen --random;
       uuidgen --random;
       uuidgen --random;
     ) | sha256sum \
       | head -c 64
    )";
  done
  # Finally, convert the ASCII hex to base64 to increase the density.
  echo -n "${out}" | xxd -r -p | base64 -w 0
}

secure_random $1