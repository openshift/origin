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
unset KUBECONFIG

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
  tag="latest"
  if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
    COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
    tag="${COMMIT}"
  fi
  USE_IMAGES="openshift/origin-\${component}:${tag}"
fi

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

TEMP_DIR=${USE_TEMP:-$(mkdir -p /tmp/openshift-cmd && mktemp -d /tmp/openshift-cmd/XXXX)}
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
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},$(openshift start --print-ip),localhost"

openshift admin ca create-master-certs \
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

oadm create-bootstrap-policy-file --filename="${MASTER_CONFIG_DIR}/policy.json"

# create openshift config
openshift start \
  --write-config=${SERVER_CONFIG_DIR} \
  --create-certs=false \
  --master="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --hostname="${KUBELET_HOST}" \
  --volume-dir="${VOLUME_DIR}" \
  --etcd-dir="${ETCD_DATA_DIR}" \
  --images="${USE_IMAGES}"


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
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta3/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

# test client not configured
[ "$(oc get services 2>&1 | grep 'Error in configuration')" ]

# Set KUBERNETES_MASTER for oc from now on
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Set certificates for oc from now on
if [[ "${API_SCHEME}" == "https" ]]; then
    # test bad certificate
    [ "$(oc get services 2>&1 | grep 'certificate signed by unknown authority')" ]
fi

# login and logout tests
# --token and --username are mutually exclusive
[ "$(oc login ${KUBERNETES_MASTER} -u test-user --token=tmp --insecure-skip-tls-verify 2>&1 | grep 'mutually exclusive')" ]
# must only accept one arg (server)
[ "$(oc login https://server1 https://server2.com 2>&1 | grep 'Only the server URL may be specified')" ]
# logs in with a valid certificate authority
oc login ${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything --api-version=v1beta3
grep -q "v1beta3" ${HOME}/.kube/config
oc logout
# logs in skipping certificate check
oc login ${KUBERNETES_MASTER} --insecure-skip-tls-verify -u test-user -p anything
# logs in by an existing and valid token
temp_token=$(oc config view -o template --template='{{range .users}}{{ index .user.token }}{{end}}')
[ "$(oc login --token=${temp_token} 2>&1 | grep 'using the token provided')" ]
oc logout
# properly parse server port
[ "$(oc login https://server1:844333 2>&1 | grep 'Not a valid port')" ]
# properly handle trailing slash
oc login --server=${KUBERNETES_MASTER}/ --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
# create a new project
oc new-project project-foo --display-name="my project" --description="boring project description"
[ "$(oc project | grep 'Using project "project-foo"')" ]
# denies access after logging out
oc logout
[ -z "$(oc get pods | grep 'system:anonymous')" ]

# log in and set project to use from now on
oc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
oc get projects
oc project project-foo
[ "$(oc whoami | grep 'test-user')" ]
[ -n "$(oc whoami -t)" ]
[ -n "$(oc whoami -c)" ]

# test config files from the --config flag
oc get services --config="${MASTER_CONFIG_DIR}/admin.kubeconfig"

# test config files from env vars
KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig" oc get services

# test config files in the home directory
mkdir -p ${HOME}/.kube
cp ${MASTER_CONFIG_DIR}/admin.kubeconfig ${HOME}/.kube/config
oc get services
mv ${HOME}/.kube/config ${HOME}/.kube/non-default-config
echo "config files: ok"
export KUBECONFIG="${HOME}/.kube/non-default-config"

# from this point every command will use config from the KUBECONFIG env var

oc get templates
oc create -f examples/sample-app/application-template-dockerbuild.json
oc get templates
oc get templates ruby-helloworld-sample
oc get template ruby-helloworld-sample -o json | oc process -f -
oc process ruby-helloworld-sample
oc describe templates ruby-helloworld-sample
[ "$(oc describe templates ruby-helloworld-sample | grep -E "BuildConfig.*ruby-sample-build")" ]
oc delete templates ruby-helloworld-sample
oc get templates
# TODO: create directly from template
echo "templates: ok"


# Test resource builder filtering of files with expected extensions inside directories, and individual files without expected extensions
[ "$(oc create -f test/resource-builder/directory -f test/resource-builder/json-no-extension -f test/resource-builder/yml-no-extension 2>&1)" ]
# Explicitly specified extensionless files
oc get secret json-no-extension yml-no-extension
# Scanned files with extensions inside directories
oc get secret json-with-extension yml-with-extension
# Ensure extensionless files inside directories are not processed by resource-builder
[ "$(oc get secret json-no-extension-in-directory 2>&1 | grep 'not found')" ]
echo "resource-builder: ok"

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
[ "$(oc 2>&1)" ]
[ "$(osc 2>&1)" ]
[ "$(oadm 2>&1)" ]
[ "$(oadm 2>&1)" ]
[ "$(origin 2>&1)" ]

# help for root commands must be consistent
[ "$(openshift | grep 'OpenShift Application Platform')" ]
[ "$(oc | grep 'OpenShift Client')" ]
[ "$(oc | grep 'Build and Deploy Commands:')" ]
[ "$(oc | grep 'Other Commands:')" ]
[ "$(oc policy --help 2>&1 | grep 'add-role-to-user')" ]
[ ! "$(oc policy --help 2>&1 | grep 'Other Commands')" ]
[ ! "$(oc 2>&1 | grep 'Options')" ]
[ ! "$(oc 2>&1 | grep 'Global Options')" ]
[ "$(openshift cli 2>&1 | grep 'OpenShift Client')" ]
[ "$(oc types | grep 'Deployment Config')" ]
[ "$(openshift kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(oadm 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift admin 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(oadm | grep 'Basic Commands:')" ]
[ "$(oadm | grep 'Install Commands:')" ]
[ "$(oadm ca | grep 'Manage certificates')" ]
[ "$(openshift start kubernetes 2>&1 | grep 'Kubernetes server components')" ]
# check deprecated admin cmds for backward compatibility
[ "$(oadm create-master-certs -h 2>&1 | grep 'Create keys and certificates')" ]
[ "$(oadm create-key-pair -h 2>&1 | grep 'Create an RSA key pair')" ]
[ "$(oadm create-server-cert -h 2>&1 | grep 'Create a key and server certificate')" ]
[ "$(oadm create-signer-cert -h 2>&1 | grep 'Create a self-signed CA')" ]

# help for root commands with --help flag must be consistent
[ "$(openshift --help 2>&1 | grep 'OpenShift Application Platform')" ]
[ "$(oc --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(oc login --help 2>&1 | grep 'Options')" ]
[ ! "$(oc login --help 2>&1 | grep 'Global Options')" ]
[ "$(oc login --help 2>&1 | grep 'insecure-skip-tls-verify')" ]
[ "$(openshift cli --help 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift kubectl --help 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(oadm --help 2>&1 | grep 'OpenShift Administrative Commands')" ]
[ "$(openshift admin --help 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for root commands through help command must be consistent
[ "$(openshift help cli 2>&1 | grep 'OpenShift Client')" ]
[ "$(openshift help kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(openshift help admin 2>&1 | grep 'OpenShift Administrative Commands')" ]

# help for given command with --help flag must be consistent
[ "$(oc get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift start --help 2>&1 | grep 'Start an OpenShift all-in-one server')" ]
[ "$(openshift start master --help 2>&1 | grep 'Start an OpenShift master')" ]
[ "$(openshift start node --help 2>&1 | grep 'Start an OpenShift node')" ]
[ "$(oc get --help 2>&1 | grep 'oc')" ]

# help for given command through help command must be consistent
[ "$(oc help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help cli get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help kubectl get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help start 2>&1 | grep 'Start an OpenShift all-in-one server')" ]
[ "$(openshift help start master 2>&1 | grep 'Start an OpenShift master')" ]
[ "$(openshift help start node 2>&1 | grep 'Start an OpenShift node')" ]
[ "$(openshift cli help update 2>&1 | grep 'openshift')" ]
[ "$(openshift cli help replace 2>&1 | grep 'openshift')" ]
[ "$(openshift cli help patch 2>&1 | grep 'openshift')" ]

# runnable commands with required flags must error consistently
[ "$(oc get 2>&1 | grep 'Required resource not specified')" ]
[ "$(openshift cli get 2>&1 | grep 'Required resource not specified')" ]
[ "$(openshift kubectl get 2>&1 | grep 'Required resource not specified')" ]

# commands that expect file paths must validate and error out correctly
[ "$(oc login --certificate-authority=/path/to/invalid 2>&1 | grep 'no such file or directory')" ]

# make sure that typoed commands come back with non-zero return codes
[ "$(openshift admin policy TYPO; echo $? | grep '1')" ]
[ "$(openshift admin TYPO; echo $? | grep '1')" ]
[ "$(openshift cli TYPO; echo $? | grep '1')" ]
[ "$(oc policy TYPO; echo $? | grep '1')" ]
[ "$(oc secrets TYPO; echo $? | grep '1')" ]


oc secrets new-dockercfg dockercfg --docker-username=sample-user --docker-password=sample-password --docker-email=fake@example.org
# can't use a go template here because the output needs to be base64 decoded.  base64 isn't installed by default in all distros
oc describe secrets/dockercfg | grep "dockercfg:" | awk '{print $2}' > ${HOME}/dockerconfig
oc secrets new from-file .dockercfg=${HOME}/dockerconfig
# check to make sure the type was correctly auto-detected
[ "$(oc get secret/from-file -t "{{ .type }}" | grep 'kubernetes.io/dockercfg')" ]
# make sure the -o works correctly
[ "$(oc secrets new-dockercfg dockercfg --docker-username=sample-user --docker-password=sample-password --docker-email=fake@example.org -o yaml | grep "kubernetes.io/dockercfg")" ]
[ "$(oc secrets new from-file .dockercfg=${HOME}/dockerconfig -o yaml | grep "kubernetes.io/dockercfg")" ]
# check to make sure malformed names fail as expected
[ "$(oc secrets new bad-name .docker=cfg=${HOME}/dockerconfig 2>&1 | grep "error: Key names or file paths cannot contain '='.")" ] 


# attach secrets to service account
# single secret with prefix
oc secrets add serviceaccounts/deployer secrets/dockercfg
# don't add the same secret twice
oc secrets add serviceaccounts/deployer secrets/dockercfg secrets/from-file
# make sure we can add as as pull secret
oc secrets add serviceaccounts/deployer secrets/dockercfg secrets/from-file --for=pull
# make sure we can add as as pull secret and mount secret at once
oc secrets add serviceaccounts/deployer secrets/dockercfg secrets/from-file --for=pull,mount
echo "secrets: ok"


oc get pods --match-server-version
oc create -f examples/hello-openshift/hello-pod.json
oc describe pod hello-openshift
oc delete pods hello-openshift
echo "pods: ok"

oc get services
oc create -f test/integration/fixtures/test-service.json
oc delete services frontend
echo "services: ok"

oc get nodes
echo "nodes: ok"

oc get images
oc create -f test/integration/fixtures/test-image.json
oc delete images test
echo "images: ok"

oc get imageStreams
oc create -f test/integration/fixtures/test-image-stream.json
# make sure stream.status.dockerImageRepository isn't set (no registry)
[ -z "$(oc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
# create the registry
oadm registry --credentials="${KUBECONFIG}" --images="${USE_IMAGES}"
# make sure stream.status.dockerImageRepository IS set
[ -n "$(oc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
# ensure the registry rc has been created
wait_for_command 'oc get rc docker-registry-1' "${TIME_MIN}"
# delete the registry resources
oc delete svc docker-registry
oc delete dc docker-registry
[ ! "$(oc get dc docker-registry)" ]
[ ! "$(oc get rc docker-registry-1)" ]
# done deleting registry resources
oc delete imageStreams test
[ -z "$(oc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
oc create -f examples/image-streams/image-streams-centos7.json
[ -n "$(oc get imageStreams ruby -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams nodejs -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams wildfly -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams mysql -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams postgresql -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams mongodb -t "{{.status.dockerImageRepository}}")" ]
# verify the image repository had its tags populated
wait_for_command 'oc get imagestreamtags wildfly:latest' "${TIME_MIN}"
[ -n "$(oc get imageStreams wildfly -t "{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}")" ]
oc delete imageStreams ruby
oc delete imageStreams nodejs
oc delete imageStreams wildfly
#oc delete imageStreams mysql
oc delete imageStreams postgresql
oc delete imageStreams mongodb
[ -z "$(oc get imageStreams ruby -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams nodejs -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams postgresql -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams mongodb -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams wildfly -t "{{.status.dockerImageRepository}}")" ]
wait_for_command 'oc get imagestreamTags mysql:latest' "${TIME_MIN}"
[ -n "$(oc get imagestreams mysql -t "{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}")" ]
oc describe istag/mysql:latest
[ "$(oc describe istag/mysql:latest | grep "Environment:")" ]
[ "$(oc describe istag/mysql:latest | grep "Image Created:")" ]
[ "$(oc describe istag/mysql:latest | grep "Image Name:")" ]
name=$(oc get istag/mysql:latest -t '{{ .image.metadata.name }}')
imagename="isimage/mysql@${name:0:7}"
oc describe "${imagename}"
[ "$(oc describe ${imagename} | grep "Environment:")" ]
[ "$(oc describe ${imagename} | grep "Image Created:")" ]
[ "$(oc describe ${imagename} | grep "Image Name:")" ]
echo "imageStreams: ok"

# oc tag
# start with an empty target image stream
echo '{"apiVersion":"v1", "kind": "ImageStream", "metadata": {"name":"tagtest"}}' | oc create -f -
echo '{"apiVersion":"v1", "kind": "ImageStream", "metadata": {"name":"tagtest2"}}' | oc create -f -

oc tag mysql:latest tagtest:tag1
[ "$(oc get is/tagtest -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]

oc tag mysql@${name} tagtest:tag2
[ "$(oc get is/tagtest -t '{{(index .spec.tags 1).from.kind}}')" == "ImageStreamImage" ]

oc tag mysql:notfound tagtest:tag3
[ "$(oc get is/tagtest -t '{{(index .spec.tags 2).from.kind}}')" == "DockerImage" ]

oc tag --source=imagestreamtag mysql:latest tagtest:tag4
[ "$(oc get is/tagtest -t '{{(index .spec.tags 3).from.kind}}')" == "ImageStreamTag" ]

oc tag --source=istag mysql:latest tagtest:tag5
[ "$(oc get is/tagtest -t '{{(index .spec.tags 4).from.kind}}')" == "ImageStreamTag" ]

oc tag --source=imagestreamimage mysql@${name} tagtest:tag6
[ "$(oc get is/tagtest -t '{{(index .spec.tags 5).from.kind}}')" == "ImageStreamImage" ]

oc tag --source=isimage mysql@${name} tagtest:tag7
[ "$(oc get is/tagtest -t '{{(index .spec.tags 6).from.kind}}')" == "ImageStreamImage" ]

oc tag --source=docker mysql:latest tagtest:tag8
[ "$(oc get is/tagtest -t '{{(index .spec.tags 7).from.kind}}')" == "DockerImage" ]

oc tag mysql:latest tagtest:zzz tagtest2:zzz
[ "$(oc get is/tagtest -t '{{(index .spec.tags 8).from.kind}}')" == "ImageStreamTag" ]
[ "$(oc get is/tagtest2 -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]

# test creating streams that don't exist
[ -z "$(oc get imageStreams tagtest3 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams tagtest4 -t "{{.status.dockerImageRepository}}")" ]
oc tag mysql:latest tagtest3:latest tagtest4:latest
[ "$(oc get is/tagtest3 -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]
[ "$(oc get is/tagtest4 -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]

oc delete is/tagtest is/tagtest2 is/tagtest3 is/tagtest4
echo "tag: ok"

[ "$(oc new-app library/php mysql -o yaml | grep 3306)" ]
[ ! "$(oc new-app unknownhubimage -o yaml)" ]
# verify we can generate a Docker image based component "mongodb" directly
[ "$(oc new-app mongo -o yaml | grep library/mongo)" ]
# the local image repository takes precedence over the Docker Hub "mysql" image
[ "$(oc new-app mysql -o yaml | grep mysql-55-centos7)" ]
oc delete all --all
oc new-app library/php mysql -l no-source=php-mysql
oc delete all -l no-source=php-mysql
# check if we can create from a stored template
oc create -f examples/sample-app/application-template-stibuild.json
oc get template ruby-helloworld-sample
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep MYSQL_USER)" ]
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep MYSQL_PASSWORD)" ]
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep ADMIN_USERNAME)" ]
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep ADMIN_PASSWORD)" ]
# check search
oc create -f examples/image-streams/image-streams-centos7.json
[ "$(oc new-app --search mysql | grep mysql-55-centos7)" ]
[ "$(oc new-app --search ruby-helloworld-sample | grep ruby-helloworld-sample)" ]
# check search - partial matches
[ "$(oc new-app --search ruby-hellow | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search --template=ruby-hel | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search --template=ruby-helloworld-sam -o yaml | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search rub | grep openshift/ruby-20-centos7)" ]
[ "$(oc new-app --search --image-stream=rub | grep openshift/ruby-20-centos7)" ]
# check search - check correct usage of filters
[ ! "$(oc new-app --search --image-stream=ruby-heloworld-sample | grep application-template-stibuild)" ]
[ ! "$(oc new-app --search --template=mongodb)" ]
[ ! "$(oc new-app --search --template=php)" ]
[ ! "$(oc new-app -S --template=nodejs)" ]
[ ! "$(oc new-app -S --template=perl)" ]
# check search - filtered, exact matches
[ "$(oc new-app --search --image-stream=mongodb | grep openshift/mongodb-24-centos7)" ]
[ "$(oc new-app --search --image-stream=mysql | grep openshift/mysql-55-centos7)" ]
[ "$(oc new-app --search --image-stream=nodejs | grep openshift/nodejs-010-centos7)" ]
[ "$(oc new-app --search --image-stream=perl | grep openshift/perl-516-centos7)" ]
[ "$(oc new-app --search --image-stream=php | grep openshift/php-55-centos7)" ]
[ "$(oc new-app --search --image-stream=postgresql | grep openshift/postgresql-92-centos7)" ]
[ "$(oc new-app -S --image-stream=python | grep openshift/python-33-centos7)" ]
[ "$(oc new-app -S --image-stream=ruby | grep openshift/ruby-20-centos7)" ]
[ "$(oc new-app -S --image-stream=wildfly | grep openshift/wildfly-8-centos)" ]
[ "$(oc new-app --search --template=ruby-helloworld-sample | grep ruby-helloworld-sample)" ]
# check search - no matches
[ "$(oc new-app -S foo-the-bar 2>&1 | grep 'no matches found')" ]
[ "$(oc new-app --search winter-is-coming 2>&1 | grep 'no matches found')" ]
# check search - mutually exclusive flags
[ "$(oc new-app -S mysql --env=FOO=BAR 2>&1 | grep "can't be used")" ]
[ "$(oc new-app --search mysql --code=https://github.com/openshift/ruby-hello-world 2>&1 | grep "can't be used")" ]
[ "$(oc new-app --search mysql --param=FOO=BAR 2>&1 | grep "can't be used")" ]
oc delete imageStreams --all
# check that we can create from the template without errors
oc new-app ruby-helloworld-sample -l app=helloworld
oc delete all -l app=helloworld
# create from template with code explicitly set is not supported
[ ! "$(oc new-app ruby-helloworld-sample~git@github.com/mfojtik/sinatra-app-example)" ]
oc delete template ruby-helloworld-sample
# override component names
[ "$(oc new-app mysql --name=db | grep db)" ]
oc new-app https://github.com/openshift/ruby-hello-world -l app=ruby
oc delete all -l app=ruby
echo "new-app: ok"

oc get routes
oc create -f test/integration/fixtures/test-route.json
oc delete routes testroute
echo "routes: ok"

oc get deploymentConfigs
oc get dc
oc create -f test/integration/fixtures/test-deployment-config.json
oc describe deploymentConfigs test-deployment-config
[ "$(oc env dc/test-deployment-config --list | grep TEST=value)" ]
[ ! "$(oc env dc/test-deployment-config TEST- --list | grep TEST=value)" ]
[ "$(oc env dc/test-deployment-config TEST=foo --list | grep TEST=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo --list | grep TEST=value)" ]
[ ! "$(oc env dc/test-deployment-config OTHER=foo -c 'ruby' --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -c 'ruby*'   --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -c '*hello*' --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -c '*world'  --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config OTHER=foo -o yaml | grep "name: OTHER")" ]
[ "$(echo "OTHER=foo" | oc env dc/test-deployment-config -e - --list | grep OTHER=foo)" ]
[ ! "$(echo "#OTHER=foo" | oc env dc/test-deployment-config -e - --list | grep OTHER=foo)" ]
[ "$(oc env dc/test-deployment-config TEST=bar OTHER=baz BAR-)" ]

[ "$(oc volume dc/test-deployment-config --list | grep vol1)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /opt)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol1 --type=secret --secret-name='$ecret' -m /data | grep overwrite)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol1 --type=emptyDir -m /data --overwrite)" ]
[ "$(oc volume dc/test-deployment-config --add -m /opt | grep exists)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby' --overwrite | grep warning)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby*' --overwrite)" ]
[ "$(oc volume dc/test-deployment-config --list --name=vol2 | grep /etc)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol3 -o yaml | grep vol3)" ]
[ "$(oc volume dc/test-deployment-config --list --name=vol3 | grep 'not found')" ]
[ "$(oc volume dc/test-deployment-config --remove 2>&1 | grep confirm)" ]
[ "$(oc volume dc/test-deployment-config --remove --name=vol2)" ]
[ ! "$(oc volume dc/test-deployment-config --list | grep vol2)" ]
[ "$(oc volume dc/test-deployment-config --remove --confirm)" ]
[ ! "$(oc volume dc/test-deployment-config --list | grep vol1)" ]
oc deploy test-deployment-config
oc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

oc process -f test/templates/fixtures/guestbook.json -l app=guestbook | oc create -f -
oc status
[ "$(oc status | grep frontend-service)" ]
echo "template+config: ok"
[ "$(OC_EDITOR='cat' oc edit svc/kubernetes 2>&1 | grep 'Edit cancelled')" ]
[ "$(OC_EDITOR='cat' oc edit svc/kubernetes | grep 'provider: kubernetes')" ]
oc delete all -l app=guestbook
echo "edit: ok"

oc delete all --all
oc create -f test/integration/fixtures/test-deployment-config.json
oc deploy test-deployment-config --latest
wait_for_command 'oc get rc/test-deployment-config-1' "${TIME_MIN}"
# scale rc via deployment configuration
oc scale dc test-deployment-config --replicas=1
# scale directly
oc scale rc test-deployment-config-1 --replicas=5
oc delete all --all
echo "scale: ok"

oc process -f examples/sample-app/application-template-dockerbuild.json -l app=dockerbuild | oc create -f -
wait_for_command 'oc get rc/database-1' "${TIME_MIN}"

oc rollback database --to-version=1 -o=yaml
oc rollback dc/database --to-version=1 -o=yaml
oc rollback dc/database --to-version=1 --dry-run
oc rollback database-1 -o=yaml
oc rollback rc/database-1 -o=yaml
# should fail because there's no previous deployment
[ ! "$(oc rollback database -o yaml)" ]
echo "rollback: ok"

oc get dc/database
oc stop dc/database
[ ! "$(oc get dc/database)" ]
[ ! "$(oc get rc/database-1)" ]
echo "stop: ok"

oc label bc ruby-sample-build acustom=label
[ "$(oc describe bc/ruby-sample-build | grep 'acustom=label')" ]
echo "label: ok"

oc delete all -l app=dockerbuild
echo "delete: ok"

oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -
oc get buildConfigs
oc get bc
oc get builds

[[ $(oc describe buildConfigs ruby-sample-build --api-version=v1beta3 | grep --text "Webhook GitHub"  | grep -F "${API_SCHEME}://${API_HOST}:${API_PORT}/osapi/v1beta3/namespaces/default/buildconfigs/ruby-sample-build/webhooks/secret101/github") ]]
[[ $(oc describe buildConfigs ruby-sample-build --api-version=v1beta3 | grep --text "Webhook Generic" | grep -F "${API_SCHEME}://${API_HOST}:${API_PORT}/osapi/v1beta3/namespaces/default/buildconfigs/ruby-sample-build/webhooks/secret101/generic") ]]
oc start-build --list-webhooks='all' ruby-sample-build
[[ $(oc start-build --list-webhooks='all' ruby-sample-build | grep --text "generic") ]]
[[ $(oc start-build --list-webhooks='all' ruby-sample-build | grep --text "github") ]]
[[ $(oc start-build --list-webhooks='github' ruby-sample-build | grep --text "secret101") ]]
[ ! "$(oc start-build --list-webhooks='blah')" ]
webhook=$(oc start-build --list-webhooks='generic' ruby-sample-build --api-version=v1beta3 | head -n 1)
oc start-build --from-webhook="${webhook}"
webhook=$(oc start-build --list-webhooks='generic' ruby-sample-build --api-version=v1 | head -n 1)
oc start-build --from-webhook="${webhook}"
oc get builds
oc delete all -l build=docker
echo "buildConfig: ok"

oc create -f test/integration/fixtures/test-buildcli.json
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
started=$(oc start-build ruby-sample-build-invalidtag)
oc describe build ${started} | grep openshift/ruby-20-centos7$
echo "start-build: ok"

oc cancel-build "${started}" --dump-logs --restart
echo "cancel-build: ok"
oc delete is/ruby-20-centos7-buildcli
oc delete bc/ruby-sample-build-validtag
oc delete bc/ruby-sample-build-invalidtag

# Test admin manage-node operations
[ "$(openshift admin manage-node --help 2>&1 | grep 'Manage nodes')" ]
[ "$(oadm manage-node --selector='' --schedulable=true | grep --text 'Ready' | grep -v 'Sched')" ]
oc create -f examples/hello-openshift/hello-pod.json
#[ "$(oadm manage-node --list-pods | grep 'hello-openshift' | grep -E '(unassigned|assigned)')" ]
#[ "$(oadm manage-node --evacuate --dry-run | grep 'hello-openshift')" ]
#[ "$(oadm manage-node --schedulable=false | grep 'SchedulingDisabled')" ]
#[ "$(oadm manage-node --evacuate 2>&1 | grep 'Unable to evacuate')" ]
#[ "$(oadm manage-node --evacuate --force | grep 'hello-openshift')" ]
#[ ! "$(oadm manage-node --list-pods | grep 'hello-openshift')" ]
oc delete pods hello-openshift
echo "manage-node: ok"

oadm policy who-can get pods
oadm policy who-can get pods -n default
oadm policy who-can get pods --all-namespaces

oadm policy add-role-to-group cluster-admin system:unauthenticated
oadm policy add-role-to-user cluster-admin system:no-user
oadm policy remove-role-from-group cluster-admin system:unauthenticated
oadm policy remove-role-from-user cluster-admin system:no-user
oadm policy remove-group system:unauthenticated
oadm policy remove-user system:no-user
oadm policy add-cluster-role-to-group cluster-admin system:unauthenticated
oadm policy remove-cluster-role-from-group cluster-admin system:unauthenticated
oadm policy add-cluster-role-to-user cluster-admin system:no-user
oadm policy remove-cluster-role-from-user cluster-admin system:no-user

oc policy add-role-to-group cluster-admin system:unauthenticated
oc policy add-role-to-user cluster-admin system:no-user
oc policy remove-role-from-group cluster-admin system:unauthenticated
oc policy remove-role-from-user cluster-admin system:no-user
oc policy remove-group system:unauthenticated
oc policy remove-user system:no-user
echo "policy: ok"

# Test the commands the UI projects page tells users to run
# These should match what is described in projects.html
oadm new-project ui-test-project --admin="createuser"
oadm policy add-role-to-user admin adduser -n ui-test-project
# Make sure project can be listed by oc (after auth cache syncs)
sleep 2 && [ "$(oc get projects | grep 'ui-test-project')" ]
# Make sure users got added
[ "$(oc describe policybinding ':default' -n ui-test-project | grep createuser)" ]
[ "$(oc describe policybinding ':default' -n ui-test-project | grep adduser)" ]
echo "ui-project-commands: ok"

# Expose service as a route
oc create -f test/integration/fixtures/test-service.json
[ ! "$(oc expose service frontend --create-external-load-balancer)" ]
[ ! "$(oc expose service frontend --port=40 --type=NodePort)" ]
oc expose service frontend
[ "$(oc get route frontend | grep 'name=frontend')" ]
oc delete svc,route -l name=frontend
echo "expose: ok"

# Test deleting and recreating a project
oadm new-project recreated-project --admin="createuser1"
oc delete project recreated-project
wait_for_command '! oc get project recreated-project' "${TIME_MIN}"
oadm new-project recreated-project --admin="createuser2"
oc describe policybinding ':default' -n recreated-project | grep createuser2
echo "new-project: ok"

# Test running a router
[ ! "$(oadm router --dry-run | grep 'does not exist')" ]
[ "$(oadm router -o yaml --credentials="${KUBECONFIG}" | grep 'openshift/origin-haproxy-')" ]
oadm router --credentials="${KUBECONFIG}" --images="${USE_IMAGES}"
[ "$(oadm router | grep 'service exists')" ]
echo "router: ok"

# Test running a registry
[ ! "$(oadm registry --dry-run | grep 'does not exist')"]
[ "$(oadm registry -o yaml --credentials="${KUBECONFIG}" | grep 'openshift/origin-docker-registry')" ]
oadm registry --credentials="${KUBECONFIG}" --images="${USE_IMAGES}"
[ "$(oadm registry | grep 'service exists')" ]
echo "registry: ok"

# Test building a dependency tree
oc process -f examples/sample-app/application-template-stibuild.json -l build=sti | oc create -f -
[ "$(openshift ex build-chain --all -o dot | grep 'graph')" ]
oc delete all -l build=sti
echo "ex build-chain: ok"

oadm new-project example --admin="createuser"
oc project example
wait_for_command 'oc get serviceaccount default' "${TIME_MIN}"
oc create -f test/fixtures/app-scenarios
oc status
echo "complex-scenarios: ok"

[ "$(oc export svc --all -t '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' | wc -l)" -ne 0 ]
[ "$(oc export svc --all --as-template=template | grep 'kind: Template')" ]
[ ! "$(oc export svc --all | grep 'clusterIP')" ]
[ ! "$(oc export svc --all --exact | grep 'clusterIP: ""')" ]
[ ! "$(oc export svc --all --raw | grep 'clusterIP: ""')" ]
[ ! "$(oc export svc --all --raw --exact)" ]
[ ! "$(oc export svc -l a=b)" ] # return error if no items match selector
[ "$(oc export svc -l a=b 2>&1 | grep 'no resources found')" ]
[ "$(oc export svc -l template=ruby-helloworld-sample)" ]
[ "$(oc export -f examples/sample-app/application-template-stibuild.json --raw --output-version=v1 | grep 'apiVersion: v1')" ]
echo "export: ok"

# Clean-up everything before testing cleaning up everything...
oc delete all --all
oc process -f examples/sample-app/application-template-stibuild.json -l name=mytemplate | oc create -f -
oc delete all -l name=mytemplate
oc new-app https://github.com/openshift/ruby-hello-world -l name=hello-world
oc delete all -l name=hello-world
echo "delete all: ok"

echo
echo
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/metrics" "metrics: " 0.25 80
echo
echo
echo "test-cmd: ok"
