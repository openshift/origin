#!/bin/bash
#
# HEKETI_TOPOLOGY_FILE can be passed as an environment variable with the
# filename of the initial topology.json. In case the heketi.db does not exist
# yet, this file will be used to populate the database.

: "${HEKETI_PATH:=/var/lib/heketi}"
: "${BACKUPDB_PATH:=/backupdb}"
: "${TMP_PATH:=/tmp}"
: "${HEKETI_DB_ARCHIVE_PATH:=${HEKETI_PATH}/archive}"
HEKETI_BIN="/usr/bin/heketi"
LOG="${HEKETI_PATH}/container.log"

# allow disabling the archive path
case "${HEKETI_DB_ARCHIVE_PATH}" in
    -|/dev/null) HEKETI_DB_ARCHIVE_PATH="" ;;
esac

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

archive_db() {
    # create an archive file for the previous copy of the db
    # before the server starts up
    mkdir -p "${HEKETI_DB_ARCHIVE_PATH}"

    # only archive the db if the db content is valid
    if ! "$HEKETI_BIN" db export --dbfile "${HEKETI_PATH}/heketi.db" --jsonfile - >/dev/null ; then
        error "Unable to export db. DB contents may not be valid"
        return 1
    fi

    mapfile -t afiles < <(find "${HEKETI_DB_ARCHIVE_PATH}" \
        -maxdepth 1 -name 'heketi.db-archive-*.gz' | sort)
    newname="heketi.db-archive-$(date +%Y-%m-%d.%s).gz"
    newpath="${HEKETI_DB_ARCHIVE_PATH}/${newname}"
    gzip -9 -c "${HEKETI_PATH}/heketi.db" > "${newpath}"
    if [[ $? -ne 0 ]]; then
        rm -f "${newpath}"
        error "Unable to create archive"
        return 1
    fi
    sha256sum "${newpath}" > "${newpath}.sha256"
    if [[ $? -ne 0 ]]; then
        rm -f "${newpath}" "${newpath}.sha256"
        error "Unable to get sha256 sum of archive"
        return 1
    fi
    # if the new file is a dupe, immediately prune it
    if [ "${#afiles[@]}" -gt 0 ]; then
        sum="$(awk '{print $1}' "${newpath}.sha256")"
        prevpath="${afiles[-1]}"
        if grep -q "$sum" "${prevpath}.sha256" >/dev/null; then
            # found same hash as previous db, remove new copy
            rm -f "${newpath}" "${newpath}.sha256"
        fi
    fi
    return 0
}

prune_archives() {
    # prune old archive copies
    HEKETI_DB_ARCHIVE_COUNT="${HEKETI_DB_ARCHIVE_COUNT:-5}"
    mapfile -t afiles < <(find "${HEKETI_DB_ARCHIVE_PATH}" \
        -maxdepth 1 -name 'heketi.db-archive-*.gz' | sort)
    if [[ "${#afiles[@]}" -gt ${HEKETI_DB_ARCHIVE_COUNT} ]]; then
        curr=0
        count=$((${#afiles[@]}-HEKETI_DB_ARCHIVE_COUNT))
        while [[ $curr -lt $count ]]; do
            rm -f "${afiles[$curr]}"*
            curr=$((curr+1))
        done
    fi
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

# this creates archival copies of the db if needed for disaster
# recovery purposes
if [[ "${HEKETI_DB_ARCHIVE_PATH}" && -f "${HEKETI_PATH}/heketi.db" ]]; then
    archive_db && prune_archives
fi

# this is used to restore secret based backups
if [[ -d "${BACKUPDB_PATH}" ]]; then
    if [[ -f "${BACKUPDB_PATH}/heketi.db.gz" ]] ; then
        gunzip -c "${BACKUPDB_PATH}/heketi.db.gz" > "${TMP_PATH}/heketi.db"
        if [[ $? -ne 0 ]]; then
            fail "Unable to extract backup database"
        fi
        cp "${TMP_PATH}/heketi.db" "${HEKETI_PATH}/heketi.db"
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
    "$HEKETI_BIN" --config=/etc/heketi/heketi.json &

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
    exec "$HEKETI_BIN" --config=/etc/heketi/heketi.json
fi
