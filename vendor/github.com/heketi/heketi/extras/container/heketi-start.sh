#!/bin/bash
#
# HEKETI_TOPOLOGY_FILE can be passed as an environment variable with the
# filename of the initial topology.json. In case the heketi.db does not exist
# yet, this file will be used to populate the database.

: "${HEKETI_PATH:=/var/lib/heketi}"
: "${BACKUPDB_PATH:=/backupdb}"
LOG="${HEKETI_PATH}/container.log"

info() {
    echo "$*" | tee -a "$LOG"
}

error() {
    echo "error: $*" | tee -a "$LOG" >&2
}

fail() {
    error "$@"
    exit 1
}

info "Setting up heketi database"

# Ensure the data dir exists
mkdir -p "${HEKETI_PATH}" 2>/dev/null
if [[ $? -ne 0 && ! -d "${HEKETI_PATH}" ]]; then
    fail "Failed to create ${HEKETI_PATH}"
fi

# Test that our volume is writable.
touch "${HEKETI_PATH}/test" && rm "${HEKETI_PATH}/test"
if [ $? -ne 0 ]; then
    fail "${HEKETI_PATH} is read-only"
fi

if [[ ! -f "${HEKETI_PATH}/heketi.db" ]]; then
    info "No database file found"
    out=$(mount | grep "${HEKETI_PATH}" | grep heketidbstorage)
    if [[ $? -eq 0 ]]; then
        info "Database volume found: ${out}"
        info "Database file is expected, waiting..."
        check=0
        while [[ ! -f "${HEKETI_PATH}/heketi.db" ]]; do
            sleep 5
            if [[ ${check} -eq 5 ]]; then
               fail "Database file did not appear, exiting."
            fi
            ((check+=1))
        done
    fi
fi

stat "${HEKETI_PATH}/heketi.db" 2>/dev/null | tee -a "${LOG}"
# Workaround for scenario where a lock on the heketi.db has not been
# released.
# This code uses a non-blocking flock in a loop rather than a blocking
# lock with timeout due to issues with current gluster and flock
# ( see rhbz#1613260 )
for _ in $(seq 1 60); do
    flock --nonblock "${HEKETI_PATH}/heketi.db" true
    flock_status=$?
    if [[ $flock_status -eq 0 ]]; then
        break
    fi
    sleep 1
done
if [[ $flock_status -ne 0 ]]; then
    fail "Database file is read-only"
fi

if [[ -d "${BACKUPDB_PATH}" ]]; then
    if [[ -f "${BACKUPDB_PATH}/heketi.db.gz" ]] ; then
        gunzip -c "${BACKUPDB_PATH}/heketi.db.gz" > "${BACKUPDB_PATH}/heketi.db"
        if [[ $? -ne 0 ]]; then
            fail "Unable to extract backup database"
        fi
    fi
    if [[ -f "${BACKUPDB_PATH}/heketi.db" ]] ; then
        cp "${BACKUPDB_PATH}/heketi.db" "${HEKETI_PATH}/heketi.db"
        if [[ $? -ne 0 ]]; then
            fail "Unable to copy backup database"
        fi
        info "Copied backup db to ${HEKETI_PATH}/heketi.db"
    fi
fi

# if the heketi.db does not exist and HEKETI_TOPOLOGY_FILE is set, start the
# heketi service in the background and load the topology. Once done, move the
# heketi service back to the foreground again.
if [[ "$(stat -c %s ${HEKETI_PATH}/heketi.db 2>/dev/null)" == 0 && -n "${HEKETI_TOPOLOGY_FILE}" ]]; then
    # start hketi in the background
    /usr/bin/heketi --config=/etc/heketi/heketi.json &

    # wait until heketi replies
    while ! curl http://localhost:8080/hello; do
        sleep 0.5
    done

    # load the topology
    if [[ -n "${HEKETI_ADMIN_KEY}" ]]; then
        HEKETI_SECRET_ARG="--secret='${HEKETI_ADMIN_KEY}'"
    fi
    heketi-cli --user=admin "${HEKETI_SECRET_ARG}" topology load --json="${HEKETI_TOPOLOGY_FILE}"
    if [[ $? -ne 0 ]]; then
        # something failed, need to exit with an error
        kill %1
        fail "failed to load topology from ${HEKETI_TOPOLOGY_FILE}"
    fi

    # bring heketi back to the foreground
    fg %1
else
    # just start in the foreground
    exec /usr/bin/heketi --config=/etc/heketi/heketi.json
fi
