#!/bin/bash

# This script pulls down example files (eg templates) from external repositories
# so they can be included directly in our repository.
# Feeds off a README.md file with well defined syntax that informs this
# script how to pull the file down.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

IMAGESTREAMS_DIR="${OS_ROOT}/examples/image-streams"
(
  cd "${IMAGESTREAMS_DIR}"

  rm -vf *.{json,yaml,yml}

  # Assume the README.md file contains lines with URLs for the raw json/yaml file to be downloaded.
  # Specifically look for a line containing https://raw.githubusercontent.com, then
  # look for the first content in ()s on that line, which will be the actual url of the file,
  # then use curl to pull that file down.
  curl -# $(grep -E '\(https://raw.githubusercontent.com.*centos7.*\)' README.md | sed -E 's/.*\((.*)\)/\1 -O/')
  jq . -s *.json | jq '. | {"kind": "ImageStreamList","apiVersion": "v1", "items": .}' > images-centos.tmp
  rm -vf *.{json,yaml,yml}

  curl -# $(grep -E '\(https://raw.githubusercontent.com.*rhel7.*\)' README.md | sed -E 's/.*\((.*)\)/\1 -O/')
  jq . -s *.json | jq '. | {"kind": "ImageStreamList","apiVersion": "v1", "items": .}' > images-rhel.tmp
  rm -vf *.{json,yaml,yml}

  mv images-centos.tmp image-streams-centos7.json
  mv images-rhel.tmp image-streams-rhel7.json

)

QUICKSTARTS_DIR="${OS_ROOT}/examples/quickstarts"
(
  cd "${QUICKSTARTS_DIR}"

  rm -vf *.{json,yaml,yml}

  # Assume the README.md file contains lines with URLs for the raw json/yaml file to be downloaded.
  # Specifically look for a line containing https://raw.githubusercontent.com, then
  # look for the first content in ()s on that line, which will be the actual url of the file,
  # then use curl to pull that file down.
  curl -# $(grep -E '\(https://raw.githubusercontent.com.*\)' README.md | sed -E 's/.*\((.*)\) -.*/\1 -O/')
  rename -- '-example' '' *.json
  
  # rename templates that have a different name in openshift/library from what we had been naming
  # them in origin.  (openshift/library names files based on the template.name field, it would
  # be better to fix the template.name but that would break compatibility)
  mv django-psql-persistent.json django-postgresql-persistent.json
  mv django-psql.json django-postgresql.json
  mv rails-pgsql-persistent.json rails-postgresql-persistent.json
  mv nodejs-mongo-persistent.json nodejs-mongodb-persistent.json


)

DB_EXAMPLES_DIR="${OS_ROOT}/examples/db-templates"
(
  cd "${DB_EXAMPLES_DIR}"

  rm -vf *.{json,yaml,yml}

  # Assume the README.md file contains lines with URLs for the raw json/yaml file to be downloaded.
  # Specifically look for a line containing (https://raw.githubusercontent.com.*), then
  # look for the first content in ()s on that line, which will be the actual url of the file,
  # then use curl to pull that file down.
  curl -# $(grep -E '\(https://raw.githubusercontent.com.*\)' README.md | sed -E 's/.*\((.*)\) -.*/\1 -O/')
  rename -- '.json' '-template.json' *.json
)

JENKINS_EXAMPLES_DIR="${OS_ROOT}/examples/jenkins"
(
  cd "${JENKINS_EXAMPLES_DIR}"

  rm -vf jenkins-*.json

  curl -# https://raw.githubusercontent.com/openshift/library/master/official/jenkins/templates/jenkins-ephemeral.json -O
  curl -# https://raw.githubusercontent.com/openshift/library/master/official/jenkins/templates/jenkins-persistent.json -O
  rename -- '.json' '-template.json' jenkins-*.json
)
