#!/bin/bash
# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# For now pull down a pre-built 'kubectl' image that has the latest
# build. We need this version in order to pick-up some recent fixes.
# Once we have an official image we can use then we can delete this section
echo Downloading kubectl image...
docker pull duglin/kubectl:latest
docker tag duglin/kubectl:latest kubectl

exit 0

# This is the real code we'll keep.
# Build a 'kubectl' image using the binary from an official build

dir=tmp$RANDOM
mkdir $dir
cd $dir

curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/latest.txt)/bin/linux/amd64/kubectl
chmod +x kubectl

cat > Dockerfile <<EOF
FROM scratch
COPY kubectl /kubectl
ENTRYPOINT [ "/kubectl" ]
EOF

docker build -t kubectl

cd ..
rm -rf $dir

exit 0

# This section will build a new personal kubectl image
# We can delete it when we delete the top section
dir=tmp-kubectl
mkdir $dir
cd $dir
cp `which kubectl` .
cat > Dockerfile <<EOF
  FROM scratch
  COPY kubectl /
  ENTRYPOINT [ "/kubectl" ]
EOF
docker build -t kubectl .
cd ..
rm -rf $dir
exit 0

