#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/newapp"
# This test validates the new-app command
os::cmd::expect_success_and_text 'oc new-app library/php mysql -o yaml' '3306'
os::cmd::expect_success_and_text 'oc new-app library/php mysql --dry-run' "Image \"library/php\" runs as the 'root' user which may not be permitted by your cluster administrator"
os::cmd::expect_failure 'oc new-app unknownhubimage -o yaml'
os::cmd::expect_failure_and_text 'oc new-app docker.io/node~https://github.com/openshift/nodejs-ex' 'the image match \"docker.io/node\" for source repository \"https://github.com/openshift/nodejs-ex\" does not appear to be a source-to-image builder.'
os::cmd::expect_failure_and_text 'oc new-app https://github.com/openshift/rails-ex' 'the image match \"ruby\" for source repository \"https://github.com/openshift/rails-ex\" does not appear to be a source-to-image builder.'
os::cmd::expect_success 'oc new-app https://github.com/openshift/rails-ex --strategy=source --dry-run'
# verify we can generate a Docker image based component "mongodb" directly
os::cmd::expect_success_and_text 'oc new-app mongo -o yaml' 'image:\s*mongo'
# the local image repository takes precedence over the Docker Hub "mysql" image
os::cmd::expect_success 'oc create -f examples/image-streams/image-streams-centos7.json'
os::cmd::try_until_success 'oc get imagestreamtags mysql:latest'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.5'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.6'
os::cmd::expect_success_and_not_text 'oc new-app mysql -o yaml' 'image:\s*mysql'
os::cmd::expect_success_and_not_text 'oc new-app mysql --dry-run' "runs as the 'root' user which may not be permitted by your cluster administrator"
# trigger and output should say 5.6
os::cmd::expect_success_and_text 'oc new-app mysql -o yaml' 'mysql:5.6'
os::cmd::expect_success_and_text 'oc new-app mysql --dry-run' 'tag "5.6" for "mysql"'
# test deployments are created with the boolean flag and printed in the UI
os::cmd::expect_success_and_text 'oc new-app mysql --dry-run --as-test' 'This image will be test deployed'
os::cmd::expect_success_and_text 'oc new-app mysql -o yaml --as-test' 'test: true'

# docker strategy with repo that has no dockerfile
os::cmd::expect_failure_and_text 'oc new-app https://github.com/openshift/nodejs-ex --strategy=docker' 'No Dockerfile was found'

# check label creation
os::cmd::try_until_success 'oc get imagestreamtags php:latest'
os::cmd::try_until_success 'oc get imagestreamtags php:5.5'
os::cmd::try_until_success 'oc get imagestreamtags php:5.6'
os::cmd::expect_success 'oc new-app php mysql -l no-source=php-mysql'
os::cmd::expect_success 'oc delete all -l no-source=php-mysql'
os::cmd::expect_success 'oc new-app php mysql'
os::cmd::expect_success 'oc delete all -l app=php'
os::cmd::expect_failure 'oc get dc/mysql'
os::cmd::expect_failure 'oc get dc/php'
os::cmd::expect_success_and_text 'oc new-app -f test/testdata/template-without-app-label.json -o yaml' 'app: ruby-helloworld-sample'

# ensure non-duplicate invalid label errors show up
os::cmd::expect_failure_and_text 'oc new-app nginx -l qwer1345%$$#=self' 'error: ImageStream "nginx" is invalid'
os::cmd::expect_failure_and_text 'oc new-app nginx -l qwer1345%$$#=self' 'DeploymentConfig "nginx" is invalid'
os::cmd::expect_failure_and_text 'oc new-app nginx -l qwer1345%$$#=self' 'Service "nginx" is invalid'

# check if we can create from a stored template
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-stibuild.json'
os::cmd::expect_success 'oc get template ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'MYSQL_USER'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'MYSQL_PASSWORD'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'ADMIN_USERNAME'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'ADMIN_PASSWORD'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample --param MYSQL_PASSWORD=hello -o yaml' 'hello'

# check that values are not split on commas
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample --param MYSQL_PASSWORD=hello,MYSQL_USER=fail -o yaml' 'value: hello,MYSQL_USER=fail'
# check that warning is printed when --param PARAM1=VAL1,PARAM2=VAL2 is used
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample --param MYSQL_PASSWORD=hello,MYSQL_USER=fail -o yaml' 'no longer accepts comma-separated list'
# check that env vars are not split on commas
os::cmd::expect_success_and_text 'oc new-app php --env PASS=one,two=three -o yaml' 'value: one,two=three'
# check that warning is printed when --env PARAM1=VAL1,PARAM2=VAL2 is used
os::cmd::expect_success_and_text 'oc new-app php --env PASS=one,two=three -o yaml' 'no longer accepts comma-separated list'
# check that warning is not printed when --param/env doesn't contain two k-v pairs
os::cmd::expect_success_and_not_text 'oc new-app php --env DEBUG=disabled -o yaml' 'no longer accepts comma-separated list'
os::cmd::expect_success_and_not_text 'oc new-app php --env LEVELS=INFO,WARNING -o yaml' 'no longer accepts comma-separated list'
os::cmd::expect_success_and_not_text 'oc new-app ruby-helloworld-sample --param MYSQL_USER=mysql -o yaml' 'no longer accepts comma-separated list'
os::cmd::expect_success_and_not_text 'oc new-app ruby-helloworld-sample --param MYSQL_PASSWORD=com,ma -o yaml' 'no longer accepts comma-separated list'
# check that warning is not printed when env vars are passed positionally
os::cmd::expect_success_and_text 'oc new-app php PASS=one,two=three -o yaml' 'value: one,two=three'
os::cmd::expect_success_and_not_text 'oc new-app php PASS=one,two=three -o yaml' 'no longer accepts comma-separated list'

# new-build
# check that env vars are not split on commas and warning is printed where they previously have
os::cmd::expect_success_and_text 'oc new-build --binary php --env X=Y,Z=W -o yaml' 'value: Y,Z=W'
os::cmd::expect_success_and_text 'oc new-build --binary php --env X=Y,Z=W -o yaml' 'no longer accepts comma-separated list'
os::cmd::expect_success_and_text 'oc new-build --binary php --env X=Y,Z,W -o yaml' 'value: Y,Z,W'
os::cmd::expect_success_and_not_text 'oc new-build --binary php --env X=Y,Z,W -o yaml' 'no longer accepts comma-separated list'
os::cmd::expect_success_and_not_text 'oc new-build --binary php --env X=Y -o yaml' 'no longer accepts comma-separated list'
os::cmd::expect_success_and_text 'oc new-build --binary php X=Y,Z=W -o yaml' 'value: Y,Z=W'
os::cmd::expect_success_and_not_text 'oc new-build --binary php X=Y,Z=W -o yaml' 'no longer accepts comma-separated list'

# verify we can create from a template when some objects in the template declare an app label
# the app label will not be applied to any objects in the template.
os::cmd::expect_success_and_not_text 'oc new-app -f test/testdata/template-with-app-label.json -o yaml' 'app: ruby-helloworld-sample'
# verify the existing app label on an object is not overridden by new-app
os::cmd::expect_success_and_text 'oc new-app -f test/testdata/template-with-app-label.json -o yaml' 'app: myapp'

# verify that a template can be passed in stdin
os::cmd::expect_success 'cat examples/sample-app/application-template-stibuild.json | oc new-app -o yaml -f -'

# check search
os::cmd::expect_success_and_text 'oc new-app --search mysql' "Tags:\s+5.5, 5.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search ruby-helloworld-sample' 'ruby-helloworld-sample'
# check search - partial matches
os::cmd::expect_success_and_text 'oc new-app --search ruby-hellow' 'ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app --search --template=ruby-hel' 'ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app --search --template=ruby-helloworld-sam -o yaml' 'ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app --search rub' "Tags:\s+2.0, 2.2, 2.3, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=rub' "Tags:\s+2.0, 2.2, 2.3, latest"
# check search - check correct usage of filters
os::cmd::expect_failure_and_not_text 'oc new-app --search --image-stream=ruby-heloworld-sample' 'application-template-stibuild'
os::cmd::expect_failure 'oc new-app --search --template=php'
os::cmd::expect_failure 'oc new-app -S --template=nodejs'
os::cmd::expect_failure 'oc new-app -S --template=perl'
# check search - filtered, exact matches
# make sure the imagestreams are imported first.
os::cmd::try_until_success 'oc get imagestreamtags mongodb:latest'
os::cmd::try_until_success 'oc get imagestreamtags mongodb:2.4'
os::cmd::try_until_success 'oc get imagestreamtags mongodb:2.6'
os::cmd::try_until_success 'oc get imagestreamtags mongodb:3.2'
os::cmd::try_until_success 'oc get imagestreamtags mysql:latest'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.5'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.6'
os::cmd::try_until_success 'oc get imagestreamtags nodejs:latest'
os::cmd::try_until_success 'oc get imagestreamtags nodejs:0.10'
os::cmd::try_until_success 'oc get imagestreamtags nodejs:4'
os::cmd::try_until_success 'oc get imagestreamtags perl:latest'
os::cmd::try_until_success 'oc get imagestreamtags perl:5.16'
os::cmd::try_until_success 'oc get imagestreamtags perl:5.20'
os::cmd::try_until_success 'oc get imagestreamtags php:latest'
os::cmd::try_until_success 'oc get imagestreamtags php:5.5'
os::cmd::try_until_success 'oc get imagestreamtags php:5.6'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:latest'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:9.2'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:9.4'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:9.5'
os::cmd::try_until_success 'oc get imagestreamtags python:latest'
os::cmd::try_until_success 'oc get imagestreamtags python:2.7'
os::cmd::try_until_success 'oc get imagestreamtags python:3.3'
os::cmd::try_until_success 'oc get imagestreamtags python:3.4'
os::cmd::try_until_success 'oc get imagestreamtags python:3.5'
os::cmd::try_until_success 'oc get imagestreamtags ruby:latest'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.0'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.2'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.3'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:latest'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:10.1'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:10.0'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:9.0'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:8.1'

os::cmd::expect_success_and_text 'oc new-app --search --image-stream=mongodb' "Tags:\s+2.4, 2.6, 3.2, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=mysql' "Tags:\s+5.5, 5.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=nodejs' "Tags:\s+0.10, 4, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=perl' "Tags:\s+5.16, 5.20, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=php' "Tags:\s+5.5, 5.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=postgresql' "Tags:\s+9.2, 9.4, 9.5, latest"
os::cmd::expect_success_and_text 'oc new-app -S --image-stream=python' "Tags:\s+2.7, 3.3, 3.4, 3.5, latest"
os::cmd::expect_success_and_text 'oc new-app -S --image-stream=ruby' "Tags:\s+2.0, 2.2, 2.3, latest"
os::cmd::expect_success_and_text 'oc new-app -S --image-stream=wildfly' "Tags:\s+10.0, 10.1, 8.1, 9.0, latest"
os::cmd::expect_success_and_text 'oc new-app --search --template=ruby-helloworld-sample' 'ruby-helloworld-sample'
# check search - no matches
os::cmd::expect_failure_and_text 'oc new-app -S foo-the-bar' 'no matches found'
os::cmd::expect_failure_and_text 'oc new-app --search winter-is-coming' 'no matches found'
# check search - mutually exclusive flags
os::cmd::expect_failure_and_text 'oc new-app -S mysql --env=FOO=BAR' "can't be used"
os::cmd::expect_failure_and_text 'oc new-app --search mysql --code=https://github.com/openshift/ruby-hello-world' "can't be used"
os::cmd::expect_failure_and_text 'oc new-app --search mysql --param=FOO=BAR' "can't be used"

# set context-dir
os::cmd::expect_success_and_text 'oc new-app https://github.com/openshift/sti-ruby.git --context-dir="2.0/test/puma-test-app" -o yaml' 'contextDir: 2.0/test/puma-test-app'
os::cmd::expect_success_and_text 'oc new-app ruby~https://github.com/openshift/sti-ruby.git --context-dir="2.0/test/puma-test-app" -o yaml' 'contextDir: 2.0/test/puma-test-app'

# set strategy
os::cmd::expect_success_and_text 'oc new-app ruby~https://github.com/openshift/ruby-hello-world.git --strategy=docker -o yaml' 'dockerStrategy'
os::cmd::expect_success_and_text 'oc new-app https://github.com/openshift/ruby-hello-world.git --strategy=source -o yaml' 'sourceStrategy'

# prints volume and root user info
os::cmd::expect_success_and_text 'oc new-app --dry-run mysql' 'This image declares volumes'
os::cmd::expect_success_and_not_text 'oc new-app --dry-run mysql' "runs as the 'root' user"
os::cmd::expect_success_and_text 'oc new-app --dry-run --docker-image=mysql' 'This image declares volumes'
os::cmd::expect_success_and_text 'oc new-app --dry-run --docker-image=mysql' "WARNING: Image \"mysql\" runs as the 'root' user"

# verify multiple errors are displayed together, a nested error is returned, and that the usage message is displayed
os::cmd::expect_failure_and_text 'oc new-app --dry-run __template_fail __templatefile_fail' 'error: no match for "__template_fail"'
os::cmd::expect_failure_and_text 'oc new-app --dry-run __template_fail __templatefile_fail' 'error: no match for "__templatefile_fail"'
os::cmd::expect_failure_and_text 'oc new-app --dry-run __template_fail __templatefile_fail' 'error: unable to find the specified template file'
os::cmd::expect_failure_and_text 'oc new-app --dry-run __template_fail __templatefile_fail' "The 'oc new-app' command will match arguments"

# verify partial match error
os::cmd::expect_failure_and_text 'oc new-app --dry-run mysq' 'error: only a partial match was found for "mysq"'
os::cmd::expect_failure_and_text 'oc new-app --dry-run mysq' 'The argument "mysq" only partially matched'
os::cmd::expect_failure_and_text 'oc new-app --dry-run mysq' "Image stream \"mysql\" \\(tag \"5.6\"\\) in project"

# verify image streams with no tags are reported correctly and that --allow-missing-imagestream-tags works
# new-app
os::cmd::expect_success 'printf "apiVersion: v1\nkind: ImageStream\nmetadata:\n  name: emptystream\n" | oc create -f -'
os::cmd::expect_failure_and_text 'oc new-app --dry-run emptystream' 'error: no tags found on matching image stream'
os::cmd::expect_success 'oc new-app --dry-run emptystream --allow-missing-imagestream-tags'
# new-build
os::cmd::expect_failure_and_text 'oc new-build --dry-run emptystream~https://github.com/openshift/ruby-ex' 'error: no tags found on matching image stream'
os::cmd::expect_success 'oc new-build --dry-run emptystream~https://github.com/openshift/ruby-ex --allow-missing-imagestream-tags --strategy=source'

# Allow setting --name when specifying grouping
os::cmd::expect_success "oc new-app mysql+ruby~https://github.com/openshift/ruby-ex --name foo -o yaml"
# but not with multiple components
os::cmd::expect_failure_and_text "oc new-app mysql ruby~https://github.com/openshift/ruby-ex --name foo -o yaml" "error: only one component or source repository can be used when specifying a name"
# do not allow specifying output image when specifying multiple input repos
os::cmd::expect_failure_and_text 'oc new-build https://github.com/openshift/nodejs-ex https://github.com/openshift/ruby-ex --to foo' 'error: only one component with source can be used when specifying an output image reference'
# but succeed with multiple input repos and no output image specified
os::cmd::expect_success 'oc new-build https://github.com/openshift/nodejs-ex https://github.com/openshift/ruby-ex -o yaml'
# check that binary build with a builder image results in a source type build
os::cmd::expect_success_and_text 'oc new-build --binary --image-stream=ruby -o yaml' 'type: Source'
# check that binary build with a specific strategy uses that strategy regardless of the image type
os::cmd::expect_success_and_text 'oc new-build --binary --image=ruby --strategy=docker -o yaml' 'type: Docker'
os::cmd::expect_success 'oc delete imageStreams --all'

# check that we can create from the template without errors
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -l app=helloworld' 'service "frontend" created'
os::cmd::expect_success 'oc delete all -l app=helloworld'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -l app=helloworld -o name' 'service/frontend'
os::cmd::expect_success 'oc delete all -l app=helloworld'
# create from template with code explicitly set is not supported
os::cmd::expect_failure 'oc new-app ruby-helloworld-sample~git@github.com:mfojtik/sinatra-app-example'
os::cmd::expect_success 'oc delete template ruby-helloworld-sample'
# override component names
os::cmd::expect_success_and_text 'oc new-app mysql --name=db' 'db'
os::cmd::expect_success 'oc new-app https://github.com/openshift/ruby-hello-world -l app=ruby'
os::cmd::expect_success 'oc delete all -l app=ruby'
# check for error when template JSON file has errors
jsonfile="${OS_ROOT}/test/testdata/invalid.json"
os::cmd::expect_failure_and_text "oc new-app '${jsonfile}'" "error: unable to load template file \"${jsonfile}\": json: line 0: invalid character '}' after object key"

# a docker compose file should be transformed into an application by the import command
os::cmd::expect_success_and_text 'oc import docker-compose -f test/testdata/app-scenarios/docker-compose/complex/docker-compose.yml --dry-run' 'warning: not all docker-compose fields were honored'
os::cmd::expect_success_and_text 'oc import docker-compose -f test/testdata/app-scenarios/docker-compose/complex/docker-compose.yml --dry-run' 'db: cpuset is not supported'
os::cmd::expect_success_and_text 'oc import docker-compose -f test/testdata/app-scenarios/docker-compose/complex/docker-compose.yml --dry-run' 'no-ports: no ports defined to send traffic to - no OpenShift service was created'
os::cmd::expect_success_and_text 'oc import docker-compose -f test/testdata/app-scenarios/docker-compose/complex/docker-compose.yml -o name --dry-run' 'service/redis'
os::cmd::expect_success_and_text 'oc import docker-compose -f test/testdata/app-scenarios/docker-compose/complex/docker-compose.yml -o name --as-template=other --dry-run' 'template/other'
if git status &>/dev/null; then
  # TODO: only consistent when running in a Git repository
  os::cmd::expect_success '[[ $(diff --suppress-common-lines -y <(oc import docker-compose -f test/testdata/app-scenarios/docker-compose/complex/docker-compose.yml -o yaml) test/testdata/app-scenarios/docker-compose/complex/docker-compose.imported.yaml | grep -vE "ref\:|secret|uri\:" | wc -l) -eq 0 ]]'
fi

# verify a docker-compose.yml schema 2 resource can be transformed, and that it sets env vars correctly.
os::cmd::expect_success_and_text 'oc import docker-compose -f test/testdata/app-scenarios/docker-compose/wordpress/docker-compose.yml -o yaml --as-template=other --dry-run' 'value: wordpress'

# check new-build
os::cmd::expect_failure_and_text 'oc new-build mysql -o yaml' 'you must specify at least one source repository URL'
os::cmd::expect_success_and_text 'oc new-build mysql --binary -o yaml --to mysql:bin' 'type: Binary'
os::cmd::expect_success_and_text 'oc new-build mysql https://github.com/openshift/ruby-hello-world --strategy=docker -o yaml' 'type: Docker'
os::cmd::expect_failure_and_text 'oc new-build mysql https://github.com/openshift/ruby-hello-world --binary' 'specifying binary builds and source repositories at the same time is not allowed'
# binary builds cannot be created unless a builder image is specified.
os::cmd::expect_failure_and_text 'oc new-build --name mybuild --binary --strategy=source -o yaml' 'you must provide a builder image when using the source strategy with a binary build'
os::cmd::expect_success_and_text 'oc new-build --name mybuild centos/ruby-22-centos7 --binary --strategy=source -o yaml' 'name: ruby-22-centos7:latest'
# binary builds can be created with no builder image if no strategy or docker strategy is specified
os::cmd::expect_success_and_text 'oc new-build --name mybuild --binary -o yaml' 'type: Binary'
os::cmd::expect_success_and_text 'oc new-build --name mybuild --binary --strategy=docker -o yaml' 'type: Binary'

# new-build image source tests
os::cmd::expect_failure_and_text 'oc new-build mysql --source-image centos' 'error: --source-image-path must be specified when --source-image is specified.'
os::cmd::expect_failure_and_text 'oc new-build mysql --source-image-path foo' 'error: --source-image must be specified when --source-image-path is specified.'

# do not allow use of non-existent image (should fail)
os::cmd::expect_failure_and_text 'oc new-app  openshift/bogusimage https://github.com/openshift/ruby-hello-world.git -o yaml' "no match for"
# allow use of non-existent image (should succeed)
os::cmd::expect_success 'oc new-app openshift/bogusimage https://github.com/openshift/ruby-hello-world.git -o yaml --allow-missing-images'

os::cmd::expect_success 'oc create -f test/testdata/installable-stream.yaml'

project=$(oc project -q)
os::cmd::expect_success 'oc policy add-role-to-user edit test-user'
os::cmd::expect_success 'oc login -u test-user -p anything'
os::cmd::try_until_success 'oc project ${project}'

os::cmd::try_until_success 'oc get imagestreamtags installable:file'
os::cmd::try_until_success 'oc get imagestreamtags installable:token'
os::cmd::try_until_success 'oc get imagestreamtags installable:serviceaccount'
os::cmd::expect_failure 'oc new-app installable:file'
os::cmd::expect_failure_and_text 'oc new-app installable:file' 'requires that you grant the image access'
os::cmd::expect_failure_and_text 'oc new-app installable:serviceaccount' "requires an 'installer' service account with project editor access"
os::cmd::expect_success_and_text 'oc new-app installable:file --grant-install-rights -o yaml' '/var/run/openshift.secret.token'
os::cmd::expect_success_and_text 'oc new-app installable:file --grant-install-rights -o yaml' 'activeDeadlineSeconds: 14400'
os::cmd::expect_success_and_text 'oc new-app installable:file --grant-install-rights -o yaml' 'openshift.io/generated-job: "true"'
os::cmd::expect_success_and_text 'oc new-app installable:file --grant-install-rights -o yaml' 'openshift.io/generated-job.for: installable:file'
os::cmd::expect_success_and_text 'oc new-app installable:token --grant-install-rights -o yaml' 'name: TOKEN_ENV'
os::cmd::expect_success_and_text 'oc new-app installable:token --grant-install-rights -o yaml' 'openshift/origin@sha256:'
os::cmd::expect_success_and_text 'oc new-app installable:serviceaccount --grant-install-rights -o yaml' 'serviceAccountName: installer'
os::cmd::expect_success_and_text 'oc new-app installable:serviceaccount --grant-install-rights -o yaml' 'fieldPath: metadata.namespace'
os::cmd::expect_success_and_text 'oc new-app installable:serviceaccount --grant-install-rights -o yaml A=B' 'name: A'

# Ensure output is valid JSON
os::cmd::expect_success 'oc new-app mongo -o json | python -m json.tool'

# Ensure custom branch/ref works
os::cmd::expect_success 'oc new-app https://github.com/openshift/ruby-hello-world#beta4'

# Ensure the resulting BuildConfig doesn't have unexpected sources
os::cmd::expect_success_and_not_text 'oc new-app https://github.com/openshift/ruby-hello-world --output-version=v1 -o=jsonpath="{.items[?(@.kind==\"BuildConfig\")].spec.source}"' 'dockerfile|binary'

echo "new-app: ok"
os::test::junit::declare_suite_end
