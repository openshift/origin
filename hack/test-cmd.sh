#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

os::log::install_errexit

function cleanup()
{
    out=$?
    pkill -P $$

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
        echo
        cat "${TEMP_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    else
        if path=$(go tool -n pprof 2>&1); then
          echo
          echo "pprof: top output"
          echo
          set +e
          go tool pprof -text ./_output/local/go/bin/openshift cpu.pprof
        fi

        echo
        echo "Complete"
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

# Prevent user environment from colliding with the test setup
unset OPENSHIFTCONFIG

USE_LOCAL_IMAGES=${USE_LOCAL_IMAGES:-true}

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_SCHEME=${API_SCHEME:-https}
API_PORT=${API_PORT:-8443}
API_HOST=${API_HOST:-127.0.0.1}
MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
KUBELET_SCHEME=${KUBELET_SCHEME:-https}
KUBELET_HOST=${KUBELET_HOST:-127.0.0.1}
KUBELET_PORT=${KUBELET_PORT:-10250}

TEMP_DIR=${USE_TEMP:-$(mktemp -d /tmp/openshift-cmd.XXXX)}
ETCD_DATA_DIR="${TEMP_DIR}/etcd"
VOLUME_DIR="${TEMP_DIR}/volumes"
FAKE_HOME_DIR="${TEMP_DIR}/openshift.local.home"
SERVER_CONFIG_DIR="${TEMP_DIR}/openshift.local.config"
MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"
CONFIG_DIR="${TEMP_DIR}/configs"
mkdir -p "${ETCD_DATA_DIR}" "${VOLUME_DIR}" "${FAKE_HOME_DIR}" "${MASTER_CONFIG_DIR}" "${NODE_CONFIG_DIR}" "${CONFIG_DIR}"

# handle profiling defaults
profile="${OPENSHIFT_PROFILE-}"
unset OPENSHIFT_PROFILE
if [[ -n "${profile}" ]]; then
    if [[ "${TEST_PROFILE-}" == "cli" ]]; then
        export CLI_PROFILE="${profile}"
    else
        export WEB_PROFILE="${profile}"
    fi
else
  export WEB_PROFILE=cpu
fi

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

# Check openshift version
out=$(openshift version)
echo openshift: $out

# profile the web
export OPENSHIFT_PROFILE="${WEB_PROFILE-}"

# Specify the scheme and port for the listen address, but let the IP auto-discover. Set --public-master to localhost, for a stable link to the console.
echo "[INFO] Create certificates for the OpenShift server to ${MASTER_CONFIG_DIR}"
# find the same IP that openshift start will bind to.  This allows access from pods that have to talk back to master
ALL_IP_ADDRESSES=`ifconfig | grep "inet " | sed 's/adr://' | awk '{print $2}'`
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost"
while read -r IP_ADDRESS
do
    SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
done <<< "${ALL_IP_ADDRESSES}"

openshift admin create-master-certs \
  --overwrite=false \
  --cert-dir="${MASTER_CONFIG_DIR}" \
  --hostnames="${SERVER_HOSTNAME_LIST}" \
  --master="${MASTER_ADDR}" \
  --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

openshift admin create-node-config \
  --listen="${KUBELET_SCHEME}://0.0.0.0:${KUBELET_PORT}" \
  --node-dir="${NODE_CONFIG_DIR}" \
  --node="${KUBELET_HOST}" \
  --hostnames="${KUBELET_HOST}" \
  --master="${MASTER_ADDR}" \
  --node-client-certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
  --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
  --signer-cert="${MASTER_CONFIG_DIR}/ca.crt" \
  --signer-key="${MASTER_CONFIG_DIR}/ca.key" \
  --signer-serial="${MASTER_CONFIG_DIR}/ca.serial.txt"

osadm create-bootstrap-policy-file --filename="${MASTER_CONFIG_DIR}/policy.json"

# create openshift config
openshift start \
  --write-config=${SERVER_CONFIG_DIR} \
  --create-certs=false \
  --master="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --hostname="${KUBELET_HOST}" \
  --volume-dir="${VOLUME_DIR}" \
  --etcd-dir="${ETCD_DATA_DIR}"


# Start openshift
OPENSHIFT_ON_PANIC=crash openshift start \
  --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
  --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
  --loglevel=4 \
  1>&2 2>"${TEMP_DIR}/openshift.log" &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
    export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
    export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
    export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"
fi

# set the home directory so we don't pick up the users .config
export HOME="${FAKE_HOME_DIR}"

wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "kubelet: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta1/minions/${KUBELET_HOST}" "apiserver(minions): " 0.25 80

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

# test client not configured
[ "$(osc get services 2>&1 | grep 'Error in configuration')" ]

# Set KUBERNETES_MASTER for osc from now on
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Set certificates for osc from now on
if [[ "${API_SCHEME}" == "https" ]]; then
    # test bad certificate
    [ "$(osc get services 2>&1 | grep 'certificate signed by unknown authority')" ]
fi

# login and logout tests
# --token and --username are mutually exclusive
[ "$(osc login ${KUBERNETES_MASTER} -u test-user --token=tmp --insecure-skip-tls-verify 2>&1 | grep 'mutually exclusive')" ]
# must only accept one arg (server)
[ "$(osc login https://server1 https://server2.com 2>&1 | grep 'Only the server URL may be specified')" ]
# logs in with a valid certificate authority
osc login ${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything --api-version=v1beta3
grep -q "v1beta3" ${HOME}/.config/openshift/config
osc logout
# logs in skipping certificate check
osc login ${KUBERNETES_MASTER} --insecure-skip-tls-verify -u test-user -p anything
# logs in by an existing and valid token
temp_token=$(osc config view -o template --template='{{range .users}}{{ index .user.token }}{{end}}')
[ "$(osc login --token=${temp_token} 2>&1 | grep 'using the token provided')" ]
osc logout
# properly parse server port
[ "$(osc login https://server1:844333 2>&1 | grep 'Not a valid port')" ]
# properly handle trailing slash
osc login --server=${KUBERNETES_MASTER}/ --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
# create a new project
osc new-project project-foo --display-name="my project" --description="boring project description"
[ "$(osc project | grep 'Using project "project-foo"')" ]
# denies access after logging out
osc logout
[ -z "$(osc get pods | grep 'system:anonymous')" ]

# log in and set project to use from now on
osc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
osc get projects
osc project project-foo

# test config files from the --config flag
osc get services --config="${MASTER_CONFIG_DIR}/admin.kubeconfig"

# test config files from env vars
OPENSHIFTCONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig" osc get services

# test config files in the home directory
mkdir -p ${HOME}/.config/openshift
cp ${MASTER_CONFIG_DIR}/admin.kubeconfig ${HOME}/.config/openshift/config
osc get services
mv ${HOME}/.config/openshift/config ${HOME}/.config/openshift/non-default-config
echo "config files: ok"
export OPENSHIFTCONFIG="${HOME}/.config/openshift/non-default-config"

# from this point every command will use config from the OPENSHIFTCONFIG env var

osc get templates
osc create -f examples/sample-app/application-template-dockerbuild.json
osc get templates
osc get templates ruby-helloworld-sample
osc process ruby-helloworld-sample
osc describe templates ruby-helloworld-sample
[ "$(osc describe templates ruby-helloworld-sample | grep -E "BuildConfig.*ruby-sample-build")" ]
osc delete templates ruby-helloworld-sample
osc get templates
# TODO: create directly from template
echo "templates: ok"

# verify some default commands
[ "$(openshift 2>&1)" ]
[ "$(openshift cli)" ]
[ "$(openshift ex)" ]
[ "$(openshift admin config 2>&1)" ]
[ "$(openshift cli config 2>&1)" ]
[ "$(openshift ex tokens)" ]
[ "$(openshift admin policy  2>&1)" ]
[ "$(openshift kubectl 2>&1)" ]
[ "$(openshift kube 2>&1)" ]
[ "$(openshift admin 2>&1)" ]
[ "$(openshift start kubernetes 2>&1)" ]
[ "$(kubernetes 2>&1)" ]
[ "$(kubectl 2>&1)" ]
[ "$(osc 2>&1)" ]
[ "$(os 2>&1)" ]
[ "$(osadm 2>&1)" ]
[ "$(oadm 2>&1)" ]
[ "$(origin 2>&1)" ]

# help for root commands must be consistent
[ "$(openshift | grep 'OpenShift Application Platform')" ]
[ "$(osc | grep 'OpenShift Client')" ]
[ "! $(osc | grep 'Options')" ]
[ "! $(osc | grep 'Global Options')" ]
[ "$(openshift cli | grep 'OpenShift Client')" ]
[ "$(openshift kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(osadm 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift admin 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift start kubernetes 2>&1 | grep 'Kubernetes server components')" ]

# help for root commands with --help flag must be consistent
[ "$(openshift --help 2>&1 | grep 'OpenShift Application Platform')" ]
[ "$(osc --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(osc login --help 2>&1 | grep 'Options')" ]
[ "! $(osc login --help 2>&1 | grep 'Global Options')" ]
[ "$(openshift cli --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift kubectl --help 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(osadm --help 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift admin --help 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for root commands through help command must be consistent
[ "$(openshift help cli 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift help kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(openshift help admin 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for given command with --help flag must be consistent
[ "$(osc get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift start --help 2>&1 | grep 'Start an OpenShift all-in-one server')" ]
[ "$(openshift start master --help 2>&1 | grep 'Start an OpenShift master')" ]
[ "$(openshift start node --help 2>&1 | grep 'Start an OpenShift node')" ]
[ "$(osc get --help 2>&1 | grep 'osc')" ]

# help for given command through help command must be consistent
[ "$(osc help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help start 2>&1 | grep 'Start an OpenShift all-in-one server')" ]
[ "$(openshift help start master 2>&1 | grep 'Start an OpenShift master')" ]
[ "$(openshift help start node 2>&1 | grep 'Start an OpenShift node')" ]
[ "$(openshift cli help update 2>&1 | grep 'openshift')" ]

# runnable commands with required flags must error consistently
[ "$(osc get 2>&1 | grep 'you must provide one or more resources')" ]
[ "$(openshift cli get 2>&1 | grep 'you must provide one or more resources')" ]
[ "$(openshift kubectl get 2>&1 | grep 'you must provide one or more resources')" ]

# commands that expect file paths must validate and error out correctly
[ "$(osc login --certificate-authority=/path/to/invalid 2>&1 | grep 'no such file or directory')" ]

osc get pods --match-server-version
osc create -f examples/hello-openshift/hello-pod.json
osc describe pod hello-openshift
osc delete pods hello-openshift
echo "pods: ok"

osc get services
osc create -f test/integration/fixtures/test-service.json
osc delete services frontend
echo "services: ok"

osc get nodes
echo "nodes: ok"

osc get images
osc create -f test/integration/fixtures/test-image.json
osc delete images test
echo "images: ok"

osc get imageStreams
osc create -f test/integration/fixtures/test-image-stream.json
# make sure stream.status.dockerImageRepository isn't set (no registry)
[ -z "$(osc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
# create the registry
osadm registry --create --credentials="${OPENSHIFTCONFIG}"
# make sure stream.status.dockerImageRepository IS set
[ -n "$(osc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
# ensure the registry rc has been created
wait_for_command 'osc get rc docker-registry-1' "${TIME_MIN}"
# delete the registry resources
osc delete dc docker-registry
osc delete svc docker-registry
[ ! "$(osc get rc docker-registry-1)" ]
# done deleting registry resources
osc delete imageStreams test
[ -z "$(osc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
osc create -f examples/image-streams/image-streams-centos7.json
[ -n "$(osc get imageStreams ruby -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams nodejs -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams wildfly -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams mysql -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams postgresql -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(osc get imageStreams mongodb -t "{{.status.dockerImageRepository}}")" ]
# verify the image repository had its tags populated
[ -n "$(osc get imageStreams wildfly -t "{{.status.tags.latest}}")" ]
[ -n "$(osc get imageStreams wildfly -t "{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}")" ]
osc delete imageStreams ruby
osc delete imageStreams nodejs
osc delete imageStreams wildfly
#osc delete imageStreams mysql
osc delete imageStreams postgresql
osc delete imageStreams mongodb
[ -z "$(osc get imageStreams ruby -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams nodejs -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams postgresql -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams mongodb -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(osc get imageStreams wildfly -t "{{.status.dockerImageRepository}}")" ]
wait_for_command 'osc get imagestreamTags mysql:latest' "${TIME_MIN}"
[ -n "$(osc get imagestreams mysql -t '{{ index .metadata.annotations "openshift.io/image.dockerRepositoryCheck"}}')" ]
osc describe istag/mysql:latest
[ "$(osc describe istag/mysql:latest | grep "Environment:")" ]
[ "$(osc describe istag/mysql:latest | grep "Image Created:")" ]
[ "$(osc describe istag/mysql:latest | grep "Image Name:")" ]
name=$(osc get istag/mysql:latest -t '{{ .imageName }}')
imagename="isimage/mysql@${name:0:7}"
osc describe ${imagename}
[ "$(osc describe ${imagename} | grep "Environment:")" ]
[ "$(osc describe ${imagename} | grep "Image Created:")" ]
[ "$(osc describe ${imagename} | grep "Image Name:")" ]
echo "imageStreams: ok"

[ "$(osc new-app library/php mysql -o yaml | grep 3306)" ]
[ ! "$(osc new-app unknownhubimage -o yaml)" ]
# verify we can generate a Docker image based component "mongodb" directly
[ "$(osc new-app mongo -o yaml | grep library/mongo)" ]
# the local image repository takes precedence over the Docker Hub "mysql" image
[ "$(osc new-app mysql -o yaml | grep mysql-55-centos7)" ]
osc delete all --all
osc new-app library/php mysql -l no-source=php-mysql
osc delete all -l no-source=php-mysql
# check if we can create from a stored template
osc create -f examples/sample-app/application-template-stibuild.json
osc get template ruby-helloworld-sample
[ "$(osc new-app ruby-helloworld-sample -o yaml | grep MYSQL_USER)" ]
[ "$(osc new-app ruby-helloworld-sample -o yaml | grep MYSQL_PASSWORD)" ]
[ "$(osc new-app ruby-helloworld-sample -o yaml | grep ADMIN_USERNAME)" ]
[ "$(osc new-app ruby-helloworld-sample -o yaml | grep ADMIN_PASSWORD)" ]
# check that we can create from the template without errors
osc new-app ruby-helloworld-sample -l app=helloworld
osc delete all -l app=helloworld
# create from template with code explicitly set is not supported
[ ! "$(osc new-app ruby-helloworld-sample~git@github.com/mfojtik/sinatra-app-example)" ]
osc delete template ruby-helloworld-sample
# override component names
[ "$(osc new-app mysql --name=db | grep db)" ]
osc new-app https://github.com/openshift/ruby-hello-world -l app=ruby
osc delete all -l app=ruby
echo "new-app: ok"

osc get routes
osc create -f test/integration/fixtures/test-route.json
osc delete routes testroute
echo "routes: ok"

osc get deploymentConfigs
osc get dc
osc create -f test/integration/fixtures/test-deployment-config.json
osc describe deploymentConfigs test-deployment-config
[ "$(osc env dc/test-deployment-config --list | grep TEST=value)" ]
[ ! "$(osc env dc/test-deployment-config TEST- --list | grep TEST=value)" ]
[ "$(osc env dc/test-deployment-config TEST=foo --list | grep TEST=foo)" ]
[ "$(osc env dc/test-deployment-config OTHER=foo --list | grep TEST=value)" ]
[ ! "$(osc env dc/test-deployment-config OTHER=foo -c 'ruby' --list | grep OTHER=foo)" ]
[ "$(osc env dc/test-deployment-config OTHER=foo -c 'ruby*'   --list | grep OTHER=foo)" ]
[ "$(osc env dc/test-deployment-config OTHER=foo -c '*hello*' --list | grep OTHER=foo)" ]
[ "$(osc env dc/test-deployment-config OTHER=foo -c '*world'  --list | grep OTHER=foo)" ]
[ "$(osc env dc/test-deployment-config OTHER=foo --list | grep OTHER=foo)" ]
[ "$(osc env dc/test-deployment-config OTHER=foo -o yaml | grep "name: OTHER")" ]
[ "$(echo "OTHER=foo" | osc env dc/test-deployment-config -e - --list | grep OTHER=foo)" ]
[ ! "$(echo "#OTHER=foo" | osc env dc/test-deployment-config -e - --list | grep OTHER=foo)" ]
[ "$(osc env dc/test-deployment-config TEST=bar OTHER=baz BAR-)" ]
osc deploy test-deployment-config
osc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

osc process -f test/templates/fixtures/guestbook.json --parameters --value="ADMIN_USERNAME=admin"
osc process -f test/templates/fixtures/guestbook.json -l app=guestbook | osc create -f -
osc status
[ "$(osc status | grep frontend-service)" ]
echo "template+config: ok"
[ "$(OSC_EDITOR='cat' osc edit svc/kubernetes 2>&1 | grep 'Edit cancelled')" ]
[ "$(OSC_EDITOR='cat' osc edit svc/kubernetes | grep 'provider: kubernetes')" ]
osc delete all -l app=guestbook
echo "edit: ok"

osc delete all --all
osc new-app https://github.com/openshift/ruby-hello-world -l app=ruby
wait_for_command 'osc get rc/ruby-hello-world-1' "${TIME_MIN}"
# resize rc via deployment configuration
osc resize dc ruby-hello-world --replicas=1
# resize directly
osc resize rc ruby-hello-world-1 --current-replicas=1 --replicas=5
[ "$(osc get rc/ruby-hello-world-1 | grep 5)" ]
osc delete all -l app=ruby
echo "resize: ok"

osc process -f examples/sample-app/application-template-dockerbuild.json -l app=dockerbuild | osc create -f -
wait_for_command 'osc get rc/database-1' "${TIME_MIN}"
osc get dc/database
osc stop dc/database
[ ! "$(osc get rc/database-1)" ]
[ ! "$(osc get dc/database)" ]
echo "stop: ok"
osc label bc ruby-sample-build acustom=label
[ "$(osc describe bc/ruby-sample-build | grep 'acustom=label')" ]
osc delete all -l app=dockerbuild
echo "label: ok"

osc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | osc create -f -
osc get buildConfigs
osc get bc
osc get builds

[[ $(osc describe buildConfigs ruby-sample-build --api-version=v1beta1 | grep --text "Webhook Github"  | grep -F "${API_SCHEME}://${API_HOST}:${API_PORT}/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/github") ]]
[[ $(osc describe buildConfigs ruby-sample-build --api-version=v1beta1 | grep --text "Webhook Generic" | grep -F "${API_SCHEME}://${API_HOST}:${API_PORT}/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic") ]]
osc start-build --list-webhooks='all' ruby-sample-build
[[ $(osc start-build --list-webhooks='all' ruby-sample-build | grep --text "generic") ]]
[[ $(osc start-build --list-webhooks='all' ruby-sample-build | grep --text "github") ]]
[[ $(osc start-build --list-webhooks='github' ruby-sample-build | grep --text "secret101") ]]
[ ! "$(osc start-build --list-webhooks='blah')" ]
webhook=$(osc start-build --list-webhooks='generic' ruby-sample-build --api-version=v1beta1 | head -n 1)
osc start-build --from-webhook="${webhook}"
webhook=$(osc start-build --list-webhooks='generic' ruby-sample-build --api-version=v1beta3 | head -n 1)
osc start-build --from-webhook="${webhook}"
osc get builds
osc delete all -l build=docker
echo "buildConfig: ok"

osc create -f test/integration/fixtures/test-buildcli.json
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
started=$(osc start-build ruby-sample-build-invalidtag)
osc describe build ${started} | grep openshift/ruby-20-centos7$
echo "start-build: ok"

osc cancel-build "${started}" --dump-logs --restart
echo "cancel-build: ok"
osc delete is/ruby-20-centos7-buildcli
osc delete bc/ruby-sample-build-validtag
osc delete bc/ruby-sample-build-invalidtag

# Test admin manage-node operations
[ "$(openshift admin manage-node --help 2>&1 | grep 'Manage node operations')" ]
[ "$(osadm manage-node --selector='' --schedulable=true | grep --text 'Ready' | grep -v 'Sched')" ]
osc create -f examples/hello-openshift/hello-pod.json
#[ "$(osadm manage-node --list-pods | grep 'hello-openshift' | grep -E '(unassigned|assigned)')" ]
#[ "$(osadm manage-node --evacuate --dry-run | grep 'hello-openshift')" ]
#[ "$(osadm manage-node --schedulable=false | grep 'SchedulingDisabled')" ]
#[ "$(osadm manage-node --evacuate 2>&1 | grep 'Unable to evacuate')" ]
#[ "$(osadm manage-node --evacuate --force | grep 'hello-openshift')" ]
#[ ! "$(osadm manage-node --list-pods | grep 'hello-openshift')" ]
osc delete pods hello-openshift
echo "manage-node: ok"

openshift admin policy add-role-to-group cluster-admin system:unauthenticated
openshift admin policy add-role-to-user cluster-admin system:no-user
openshift admin policy remove-role-from-group cluster-admin system:unauthenticated
openshift admin policy remove-role-from-user cluster-admin system:no-user
openshift admin policy remove-group system:unauthenticated
openshift admin policy remove-user system:no-user
openshift admin policy add-cluster-role-to-group cluster-admin system:unauthenticated
openshift admin policy remove-cluster-role-from-group cluster-admin system:unauthenticated
openshift admin policy add-cluster-role-to-user cluster-admin system:no-user
openshift admin policy remove-cluster-role-from-user cluster-admin system:no-user
echo "policy: ok"

# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
osadm new-project ui-test-project --admin="createuser"
osadm policy add-role-to-user admin adduser -n ui-test-project
# Make sure project can be listed by osc (after auth cache syncs)
sleep 2 && [ "$(osc get projects | grep 'ui-test-project')" ]
# Make sure users got added
[ "$(osc describe policybinding ':default' -n ui-test-project | grep createuser)" ]
[ "$(osc describe policybinding ':default' -n ui-test-project | grep adduser)" ]
echo "ui-project-commands: ok"

# Expose service as a route
osc delete svc/frontend
osc create -f test/integration/fixtures/test-service.json
osc expose service frontend
[ "$(osc get route frontend | grep 'name=frontend')" ]
osc delete svc,route -l name=frontend
echo "expose: ok"

# Test deleting and recreating a project
osadm new-project recreated-project --admin="createuser1"
osc delete project recreated-project
wait_for_command '! osc get project recreated-project' "${TIME_MIN}"
osadm new-project recreated-project --admin="createuser2"
osc describe policybinding ':default' -n recreated-project | grep createuser2
echo "new-project: ok"

# Test running a router
[ ! "$(osadm router --dry-run | grep 'does not exist')" ]
[ "$(osadm router -o yaml --credentials="${OPENSHIFTCONFIG}" | grep 'openshift/origin-haproxy-')" ]
osadm router --create --credentials="${OPENSHIFTCONFIG}"
[ "$(osadm router | grep 'service exists')" ]
echo "ex router: ok"

# Test running a registry
[ ! "$(osadm registry --dry-run | grep 'does not exist')"]
[ "$(osadm registry -o yaml --credentials="${OPENSHIFTCONFIG}" | grep 'openshift/origin-docker-registry')" ]
osadm registry --create --credentials="${OPENSHIFTCONFIG}"
[ "$(osadm registry | grep 'service exists')" ]
echo "ex registry: ok"

# Test building a dependency tree
osc process -f examples/sample-app/application-template-stibuild.json -l build=sti | osc create -f -
[ "$(openshift ex build-chain --all -o dot | grep 'graph')" ]
osc delete all -l build=sti
echo "ex build-chain: ok"

osadm new-project example --admin="createuser"
osc project example
wait_for_command 'osc get serviceaccount default' "${TIME_MIN}"
osc create -f test/fixtures/app-scenarios
osc status
echo "complex-scenarios: ok"

# Clean-up everything before testing cleaning up everything...
osc delete all --all
osc process -f examples/sample-app/application-template-stibuild.json -l name=mytemplate | osc create -f -
osc delete all -l name=mytemplate
osc new-app https://github.com/openshift/ruby-hello-world -l name=hello-world
osc delete all -l name=hello-world
echo "delete all: ok"

echo
echo
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/metrics" "metrics: " 0.25 80
echo
echo
echo "test-cmd: ok"
