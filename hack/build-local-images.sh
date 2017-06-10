#!/bin/sh -e
# This script updates infrastructure images with a new "openshift" binary from your build workspace.
# Only infrastructure images that run the openshift binary (directly or via symlink) are updated.
# After running this, you can use the new images via "oc cluster up --version=latest" or
# "openshift start --latest-images=true"
# You can filter which images are built by passing one or more image name arguments, e.g.:
# "hack/build-local-images.sh docker-builder sti-builder"
# Only exact matching of image names (minus the ose/origin prefix) is supported.

#imagenames=(docker-registry origin deployer recycler docker-builder                sti-builder                f5-router node openvswitch docker-registry cluster-capacity federation)
#imagedirs=( dockerregistry  origin deployer recycler builder/docker/docker-builder builder/docker/sti-builder router/f5 node openvswitch docker-registry cluster-capacity federation)
imagenames=(origin deployer recycler docker-builder                sti-builder                f5-router node openvswitch)
imagedirs=( origin deployer recycler builder/docker/docker-builder builder/docker/sti-builder router/f5 node openvswitch)

count=${#imagenames[@]}
crc_count=${#imagedirs[@]}
if [ $count -ne $crc_count ]; then
    echo "imagenames and imagedirs arrays are different sizes, they need to be parallel arrays."
    echo "$count $crc_count"
    exit 1
fi

# reuse the same bin directory for all the docker builds
# to save time copying stuff around.
BIN_TMP=`mktemp -d`
echo "Staging bin files in $BIN_TMP"
#mkdir -p $BIN_TMP/bin
#cp -a `pwd`/_output/local/bin/linux/amd64/* $BIN_TMP/bin
ln -s `pwd`/_output/local/bin/linux/amd64 $BIN_TMP/bin

pushd $BIN_TMP/bin

trap cleanup exit
function cleanup {
    popd
    rm -rf $BIN_TMP    
}

c=-1
while [ $c -lt $((count-1)) ]; do 
    c=$((c+1))
    imagename=${imagenames[$c]}
    imagedir=${imagedirs[$c]}
    tag_prefix="openshift/origin-"
    if [[ $imagename == "origin" || $imagename == "node" || $imagename ==  "openvswitch" ]]; then
        tag_prefix="openshift/"
    fi


    # for most images we are replacing the openshift binary, but for the
    # docker registry image, we're replacing the dockerregistry binary.
    # similar hacks for other images.
    filename="openshift"
    #if [ $imagename == "docker-registry" ]; then
    #    filename="dockerregistry"        
    #fi

    #if [ $imagename == "federation" ]; then
    #    filename="hyperkube"
    #fi

    #if [ $imagename == "cluster-capacity" ]; then
    #    filename="cluster-capacity"
    #fi

    echo "Considering building image ${tag_prefix}${imagename} from dir ${imagedir}...."

    build=true

    # if arguments were supplied, only build the images with names matching the
    # arguments provided.
    if [ $# -gt 0 ]; then
        build=false
        for arg in $@; do
            if [ $arg == $imagename ]; then
                build=true
            fi
            # found a match, no need to check any more arguments
            if [ $build == "true" ]; then
                break
            fi
        done
    fi

    # if arguments were supplied and we didn't match this imagename against
    # one of them, don't build it.
    if [ $build == "false" ]; then
        echo "Skipping it."
        continue
    fi
    echo "Building it."
    echo "#########################################"


    # write out a new dockerfile that will base this image on the existing
    # one, and redo any COPY/ADD statements with the local content.
    echo -e "FROM ${tag_prefix}${imagename}\nCOPY ${filename} /usr/bin/${filename}\n" > Dockerfile
    docker build -t ${tag_prefix}${imagename} -f Dockerfile .
    echo "Done building ${tag_prefix}${imagename}"
    echo "#########################################"
done
