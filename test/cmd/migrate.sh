#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/migrate"
# This test validates storage migration

os::cmd::expect_success 'oc login -u system:admin'

os::test::junit::declare_suite_start "cmd/migrate/storage"
os::cmd::expect_success_and_text     'oadm migrate storage' 'summary \(dry run\)'
os::cmd::expect_success_and_text     'oadm migrate storage --loglevel=2' 'migrated \(dry run\): serviceaccounts/deployer'
os::cmd::expect_success_and_not_text 'oadm migrate storage --loglevel=2 --include=pods' 'migrated \(dry run\): serviceaccounts/deployer'
os::cmd::expect_success_and_text     'oadm migrate storage --loglevel=2 --include=sa --from-key=default/ --to-key=default/\xFF' 'migrated \(dry run\): serviceaccounts/deployer'
os::cmd::expect_success_and_not_text 'oadm migrate storage --loglevel=2 --include=sa --from-key=default/ --to-key=default/deployer' 'migrated \(dry run\): serviceaccounts/deployer'
os::cmd::expect_success_and_text     'oadm migrate storage --loglevel=2 --confirm' 'unchanged:'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/migrate/imagereferences"
# create alternating items in history
os::cmd::expect_success 'oc import-image --from=mysql:latest test:1 --confirm'
os::cmd::expect_success 'oc import-image --from=php:latest test:2 --confirm'
os::cmd::expect_success 'oc tag --source=docker php:latest test:1'
os::cmd::expect_success 'oc tag --source=docker mysql:latest test:1'
os::cmd::expect_success 'oc tag --source=docker mysql:latest test:2'
os::cmd::expect_success 'oc tag --source=docker php:latest test:2'
os::cmd::expect_success 'oc tag --source=docker myregistry.com/php:latest test:3'
# verify error cases
os::cmd::expect_failure_and_text     'oadm migrate image-references' 'at least one mapping argument must be specified: REGISTRY/NAME=REGISTRY/NAME'
os::cmd::expect_failure_and_text     'oadm migrate image-references my.docker.io=docker.io/* --loglevel=1' 'all arguments'
os::cmd::expect_failure_and_text     'oadm migrate image-references my.docker.io/=docker.io/* --loglevel=1' 'not a valid source'
os::cmd::expect_failure_and_text     'oadm migrate image-references /*=docker.io/* --loglevel=1' 'not a valid source'
os::cmd::expect_failure_and_text     'oadm migrate image-references my.docker.io/*=docker.io --loglevel=1' 'all arguments'
os::cmd::expect_failure_and_text     'oadm migrate image-references my.docker.io/*=docker.io/ --loglevel=1' 'not a valid target'
os::cmd::expect_failure_and_text     'oadm migrate image-references my.docker.io/*=/x --loglevel=1' 'not a valid target'
os::cmd::expect_failure_and_text     'oadm migrate image-references my.docker.io/*=*/* --loglevel=1' 'at least one change'
os::cmd::expect_failure_and_text     'oadm migrate image-references a/b=a/b --loglevel=1' 'at least one field'
os::cmd::expect_failure_and_text     'oadm migrate image-references */*=*/* --loglevel=1' 'at least one change'
# verify dry run
os::cmd::expect_success_and_text     'oadm migrate image-references my.docker.io/*=docker.io/* --loglevel=1' 'migrated=0'
os::cmd::expect_success_and_text     'oadm migrate image-references --include=imagestreams docker.io/*=my.docker.io/* --loglevel=1' 'migrated \(dry run\): imagestreams/test -n '
os::cmd::expect_success_and_text     'oadm migrate image-references --include=imagestreams docker.io/mysql=my.docker.io/* --all-namespaces=false --loglevel=1' 'migrated=1'
os::cmd::expect_success_and_text     'oadm migrate image-references --include=imagestreams docker.io/mysql=my.docker.io/* --all-namespaces=false --loglevel=1 -o yaml' 'dockerImageReference: my.docker.io/mysql@sha256:'
os::cmd::expect_success_and_text     'oadm migrate image-references --include=imagestreams docker.io/other=my.docker.io/* --all-namespaces=false --loglevel=1' 'migrated=0'
# only mysql references are changed
os::cmd::expect_success_and_text     'oadm migrate image-references --include=imagestreams docker.io/mysql=my.docker.io/mysql2 --all-namespaces=false --loglevel=1 --confirm' 'migrated=1'
os::cmd::expect_success_and_text     'oc get istag test:1 --template "{{ .image.dockerImageReference }}"' '^my.docker.io/mysql2@sha256:'
os::cmd::expect_success_and_text     'oc get istag test:2 --template "{{ .image.dockerImageReference }}"' '^php@sha256:'
# all items in history are changed
os::cmd::expect_success_and_text     'oadm migrate image-references --include=imagestreams docker.io/*=my.docker.io/* --all-namespaces=false --loglevel=1 --confirm' 'migrated=1'
os::cmd::expect_success_and_not_text 'oc get is test --template "{{ range .status.tags }}{{ range .items }}{{ .dockerImageReference }}{{ \"\n\" }}{{ end }}{{ end }}"' '^php'
os::cmd::expect_success_and_not_text 'oc get is test --template "{{ range .status.tags }}{{ range .items }}{{ .dockerImageReference }}{{ \"\n\" }}{{ end }}{{ end }}"' '^mysql'
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end

