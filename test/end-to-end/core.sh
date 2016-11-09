#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

os::util::environment::setup_time_vars
trap os::test::junit::reconcile_output EXIT

export VERBOSE=true

function wait_for_app() {
  os::log::info "Waiting for app in namespace $1"
  os::log::info "Waiting for database pod to start"
  os::cmd::try_until_text "oc get -n $1 pods -l name=database" 'Running'
  os::cmd::expect_success "oc logs dc/database -n $1"

  os::log::info "Waiting for database service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'database' "$(( 2 * TIME_MIN ))"
  DB_IP=$(oc get -n "$1" --output-version=v1 --template="{{ .spec.clusterIP }}" service database)

  os::log::info "Waiting for frontend pod to start"
  os::cmd::try_until_text "oc get -n $1 pods -l name=frontend" 'Running' "$(( 2 * TIME_MIN ))"
  os::cmd::expect_success "oc logs dc/frontend -n $1"

  os::log::info "Waiting for frontend service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'frontend' "$(( 2 * TIME_MIN ))"
  FRONTEND_IP=$(oc get -n "$1" --output-version=v1 --template="{{ .spec.clusterIP }}" service frontend)

  os::log::info "Waiting for database to start..."
  os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://${DB_IP}:5434'" "$((3*TIME_MIN))"

  os::log::info "Waiting for app to start..."
  os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://${FRONTEND_IP}:5432'" "$((2*TIME_MIN))"

  os::log::info "Testing app"
  os::cmd::try_until_text "curl -s -X POST http://${FRONTEND_IP}:5432/keys/foo -d value=1337" "Key created"
  os::cmd::try_until_text "curl -s http://${FRONTEND_IP}:5432/keys/foo" "1337"
}

function remove_docker_images() {
    local name="$1"
    local tag="${2:-\S\+}"
    local imageids=$(docker images | sed -n "s,^.*$name\s\+$tag\s\+\(\S\+\).*,\1,p" | sort -u | tr '\n' ' ')
    os::cmd::expect_success_and_text "echo '${imageids}' | wc -w" '^[1-9][0-9]*$'
    os::cmd::expect_success "docker rmi -f ${imageids}"
}

os::test::junit::declare_suite_start "end-to-end/core"

os::log::info "openshift version: `openshift version`"
os::log::info "oc version:        `oc version`"

# service dns entry is visible via master service
# find the IP of the master service by asking the API_HOST to verify DNS is running there
MASTER_SERVICE_IP="$(dig "@${API_HOST}" "kubernetes.default.svc.cluster.local." +short A | head -n 1)"
# find the IP of the master service again by asking the IP of the master service, to verify port 53 tcp/udp is routed by the service
os::cmd::expect_success_and_text "dig +tcp @${MASTER_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"
os::cmd::expect_success_and_text "dig +notcp @${MASTER_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"

# add e2e-user as a viewer for the default namespace so we can see infrastructure pieces appear
os::cmd::expect_success 'openshift admin policy add-role-to-user view e2e-user --namespace=default'

# pre-load some image streams and templates
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-stibuild.json --namespace=openshift'
os::cmd::expect_success 'oc create -f examples/jenkins/application-template.json --namespace=openshift'

# create test project so that this shows up in the console
os::cmd::expect_success "openshift admin new-project test --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project docker --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project custom --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "openshift admin new-project cache --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"

echo "The console should be available at ${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}/console."
echo "Log in as 'e2e-user' to see the 'test' project."

os::log::info "Pre-pulling and pushing ruby-22-centos7"
os::cmd::expect_success 'docker pull centos/ruby-22-centos7:latest'
os::log::info "Pulled ruby-22-centos7"

os::cmd::expect_success "openshift admin policy add-scc-to-user privileged -z ipfailover"
os::cmd::expect_success "openshift admin ipfailover --images='${USE_IMAGES}' --virtual-ips='1.2.3.4' --service-account=ipfailover"

os::log::info "Waiting for Docker registry pod to start"
os::cmd::expect_success 'oc rollout status dc/docker-registry'

os::log::info "Waiting for IP failover to deploy"
os::cmd::expect_success 'oc rollout status dc/ipfailover'
os::cmd::expect_success "oc delete all -l ipfailover=ipfailover"

# check to make sure that logs for rc works
os::cmd::expect_success "oc logs rc/docker-registry-1 > /dev/null"
# check that we can get a remote shell to a dc or rc
os::cmd::expect_success_and_text "oc rsh dc/docker-registry cat config.yml" "5000"
os::cmd::expect_success_and_text "oc rsh rc/docker-registry-1 cat config.yml" "5000"

# services can end up on any IP.  Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY=$(oc get --output-version=v1 --template="{{ .spec.clusterIP }}:{{ (index .spec.ports 0).port }}" service docker-registry)

os::cmd::expect_success_and_text "dig @${API_HOST} docker-registry.default.svc.cluster.local. +short A | head -n 1" "${DOCKER_REGISTRY/:5000}"

os::log::info "Verifying the docker-registry is up at ${DOCKER_REGISTRY}"
os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://${DOCKER_REGISTRY}/'" "$((2*TIME_MIN))"
# ensure original healthz route works as well
os::cmd::expect_success "curl -f http://${DOCKER_REGISTRY}/healthz"

os::cmd::expect_success "dig @${API_HOST} docker-registry.default.local. A"

# Client setup (log in as e2e-user and set 'test' as the default project)
# This is required to be able to push to the registry!
os::log::info "Logging in as a regular user (e2e-user:pass) with project 'test'..."
os::cmd::expect_success 'oc login -u e2e-user -p pass'
os::cmd::expect_success_and_text 'oc whoami' 'e2e-user'

# check to make sure that cluster-admin and node-reader can see node endpoint
os::test::junit::declare_suite_start "end-to-end/core/node-access"
os::cmd::expect_success "oc get --context='${CLUSTER_ADMIN_CONTEXT}' --server='https://${KUBELET_HOST}:${KUBELET_PORT}' --raw spec/"
os::cmd::expect_success "openshift admin policy add-cluster-role-to-user --context='${CLUSTER_ADMIN_CONTEXT}' system:node-reader e2e-user"
os::cmd::try_until_text "oc policy can-i get nodes/spec" "yes"
os::cmd::expect_success "oc get --server='https://${KUBELET_HOST}:${KUBELET_PORT}' --raw spec/"
os::test::junit::declare_suite_end

# make sure viewers can see oc status
os::cmd::expect_success 'oc status -n default'

# check to make sure a project admin can push an image to an image stream that doesn't exist
os::cmd::expect_success 'oc project cache'
e2e_user_token="$(oc whoami -t)"

os::log::info "Docker login as e2e-user to ${DOCKER_REGISTRY}"
os::cmd::expect_success "docker login -u e2e-user -p ${e2e_user_token} -e e2e-user@openshift.com ${DOCKER_REGISTRY}"
os::log::info "Docker login successful"

os::log::info "Tagging and pushing ruby-22-centos7 to ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::cmd::expect_success "docker tag centos/ruby-22-centos7:latest ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::log::info "Pushed ruby-22-centos7"

# get image's digest
rubyimagedigest="$(oc get -o jsonpath='{.status.tags[?(@.tag=="latest")].items[0].image}' is/ruby-22-centos7)"
os::log::info "Ruby image digest: $rubyimagedigest"
# get a random, non-empty blob
rubyimageblob="$(oc get isimage -o go-template='{{range .image.dockerImageLayers}}{{if gt .size 1024.}}{{.name}},{{end}}{{end}}' "ruby-22-centos7@${rubyimagedigest}" | cut -d , -f 1)"
os::log::info "Ruby's testing blob digest: $rubyimageblob"

# verify remote images can be pulled directly from the local registry
os::log::info "Docker pullthrough"
os::cmd::expect_success "oc import-image --confirm --from=mysql:latest mysql:pullthrough"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/mysql:pullthrough"

os::log::info "Docker registry start with GCS"
os::cmd::expect_failure_and_text "docker run -e REGISTRY_STORAGE=\"gcs: {}\" openshift/origin-docker-registry:${TAG}" "No bucket parameter provided"

os::log::info "Docker pull from istag"
os::cmd::expect_success "oc import-image --confirm --from=hello-world:latest -n test hello-world:pullthrough"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/test/hello-world:pullthrough"
os::cmd::expect_success "docker tag ${DOCKER_REGISTRY}/test/hello-world:pullthrough ${DOCKER_REGISTRY}/cache/hello-world:latest"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/hello-world:latest"

# verify we can pull from tagged image (using tag)
remove_docker_images 'cache/hello-world'
os::log::info "Tagging hello-world:latest to the same image stream and pulling it"
os::cmd::expect_success "oc tag hello-world:latest hello-world:new-tag"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/hello-world:new-tag"
os::log::info "The same image stream pull successful"

remove_docker_images "${DOCKER_REGISTRY}/cache/hello-world" new-tag
os::log::info "Tagging hello-world:latest to cross repository and pulling it"
os::cmd::expect_success "oc tag hello-world:latest cross:repo-pull"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/cross:repo-pull"
os::log::info "Cross repository pull successful"

remove_docker_images "${DOCKER_REGISTRY}/cache/cross" "repo-pull"
os::log::info "Tagging hello-world:latest to cross namespace and pulling it"
os::cmd::expect_success "oc tag cache/hello-world:latest cross:namespace-pull -n custom"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull"
os::log::info "Cross namespace pull successful"

# verify we can pull from tagged image (using image digest)
remove_docker_images "${DOCKER_REGISTRY}/custom/cross"  namespace-pull
imagedigest="$(oc get istag hello-world:latest --template='{{.image.metadata.name}}')"
os::log::info "Tagging hello-world@${imagedigest} to the same image stream and pulling it"
os::cmd::expect_success "oc tag hello-world@${imagedigest} hello-world:new-id-tag"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/hello-world:new-id-tag"
os::log::info "The same image stream pull successful"

remove_docker_images "${DOCKER_REGISTRY}/cache/hello-world" new-id-tag
os::log::info "Tagging hello-world@${imagedigest} to cross repository and pulling it"
os::cmd::expect_success "oc tag hello-world@${imagedigest} cross:repo-pull-id"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/cross:repo-pull-id"
os::log::info "Cross repository pull successful"

remove_docker_images "${DOCKER_REGISTRY}/cache/cross" repo-pull-id
os::log::info "Tagging hello-world@${imagedigest} to cross repository and pulling it by id"
os::cmd::expect_success "oc tag hello-world@${imagedigest} cross:repo-pull-id"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/cross@${imagedigest}"
os::log::info "Cross repository pull successful"

remove_docker_images "${DOCKER_REGISTRY}/cache/cross"
os::log::info "Tagging hello-world@${imagedigest} to cross namespace and pulling it"
os::cmd::expect_success "oc tag cache/hello-world@${imagedigest} cross:namespace-pull-id -n custom"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull-id"
os::log::info "Cross namespace pull successful"

os::cmd::expect_success 'oc login -u schema2-user -p pass'
os::cmd::expect_success "oc new-project schema2"
os::cmd::expect_success "oc project schema2"
schema2_user_token="$(oc whoami -t)"

# check to make sure that schema2-user is denied node access
os::test::junit::declare_suite_start "end-to-end/core/node-access-denial"
os::cmd::expect_failure_and_text "oc get --server='https://${KUBELET_HOST}:${KUBELET_PORT}' --raw spec/" "Forbidden"
os::test::junit::declare_suite_end


os::log::info "Fetch manifest V2 schema 2 image with old client using pullthrough"
os::cmd::expect_success "oc import-image --confirm --from=hello-world:latest hello-world:pullthrough"
os::cmd::expect_success_and_text "oc get -o jsonpath='{.image.dockerImageManifestMediaType}' istag hello-world:pullthrough" 'application/vnd\.docker\.distribution\.manifest\.v2\+json'
hello_world_name="$(oc get -o 'jsonpath={.image.metadata.name}' istag hello-world:pullthrough)"
os::cmd::expect_success_and_text "echo '${hello_world_name}'" '.+'
# dockerImageManifest is retrievable only with "images" resource
hello_world_config_name="$(oc get -o 'jsonpath={.dockerImageManifest}' image "${hello_world_name}" --context="${CLUSTER_ADMIN_CONTEXT}" | jq -r '.config.digest')"
hello_world_config_image="$(oc get -o 'jsonpath={.image.dockerImageConfig}' istag hello-world:pullthrough | jq -r '.container_config.Image')"
os::cmd::expect_success_and_text "echo '${hello_world_config_name},${hello_world_config_image}'" '.+,.+'
# verify we can fetch the config
os::cmd::expect_success_and_text "curl -u 'schema2-user:${schema2_user_token}' -v -s -o '${ARTIFACT_DIR}/hello-world-config.json' '${DOCKER_REGISTRY}/v2/schema2/hello-world/blobs/${hello_world_config_name}' 2>&1" "Docker-Content-Digest:\s*${hello_world_config_name}"
os::cmd::expect_success_and_text "jq -r '.container_config.Image' '${ARTIFACT_DIR}/hello-world-config.json'" "${hello_world_config_image}"
# no accept header provided, the registry will convert schema 2 to schema 1 on-the-fly
hello_world_schema1_digest="$(curl -u "schema2-user:${schema2_user_token}" -s -v -o "${ARTIFACT_DIR}/hello-world-manifest.json" "${DOCKER_REGISTRY}/v2/schema2/hello-world/manifests/pullthrough" |& sed -n 's/.*Docker-Content-Digest:\s*\(\S\+\).*/\1/p')"
# ensure the manifest was converted to schema 1
os::cmd::expect_success_and_text "jq -r '.schemaVersion' ${ARTIFACT_DIR}/hello-world-manifest.json" '^1$'
os::cmd::expect_success_and_not_text "echo '${hello_world_schema1_digest}'" "${hello_world_name}"
os::cmd::expect_success_and_text "echo '${hello_world_schema1_digest}'" ".+"
os::cmd::expect_success_and_text "curl -I -u 'schema2-user:${schema2_user_token}' '${DOCKER_REGISTRY}/v2/schema2/hello-world/manifests/${hello_world_schema1_digest}'" "404 Not Found"
os::log::info "Manifest V2 schema 2 image fetched successfully with old client"

os::log::info "Back to 'default' project with 'admin' user..."
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
os::cmd::expect_success_and_text 'oc whoami' 'system:admin'

# check to make sure an image-pusher can push an image
os::cmd::expect_success 'oc policy add-role-to-user system:image-pusher -n cache pusher'
os::cmd::expect_success 'oc login -u pusher -p pass'
pusher_token="$(oc whoami -t)"

os::log::info "Docker login as pusher to ${DOCKER_REGISTRY}"
os::cmd::expect_success "docker login -u e2e-user -p ${pusher_token} -e pusher@openshift.com ${DOCKER_REGISTRY}"
os::log::info "Docker login successful"

os::log::info "Anonymous registry access"
# setup: log out of docker, log into openshift as e2e-user to run policy commands, tag image to use for push attempts
os::cmd::expect_success 'oc login -u e2e-user'
os::cmd::expect_success 'docker pull busybox'
os::cmd::expect_success "docker tag busybox ${DOCKER_REGISTRY}/missing/image:tag"
os::cmd::expect_success "docker logout ${DOCKER_REGISTRY}"
# unauthorized pulls return "not found" errors to anonymous users, regardless of backing data
os::cmd::expect_failure_and_text "docker pull ${DOCKER_REGISTRY}/missing/image:tag"              "not found"
os::cmd::expect_failure_and_text "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull"    "not found"
os::cmd::expect_failure_and_text "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull-id" "not found"
# test anonymous pulls
os::cmd::expect_success 'oc policy add-role-to-user system:image-puller system:anonymous -n custom'
os::cmd::try_until_text 'oc policy who-can get imagestreams/layers -n custom' 'system:anonymous'
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull"
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull-id"
# unauthorized pushes return authorization errors, regardless of backing data
os::cmd::expect_failure_and_text "docker push '${DOCKER_REGISTRY}/missing/image:tag'"              "authentication required"
os::cmd::expect_failure_and_text "docker push '${DOCKER_REGISTRY}/custom/cross:namespace-pull'"    "authentication required"
os::cmd::expect_failure_and_text "docker push '${DOCKER_REGISTRY}/custom/cross:namespace-pull-id'" "authentication required"
# test anonymous pushes
os::cmd::expect_success 'oc policy add-role-to-user system:image-pusher system:anonymous -n custom'
os::cmd::try_until_text 'oc policy who-can update imagestreams/layers -n custom' 'system:anonymous'
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/custom/cross:namespace-pull"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/custom/cross:namespace-pull-id"
os::log::info "Anonymous registry access successfull"

# log back into docker as e2e-user again
os::cmd::expect_success "docker login -u e2e-user -p ${e2e_user_token} -e e2e-user@openshift.com ${DOCKER_REGISTRY}"

os::cmd::expect_success "oc new-project crossmount"
os::cmd::expect_success "oc create imagestream repo"

os::log::info "Back to 'default' project with 'admin' user..."
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
os::cmd::expect_success_and_text 'oc whoami' 'system:admin'
os::cmd::expect_success "oc tag --source docker centos/ruby-22-centos7:latest -n custom ruby-22-centos7:latest"
os::cmd::expect_success 'oc policy add-role-to-user registry-viewer pusher -n custom'
os::cmd::expect_success 'oc policy add-role-to-user system:image-pusher pusher -n crossmount'

os::log::info "Docker cross-repo mount"
os::cmd::expect_success_and_text "curl -I -X HEAD -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/cache/ruby-22-centos7/blobs/$rubyimageblob'" "200 OK"
os::cmd::try_until_text "oc get -n custom is/ruby-22-centos7 -o 'jsonpath={.status.tags[*].tag}'" "latest" $((20*TIME_SEC))
os::cmd::expect_success_and_text "curl -I -X HEAD -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/custom/ruby-22-centos7/blobs/$rubyimageblob'" "200 OK"
os::cmd::try_until_text "oc policy can-i update imagestreams/layers -n crossmount '--token=${pusher_token}'" "yes"
os::cmd::expect_success_and_text "curl -I -X HEAD -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/crossmount/repo/blobs/$rubyimageblob'" "404 Not Found"
# 202 means that cross-repo mount has failed (in this case because of blob doesn't exist in the source repository), client needs to reupload the blob
os::cmd::expect_success_and_text "curl -I -X POST -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/crossmount/repo/blobs/uploads/?mount=$rubyimageblob&from=cache/hello-world'" "202 Accepted"
# 201 means that blob has been cross mounted from given repository
os::cmd::expect_success_and_text "curl -I -X POST -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/crossmount/repo/blobs/uploads/?mount=$rubyimageblob&from=cache/ruby-22-centos7'" "201 Created"
# check that the blob is linked now
os::cmd::expect_success_and_text "curl -I -X HEAD -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/crossmount/repo/blobs/$rubyimageblob'" "200 OK"
# remove pusher's permissions to read from the source repository
os::cmd::expect_success "oc policy remove-role-from-user system:image-pusher pusher -n cache"
os::cmd::try_until_text "oc policy can-i get imagestreams/layers -n cache '--token=${pusher_token}'" "no"
# cross-repo mount failed because of access denied
os::cmd::expect_success_and_text "curl -I -X POST -u 'pusher:${pusher_token}' '${DOCKER_REGISTRY}/v2/crossmount/repo/blobs/uploads/?mount=$rubyimageblob&from=cache/ruby-22-centos7'" "202 Accepted"
os::log::info "Docker cross-repo mount successful"

# The build requires a dockercfg secret in the builder service account in order
# to be able to push to the registry.  Make sure it exists first.
os::log::info "Waiting for dockercfg secrets to be generated in project 'test' before building"
os::cmd::try_until_text 'oc get -n test serviceaccount/builder -o yaml' 'dockercfg'

# Process template and create
os::log::info "Submitting application template json for processing..."
STI_CONFIG_FILE="${ARTIFACT_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${ARTIFACT_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${ARTIFACT_DIR}/customAppConfig.json"
os::cmd::expect_success "oc process -n test -f examples/sample-app/application-template-stibuild.json > '${STI_CONFIG_FILE}'"
os::cmd::expect_success "oc process -n docker -f examples/sample-app/application-template-dockerbuild.json > '${DOCKER_CONFIG_FILE}'"
os::cmd::expect_success "oc process -n custom -f examples/sample-app/application-template-custombuild.json > '${CUSTOM_CONFIG_FILE}'"

os::log::info "Back to 'test' context with 'e2e-user' user"
os::cmd::expect_success 'oc login -u e2e-user'
os::cmd::expect_success 'oc project test'
os::cmd::expect_success 'oc whoami'

os::log::info "Running a CLI command in a container using the service account"
os::cmd::expect_success 'oc policy add-role-to-user view -z default'
oc run cli-with-token --attach --image="openshift/origin:${TAG}" --restart=Never -- cli status --loglevel=4 > "${LOG_DIR}/cli-with-token.log" 2>&1
# TODO Switch back to using cat once https://github.com/docker/docker/pull/26718 is in our Godeps
#os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token.log'" 'Using in-cluster configuration'
#os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token.log'" 'In project test'
os::cmd::expect_success_and_text "oc logs cli-with-token" 'Using in-cluster configuration'
os::cmd::expect_success_and_text "oc logs cli-with-token" 'In project test'
os::cmd::expect_success 'oc delete pod cli-with-token'
oc run cli-with-token-2 --attach --image="openshift/origin:${TAG}" --restart=Never -- cli whoami --loglevel=4 > "${LOG_DIR}/cli-with-token2.log" 2>&1
# TODO Switch back to using cat once https://github.com/docker/docker/pull/26718 is in our Godeps
#os::cmd::expect_success_and_text "cat '${LOG_DIR}/cli-with-token2.log'" 'system:serviceaccount:test:default'
os::cmd::expect_success_and_text "oc logs cli-with-token-2" 'system:serviceaccount:test:default'
os::cmd::expect_success 'oc delete pod cli-with-token-2'
oc run kubectl-with-token --attach --image="openshift/origin:${TAG}" --restart=Never --command -- kubectl get pods --loglevel=4 > "${LOG_DIR}/kubectl-with-token.log" 2>&1
# TODO Switch back to using cat once https://github.com/docker/docker/pull/26718 is in our Godeps
#os::cmd::expect_success_and_text "cat '${LOG_DIR}/kubectl-with-token.log'" 'Using in-cluster configuration'
#os::cmd::expect_success_and_text "cat '${LOG_DIR}/kubectl-with-token.log'" 'kubectl-with-token'
os::cmd::expect_success_and_text "oc logs kubectl-with-token" 'Using in-cluster configuration'
os::cmd::expect_success_and_text "oc logs kubectl-with-token" 'kubectl-with-token'

os::log::info "Testing deployment logs and failing pre and mid hooks ..."
# test hook selectors
os::cmd::expect_success "oc create -f ${OS_ROOT}/test/testdata/complete-dc-hooks.yaml"
os::cmd::try_until_text 'oc get pods -l openshift.io/deployer-pod.type=hook-pre  -o jsonpath={.items[*].status.phase}' '^Succeeded$'
os::cmd::try_until_text 'oc get pods -l openshift.io/deployer-pod.type=hook-mid  -o jsonpath={.items[*].status.phase}' '^Succeeded$'
os::cmd::try_until_text 'oc get pods -l openshift.io/deployer-pod.type=hook-post -o jsonpath={.items[*].status.phase}' '^Succeeded$'
# test the pre hook on a rolling deployment
os::cmd::expect_success 'oc create -f test/testdata/failing-dc.yaml'
os::cmd::try_until_success 'oc get rc/failing-dc-1'
os::cmd::expect_success 'oc logs -f dc/failing-dc'
os::cmd::expect_failure 'oc rollout status dc/failing-dc'
os::cmd::expect_success_and_text 'oc logs dc/failing-dc' 'test pre hook executed'
os::cmd::expect_success_and_text 'oc rollout latest failing-dc --again -o revision' '2'
os::cmd::expect_success_and_text 'oc logs --version=1 dc/failing-dc' 'test pre hook executed'
os::cmd::expect_success_and_text 'oc logs --previous dc/failing-dc'  'test pre hook executed'
# Make sure --since-time adds the right query param, and actually returns logs
os::cmd::expect_success_and_text 'oc logs --previous --since-time=2000-01-01T12:34:56Z --loglevel=6 dc/failing-dc 2>&1' 'sinceTime=2000\-01\-01T12%3A34%3A56Z'
os::cmd::expect_success_and_text 'oc logs --previous --since-time=2000-01-01T12:34:56Z --loglevel=6 dc/failing-dc 2>&1' 'test pre hook executed'
os::cmd::expect_success 'oc delete dc/failing-dc'
# test the mid hook on a recreate deployment and the health check
os::cmd::expect_success 'oc create -f test/testdata/failing-dc-mid.yaml'
os::cmd::try_until_success 'oc get rc/failing-dc-mid-1'
os::cmd::expect_success 'oc logs -f dc/failing-dc-mid'
os::cmd::expect_failure 'oc rollout status dc/failing-dc-mid'
os::cmd::expect_success_and_text 'oc logs dc/failing-dc-mid' 'test mid hook executed'
# The following command is the equivalent of 'oc deploy --latest' on old clients
# Ensures we won't break those while removing the dc status update from oc
os::cmd::expect_success "oc patch dc/failing-dc-mid -p '{\"status\":{\"latestVersion\":2}}'"
os::cmd::expect_success_and_text 'oc logs --version=1 dc/failing-dc-mid' 'test mid hook executed'
os::cmd::expect_success_and_text 'oc logs --previous dc/failing-dc-mid'  'test mid hook executed'

os::log::info "Run pod diagnostics"
# Requires a node to run the origin-deployer pod; expects registry deployed, deployer image pulled
# TODO: Find out why this would flake expecting PodCheckDns to run
# https://github.com/openshift/origin/issues/9888
#os::cmd::expect_success_and_text 'oadm diagnostics DiagnosticPod --images='"'""${USE_IMAGES}""'" 'Running diagnostic: PodCheckDns'
os::cmd::expect_success_and_not_text "oadm diagnostics DiagnosticPod --images='${USE_IMAGES}'" ERROR

os::log::info "Applying STI application config"
os::cmd::expect_success "oc create -f ${STI_CONFIG_FILE}"

# Wait for build which should have triggered automatically
os::cmd::try_until_text "oc get builds --namespace test -o jsonpath='{.items[0].status.phase}'" "Running" "$(( 10*TIME_MIN ))"
BUILD_ID="$( oc get builds --namespace test -o jsonpath='{.items[0].metadata.name}' )"
# Ensure that the build pod doesn't allow exec
os::cmd::expect_failure_and_text "oc rsh ${BUILD_ID}-build" 'forbidden'
os::cmd::try_until_text "oc get builds --namespace test -o jsonpath='{.items[0].status.phase}'" "Complete" "$(( 10*TIME_MIN ))"
os::cmd::expect_success "oc build-logs --namespace test '${BUILD_ID}' > '${LOG_DIR}/test-build.log'"
wait_for_app "test"

# logs can't be tested without a node, so has to be in e2e
POD_NAME=$(oc get pods -n test --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success "oc logs pod/${POD_NAME} --loglevel=6"
os::cmd::expect_success "oc logs ${POD_NAME} --loglevel=6"

BUILD_NAME=$(oc get builds -n test --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success "oc logs build/${BUILD_NAME} --loglevel=6"
os::cmd::expect_success "oc logs build/${BUILD_NAME} --loglevel=6"
os::cmd::expect_success 'oc logs bc/ruby-sample-build --loglevel=6'
os::cmd::expect_success 'oc logs buildconfigs/ruby-sample-build --loglevel=6'
os::cmd::expect_success 'oc logs buildconfig/ruby-sample-build --loglevel=6'
echo "logs: ok"

os::log::info "Starting build from ${STI_CONFIG_FILE} with non-existing commit..."
os::cmd::expect_failure 'oc start-build test --commit=fffffff --wait'

# Remote command execution
os::log::info "Validating exec"
frontend_pod=$(oc get pod -l deploymentconfig=frontend --template='{{(index .items 0).metadata.name}}')
# when running as a restricted pod the registry will run with a pre-allocated
# user in the neighborhood of 1000000+.  Look for a substring of the pre-allocated uid range
os::cmd::expect_success_and_text "oc exec -p ${frontend_pod} id" '1000'
os::cmd::expect_success_and_text "oc rsh pod/${frontend_pod} id -u" '1000'
os::cmd::expect_success_and_text "oc rsh -T ${frontend_pod} id -u" '1000'
# Wait for the rollout to finish
os::cmd::expect_success "oc rollout status dc/frontend --revision=1"
# Test retrieving application logs from dc
os::cmd::expect_success_and_text "oc logs dc/frontend" 'Connecting to production database'
os::cmd::expect_success_and_text "oc deploy frontend" 'deployed'

# Port forwarding
os::log::info "Validating port-forward"
os::cmd::expect_success "oc port-forward -p ${frontend_pod} 10080:8080  &> '${LOG_DIR}/port-forward.log' &"
os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://localhost:10080'" "$((10*TIME_SEC))"

# Rsync
os::log::info "Validating rsync"
os::cmd::expect_success "oc rsync examples/sample-app ${frontend_pod}:/tmp"
os::cmd::expect_success_and_text "oc rsh ${frontend_pod} ls /tmp/sample-app" 'application-template-stibuild'

#os::log::info "Applying Docker application config"
#oc create -n docker -f "${DOCKER_CONFIG_FILE}"
#os::log::info "Invoking generic web hook to trigger new docker build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/oapi/v1/namespaces/docker/buildconfigs/ruby-sample-build/webhooks/secret101/generic && sleep 3
# BUILD_ID="$( oc get builds --namespace docker -o jsonpath='{.items[0].metadata.name}' )"
# os::cmd::try_until_text "oc get builds --namespace docker -o jsonpath='{.items[0].status.phase}'" "Complete" "$(( 10*TIME_MIN ))"
# os::cmd::expect_success "oc build-logs --namespace docker '${BUILD_ID}' > '${LOG_DIR}/docker-build.log'"
#wait_for_app "docker"

#os::log::info "Applying Custom application config"
#oc create -n custom -f "${CUSTOM_CONFIG_FILE}"
#os::log::info "Invoking generic web hook to trigger new custom build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/oapi/v1/namespaces/custom/buildconfigs/ruby-sample-build/webhooks/secret101/generic && sleep 3
# BUILD_ID="$( oc get builds --namespace custom -o jsonpath='{.items[0].metadata.name}' )"
# os::cmd::try_until_text "oc get builds --namespace custom -o jsonpath='{.items[0].status.phase}'" "Complete" "$(( 10*TIME_MIN ))"
# os::cmd::expect_success "oc build-logs --namespace custom '${BUILD_ID}' > '${LOG_DIR}/custom-build.log'"
#wait_for_app "custom"

os::log::info "Back to 'default' project with 'admin' user..."
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"

# ensure the router is started
# TODO: simplify when #4702 is fixed upstream
os::cmd::try_until_text "oc get endpoints router --output-version=v1 --template='{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}'" '[1-9]+' $((5*TIME_MIN))
os::log::info "Waiting for router to start..."
router_pod=$(oc get pod -n default -l deploymentconfig=router --template='{{(index .items 0).metadata.name}}')
healthz_uri="http://$(oc get pod "${router_pod}" --template='{{.status.podIP}}'):1936/healthz"
os::cmd::try_until_success "curl --max-time 2 --fail --silent '${healthz_uri}'" "$((5*TIME_MIN))"

# Check for privileged exec limitations.
os::log::info "Validating privileged pod exec"
os::cmd::expect_success 'oc policy add-role-to-user admin e2e-default-admin'
# system:admin should be able to exec into it
os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
os::cmd::expect_success "oc exec -n default -tip ${router_pod} ls"


os::log::info "Validating routed app response..."
# 172.17.42.1 is no longer the default ip of the docker bridge as of
# docker 1.9.  Since the router is using hostNetwork=true, the router
# will be reachable via the ip of its pod.
router_ip=$(oc get pod "${router_pod}" --template='{{.status.podIP}}')
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-${router_ip}}"
os::cmd::try_until_text "curl -s -k --resolve 'www.example.com:443:${CONTAINER_ACCESSIBLE_API_HOST}' https://www.example.com" "Hello from OpenShift" "$((10*TIME_SEC))"
# Validate that oc create route edge will create an edge terminated route.
os::cmd::expect_success 'oc delete route/route-edge -n test'
os::cmd::expect_success "oc create route edge --service=frontend --cert=${MASTER_CONFIG_DIR}/ca.crt \
                                              --key=${MASTER_CONFIG_DIR}/ca.key                     \
                                              --ca-cert=${MASTER_CONFIG_DIR}/ca.crt                 \
                                              --hostname=www.example.com -n test"
os::cmd::try_until_text "curl -s -k --resolve 'www.example.com:443:${CONTAINER_ACCESSIBLE_API_HOST}' https://www.example.com" "Hello from OpenShift" "$((10*TIME_SEC))"

# Pod node selection
os::log::info "Validating pod.spec.nodeSelector rejections"
# Create a project that enforces an impossible to satisfy nodeSelector, and two pods, one of which has an explicit node name
os::cmd::expect_success "openshift admin new-project node-selector --description='This is an example project to test node selection prevents deployment' --admin='e2e-user' --node-selector='impossible-label=true'"
os::cmd::expect_success "oc process -n node-selector -v NODE_NAME='$(oc get node -o jsonpath='{.items[0].metadata.name}')' -f test/testdata/node-selector/pods.json | oc create -n node-selector -f -"
# The pod without a node name should fail to schedule
os::cmd::try_until_text 'oc get events -n node-selector' 'pod-without-node-name.+FailedScheduling' $((20*TIME_SEC))
# The pod with a node name should be rejected by the kubelet
os::cmd::try_until_text 'oc get events -n node-selector' 'pod-with-node-name.+MatchNodeSelector' $((20*TIME_SEC))


# Image pruning
os::log::info "Validating image pruning"
# builder service account should have the power to create new image streams: prune in this case
os::cmd::expect_success "docker login -u e2e-user -p $(oc sa get-token builder -n cache) -e builder@openshift.com ${DOCKER_REGISTRY}"
os::cmd::expect_success 'docker pull busybox'
os::cmd::expect_success 'docker pull gcr.io/google_containers/pause'
os::cmd::expect_success 'docker pull openshift/hello-openshift'

# tag and push 1st image - layers unique to this image will be pruned
os::cmd::expect_success "docker tag busybox ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# tag and push 2nd image - layers unique to this image will be pruned
os::cmd::expect_success "docker tag openshift/hello-openshift ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# tag and push 3rd image - it won't be pruned
os::cmd::expect_success "docker tag gcr.io/google_containers/pause ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# record the storage before pruning
registry_pod=$(oc get pod -l deploymentconfig=docker-registry --template='{{(index .items 0).metadata.name}}')
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.before.txt'"

# set up pruner user
os::cmd::expect_success 'oadm policy add-cluster-role-to-user system:image-pruner e2e-pruner'
os::cmd::try_until_text 'oadm policy who-can list images' 'e2e-pruner'
os::cmd::expect_success 'oc login -u e2e-pruner -p pass'

# run image pruning
os::cmd::expect_success_and_not_text "oadm prune images --keep-younger-than=0 --keep-tag-revisions=1 --confirm" 'error'

os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
# record the storage after pruning
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.after.txt'"

# make sure there were changes to the registry's storage
os::cmd::expect_code "diff ${LOG_DIR}/prune-images.before.txt ${LOG_DIR}/prune-images.after.txt" 1
os::log::info "Validated image pruning"

# with registry's re-deployment we loose all the blobs stored in its storage until now
os::log::info "Configure registry to accept manifest V2 schema 2"
os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
os::cmd::expect_success 'oc env -n default dc/docker-registry REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ACCEPTSCHEMA2=true'
os::cmd::expect_success 'oc rollout status dc/docker-registry'
os::log::info "Registry configured to accept manifest V2 schema 2"

os::log::info "Accept manifest V2 schema 2"
os::cmd::expect_success "oc login -u schema2-user -p pass"
os::cmd::expect_success "oc project schema2"
# tagging remote docker.io/busybox image
os::cmd::expect_success "docker tag busybox '${DOCKER_REGISTRY}/schema2/busybox'"
os::cmd::expect_success "docker login -u e2e-user -p '${schema2_user_token}' -e e2e-user@openshift.com '${DOCKER_REGISTRY}'"
os::cmd::expect_success "docker push '${DOCKER_REGISTRY}/schema2/busybox'"
# image accepted as schema 2
os::cmd::expect_success_and_text "oc get -o jsonpath='{.image.dockerImageManifestMediaType}' istag busybox:latest" 'application/vnd\.docker\.distribution\.manifest\.v2\+json'
os::log::info "Manifest V2 schema 2 successfully accepted"

os::log::info "Convert manifest V2 schema 2 to schema 1 for older client"
os::cmd::expect_success 'oc login -u schema2-user -p pass'
os::cmd::expect_success "oc new-project schema2tagged"
os::cmd::expect_success "oc tag --source=istag schema2/busybox:latest busybox:latest"
busybox_name="$(oc get -o 'jsonpath={.image.metadata.name}' istag busybox:latest)"
os::cmd::expect_success_and_text "echo '${busybox_name}'" '.+'
# no accept header provided, registry converts on-the-fly to schema 1
busybox_schema1_digest="$(curl -u "schema2-user:${schema2_user_token}" -s -v -o "${ARTIFACT_DIR}/busybox-manifest.json" "${DOCKER_REGISTRY}/v2/schema2tagged/busybox/manifests/latest" |& sed -n 's/.*Docker-Content-Digest:\s*\(\S\+\).*/\1/p')"
# ensure the manifest was converted to schema 1
os::cmd::expect_success_and_text "jq -r '.schemaVersion' '${ARTIFACT_DIR}/busybox-manifest.json'" '^1$'
os::cmd::expect_success_and_not_text "echo '${busybox_schema1_digest}'" "${busybox_name}"
os::cmd::expect_success_and_text "echo '${busybox_schema1_digest}'" ".+"
# schema 1 is generated on-the-fly, it's not stored in the registry, thus Not Found
os::cmd::expect_success_and_text "curl -I -u 'schema2-user:${schema2_user_token}' '${DOCKER_REGISTRY}/v2/schema2tagged/busybox/manifests/${busybox_schema1_digest}'" "404 Not Found"
# ensure we can fetch it back as schema 2
os::cmd::expect_success_and_text "curl -I -u 'schema2-user:${schema2_user_token}' -H 'Accept: application/vnd.docker.distribution.manifest.v2+json' '${DOCKER_REGISTRY}/v2/schema2tagged/busybox/manifests/latest'" "Docker-Content-Digest:\s*${busybox_name}"
os::log::info "Manifest V2 schema 2 successfully converted to schema 1"

os::log::info "Verify image size calculation"
busybox_expected_size="$(oc get -o 'jsonpath={.dockerImageManifest}' image "${busybox_name}" --context="${CLUSTER_ADMIN_CONTEXT}" | jq -r '[.. | .size?] | add')"
busybox_calculated_size="$(oc get -o go-template='{{.dockerImageMetadata.Size}}' image "${busybox_name}" --context="${CLUSTER_ADMIN_CONTEXT}")"
os::cmd::expect_success_and_text "echo '${busybox_expected_size}:${busybox_calculated_size}'" '^[1-9][0-9]*:[1-9][0-9]*$'
os::cmd::expect_success_and_text "echo '${busybox_expected_size}'" "${busybox_calculated_size}"
os::log::info "Image size matches"

os::test::junit::declare_suite_end
unset VERBOSE
