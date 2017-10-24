#!/bin/bash -xe
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

HELM_URL=https://storage.googleapis.com/kubernetes-helm
HELM_TARBALL=helm-v2.6.2-linux-amd64.tar.gz
SVC_CATALOG_BUCKET=svc-catalog-charts
SVC_CATALOG_REPO_URL=https://$SVC_CATALOG_BUCKET.storage.googleapis.com/

# Setup Helm
wget -q ${HELM_URL}/${HELM_TARBALL}
tar xzfv ${HELM_TARBALL}
PATH=`pwd`/linux-amd64/:$PATH
helm init --client-only

# Authenticate before uploading to Google Cloud Storage
cat > sa.json <<EOF
$SERVICE_ACCOUNT_JSON
EOF
gcloud auth activate-service-account --key-file sa.json

# Create the repository
SVC_CATALOG_REPO_DIR=svc-catalog-repo
mkdir -p ${SVC_CATALOG_REPO_DIR}
cd ${SVC_CATALOG_REPO_DIR}
  gsutil cp gs://$SVC_CATALOG_BUCKET/index.yaml .
  for dir in `ls ../charts`;do
    helm dep build ../charts/$dir
    helm package ../charts/$dir
  done
  helm repo index --url ${SVC_CATALOG_REPO_URL} --merge ./index.yaml .
  gsutil -m rsync ./ gs://$SVC_CATALOG_BUCKET/
cd ..
ls -l ${SVC_CATALOG_REPO_DIR}
