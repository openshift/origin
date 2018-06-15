#!/bin/bash -eu

# Copyright 2013-2015 Apcera Inc. All rights reserved.

result="NOT OK FAILED"

# boot2docker doesn't seem to like /tmp so use the home direcotry for the build
BASE_DIR="$(cd .. && pwd)"
export TEST_DIR="$HOME/tmp/$(uuidgen)"
mkdir -p -- "$TEST_DIR"
cp -R "$BASE_DIR" "$TEST_DIR"
DOCKER_DIR="$TEST_DIR/gssapi/test/docker"

if [[ "$OSTYPE" == "darwin"* ]]; then
        DOCKER=docker
else
        DOCKER='sudo docker'
fi

function log() {
        printf "go-gssapi-test: %s\n" "$*" >&2
}

function cleanup_containers() {
        log "Clean up running containers"
        running=`$DOCKER ps --all | grep 'go-gssapi-test' | awk '{print $1}'`
        if [[ "$running" != "" ]]; then
                echo $running | xargs $DOCKER stop >/dev/null
                echo $running | xargs $DOCKER rm >/dev/null
        fi
}

function cleanup() {
        set +e

        if [[ "${EXT_KDC_HOST:-}" == "" ]]; then
                log "kdc logs:

"
                $DOCKER logs kdc 2>&1
        fi

        log "service logs:

"
        if [[ "${SERVICE_LOG_FILTER:-}" != "" ]]; then
                $DOCKER logs service 2>&1 | egrep -v "gssapi-sample:\t[0-9 /:]+ ACCESS "
        else
                $DOCKER logs service 2>&1
        fi

        cleanup_containers

        log "Clean up build directory"
        rm -rf -- "${TEST_DIR:?}"

        log $result
}

function build_image() {
        comp="$1"
        name="$2"
        func="$3"
        img="go-gssapi-test-${name}"
        image="$($DOCKER images --quiet ${img})"

        if [[ "${REUSE_DOCKER_IMAGES:-}" != "" && "$image" != "" ]]; then
                log "Reuse cached docker image ${img} ${image}"
        else
                log "Build docker image ${img}"
                if [[ "$func" != "" ]]; then
                        (${func})
                fi

                $DOCKER build \
                        --rm \
                        --quiet \
                        --tag=${img} \
                        "$DOCKER_DIR/${comp}"
        fi
}

# Caveat: Quote characters in USER_PASSWORD may cause Severe Pain.
#         Don't do that.
#         This only has to handle Docker tests, not quite the Real World,
#         so we can get away with this restriction.
#
function run_image() {
        comp="$1"
        name="$2"
        options="$3"
        img="go-gssapi-test-${name}"
        log "Run docker image ${img}"
        options="${options} \
                --hostname=${comp} \
                --name=${comp} \
                --env SERVICE_NAME=${SERVICE_NAME} \
                --env USER_NAME=${USER_NAME} \
                --env USER_PASSWORD=${USER_PASSWORD} \
                --env REALM_NAME=${REALM_NAME} \
                --env DOMAIN_NAME=${DOMAIN_NAME}"
        $DOCKER run -P ${options} ${img}
}

function map_ports() {
        comp="$1"
        port="$2"
        COMP="$(printf "%s\n" "$comp" | tr '[:lower:]' '[:upper:]')"
        if [[ "${OSTYPE}" == "darwin"* ]]; then
                b2d_ip=$(docker-machine ip default)
                export ${COMP}_PORT_${port}_TCP_ADDR=${b2d_ip}
        else
                export ${COMP}_PORT_${port}_TCP_ADDR=127.0.0.1
        fi
        export ${COMP}_PORT_${port}_TCP_PORT=$($DOCKER port ${comp} ${port} | cut -f2 -d ':')
}

function wait_until_available() {
        comp="$1"
        addr="$2"
        port="$3"

        let i=1
        while ! echo exit | nc $addr $port >/dev/null; do
                echo "Waiting for $comp to start"
                sleep 1
                let i++
                if (( i > 10 )); then
                       echo "Timed out waiting for ${comp} to start at ${addr}:${port}"
                       exit 1
                fi
        done
}

# Cleanup
trap 'cleanup' INT TERM EXIT
cleanup_containers

env_suffix=$(/bin/echo "${REALM_NAME}-${SERVICE_NAME}" | shasum | cut -f1 -d ' ')

# KDC
if [[ "${EXT_KDC_HOST}" == "" ]]; then
        cat "$DOCKER_DIR/kdc/krb5.conf.template" \
                | sed -e "s/KDC_ADDRESS/0.0.0.0:88/g" \
                | sed -e "s/DOMAIN_NAME/${DOMAIN_NAME}/g" \
                | sed -e "s/REALM_NAME/${REALM_NAME}/g" \
                > "$DOCKER_DIR/kdc/krb5.conf"

        build_image "kdc" "kdc-${env_suffix}" "" >/dev/null
        run_image "kdc" "kdc-${env_suffix}" "--detach" >/dev/null
        map_ports "kdc" 88
else
        export KDC_PORT_88_TCP_ADDR=${EXT_KDC_HOST}
        export KDC_PORT_88_TCP_PORT=${EXT_KDC_PORT}
fi
wait_until_available "kdc" $KDC_PORT_88_TCP_ADDR $KDC_PORT_88_TCP_PORT

function keytab_from_kdc() {
        $DOCKER cp kdc:/etc/docker-kdc/krb5.keytab "$DOCKER_DIR/service"
}

function keytab_from_options() {
        cp "${KEYTAB_FILE}" "$DOCKER_DIR/service/krb5.keytab"
}

if [[ "${EXT_KDC_HOST:-}" == "" ]]; then
        DOCKER_KDC_OPTS='--link=kdc:kdc'
        KEYTAB_FUNCTION='keytab_from_kdc'
else
        DOCKER_KDC_OPTS="--env KDC_PORT_88_TCP_ADDR=${EXT_KDC_HOST} \
                --env KDC_PORT_88_TCP_PORT=${EXT_KDC_PORT}"
        KEYTAB_FUNCTION='keytab_from_options'
fi

# GSSAPI service
log "Build and unit-test gssapi on host"
go test github.com/apcera/gssapi

build_image "service" "service-${env_suffix}" "$KEYTAB_FUNCTION" >/dev/null
run_image "service" \
        "service-${env_suffix}" \
        "--detach \
        $DOCKER_KDC_OPTS \
        --volume $TEST_DIR/gssapi:/opt/go/src/github.com/apcera/gssapi" >/dev/null
map_ports "service" 80
wait_until_available "service" $SERVICE_PORT_80_TCP_ADDR $SERVICE_PORT_80_TCP_PORT

# GSSAPI client
if [[ "$OSTYPE" != "darwin"* || "$CLIENT_IN_CONTAINER" != "" ]]; then
        build_image "client" "client" "" >/dev/null
        run_image "client" \
                "client" \
                "--link=service:service \
                $DOCKER_KDC_OPTS \
                --volume $TEST_DIR/gssapi:/opt/go/src/github.com/apcera/gssapi"
else
        log "Run gssapi sample client on host"
        KRB5_CONFIG_TEMPLATE=${DOCKER_DIR}/client/krb5.conf.template \
                DOMAIN_NAME="${DOMAIN_NAME}" \
                GSSAPI_PATH=/opt/local/lib/libgssapi_krb5.dylib \
                KRB5_CONFIG="${TEST_DIR}/krb5.conf" \
                REALM_NAME="${REALM_NAME}" \
                SERVICE_NAME="${SERVICE_NAME}" \
                USER_NAME="${USER_NAME}" \
                USER_PASSWORD="${USER_PASSWORD}" \
                "${DOCKER_DIR}/client/entrypoint.sh"
fi

result="OK TEST PASSED"
