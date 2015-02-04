#!/bin/bash

# The source_dir is the last segment from repository URL
source_dir=$(echo $SOURCE_URI | grep -o -e "[^/]*$" | sed -e "s/\.git$//")

result=1

if [ -z "${IMAGE_NAME}" ]; then
  echo "[ERROR] The IMAGE_NAME environment variable must be set"
  exit $result
fi

# Clone the STI image repository
git clone $SOURCE_URI
if ! [ $? -eq 0 ]; then
  echo "[ERROR] Unable to clone the STI image repository."
  exit $result
fi

# If the STI image Dockerfile does not exists in the root of the repository,
# you can specify the CONTEXT_DIR.
context_dir=${CONTEXT_DIR:-"."}

pushd "${source_dir}/${context_dir}" >/dev/null
  # Checkout desired ref
  if ! [ -z "$SOURCE_REF" ]; then
    git checkout $SOURCE_REF
  fi

  docker build -t ${IMAGE_NAME}-candidate .
  result=$?
  if ! [ $result -eq 0 ]; then
    echo "[ERROR] Unable to build ${IMAGE_NAME}-candidate image (${result})"
  fi

  # Verify the 'test/run' is present
  if ! [ -x "./test/run" ]; then
    echo "[ERROR] Unable to locate the 'test/run' command for the image"
    exit 1
  fi

  # Execute tests
  IMAGE_NAME=${IMAGE_NAME}-candidate ./test/run
  result=$?
  if [ $result -eq 0 ]; then
    echo "[SUCCESS] ${IMAGE_NAME} image tests executed successfully"
  else
    echo "[FAILURE] ${IMAGE_NAME} image tests failed ($result)"
    exit $result
  fi
popd >/dev/null

if ! [ -z "${OUTPUT_REGISTRY}" ]; then
  image_id=$(docker inspect --format="{{ .Id }}" "${IMAGE_NAME}-candidate:latest")
  echo "Pushing the "${OUTPUT_REGISTRY}/${OUTPUT_IMAGE}" image..."
  OUTPUT_IMAGE="${OUTPUT_IMAGE:-$IMAGE_NAME}"
  set -x
  docker tag ${image_id} "${OUTPUT_REGISTRY}/${OUTPUT_IMAGE}"
  docker push "${OUTPUT_REGISTRY}/${OUTPUT_IMAGE}"
  set +x
fi

# Cleanup the temporary Docker image
docker rmi "${IMAGE_NAME}-candidate"
