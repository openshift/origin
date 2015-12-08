#!/bin/bash

set -e
set -o pipefail

if [ ! -e "${DOCKER_SOCKET}" ]; then
  echo "Docker socket missing at ${DOCKER_SOCKET}"
  exit 1
fi

SECRET_PATH=${PUSH_DOCKERCFG_PATH:-}

if [ -z "${SECRET_PATH}" ]; then
  echo "The dockercfg not found in /var/run/secrets/openshift.io/push"
  exit 1
fi

if [ -n "${OUTPUT_IMAGE}" ]; then
  TAG="${OUTPUT_REGISTRY}/${OUTPUT_IMAGE}"
fi

mkdir -p /tmp/build && cd /tmp/build
cp -v $SECRET_PATH /tmp/build/dockercfg
chmod 0666 /tmp/build/dockercfg

# This ruby app just output content of file referenced by the environment
# variable. For example FOO=/tmp/test and then GET /FOO returns content of
# /tmp/test
cat > config.ru <<- EOF
def readfile(name); File.read(ENV[name]) rescue "not found #{ENV[name]}"; end
run Proc.new { |env|
  path = env['PATH_INFO'].gsub(/^\//,'')
  [200, {"Content-Type" => "text/raw"}, [readfile(path)]]
}
EOF

cat > Dockerfile <<- EOF
FROM centos/ruby-22-centos7
ENV SECRET_FILE /opt/openshift/src/dockercfg
COPY dockercfg ./
COPY config.ru ./
CMD /usr/local/sti/run
EOF

docker build --rm -t "${TAG}" .
docker push "${TAG}"
