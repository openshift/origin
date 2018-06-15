#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

export PATH="$PWD/_output/local/bin/$(go env GOHOSTOS)/$(go env GOHOSTARCH):$PATH"

function time_now()
{
    date +%s000
}

mkdir -p /tmp/sti
WORK_DIR=$(mktemp -d /tmp/sti/test-work.XXXX)
S2I_WORK_DIR=${WORK_DIR}
if [[ "$OSTYPE" == "cygwin" ]]; then
    S2I_WORK_DIR=$(cygpath -w ${WORK_DIR})
fi
mkdir -p ${WORK_DIR}
NEEDKILL="yes"
S2I_PID=""
function cleanup()
{
    set +e
    #some failures will exit the shell script before check_result() can dump the logs (ssh seems to be such a case)
    if [ -a "${WORK_DIR}/ran-clean" ]; then
        echo "Cleaning up working dir ${WORK_DIR}"
    else
        echo "Dumping logs since did not run successfully before cleanup of ${WORK_DIR} ..."
        cat ${WORK_DIR}/*.log
    fi
    rm -rf ${WORK_DIR}
    # use sigint so that s2i post processing will remove docker container
    if [ -n "${NEEDKILL}" ]; then
        if [ -n "${S2I_PID}" ]; then
            kill -2 "${S2I_PID}"
        fi
    fi
    echo
    echo "Complete"
}

function check_result() {
    local result=$1
    if [ $result -eq 0 ]; then
        echo
        echo "TEST PASSED"
        echo
        if [ -n "${2}" ]; then
            rm $2
        fi
    else
        echo
        echo "TEST FAILED ${result}"
        echo
        cat $2
        cleanup
        exit $result
    fi
}

function test_debug() {
    echo
    echo $1
    echo
}

trap cleanup EXIT SIGINT

echo "working dir:  ${WORK_DIR}"
echo "s2i working dir:  ${S2I_WORK_DIR}"
pushd ${WORK_DIR}

test_debug "cloning source into working dir"

git clone https://github.com/openshift/cakephp-ex &> "${WORK_DIR}/s2i-git-clone.log"
check_result $? "${WORK_DIR}/s2i-git-clone.log"

test_debug "s2i build with relative path without file://"

s2i build cakephp-ex docker.io/centos/php-70-centos7 test --loglevel=5 &> "${WORK_DIR}/s2i-rel-noproto.log"
check_result $? "${WORK_DIR}/s2i-rel-noproto.log"

test_debug "s2i build with volume options"
s2i build cakephp-ex docker.io/centos/php-70-centos7 test --volume "${WORK_DIR}:/home/:z" --loglevel=5 &> "${WORK_DIR}/s2i-volume-correct.log"
check_result $? "${WORK_DIR}/s2i-volume-correct.log"

popd

test_debug "s2i build with absolute path with file://"

if [[ "$OSTYPE" == "cygwin" ]]; then
  S2I_WORK_DIR_URL="file:///${S2I_WORK_DIR//\\//}/cakephp-ex"
else
  S2I_WORK_DIR_URL="file://${S2I_WORK_DIR}/cakephp-ex"
fi

s2i build "${S2I_WORK_DIR_URL}" docker.io/centos/php-70-centos7 test --loglevel=5 &> "${WORK_DIR}/s2i-abs-proto.log"
check_result $? "${WORK_DIR}/s2i-abs-proto.log"

test_debug "s2i build with absolute path without file://"

s2i build "${S2I_WORK_DIR}/cakephp-ex" docker.io/centos/php-70-centos7 test --loglevel=5 &> "${WORK_DIR}/s2i-abs-noproto.log"
check_result $? "${WORK_DIR}/s2i-abs-noproto.log"

## don't do ssh tests here because credentials are needed (even for the git user), which
## don't exist in the vagrant/jenkins setup

test_debug "s2i build with non-git repo file location"

rm -rf "${WORK_DIR}/cakephp-ex/.git"
s2i build "${S2I_WORK_DIR}/cakephp-ex" docker.io/centos/php-70-centos7 test --loglevel=5 --loglevel=5 &> "${WORK_DIR}/s2i-non-repo.log"
check_result $? ""
grep "Copying sources" "${WORK_DIR}/s2i-non-repo.log"
check_result $? "${WORK_DIR}/s2i-non-repo.log"

test_debug "s2i rebuild"
s2i build https://github.com/sclorg/s2i-php-container.git --context-dir=5.5/test/test-app registry.access.redhat.com/openshift3/php-55-rhel7 rack-test-app --incremental=true --loglevel=5 &> "${WORK_DIR}/s2i-pre-rebuild.log"
check_result $? "${WORK_DIR}/s2i-pre-rebuild.log"
s2i rebuild rack-test-app:latest rack-test-app:v1 -p never --loglevel=5 &> "${WORK_DIR}/s2i-rebuild.log"
check_result $? "${WORK_DIR}/s2i-rebuild.log"

test_debug "s2i usage"

s2i usage docker.io/centos/ruby-24-centos7 --loglevel=5 &> "${WORK_DIR}/s2i-usage.log"
check_result $? ""
grep "Sample invocation" "${WORK_DIR}/s2i-usage.log"
check_result $? "${WORK_DIR}/s2i-usage.log"

test_debug "s2i build with overriding assemble/run scripts"
s2i build https://github.com/openshift/source-to-image docker.io/centos/php-70-centos7 test --context-dir=test_apprepo >& "${WORK_DIR}/s2i-override-build.log"
grep "Running custom assemble" "${WORK_DIR}/s2i-override-build.log"
check_result $? "${WORK_DIR}/s2i-override-build.log"
docker run test >& "${WORK_DIR}/s2i-override-run.log"
grep "Running custom run" "${WORK_DIR}/s2i-override-run.log"
check_result $? "${WORK_DIR}/s2i-override-run.log"

test_debug "s2i build with remote git repo"
s2i build https://github.com/openshift/cakephp-ex docker.io/centos/php-70-centos7 test --loglevel=5 &> "${WORK_DIR}/s2i-git-proto.log"
check_result $? "${WORK_DIR}/s2i-git-proto.log"

test_debug "s2i build with runtime image"
s2i build --ref=10.x --context-dir=helloworld https://github.com/wildfly/quickstart docker.io/openshift/wildfly-101-centos7 test-jee-app-thin --runtime-image=docker.io/openshift/wildfly-101-centos7 &> "${WORK_DIR}/s2i-runtime-image.log"
check_result $? "${WORK_DIR}/s2i-runtime-image.log"

test_debug "s2i build with --run==true option"
if [[ "$OSTYPE" == "cygwin" ]]; then
  ( cd hack/windows/sigintwrap && make )
  hack/windows/sigintwrap/sigintwrap 's2i build --ref=10.x --context-dir=helloworld https://github.com/wildfly/quickstart openshift/wildfly-101-centos7 test-jee-app --run=true --loglevel=5' &> "${WORK_DIR}/s2i-run.log" &
else
  s2i build --ref=10.x --context-dir=helloworld https://github.com/wildfly/quickstart docker.io/openshift/wildfly-101-centos7 test-jee-app --run=true --loglevel=5 &> "${WORK_DIR}/s2i-run.log" &
fi
S2I_PID=$!
TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))
max_wait=15*TIME_MIN
echo "Waiting up to ${max_wait} for the build to finish ..."
expire=$(($(time_now) + $max_wait))

set +e
while [[ $(time_now) -lt $expire ]]; do
    grep  "as a result of the --run=true option" "${WORK_DIR}/s2i-run.log"
    if [ $? -eq 0 ]; then
        echo "[INFO] Success running command s2i --run=true"

        # use sigint so that s2i post processing will remove docker container
        kill -2 "${S2I_PID}"
        NEEDKILL=""
        sleep 30
        docker ps -a | grep test-jee-app

        if [ $? -eq 1 ]; then
            echo "[INFO] Success terminating associated docker container"
            touch "${WORK_DIR}/ran-clean"
            exit 0
        else
            echo "[INFO] Associated docker container still found, review docker ps -a output above, and here is the dump of ${WORK_DIR}/s2i-run.log"
            cat "${WORK_DIR}/s2i-run.log"
            exit 1
        fi
    fi
    sleep 1
done

echo "[INFO] Problem with s2i --run=true, dumping ${WORK_DIR}/s2i-run.log"
cat "${WORK_DIR}/s2i-run.log"
set -e
exit 1
