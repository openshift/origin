package templates

const TestRunScript = `
#!/bin/bash
#
# The 'run' performs a simple test that verifies that STI image.
# The main focus here is to excersise the STI scripts.
#
# For more informations see the documentation:
#	https://github.com/openshift/source-to-image/blob/master/docs/builder_image.md
#
# IMAGE_NAME specifies a name of the candidate image used for testing.
# The image has to be available before this script is executed.
#
IMAGE_NAME=${IMAGE_NAME-{{.ImageName}}-candidate}

test_dir="$(readlink -zf $(dirname "${BASH_SOURCE[0]}"))"
image_dir=$(readlink -zf ${test_dir}/..)
scripts_url="file://${image_dir}/.sti/bin"
cid_file=$(mktemp -u --suffix=.cid)

# Since we built the candidate image locally, we don't want STI attempt to pull
# it from Docker hub
sti_args="--forcePull=false -s ${scripts_url}"

# TODO: This should be part of the image metadata
test_port=8000

image_exists() {
  docker inspect $1 &>/dev/null
}

container_exists() {
  image_exists $(cat $cid_file)
}

container_ip() {
  docker inspect --format="\{\{ .NetworkSettings.IPAddress \}\}" $(cat $cid_file)
}

run_sti_build() {
  sti build ${sti_args} file://${test_dir}/test-app ${IMAGE_NAME} ${IMAGE_NAME}-testapp
}

prepare() {
  if ! image_exists ${IMAGE_NAME}; then
    echo "ERROR: The image ${IMAGE_NAME} must exist before this script is executed."
    exit 1
  fi
  # TODO: STI build require the application is a valid 'GIT' repository, we
  # should remove this restriction in the future when a file:// is used.
  pushd ${test_dir}/test-app >/dev/null
  git init
  git config user.email "build@localhost" && git config user.name "builder"
  git add -A && git commit -m "Sample commit"
  popd >/dev/null
  run_sti_build
}

run_test_application() {
  docker run --rm --cidfile=${cid_file} -p ${test_port} ${IMAGE_NAME}-testapp
}

cleanup() {
  if [ -f $cid_file ]; then
    if container_exists; then
      docker stop $(cat $cid_file)
    fi
  fi
  if image_exists ${IMAGE_NAME}-testapp; then
    docker rmi ${IMAGE_NAME}-testapp
  fi
  rm -rf ${test_dir}/test-app/.git
}

check_result() {
  local result="$1"
  if [[ "$result" != "0" ]]; then
    echo "STI image '${IMAGE_NAME}' test FAILED (exit code: ${result})"
    cleanup
    exit $result
  fi
}

wait_for_cid() {
  local max_attempts=10
  local sleep_time=1
  local attempt=1
  local result=1
  while [ $attempt -le $max_attempts ]; do
    [ -f $cid_file ] && break
    echo "Waiting for container start..."
    attempt=$(( $attempt + 1 ))
    sleep $sleep_time
  done
}

test_usage() {
  echo "Testing 'sti usage'..."
  sti usage ${sti_args} ${IMAGE_NAME} &>/dev/null
}

test_connection() {
  echo "Testing HTTP connection..."
  local max_attempts=10
  local sleep_time=1
  local attempt=1
  local result=1
  while [ $attempt -le $max_attempts ]; do
    echo "Sending GET request to http://$(container_ip):${test_port}/"
    response_code=$(curl -s -w %{http_code} -o /dev/null http://$(container_ip):${test_port}/)
    status=$?
    if [ $status -eq 0 ]; then
      if [ $response_code -eq 200 ]; then
        result=0
      fi
      break
    fi
    attempt=$(( $attempt + 1 ))
    sleep $sleep_time
  done
  return $result
}

# Build the application image twice to ensure the 'save-artifacts' and
# 'restore-artifacts' scripts are working properly
prepare
run_sti_build
check_result $?

# Verify the 'usage' script is working properly
test_usage
check_result $?

# Verify that the HTTP connection can be established to test application container
run_test_application &

# Wait for the container to write it's CID file
wait_for_cid

test_connection
check_result $?

cleanup

`

const Makefile = `
IMAGE_NAME = {{.ImageName}}

build:
	docker build -t $(IMAGE_NAME) .

.PHONY: test
test:
	docker build -t $(IMAGE_NAME)-candidate .
	IMAGE_NAME=$(IMAGE_NAME)-candidate test/run
`
