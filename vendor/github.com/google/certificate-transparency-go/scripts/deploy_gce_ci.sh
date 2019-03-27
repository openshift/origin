#!/usr/bin/env bash
#set -o pipefail
#set -o errexit
#set -o nounset
#set -o xtrace

export PROJECT_NAME_CI=trillian-opensource-ci
export CLUSTER_NAME_CI=trillian-opensource-ci
export CLOUDSDK_COMPUTE_ZONE=us-central1-a


gcloud --quiet config set project ${PROJECT_NAME_CI}
gcloud --quiet config set container/cluster ${CLUSTER_NAME_CI}
gcloud --quiet config set compute/zone ${CLOUDSDK_COMPUTE_ZONE}
gcloud --quiet container clusters get-credentials ${CLUSTER_NAME_CI}

configmaps=$(kubectl get configmaps)
if [[ ! "${configmaps}" =~ "ctfe-configmap" ]]; then
  echo "Missing ctfe config map."
  echo
  echo "Ensure you have a PEM file containing all the roots your log should accept."
  echo "and a working CTFE configuration file, then create a CTFE configmap by"
  echo "running the following command:"
  echo "  kubectl create configmap ctfe-configmap \\"
  echo "     --from-file=roots=path/to/all-roots.pem \\"
  echo "     --from-file=ctfe-config-file=path/to/ct_server.cfg \\"
  echo "     --from-literal=cloud-project=${PROJECT_NAME_CI}"
  exit 1
fi


echo "Building docker images.."
cd $GOPATH/src/github.com/google/certificate-transparency-go
docker build --quiet -f trillian/examples/deployment/docker/ctfe/Dockerfile -t gcr.io/${PROJECT_NAME_CI}/ctfe:${TRAVIS_COMMIT} .

echo "Pushing docker image..."
gcloud docker -- push gcr.io/${PROJECT_NAME_CI}/ctfe:${TRAVIS_COMMIT}

echo "Tagging docker image..."
gcloud --quiet container images add-tag gcr.io/${PROJECT_NAME_CI}/ctfe:${TRAVIS_COMMIT} gcr.io/${PROJECT_NAME_CI}/ctfe:latest

echo "Updating jobs..."
envsubst < trillian/examples/deployment/kubernetes/ctfe-deployment.yaml | kubectl apply -f -
envsubst < trillian/examples/deployment/kubernetes/ctfe-service.yaml | kubectl apply -f -
envsubst < trillian/examples/deployment/kubernetes/ctfe-ingress.yaml | kubectl apply -f -
kubectl set image deployment/trillian-ctfe-deployment trillian-ctfe=gcr.io/${PROJECT_NAME_CI}/ctfe:${TRAVIS_COMMIT}
