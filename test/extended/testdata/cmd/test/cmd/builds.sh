#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/builds"

# TODO move this file to test/extended/cli/builds.go

os::test::junit::declare_suite_start "cmd/builds/start-build"
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-buildcli.json'
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
# Use basename to transform "build/build-name" into "build-name"
started="$(basename $(oc start-build -o=name ruby-sample-build-invalidtag))"
os::cmd::expect_success_and_text "oc describe build ${started}" 'openshift/ruby$'
frombuild="$(basename $(oc start-build -o=name --from-build="${started}"))"
os::cmd::expect_success_and_text "oc describe build ${frombuild}" 'openshift/ruby$'
os::cmd::expect_failure_and_text "oc start-build ruby-sample-build-invalid-tag --from-dir=. --from-build=${started}" "cannot use '--from-build' flag with binary builds"
os::cmd::expect_failure_and_text "oc start-build ruby-sample-build-invalid-tag --from-file=. --from-build=${started}" "cannot use '--from-build' flag with binary builds"
os::cmd::expect_failure_and_text "oc start-build ruby-sample-build-invalid-tag --from-repo=. --from-build=${started}" "cannot use '--from-build' flag with binary builds"
# --incremental flag should override Spec.Strategy.SourceStrategy.Incremental
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-s2i-build.json'
build_name="$(oc start-build -o=name test)"
os::cmd::expect_success_and_not_text "oc describe ${build_name}" 'Incremental Build'
build_name="$(oc start-build -o=name --incremental test)"
os::cmd::expect_success_and_text "oc describe ${build_name}" 'Incremental Build'
os::cmd::expect_success "oc patch bc/test -p '{\"spec\":{\"strategy\":{\"sourceStrategy\":{\"incremental\": true}}}}'"
build_name="$(oc start-build -o=name test)"
os::cmd::expect_success_and_text "oc describe ${build_name}" 'Incremental Build'
build_name="$(oc start-build -o=name --incremental=false test)"
os::cmd::expect_success_and_not_text "oc describe ${build_name}" 'Incremental Build'
os::cmd::expect_success "oc patch bc/test -p '{\"spec\":{\"strategy\":{\"sourceStrategy\":{\"incremental\": false}}}}'"
build_name="$(oc start-build -o=name test)"
os::cmd::expect_success_and_not_text "oc describe ${build_name}" 'Incremental Build'
build_name="$(oc start-build -o=name --incremental test)"
os::cmd::expect_success_and_text "oc describe ${build_name}" 'Incremental Build'
os::cmd::expect_failure_and_text "oc start-build test --no-cache" 'Cannot specify Docker build specific options'
os::cmd::expect_failure_and_text "oc start-build test --build-arg=a=b" 'Cannot specify Docker build specific options'
os::cmd::expect_success 'oc delete all --selector="name=test"'
# --no-cache flag should override Spec.Strategy.SourceStrategy.NoCache
os::cmd::expect_success 'oc create -f ${TEST_DATA}/test-docker-build.json'
build_name="$(oc start-build -o=name test)"
os::cmd::expect_success_and_not_text "oc describe ${build_name}" 'No Cache'
build_name="$(oc start-build -o=name --no-cache test)"
os::cmd::expect_success_and_text "oc describe ${build_name}" 'No Cache'
os::cmd::expect_success "oc patch bc/test -p '{\"spec\":{\"strategy\":{\"dockerStrategy\":{\"noCache\": true}}}}'"
build_name="$(oc start-build -o=name test)"
os::cmd::expect_success_and_text "oc describe ${build_name}" 'No Cache'
build_name="$(oc start-build -o=name --no-cache=false test)"
os::cmd::expect_success_and_not_text "oc describe ${build_name}" 'No Cache'
os::cmd::expect_success "oc patch bc/test -p '{\"spec\":{\"strategy\":{\"dockerStrategy\":{\"noCache\": false}}}}'"
build_name="$(oc start-build -o=name test)"
os::cmd::expect_success_and_not_text "oc describe ${build_name}" 'No Cache'
build_name="$(oc start-build -o=name --no-cache test)"
os::cmd::expect_success_and_text "oc describe ${build_name}" 'No Cache'
os::cmd::expect_failure_and_text "oc start-build test --incremental" 'Cannot specify Source build specific options'
# ensure a specific version can be specified for buildconfigs
os::cmd::expect_failure_and_not_text "oc logs bc/test --version=1" "cannot specify a version and a build"
os::cmd::expect_success 'oc delete all --selector="name=test"'
echo "start-build: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/builds/cancel-build"
os::cmd::expect_success_and_text "oc cancel-build ${started} --dump-logs --restart" "build.build.openshift.io/${started} restarted"
os::cmd::expect_success 'oc delete all --all'
os::cmd::expect_success 'oc process -f ${TEST_DATA}/application-template-dockerbuild.json -l build=docker | oc create -f -'
os::cmd::try_until_success 'oc get build/ruby-sample-build-1'
# Uses type/name resource syntax to cancel the build and check for proper message
os::cmd::expect_success_and_text 'oc cancel-build build/ruby-sample-build-1' 'build.build.openshift.io/ruby-sample-build-1 cancelled'
# Make sure canceling already cancelled build returns proper message
os::cmd::expect_success 'oc cancel-build build/ruby-sample-build-1'
# Cancel all builds from a build configuration
os::cmd::expect_success "oc start-build bc/ruby-sample-build"
os::cmd::expect_success "oc start-build bc/ruby-sample-build"
lastbuild="$(basename $(oc start-build -o=name bc/ruby-sample-build))"
os::cmd::expect_success_and_text 'oc cancel-build bc/ruby-sample-build' "build.build.openshift.io/${lastbuild} cancelled"
os::cmd::expect_success_and_text "oc get build ${lastbuild} -o template --template '{{.status.phase}}'" 'Cancelled'
builds=$(oc get builds -o template --template '{{range .items}}{{ .status.phase }} {{end}}')
for state in $builds; do
  os::cmd::expect_success "[ \"${state}\" == \"Cancelled\" ]"
done
# Running this command again when all builds are cancelled should be no-op.
os::cmd::expect_success 'oc cancel-build bc/ruby-sample-build'
os::cmd::expect_success 'oc delete all --all'
os::cmd::expect_success 'oc delete secret dbsecret'
echo "cancel-build: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
