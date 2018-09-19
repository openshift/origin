#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
os::util::ensure_tmpfs "${ETCD_DATA_DIR}"

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
  DB_IP=$(oc get -n "$1" --template="{{ .spec.clusterIP }}" service database)

  os::log::info "Waiting for frontend pod to start"
  os::cmd::try_until_text "oc get -n $1 pods -l name=frontend" 'Running' "$(( 2 * TIME_MIN ))"
  os::cmd::expect_success "oc logs dc/frontend -n $1"

  os::log::info "Waiting for frontend service to start"
  os::cmd::try_until_text "oc get -n $1 services" 'frontend' "$(( 2 * TIME_MIN ))"
  FRONTEND_IP=$(oc get -n "$1" --template="{{ .spec.clusterIP }}" service frontend)

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

# Ensure that the master service responds to DNS requests. At this point 'oc cluster up' has verified
# that the service is up
MASTER_SERVICE_IP="172.30.0.1"
DNS_SERVICE_IP="172.30.0.2"
# find the IP of the master service again by asking the IP of the master service, to verify port 53 tcp/udp is routed by the service
os::cmd::expect_success_and_text "dig +tcp @${DNS_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"
os::cmd::expect_success_and_text "dig +notcp @${DNS_SERVICE_IP} kubernetes.default.svc.cluster.local. +short A | head -n 1" "${MASTER_SERVICE_IP}"

# add e2e-user as a viewer for the default namespace so we can see infrastructure pieces appear
os::cmd::expect_success 'oc adm policy add-role-to-user view e2e-user --namespace=default'

os::cmd::expect_success "oc adm new-project test --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "oc adm new-project docker --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "oc adm new-project custom --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"
os::cmd::expect_success "oc adm new-project cache --description='This is an example project to demonstrate OpenShift v3' --admin='e2e-user'"

os::log::info "Waiting for Docker registry pod to start"
os::cmd::expect_success 'oc project default'
# this should not be needed but the DC is creating an RC w/a replicacount==0 for unknown reasons.
os::cmd::expect_success 'oc scale --replicas=1 replicationcontrollers docker-registry-1'

# this should not be necessary but "oc rollout status dc/docker-registry" returns a failure
# because it thinks the deployment is not progressing even though the RC reports availability.
sleep 60
#os::cmd::expect_success 'oc rollout status dc/docker-registry'

# dump the logs for the registry pod so we can confirm the version, among other things.
oc get pods -n default | grep registry | awk '{print $1}' | xargs -n 1 oc logs

# check to make sure that logs for rc works
os::cmd::expect_success "oc logs rc/docker-registry-1 > /dev/null"
# check that we can get a remote shell to a dc or rc
os::cmd::expect_success_and_text "oc rsh dc/docker-registry cat config.yml" "5000"
os::cmd::expect_success_and_text "oc rsh rc/docker-registry-1 cat config.yml" "5000"

# services can end up on any IP.  Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY=$(oc get --template="{{ .spec.clusterIP }}:{{ (index .spec.ports 0).port }}" service docker-registry)

os::cmd::expect_success_and_text "dig @${DNS_SERVICE_IP} docker-registry.default.svc.cluster.local. +short A | head -n 1" "${DOCKER_REGISTRY/:5000}"

os::log::info "Verifying the docker-registry is up at ${DOCKER_REGISTRY}"
os::cmd::try_until_success "curl --max-time 2 --fail --silent 'http://${DOCKER_REGISTRY}/'" "$((2*TIME_MIN))"
# ensure original healthz route works as well
os::cmd::expect_success "curl -f http://${DOCKER_REGISTRY}/healthz"

os::cmd::expect_success "dig @${DNS_SERVICE_IP} docker-registry.default.local. A"

os::log::info "Configure registry to disable mirroring"
os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
os::cmd::expect_success 'oc set env -n default dc/docker-registry REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_MIRRORPULLTHROUGH=false'
# this should not be needed but the DC is creating an RC w/a replicacount==0 for unknown reasons.
os::cmd::expect_success 'oc scale --replicas=1 replicationcontrollers docker-registry-1'

# this should not be necessary but "oc rollout status dc/docker-registry" returns a failure
# because it thinks the deployment is not progressing even though the RC reports availability.
sleep 60
#os::cmd::expect_success 'oc rollout status dc/docker-registry'
os::log::info "Registry configured to disable mirroring"

os::log::info "Verify that an image based on a remote image can be pushed to the same image stream while pull-through enabled."
os::cmd::expect_success "oc login -u user0 -p pass"
os::cmd::expect_success "oc new-project project0"
# An image stream will be created that will track the source image and the resulting image
# will be pushed to image stream "busybox:latest"
os::cmd::expect_success "oc new-build -D \$'FROM busybox:glibc\nRUN echo abc >/tmp/foo'"
os::cmd::try_until_text "oc get build/busybox-1 -o 'jsonpath={.metadata.name}'" "busybox-1" "$(( 10*TIME_MIN ))"
os::cmd::try_until_text "oc get build/busybox-1 -o 'jsonpath={.status.phase}'" '^(Complete|Failed|Error)$' "$(( 10*TIME_MIN ))"
os::cmd::expect_success_and_text "oc get build/busybox-1 -o 'jsonpath={.status.phase}'" 'Complete'
os::cmd::expect_success "oc get istag/busybox:latest"

os::log::info "Restore registry mirroring"
os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
os::cmd::expect_success 'oc set env -n default dc/docker-registry REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_MIRRORPULLTHROUGH=true'
# this should not be needed but the DC is creating an RC w/a replicacount==0 for unknown reasons.
os::cmd::expect_success 'oc scale --replicas=1 replicationcontrollers docker-registry-1'

# this should not be necessary but "oc rollout status dc/docker-registry" returns a failure
# because it thinks the deployment is not progressing even though the RC reports availability.
sleep 60
#os::cmd::expect_success 'oc rollout status dc/docker-registry'
os::log::info "Restore configured to enable mirroring"

registry_pod="$(oc get pod -n default -l deploymentconfig=docker-registry --template '{{range .items}}{{if not .metadata.deletionTimestamp}}{{.metadata.name}}{{end}}{{end}}')"

# Client setup (log in as e2e-user and set 'test' as the default project)
# This is required to be able to push to the registry!
os::log::info "Logging in as a regular user (e2e-user:pass) with project 'test'..."
os::cmd::expect_success 'oc login -u e2e-user -p pass'
os::cmd::expect_success_and_text 'oc whoami' 'e2e-user'

# check to make sure a project admin can push an image to an image stream that doesn't exist
os::cmd::expect_success 'oc project cache'
e2e_user_token="$(oc whoami -t)"

os::log::info "Docker login as e2e-user to ${DOCKER_REGISTRY}"
os::cmd::expect_success "docker login -u e2e-user -p ${e2e_user_token} ${DOCKER_REGISTRY}"
os::log::info "Docker login successful"

os::log::info "Tagging and pushing ruby-22-centos7 to ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::cmd::expect_success "docker pull centos/ruby-22-centos7:latest"
os::cmd::expect_success "docker tag centos/ruby-22-centos7:latest ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/ruby-22-centos7:latest"
os::log::info "Pushed ruby-22-centos7"

# get image's digest
rubyimagedigest="$(oc get -o jsonpath='{.status.tags[?(@.tag=="latest")].items[0].image}' is/ruby-22-centos7)"
os::log::info "Ruby image digest: ${rubyimagedigest}"
# get a random, non-empty blob
rubyimageblob="$(oc get isimage -o go-template='{{range .image.dockerImageLayers}}{{if gt .size 1024.}}{{.name}},{{end}}{{end}}' "ruby-22-centos7@${rubyimagedigest}" | cut -d , -f 1)"
os::log::info "Ruby's testing blob digest: ${rubyimageblob}"

# verify remote images can be pulled directly from the local registry
os::log::info "Docker pullthrough"
os::cmd::expect_success "oc import-image --confirm --from=mysql:latest mysql:pullthrough"
os::cmd::try_until_success 'oc get imagestreamtags mysql:pullthrough' $((2*TIME_MIN))
os::cmd::expect_success "docker pull ${DOCKER_REGISTRY}/cache/mysql:pullthrough"
mysqlblob="$(oc get istag -o go-template='{{range .image.dockerImageLayers}}{{if gt .size 4096.}}{{.name}},{{end}}{{end}}' "mysql:pullthrough" | cut -d , -f 1)"
# directly hit the image to trigger mirroring in case the layer already exists on disk
os::cmd::expect_success "curl -H 'Authorization: bearer $(oc whoami -t)' 'http://${DOCKER_REGISTRY}/v2/cache/mysql/blobs/${mysqlblob}' 1>/dev/null"
# verify the blob exists on disk in the registry due to mirroring under .../blobs/sha256/<2 char prefix>/<sha value>
os::cmd::try_until_success "oc exec --context='${CLUSTER_ADMIN_CONTEXT}' -n default '${registry_pod}' -- du /registry | tee '${LOG_DIR}/registry-images.txt' | grep '${mysqlblob:7:100}' | grep blobs"

os::log::info "Docker registry refuses manifest with missing dependencies"
os::cmd::expect_success 'oc new-project verify-manifest'
os::cmd::expect_success "curl \
    --location                                                                      \
    --header 'Content-Type: application/vnd.docker.distribution.manifest.v1+json'   \
    --user 'e2e_user:${e2e_user_token}'                                             \
    --output '${ARTIFACT_DIR}/rubyimagemanifest.json'                               \
    'http://${DOCKER_REGISTRY}/v2/cache/ruby-22-centos7/manifests/latest'"
os::cmd::expect_success "curl --head                                                \
    --silent                                                                        \
    --request PUT                                                                   \
    --header 'Content-Type: application/vnd.docker.distribution.manifest.v1+json'   \
    --user 'e2e_user:${e2e_user_token}'                                             \
    --upload-file '${ARTIFACT_DIR}/rubyimagemanifest.json'                          \
    'http://${DOCKER_REGISTRY}/v2/verify-manifest/ruby-22-centos7/manifests/latest' \
    >'${ARTIFACT_DIR}/curl-ruby-manifest-put.txt'"
os::cmd::expect_success_and_text "cat '${ARTIFACT_DIR}/curl-ruby-manifest-put.txt'" '400 Bad Request'
os::cmd::expect_success_and_text "cat '${ARTIFACT_DIR}/curl-ruby-manifest-put.txt'" '"errors":.*MANIFEST_BLOB_UNKNOWN'
os::cmd::expect_success_and_text "cat '${ARTIFACT_DIR}/curl-ruby-manifest-put.txt'" '"errors":.*blob unknown to registry'
os::log::info "Docker registry successfuly refused manifest with missing dependencies"

os::cmd::expect_success 'oc project cache'

IMAGE_PREFIX="${OS_IMAGE_PREFIX:-"openshift/origin"}"

os::log::info "Docker registry start with GCS"
os::cmd::expect_failure_and_text "docker run -e OPENSHIFT_DEFAULT_REGISTRY=localhost:5000 -e REGISTRY_STORAGE=\"gcs: {}\" ${IMAGE_PREFIX}-docker-registry:${TAG}" "No bucket parameter provided"

os::log::info "Docker registry start with OSS"
os::cmd::expect_failure_and_text "docker run -e OPENSHIFT_DEFAULT_REGISTRY=localhost:5000 -e REGISTRY_STORAGE=\"oss: {}\" ${IMAGE_PREFIX}-docker-registry:${TAG}" "No accesskeyid parameter provided"

os::log::info "Docker pull from istag"
os::cmd::expect_success "oc import-image --confirm --from=hello-world:latest -n test hello-world:pullthrough"
os::cmd::try_until_success 'oc get imagestreamtags hello-world:pullthrough -n test' $((2*TIME_MIN))
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
os::cmd::expect_failure_and_text "oc get --server='https://${KUBELET_HOST}:${KUBELET_PORT}' --insecure-skip-tls-verify --raw /spec/" "Forbidden"
os::test::junit::declare_suite_end


os::log::info "Fetch manifest V2 schema 2 image with old client using pullthrough"
os::cmd::expect_success "oc import-image --confirm --from=hello-world:latest hello-world:pullthrough"
os::cmd::try_until_success 'oc get imagestreamtags hello-world:pullthrough' $((2*TIME_MIN))
os::cmd::expect_success_and_text "oc get -o jsonpath='{.image.dockerImageManifestMediaType}' istag hello-world:pullthrough" 'application/vnd\.docker\.distribution\.manifest\.v2\+json'
hello_world_name="$(oc get -o 'jsonpath={.image.metadata.name}' istag hello-world:pullthrough)"
os::cmd::expect_success_and_text "echo '${hello_world_name}'" '.+'
# dockerImageManifest isn't retrievable only with "images" resource
hello_world_config_name="$(oc get -o 'jsonpath={.dockerImageManifest}' image "${hello_world_name}" --context="${CLUSTER_ADMIN_CONTEXT}" | jq -r '.config.digest')"
hello_world_config_image="$(oc get -o 'jsonpath={.image.dockerImageConfig}' istag hello-world:pullthrough | jq -r '.container_config.Image')"
os::cmd::expect_success_and_text "echo '${hello_world_config_name},${hello_world_config_image}'" '^,$'
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
os::cmd::expect_success "docker login -u e2e-user -p ${pusher_token} ${DOCKER_REGISTRY}"
os::log::info "Docker login successful"

os::log::info "Anonymous registry access"
# setup: log out of docker, log into openshift as e2e-user to run policy commands, tag image to use for push attempts
os::cmd::expect_success 'oc login -u e2e-user'
os::cmd::expect_success 'docker pull busybox'
os::cmd::expect_success "docker tag busybox ${DOCKER_REGISTRY}/missing/image:tag"
os::cmd::expect_success "docker logout ${DOCKER_REGISTRY}"
# unauthorized pulls return "not found" errors to anonymous users, regardless of backing data
os::cmd::expect_failure_and_text "docker pull ${DOCKER_REGISTRY}/missing/image:tag </dev/null" "not found|unauthorized|Cannot perform an interactive login"
os::cmd::expect_failure_and_text "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull </dev/null" "not found|unauthorized|Cannot perform an interactive login"
os::cmd::expect_failure_and_text "docker pull ${DOCKER_REGISTRY}/custom/cross:namespace-pull-id </dev/null" "not found|unauthorized|Cannot perform an interactive login"
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
os::log::info "Anonymous registry access successful"

# log back into docker as e2e-user again
os::cmd::expect_success "docker login -u e2e-user -p ${e2e_user_token} ${DOCKER_REGISTRY}"

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

# Image pruning
os::log::info "Validating image pruning"
# builder service account should have the power to create new image streams: prune in this case
os::cmd::expect_success "docker login -u e2e-user -p $(oc sa get-token builder -n cache) ${DOCKER_REGISTRY}"
os::cmd::expect_success 'docker pull busybox'
GCR_PAUSE_IMAGE="gcr.io/google_containers/pause:3.0"
os::cmd::expect_success "docker pull ${GCR_PAUSE_IMAGE}"
os::cmd::expect_success 'docker pull openshift/hello-openshift'

# tag and push 1st image - layers unique to this image will be pruned
os::cmd::expect_success "docker tag busybox ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# tag and push 2nd image - layers unique to this image will be pruned
os::cmd::expect_success "docker tag openshift/hello-openshift ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# tag and push 3rd image - it won't be pruned
os::cmd::expect_success "docker tag ${GCR_PAUSE_IMAGE} ${DOCKER_REGISTRY}/cache/prune"
os::cmd::expect_success "docker push ${DOCKER_REGISTRY}/cache/prune"

# record the storage before pruning
registry_pod="$(oc get pod -n default -l deploymentconfig=docker-registry --template '{{range .items}}{{if not .metadata.metadata.deletionTimestamp}}{{.metadata.name}}{{end}}{{end}}')"
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.before.txt'"

# set up pruner user
os::cmd::expect_success 'oc adm policy add-cluster-role-to-user system:image-pruner system:serviceaccount:cache:builder'
os::cmd::try_until_text 'oc adm policy who-can list images' 'system:serviceaccount:cache:builder'

# run image pruning
os::cmd::expect_success_and_not_text "oc adm prune images --token='$(oc sa get-token builder -n cache)' --keep-younger-than=0 --keep-tag-revisions=1 --confirm" 'error'

# record the storage after pruning
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.after.txt'"

# make sure there were changes to the registry's storage
os::cmd::expect_code "diff ${LOG_DIR}/prune-images.before.txt ${LOG_DIR}/prune-images.after.txt" 1

# prune a mirror, external image that is no longer referenced
os::cmd::expect_success "oc import-image fedora --confirm -n cache"
fedorablob="$(oc get istag -o go-template='{{range .image.dockerImageLayers}}{{if gt .size 4096.}}{{.name}},{{end}}{{end}}' "fedora:latest" -n cache | cut -d , -f 1)"
# directly hit the image to trigger mirroring in case the layer already exists on disk
os::cmd::expect_success "curl -H 'Authorization: bearer $(oc sa get-token builder -n cache)' 'http://${DOCKER_REGISTRY}/v2/cache/fedora/blobs/${fedorablob}' 1>/dev/null"
# verify the blob exists on disk in the registry due to mirroring under .../blobs/sha256/<2 char prefix>/<sha value>
os::cmd::try_until_success "oc exec --context='${CLUSTER_ADMIN_CONTEXT}' -n default -p '${registry_pod}' du /registry | tee '${LOG_DIR}/registry-images.txt' | grep '${fedorablob:7:100}' | grep blobs"
os::cmd::expect_success "oc delete is fedora -n cache"
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.before.txt'"
os::cmd::expect_success_and_not_text "oc adm prune images --token='$(oc sa get-token builder -n cache)' --keep-younger-than=0 --confirm --all --registry-url='${DOCKER_REGISTRY}'" 'error'
os::cmd::expect_failure "oc exec --context='${CLUSTER_ADMIN_CONTEXT}' -n default -p '${registry_pod}' du /registry | tee '${LOG_DIR}/registry-images.txt' | grep '${fedorablob:7:100}' | grep blobs"
os::cmd::expect_success "oc exec -p ${registry_pod} du /registry > '${LOG_DIR}/prune-images.after.txt'"
os::cmd::expect_code "diff '${LOG_DIR}/prune-images.before.txt' '${LOG_DIR}/prune-images.after.txt'" 1
os::log::info "Validated image pruning"

# with registry's re-deployment we loose all the blobs stored in its storage until now
os::log::info "Configure registry to accept manifest V2 schema 2"
os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
os::cmd::expect_success 'oc set env -n default dc/docker-registry REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ACCEPTSCHEMA2=true'
# this should not be needed but the DC is creating an RC w/a replicacount==0 for unknown reasons.
os::cmd::expect_success 'oc scale --replicas=1 replicationcontrollers docker-registry-1'

# this should not be necessary but "oc rollout status dc/docker-registry" returns a failure
# because it thinks the deployment is not progressing even though the RC reports availability.
sleep 60
#os::cmd::expect_success 'oc rollout status dc/docker-registry'
os::log::info "Registry configured to accept manifest V2 schema 2"

os::log::info "Accept manifest V2 schema 2"
os::cmd::expect_success "oc login -u schema2-user -p pass"
os::cmd::expect_success "oc project schema2"
# tagging remote docker.io/busybox image
os::cmd::expect_success "docker tag busybox '${DOCKER_REGISTRY}/schema2/busybox'"
os::cmd::expect_success "docker login -u e2e-user -p '${schema2_user_token}' '${DOCKER_REGISTRY}'"
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
busybox_calculated_size="$(oc get -o go-template='{{.dockerImageMetadata.Size}}' image "${busybox_name}" --context="${CLUSTER_ADMIN_CONTEXT}")"
os::cmd::expect_success_and_text "echo '${busybox_calculated_size}'" '^[1-9][0-9]*$'
os::log::info "Image size matches"

os::test::junit::declare_suite_end
unset VERBOSE
