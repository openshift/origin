#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates the new-app command

os::cmd::expect_success_and_text 'oc new-app library/php mysql -o yaml' '3306'
os::cmd::expect_failure 'oc new-app unknownhubimage -o yaml'
# verify we can generate a Docker image based component "mongodb" directly
os::cmd::expect_success_and_text 'oc new-app mongo -o yaml' 'library/mongo'
# the local image repository takes precedence over the Docker Hub "mysql" image
os::cmd::expect_success 'oc create -f examples/image-streams/image-streams-centos7.json'
os::cmd::try_until_success 'oc get imagestreamtags mysql:latest'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.5'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.6'
os::cmd::expect_success_and_not_text 'oc new-app mysql -o yaml' 'library/mysql'

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

# check if we can create from a stored template
os::cmd::expect_success 'oc create -f examples/sample-app/application-template-stibuild.json'
os::cmd::expect_success 'oc get template ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'MYSQL_USER'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'MYSQL_PASSWORD'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'ADMIN_USERNAME'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -o yaml' 'ADMIN_PASSWORD'

# check search
os::cmd::expect_success_and_text 'oc new-app --search mysql' "Tags:\s+5.5, 5.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search ruby-helloworld-sample' 'ruby-helloworld-sample'
# check search - partial matches
os::cmd::expect_success_and_text 'oc new-app --search ruby-hellow' 'ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app --search --template=ruby-hel' 'ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app --search --template=ruby-helloworld-sam -o yaml' 'ruby-helloworld-sample'
os::cmd::expect_success_and_text 'oc new-app --search rub' "Tags:\s+2.0, 2.2, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=rub' "Tags:\s+2.0, 2.2, latest"
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
os::cmd::try_until_success 'oc get imagestreamtags mysql:latest'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.5'
os::cmd::try_until_success 'oc get imagestreamtags mysql:5.6'
os::cmd::try_until_success 'oc get imagestreamtags nodejs:latest'
os::cmd::try_until_success 'oc get imagestreamtags nodejs:0.10'
os::cmd::try_until_success 'oc get imagestreamtags perl:latest'
os::cmd::try_until_success 'oc get imagestreamtags perl:5.16'
os::cmd::try_until_success 'oc get imagestreamtags perl:5.20'
os::cmd::try_until_success 'oc get imagestreamtags php:latest'
os::cmd::try_until_success 'oc get imagestreamtags php:5.5'
os::cmd::try_until_success 'oc get imagestreamtags php:5.6'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:latest'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:9.2'
os::cmd::try_until_success 'oc get imagestreamtags postgresql:9.4'
os::cmd::try_until_success 'oc get imagestreamtags python:latest'
os::cmd::try_until_success 'oc get imagestreamtags python:2.7'
os::cmd::try_until_success 'oc get imagestreamtags python:3.3'
os::cmd::try_until_success 'oc get imagestreamtags python:3.4'
os::cmd::try_until_success 'oc get imagestreamtags ruby:latest'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.0'
os::cmd::try_until_success 'oc get imagestreamtags ruby:2.2'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:latest'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:8.1'

os::cmd::expect_success_and_text 'oc new-app --search --image-stream=mongodb' "Tags:\s+2.4, 2.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=mysql' "Tags:\s+5.5, 5.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=nodejs' "Tags:\s+0.10, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=perl' "Tags:\s+5.16, 5.20, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=php' "Tags:\s+5.5, 5.6, latest"
os::cmd::expect_success_and_text 'oc new-app --search --image-stream=postgresql' "Tags:\s+9.2, 9.4, latest"
os::cmd::expect_success_and_text 'oc new-app -S --image-stream=python' "Tags:\s+2.7, 3.3, 3.4, latest"
os::cmd::expect_success_and_text 'oc new-app -S --image-stream=ruby' "Tags:\s+2.0, 2.2, latest"
os::cmd::expect_success_and_text 'oc new-app -S --image-stream=wildfly' "Tags:\s+8.1, latest"
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

os::cmd::expect_success 'oc delete imageStreams --all'
# check that we can create from the template without errors
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -l app=helloworld' 'Service "frontend" created'
os::cmd::expect_success 'oc delete all -l app=helloworld'
os::cmd::expect_success_and_text 'oc new-app ruby-helloworld-sample -l app=helloworld -o name' 'Service/frontend'
os::cmd::expect_success 'oc delete all -l app=helloworld'
# create from template with code explicitly set is not supported
os::cmd::expect_failure 'oc new-app ruby-helloworld-sample~git@github.com:mfojtik/sinatra-app-example'
os::cmd::expect_success 'oc delete template ruby-helloworld-sample'
# override component names
os::cmd::expect_success_and_text 'oc new-app mysql --name=db' 'db'
os::cmd::expect_success 'oc new-app https://github.com/openshift/ruby-hello-world -l app=ruby'
os::cmd::expect_success 'oc delete all -l app=ruby'

# check new-build
os::cmd::expect_failure_and_text 'oc new-build mysql -o yaml' 'you must specify at least one source repository URL'
os::cmd::expect_success_and_text 'oc new-build mysql --binary -o yaml --to mysql:bin' 'type: Binary'
os::cmd::expect_success_and_text 'oc new-build mysql https://github.com/openshift/ruby-hello-world --strategy=docker -o yaml' 'type: Docker'
os::cmd::expect_failure_and_text 'oc new-build mysql https://github.com/openshift/ruby-hello-world --binary' 'specifying binary builds and source repositories at the same time is not allowed'

# do not allow use of non-existent image (should fail)
os::cmd::expect_failure_and_text 'oc new-app  openshift/bogusImage https://github.com/openshift/ruby-hello-world.git -o yaml' "no match for"
# allow use of non-existent image (should succeed)
os::cmd::expect_success 'oc new-app openshift/bogusImage https://github.com/openshift/ruby-hello-world.git -o yaml --allow-missing-images'

os::cmd::expect_success 'oc create -f test/fixtures/installable-stream.yaml'

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

echo "new-app: ok"
