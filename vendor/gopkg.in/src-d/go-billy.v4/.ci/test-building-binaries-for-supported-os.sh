#!/bin/bash

set -e

os_archs=(
    darwin/amd64
    freebsd/amd64
    linux/amd64
    solaris/amd64
    windows/amd64
)

for os_arch in "${os_archs[@]}"
do
    goos=${os_arch%/*}
    goarch=${os_arch#*/}
    echo "Building $goos/$goarch..."
    CGO_ENABLED=0 GOOS=${goos} GOARCH=${goarch} go build -o /dev/null ./...
done

echo "Succeeded building binaries for all supported OS/ARCH pairs!"