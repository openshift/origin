#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Cleanup cluster resources created by this test
(
  set +e
  oc delete images test
  exit 0
) 2>/dev/null 1>&2

defaultimage="openshift/origin-\${component}:latest"
USE_IMAGES=${USE_IMAGES:-$defaultimage}

# This test validates images and image streams along with the tag and import-image commands

oc get images
oc create -f test/integration/fixtures/test-image.json
oc delete images test
echo "images: ok"

oc get imageStreams
oc create -f test/integration/fixtures/test-image-stream.json
# verify that creating a registry fills out .status.dockerImageRepository
if [ -z "$(oc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]; then
  # create the registry
  oadm registry --credentials="${KUBECONFIG}" --images="${USE_IMAGES}" -n default
  # make sure stream.status.dockerImageRepository IS set
  [ -n "$(oc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]
fi
oc delete imageStreams test
[ -z "$(oc get imageStreams test -t "{{.status.dockerImageRepository}}")" ]

oc create -f examples/image-streams/image-streams-centos7.json
[ -n "$(oc get imageStreams ruby -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams nodejs -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams wildfly -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams mysql -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams postgresql -t "{{.status.dockerImageRepository}}")" ]
[ -n "$(oc get imageStreams mongodb -t "{{.status.dockerImageRepository}}")" ]
# verify the image repository had its tags populated
tryuntil oc get imagestreamtags wildfly:latest
[ -n "$(oc get imageStreams wildfly -t "{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}")" ]
oc delete imageStreams ruby
oc delete imageStreams nodejs
oc delete imageStreams wildfly
#oc delete imageStreams mysql
oc delete imageStreams postgresql
oc delete imageStreams mongodb
[ -z "$(oc get imageStreams ruby -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams nodejs -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams postgresql -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams mongodb -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams wildfly -t "{{.status.dockerImageRepository}}")" ]
tryuntil oc get imagestreamTags mysql:latest
[ -n "$(oc get imagestreams mysql -t "{{ index .metadata.annotations \"openshift.io/image.dockerRepositoryCheck\"}}")" ]
oc describe istag/mysql:latest
[ "$(oc describe istag/mysql:latest | grep "Environment:")" ]
[ "$(oc describe istag/mysql:latest | grep "Image Created:")" ]
[ "$(oc describe istag/mysql:latest | grep "Image Name:")" ]
name=$(oc get istag/mysql:latest -t '{{ .image.metadata.name }}')
imagename="isimage/mysql@${name:0:7}"
oc describe "${imagename}"
[ "$(oc describe ${imagename} | grep "Environment:")" ]
[ "$(oc describe ${imagename} | grep "Image Created:")" ]
[ "$(oc describe ${imagename} | grep "Image Name:")" ]
echo "imageStreams: ok"

[ ! "$(oc import-image mysql --from=mysql)" ]
[ "$(oc import-image mysql --from=mysql --confirm | grep "sha256:")" ]
oc describe is/mysql
echo "import-image: ok"

# oc tag
oc tag mysql:latest tagtest:tag1
[ "$(oc get is/tagtest -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]

oc tag mysql@${name} tagtest:tag2
[ "$(oc get is/tagtest -t '{{(index .spec.tags 1).from.kind}}')" == "ImageStreamImage" ]

oc tag mysql:notfound tagtest:tag3
[ "$(oc get is/tagtest -t '{{(index .spec.tags 2).from.kind}}')" == "ImageStreamTag" ]

oc tag --source=imagestreamtag mysql:latest tagtest:tag4
[ "$(oc get is/tagtest -t '{{(index .spec.tags 3).from.kind}}')" == "ImageStreamTag" ]

oc tag --source=istag mysql:latest tagtest:tag5
[ "$(oc get is/tagtest -t '{{(index .spec.tags 4).from.kind}}')" == "ImageStreamTag" ]

oc tag --source=imagestreamimage mysql@${name} tagtest:tag6
[ "$(oc get is/tagtest -t '{{(index .spec.tags 5).from.kind}}')" == "ImageStreamImage" ]

oc tag --source=isimage mysql@${name} tagtest:tag7
[ "$(oc get is/tagtest -t '{{(index .spec.tags 6).from.kind}}')" == "ImageStreamImage" ]

oc tag --source=docker mysql:latest tagtest:tag8
[ "$(oc get is/tagtest -t '{{(index .spec.tags 7).from.kind}}')" == "DockerImage" ]

oc tag mysql:latest tagtest:zzz tagtest2:zzz
[ "$(oc get is/tagtest -t '{{(index .spec.tags 8).from.kind}}')" == "ImageStreamTag" ]
[ "$(oc get is/tagtest2 -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]

# TODO: bug
# oc tag registry-1.docker.io/openshift/origin:v1.0.4 newrepo:latest

# test creating streams that don't exist
[ -z "$(oc get imageStreams tagtest3 -t "{{.status.dockerImageRepository}}")" ]
[ -z "$(oc get imageStreams tagtest4 -t "{{.status.dockerImageRepository}}")" ]
oc tag mysql:latest tagtest3:latest tagtest4:latest
[ "$(oc get is/tagtest3 -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]
[ "$(oc get is/tagtest4 -t '{{(index .spec.tags 0).from.kind}}')" == "ImageStreamTag" ]

oc delete is/tagtest is/tagtest2 is/tagtest3 is/tagtest4
echo "tag: ok"
