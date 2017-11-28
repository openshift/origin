#!/bin/bash

set -e

glide update --strip-vendor

find vendor/ -name *.proto | xargs sed -i '/k8s.io\/apiextensions-apiserver\/pkg\/apis\/apiextensions\/v1beta1\/generated.proto/d'