#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


url=":${API_PORT:-8443}"
project="$(oc project -q)"

os::test::junit::declare_suite_start "cmd/builds"
# This test validates builds and build related commands

os::cmd::expect_success 'oc new-build centos/ruby-22-centos7 https://github.com/openshift/ruby-hello-world.git'
os::cmd::expect_success 'oc get bc/ruby-hello-world'

os::cmd::expect_success "cat '${OS_ROOT}/images/origin/Dockerfile' | oc new-build -D - --name=test"
os::cmd::expect_success 'oc get bc/test'

template='{{with .spec.output.to}}{{.kind}} {{.name}}{{end}}'

# Build from Dockerfile with output to ImageStreamTag
os::cmd::expect_success "oc new-build --dockerfile=\$'FROM centos:7\nRUN yum install -y httpd'"
os::cmd::expect_success_and_text "oc get bc/centos --template '${template}'" '^ImageStreamTag centos:latest$'

# Build from a binary with no inputs requires name
os::cmd::expect_failure_and_text "oc new-build --binary" "you must provide a --name"

# Build from a binary with inputs creates a binary build
os::cmd::expect_success "oc new-build --binary --name=binary-test"
os::cmd::expect_success_and_text "oc get bc/binary-test" 'Binary'

os::cmd::expect_success 'oc delete is/binary-test bc/binary-test'

# Build from Dockerfile with output to DockerImage
os::cmd::expect_success "oc new-build -D \$'FROM openshift/origin:v1.1' --to-docker"
os::cmd::expect_success_and_text "oc get bc/origin --template '${template}'" '^DockerImage origin:latest$'

os::cmd::expect_success 'oc delete is/origin'

# Build from Dockerfile with given output ImageStreamTag spec
os::cmd::expect_success "oc new-build -D \$'FROM openshift/origin:v1.1\nENV ok=1' --to origin-test:v1.1"
os::cmd::expect_success_and_text "oc get bc/origin-test --template '${template}'" '^ImageStreamTag origin-test:v1.1$'

os::cmd::expect_success 'oc delete is/origin bc/origin'

# Build from Dockerfile with given output DockerImage spec
os::cmd::expect_success "oc new-build -D \$'FROM openshift/origin:v1.1\nENV ok=1' --to-docker --to openshift/origin:v1.1-test"
os::cmd::expect_success_and_text "oc get bc/origin --template '${template}'" '^DockerImage openshift/origin:v1.1-test$'

os::cmd::expect_success 'oc delete is/origin'

# Build from Dockerfile with custom name and given output ImageStreamTag spec
os::cmd::expect_success "oc new-build -D \$'FROM openshift/origin:v1.1\nENV ok=1' --to origin-name-test --name origin-test2"
os::cmd::expect_success_and_text "oc get bc/origin-test2 --template '${template}'" '^ImageStreamTag origin-name-test:latest$'

os::cmd::try_until_text 'oc get is ruby-22-centos7' 'latest'
os::cmd::expect_failure_and_text 'oc new-build ruby-22-centos7~https://github.com/openshift/ruby-ex ruby-22-centos7~https://github.com/openshift/ruby-ex --to invalid/argument' 'error: only one component with source can be used when specifying an output image reference'

os::cmd::expect_success 'oc delete all --all'

os::cmd::expect_success "oc new-build -D \$'FROM centos:7' --no-output"
os::cmd::expect_success_and_not_text 'oc get bc/centos -o=jsonpath="{.spec.output.to}"' '.'

# Ensure output is valid JSON
os::cmd::expect_success 'oc new-build -D "FROM centos:7" -o json | python -m json.tool'

os::cmd::expect_success 'oc delete all --all'
os::cmd::expect_success 'oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -'
os::cmd::expect_success 'oc get buildConfigs'
os::cmd::expect_success 'oc get bc'
os::cmd::expect_success 'oc get builds'

# make sure the imagestream has the latest tag before trying to test it or start a build with it
os::cmd::try_until_text 'oc get is ruby-22-centos7' 'latest'

os::test::junit::declare_suite_start "cmd/builds/patch-anon-fields"
REAL_OUTPUT_TO=$(oc get bc/ruby-sample-build --template='{{ .spec.output.to.name }}')
os::cmd::expect_success "oc patch bc/ruby-sample-build -p '{\"spec\":{\"output\":{\"to\":{\"name\":\"different:tag1\"}}}}'"
os::cmd::expect_success_and_text "oc get bc/ruby-sample-build --template='{{ .spec.output.to.name }}'" 'different'
os::cmd::expect_success "oc patch bc/ruby-sample-build -p '{\"spec\":{\"output\":{\"to\":{\"name\":\"${REAL_OUTPUT_TO}\"}}}}'"
echo "patchAnonFields: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/builds/config"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "${url}/apis/build.openshift.io/v1/namespaces/${project}/buildconfigs/ruby-sample-build/webhooks/<secret>/github"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "Webhook GitHub"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "${url}/apis/build.openshift.io/v1/namespaces/${project}/buildconfigs/ruby-sample-build/webhooks/<secret>/generic"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "Webhook Generic"
os::cmd::expect_success 'oc start-build --list-webhooks=all ruby-sample-build'
os::cmd::expect_success_and_text 'oc start-build --list-webhooks=all bc/ruby-sample-build' 'generic'
os::cmd::expect_success_and_text 'oc start-build --list-webhooks=all ruby-sample-build' 'github'
os::cmd::expect_success_and_text 'oc start-build --list-webhooks=github ruby-sample-build' '<secret>'
os::cmd::expect_failure 'oc start-build --list-webhooks=blah'
hook=$(oc start-build --list-webhooks='generic' ruby-sample-build | head -n 1)
hook=${hook/<secret>/secret101}
os::cmd::expect_success_and_text "oc start-build --from-webhook=${hook}" "build \"ruby-sample-build-[0-9]\" started"
os::cmd::expect_failure_and_text "oc start-build --from-webhook=${hook}/foo" "error: server rejected our request"
os::cmd::expect_success "oc patch bc/ruby-sample-build -p '{\"spec\":{\"strategy\":{\"dockerStrategy\":{\"from\":{\"name\":\"asdf:7\"}}}}}'"
os::cmd::expect_failure_and_text "oc start-build --from-webhook=${hook}" "Error resolving ImageStreamTag asdf:7"
os::cmd::expect_success 'oc get builds'
os::cmd::expect_success 'oc set triggers bc/ruby-sample-build --from-github --remove'
os::cmd::expect_success_and_not_text 'oc describe buildConfigs ruby-sample-build' "Webhook GitHub"
# make sure we describe webhooks using secretReferences properly
os::cmd::expect_success "oc patch bc/ruby-sample-build -p '{\"spec\":{\"triggers\":[{\"github\":{\"secretReference\":{\"name\":\"mysecret\"}},\"type\":\"GitHub\"}]}}'"
os::cmd::expect_success_and_text 'oc describe buildConfigs ruby-sample-build' "Webhook GitHub"
os::cmd::expect_success 'oc delete all -l build=docker'

echo "buildConfig: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/builds/start-build"
os::cmd::expect_success 'oc create -f test/integration/testdata/test-buildcli.json'
# a build for which there is not an upstream tag in the corresponding imagerepo, so
# the build should use the image field as defined in the buildconfig
# Use basename to transform "build/build-name" into "build-name"
started="$(basename $(oc start-build -o=name ruby-sample-build-invalidtag))"
os::cmd::expect_success_and_text "oc describe build ${started}" 'centos/ruby-22-centos7$'
frombuild="$(basename $(oc start-build -o=name --from-build="${started}"))"
os::cmd::expect_success_and_text "oc describe build ${frombuild}" 'centos/ruby-22-centos7$'
os::cmd::expect_failure_and_text "oc start-build ruby-sample-build-invalid-tag --from-dir=. --from-build=${started}" "Cannot use '--from-build' flag with binary builds"
os::cmd::expect_failure_and_text "oc start-build ruby-sample-build-invalid-tag --from-file=. --from-build=${started}" "Cannot use '--from-build' flag with binary builds"
os::cmd::expect_failure_and_text "oc start-build ruby-sample-build-invalid-tag --from-repo=. --from-build=${started}" "Cannot use '--from-build' flag with binary builds"
# --incremental flag should override Spec.Strategy.SourceStrategy.Incremental
os::cmd::expect_success 'oc create -f test/extended/testdata/builds/test-s2i-build.json'
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
os::cmd::expect_success 'oc create -f test/extended/testdata/builds/test-docker-build.json'
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
os::cmd::expect_success 'oc delete all --selector="name=test"'
echo "start-build: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/builds/cancel-build"
os::cmd::expect_success_and_text "oc cancel-build ${started} --dump-logs --restart" "restarted build \"${started}\""
os::cmd::expect_success 'oc delete all --all'
os::cmd::expect_success 'oc delete secret dbsecret'
os::cmd::expect_success 'oc process -f examples/sample-app/application-template-dockerbuild.json -l build=docker | oc create -f -'
os::cmd::try_until_success 'oc get build/ruby-sample-build-1'
# Uses type/name resource syntax to cancel the build and check for proper message
os::cmd::expect_success_and_text 'oc cancel-build build/ruby-sample-build-1' 'build "ruby-sample-build-1" cancelled'
# Make sure canceling already cancelled build returns proper message
os::cmd::expect_success 'oc cancel-build build/ruby-sample-build-1'
# Cancel all builds from a build configuration
os::cmd::expect_success "oc start-build bc/ruby-sample-build"
os::cmd::expect_success "oc start-build bc/ruby-sample-build"
lastbuild="$(basename $(oc start-build -o=name bc/ruby-sample-build))"
os::cmd::expect_success_and_text 'oc cancel-build bc/ruby-sample-build', "\"${lastbuild}\" cancelled"
os::cmd::expect_success_and_text "oc get build ${lastbuild} -o template --template '{{.status.phase}}'", 'Cancelled'
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
