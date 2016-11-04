#!/bin/bash

# This script builds all images locally except the base and release images,
# which are handled by hack/build-base-images.sh.

# NOTE:  you only need to run this script if your code changes are part of
# any images OpenShift runs internally such as origin-sti-builder, origin-docker-builder,
# origin-deployer, etc.
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
source "${OS_ROOT}/contrib/node/install-sdn.sh"

if [[ "${OS_RELEASE:-}" == "n" ]]; then
  # Use local binaries
  imagedir="${OS_OUTPUT_BINPATH}/linux/amd64"
  # identical to build-cross.sh
  os::build::os_version_vars
  OS_RELEASE_COMMIT="${OS_GIT_VERSION//+/-}"
  OS_BUILD_PLATFORMS=("${OS_IMAGE_COMPILE_PLATFORMS[@]-}")

  echo "Building images from source ${OS_RELEASE_COMMIT}:"
  echo
  OS_GOFLAGS="${OS_GOFLAGS:-} ${OS_IMAGE_COMPILE_GOFLAGS}" os::build::build_static_binaries "${OS_IMAGE_COMPILE_TARGETS[@]-}" "${OS_SCRATCH_IMAGE_COMPILE_TARGETS[@]-}"
	os::build::place_bins "${OS_IMAGE_COMPILE_BINARIES[@]}"
  echo
else
  # Get the latest Linux release
  if [[ ! -d _output/local/releases ]]; then
    echo "No release has been built. Run hack/build-release.sh"
    exit 1
  fi

  # Extract the release achives to a staging area.
  os::build::detect_local_release_tars "linux-64bit"

  echo "Building images from release tars for commit ${OS_RELEASE_COMMIT}:"
  echo " primary: $(basename ${OS_PRIMARY_RELEASE_TAR})"
  echo " image:   $(basename ${OS_IMAGE_RELEASE_TAR})"

  imagedir="${OS_OUTPUT}/images"
  rm -rf ${imagedir}
  mkdir -p ${imagedir}
  os::build::extract_tar "${OS_PRIMARY_RELEASE_TAR}" "${imagedir}"
  os::build::extract_tar "${OS_IMAGE_RELEASE_TAR}" "${imagedir}"
fi

oc="$(os::build::find-binary oc ${OS_ROOT})"
if [[ -z "${oc}" ]]; then
  "${OS_ROOT}/hack/build-go.sh" cmd/oc
  oc="$(os::build::find-binary oc ${OS_ROOT})"
fi

function build() {
  eval "'${oc}' ex dockerbuild $2 $1 ${OS_BUILD_IMAGE_ARGS:-}"
}

# Create link to file if the FS supports hardlinks, otherwise copy the file
function ln_or_cp {
  local src_file=$1
  local dst_dir=$2
  if os::build::is_hardlink_supported "${dst_dir}" ; then
    ln -f "${src_file}" "${dst_dir}"
  else
    cp -pf "${src_file}" "${dst_dir}"
  fi
}

# Link or copy primary binaries to the appropriate locations.
ln_or_cp "${imagedir}/openshift" images/origin/bin

# Link or copy image binaries to the appropriate locations.
ln_or_cp "${imagedir}/pod"             images/pod/bin
ln_or_cp "${imagedir}/hello-openshift" examples/hello-openshift/bin
ln_or_cp "${imagedir}/deployment"      examples/deployment/bin
ln_or_cp "${imagedir}/gitserver"       examples/gitserver/bin
ln_or_cp "${imagedir}/oc"              examples/gitserver/bin
ln_or_cp "${imagedir}/dockerregistry"  images/dockerregistry/bin

# Copy SDN scripts into images/node
os::provision::install-sdn "${OS_ROOT}" "${imagedir}" "${OS_ROOT}/images/node"
mkdir -p images/node/conf/
cp -pf "${OS_ROOT}/contrib/systemd/openshift-sdn-ovs.conf" images/node/conf/

# builds an image and tags it two ways - with latest, and with the release tag
function image {
  local STARTTIME=$(date +%s)
  echo "--- $1 ---"
  build $1:latest $2
  #docker build -t $1:latest $2
  docker tag $1:latest $1:${OS_RELEASE_COMMIT}
  git clean -fdx $2
  local ENDTIME=$(date +%s); echo "--- $1 took $(($ENDTIME - $STARTTIME)) seconds ---"
  echo
  echo
}

# images that depend on scratch / centos
image openshift/origin-pod                   images/pod
image openshift/openvswitch                  images/openvswitch
# images that depend on openshift/origin-base
image openshift/origin                       images/origin
image openshift/origin-haproxy-router        images/router/haproxy
image openshift/origin-keepalived-ipfailover images/ipfailover/keepalived
image openshift/origin-docker-registry       images/dockerregistry
image openshift/origin-egress-router         images/router/egress
image openshift/origin-gitserver             examples/gitserver
# images that depend on openshift/origin
image openshift/origin-deployer              images/deployer
image openshift/origin-recycler              images/recycler
image openshift/origin-docker-builder        images/builder/docker/docker-builder
image openshift/origin-sti-builder           images/builder/docker/sti-builder
image openshift/origin-f5-router             images/router/f5
image openshift/node                         images/node

# extra images (not part of infrastructure)
image openshift/hello-openshift              examples/hello-openshift
docker build --no-cache -t openshift/deployment-example:v1 examples/deployment
docker build --no-cache -t openshift/deployment-example:v2 -f examples/deployment/Dockerfile.v2 examples/deployment

echo
echo
echo "++ Active images"

docker images | grep openshift/ | grep ${OS_RELEASE_COMMIT} | sort
echo

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
