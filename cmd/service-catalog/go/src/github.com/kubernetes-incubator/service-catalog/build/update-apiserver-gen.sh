#!/bin/bash

# Copyright 2018 The Kubernetes Authors.
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

# The only argument this script should ever be called with is '--verify-only'

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

REPO_ROOT=$(realpath $(dirname "${BASH_SOURCE}")/..)
BINDIR=${REPO_ROOT}/bin
SC_PKG='github.com/kubernetes-incubator/service-catalog'

# Generate defaults
${BINDIR}/defaulter-gen "$@" \
	 --v 1 --logtostderr \
	 --go-header-file "vendor/github.com/kubernetes/repo-infra/verify/boilerplate/boilerplate.go.txt" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog/v1beta1" \
	 --extra-peer-dirs "${SC_PKG}/pkg/apis/servicecatalog" \
	 --extra-peer-dirs "${SC_PKG}/pkg/apis/servicecatalog/v1beta1" \
	 --output-file-base "zz_generated.defaults"
# Generate deep copies
${BINDIR}/deepcopy-gen "$@" \
	 --v 1 --logtostderr \
	 --go-header-file "vendor/github.com/kubernetes/repo-infra/verify/boilerplate/boilerplate.go.txt" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog/v1beta1" \
	 --bounding-dirs "github.com/kubernetes-incubator/service-catalog" \
	 --output-file-base zz_generated.deepcopy
# Generate conversions
${BINDIR}/conversion-gen "$@" \
	 --v 1 --logtostderr \
	 --extra-peer-dirs k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
	 --go-header-file "vendor/github.com/kubernetes/repo-infra/verify/boilerplate/boilerplate.go.txt" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog/v1beta1" \
	 --output-file-base zz_generated.conversion
# generate openapi
${BINDIR}/openapi-gen "$@" \
	 --v 1 --logtostderr \
	 --go-header-file "vendor/github.com/kubernetes/repo-infra/verify/boilerplate/boilerplate.go.txt" \
	 --input-dirs "${SC_PKG}/pkg/apis/servicecatalog/v1beta1,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/version,k8s.io/apimachinery/pkg/runtime" \
	 --output-package "${SC_PKG}/pkg/openapi"
