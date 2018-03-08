# Copyright 2016 The Kubernetes Authors.
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

FROM golang:GO_VERSION

# Install dep as root
ENV DEP_VERSION=v0.4.1
RUN curl -sSL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/$DEP_VERSION/dep-linux-amd64 && \
    chmod +x /usr/local/bin/dep

# Install etcd
RUN curl -sSL https://github.com/coreos/etcd/releases/download/v3.1.10/etcd-v3.1.10-linux-amd64.tar.gz \
    | tar -vxz -C /usr/local/bin --strip=1 etcd-v3.1.10-linux-amd64/etcd

# Install the golint, use this to check our source for niceness
RUN go get -u github.com/golang/lint/golint

# Install the href checker for our md files, update PATH to include it
RUN git clone https://github.com/duglin/vlinker.git /vlinker
ENV PATH=$PATH:/vlinker/bin

# Install the azure client, used to publish svcat binaries
ENV AZCLI_VERSION=2.0.25
RUN apt-get update && apt-get install -y python-pip && \
    rm -rf /var/lib/apt/lists/*
RUN pip install --disable-pip-version-check --no-cache-dir --upgrade cryptography azure-cli==${AZCLI_VERSION}

# Create the full dir tree that we'll mount our src into when we run the image
RUN mkdir -p /go/src/github.com/kubernetes-incubator/service-catalog

# Default to our src dir
WORKDIR /go/src/github.com/kubernetes-incubator/service-catalog
