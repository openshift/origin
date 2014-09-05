#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

function usage {
  cat << EOF
usage: $0 [OPTIONS]

OPTIONS:
    -l    launch a local cluster following a successful build
    -c    reuse existing build images
    -w    watch cluster logs when launching
    -n    number of nodes to launch (default: 1)
    -d    local GOPATH packages (comma delimited) to override Godeps during build
    -h    show help
EOF

exit 1
}

function build {
  if $use_cached_builder; then
    echo "Using cached origin-build image"
  else
    echo "Making origin-build image..."
    time build-origin-build
  fi

  echo "Building local source into origin-build container..."
  local cmd=""
  cmd+="docker run --name origin-build"
  cmd+=" -v $(pwd):/go/src/github.com/openshift/origin:ro"

  if [ ${#dep_overrides[@]} -gt 0 ]; then
    for d in ${dep_overrides[@]:()}; do
      cmd+=" -v $GOPATH/src/${d}:/go/src/${d}:ro"
    done
  fi

  cmd+=" -e GOPATH=/go:/go/src/github.com/openshift/origin/Godeps/_workspace"
  cmd+=" openshift/origin-build go build -o /origin/openshift ./cmd/openshift"
  echo "executing cmd: $cmd"

  if ! time $(eval "$cmd") ; then
    cleanup "origin-build"
    exit 1
  fi 
}

function build-origin-build {
  pushd hack/origin-build &>/dev/null
  echo "Building openshift/origin-build"
  docker build -q -t openshift/origin-build .
  popd &>/dev/null
}

function ip-of {
  local ip=$(docker inspect -f "{{ .NetworkSettings.IPAddress }}" $1 | tr -d '\n')
  echo $ip
}

function launch-cluster {
  trap cleanup EXIT

  echo "Launching an OpenShift cluster with ${#nodes[@]} nodes"

  local cid=""

  echo "Creating master"
  cid=$(
  docker run \
    --name origin-master \
    --detach \
    --volumes-from origin-build \
    --workdir /tmp \
    openshift/origin-build \
    bash -c "OPENSHIFT_BIND_ADDR=\$(hostname -i) OPENSHIFT_MASTER=\$(hostname -i) /origin/openshift start"
  ) >/dev/null
  local master_ip=$(ip-of $cid)

  for node_name in ${nodes[@]}; do
    echo "Creating node ${node_name}"
    cid=$(
    docker run \
      --name ${node_name} \
      --detach \
      --volumes-from origin-build \
      --privileged \
      --workdir /tmp \
      --env OPENSHIFT_MASTER=${master_ip} \
      openshift/origin-build \
      bash -c "/usr/bin/dind ; OPENSHIFT_BIND_ADDR=\$(hostname -i) /origin/openshift start"
    ) >/dev/null
    local node_ip=$(ip-of ${cid})

    register-node ${node_name} ${node_ip} ${master_ip}
  done

  echo "Cluster started. Press Ctrl-C to stop..."

  if $watch_logs; then
    echo -e "\nWatching cluster logs..."
    watch-logs
  else
    echo -e "\nWatch logs with the following commands:"
    for c in "${containers[@]}"; do
      echo "  docker logs -f ${c}"
    done
  fi

  while true; do read x; done
}

function register-node {
  local node_name=$1
  local node_ip=$2
  local master_ip=$3
 
  for i in {1..10}; do
    echo "Registering node ${node_name} (${node_ip}) with master ${master_ip} (attempt ${i} of 10)"

    set +e
    docker run --rm -t openshift/origin-build http --timeout 2 --check-status POST ${master_ip}:8080/api/v1beta1/minions kind=Minion id=${node_ip} apiVersion=v1beta1 hostIP=${node_ip} >/dev/null
    set -e

    if [ $? == 0 ]; then
      echo "Node ${node_name} is registered with master"
      break
    else
      sleep 1
    fi 
  done
}

function cleanup {
  local target=${1:-}

  echo -e "\nCleaning up..."
  
  local containers=()

  if [ ! -z ${target} ]; then
    containers+=(${target})
  else
    for node in ${nodes[@]}; do containers+=(${node}); done
    containers+=(origin-master origin-build)
  fi

  for c in ${containers[@]}; do
    echo " --> killing container: ${c}"
    time docker kill $c >/dev/null || true
    echo " --> removing container: ${c}"
    time docker rm -f $c >/dev/null || true
  done

  echo "Done cleaning up"
}

function watch-logs {
  docker logs -f origin-master &
  for node in ${nodes[@]}; do
    docker logs -f ${node} &
  done
}

use_cached_builder=false
launch_after_build=false
watch_logs=false
preserve_build=false
dep_overrides=()
node_count=1
nodes=()

while getopts "lcwhpd:n:" o; do
  case "${o}" in
    l) launch_after_build=true;;
    c) use_cached_builder=true;;
    w) watch_logs=true;;
    d) IFS=, read -a dep_overrides <<< "$OPTARG";;
    p) preserve_build=true;;
    n) node_count=$OPTARG;;
    h) usage;;
    *) usage;;
  esac
done

TIMEFORMAT=%R

for i in $(seq 1 $node_count); do
  nodes+=(origin-node-${i})
done

if [ ${#dep_overrides[@]} -gt 0 ]; then
  echo "Overriding packages host GOPATH: ${dep_overrides[@]}"
fi

build

if $launch_after_build; then
  launch-cluster
else
  if ! $preserve_build; then
    cleanup "origin-build"
  fi
fi

