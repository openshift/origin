#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates the new-app command

oc create -f examples/image-streams/image-streams-centos7.json

[ "$(oc new-app library/php mysql -o yaml | grep 3306)" ]
[ ! "$(oc new-app unknownhubimage -o yaml)" ]
# verify we can generate a Docker image based component "mongodb" directly
[ "$(oc new-app mongo -o yaml | grep library/mongo)" ]
# the local image repository takes precedence over the Docker Hub "mysql" image
tryuntil oc get imagestreamtags mysql:latest
tryuntil oc get imagestreamtags mysql:5.5
tryuntil oc get imagestreamtags mysql:5.6
[ "$(oc new-app mysql -o yaml | grep mysql)" ]
tryuntil oc get imagestreamtags php:latest
tryuntil oc get imagestreamtags php:5.5
tryuntil oc get imagestreamtags php:5.6

# check label creation
oc new-app php mysql -l no-source=php-mysql
oc delete all -l no-source=php-mysql
oc new-app php mysql
oc delete all -l app=php
[ ! "$(oc get dc/mysql)" ]
[ ! "$(oc get dc/php)" ]

# check if we can create from a stored template
oc create -f examples/sample-app/application-template-stibuild.json
oc get template ruby-helloworld-sample
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep MYSQL_USER)" ]
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep MYSQL_PASSWORD)" ]
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep ADMIN_USERNAME)" ]
[ "$(oc new-app ruby-helloworld-sample -o yaml | grep ADMIN_PASSWORD)" ]

# check search
[ "$(oc new-app --search mysql | grep -E "Tags:\s+5.5, 5.6, latest")" ]
[ "$(oc new-app --search ruby-helloworld-sample | grep ruby-helloworld-sample)" ]
# check search - partial matches
[ "$(oc new-app --search ruby-hellow | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search --template=ruby-hel | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search --template=ruby-helloworld-sam -o yaml | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search rub | grep -E "Tags:\s+2.0, 2.2, latest")" ]
[ "$(oc new-app --search --image-stream=rub | grep -E "Tags:\s+2.0, 2.2, latest")" ]
# check search - check correct usage of filters
[ ! "$(oc new-app --search --image-stream=ruby-heloworld-sample | grep application-template-stibuild)" ]
[ ! "$(oc new-app --search --template=mongodb)" ]
[ ! "$(oc new-app --search --template=php)" ]
[ ! "$(oc new-app -S --template=nodejs)" ]
[ ! "$(oc new-app -S --template=perl)" ]
# check search - filtered, exact matches
[ "$(oc new-app --search --image-stream=mongodb | grep -E "Tags:\s+2.4, 2.6, latest")" ]
[ "$(oc new-app --search --image-stream=mysql | grep -E "Tags:\s+5.5, 5.6, latest")" ]
[ "$(oc new-app --search --image-stream=nodejs | grep -E "Tags:\s+0.10, latest")" ]
[ "$(oc new-app --search --image-stream=perl | grep -E "Tags:\s+5.16, 5.20, latest")" ]
[ "$(oc new-app --search --image-stream=php | grep -E "Tags:\s+5.5, 5.6, latest")" ]
[ "$(oc new-app --search --image-stream=postgresql | grep -E "Tags:\s+9.2, 9.4, latest")" ]
[ "$(oc new-app -S --image-stream=python | grep -E "Tags:\s+2.7, 3.3, 3.4, latest")" ]
[ "$(oc new-app -S --image-stream=ruby | grep -E "Tags:\s+2.0, 2.2, latest")" ]
[ "$(oc new-app -S --image-stream=wildfly | grep -E "Tags:\s+8.1, latest")" ]
[ "$(oc new-app --search --template=ruby-helloworld-sample | grep ruby-helloworld-sample)" ]
# check search - no matches
[ "$(oc new-app -S foo-the-bar 2>&1 | grep 'no matches found')" ]
[ "$(oc new-app --search winter-is-coming 2>&1 | grep 'no matches found')" ]
# check search - mutually exclusive flags
[ "$(oc new-app -S mysql --env=FOO=BAR 2>&1 | grep "can't be used")" ]
[ "$(oc new-app --search mysql --code=https://github.com/openshift/ruby-hello-world 2>&1 | grep "can't be used")" ]
[ "$(oc new-app --search mysql --param=FOO=BAR 2>&1 | grep "can't be used")" ]
oc delete imageStreams --all
# check that we can create from the template without errors
[ "$(oc new-app ruby-helloworld-sample -l app=helloworld 2>&1 | grep 'Service "frontend" created')" ]
oc delete all -l app=helloworld
[ "$(oc new-app ruby-helloworld-sample -l app=helloworld -o name 2>&1 | grep 'Service/frontend')" ]
oc delete all -l app=helloworld
# create from template with code explicitly set is not supported
[ ! "$(oc new-app ruby-helloworld-sample~git@github.com/mfojtik/sinatra-app-example)" ]
oc delete template ruby-helloworld-sample
# override component names
[ "$(oc new-app mysql --name=db | grep db)" ]
oc new-app https://github.com/openshift/ruby-hello-world -l app=ruby
oc delete all -l app=ruby

# check new-build
[ "$(oc new-build mysql -o yaml 2>&1 | grep -F 'you must specify at least one source repository URL')" ]
[ "$(oc new-build mysql --binary -o yaml | grep -F 'type: Binary')" ]
[ "$(oc new-build mysql https://github.com/openshift/ruby-hello-world --strategy=docker -o yaml | grep -F 'type: Docker')" ]
[ "$(oc new-build mysql https://github.com/openshift/ruby-hello-world --binary 2>&1 | grep -F 'specifying binary builds and source repositories at the same time is not allowed')" ]

# do not allow use of non-existent image (should fail)
[ "$(oc new-app  openshift/bogusImage https://github.com/openshift/ruby-hello-world.git -o yaml 2>&1 | grep "no match for")" ]
# allow use of non-existent image (should succeed)
[ "$(oc new-app  openshift/bogusImage https://github.com/openshift/ruby-hello-world.git -o yaml --allow-missing-images)" ]

oc create -f test/fixtures/installable-stream.yaml

project=$(oc project -q)
oc policy add-role-to-user edit test-user
oc login -u test-user -p anything
tryuntil oc project "${project}"

tryuntil oc get imagestreamtags installable:file
tryuntil oc get imagestreamtags installable:token
tryuntil oc get imagestreamtags installable:serviceaccount
[ ! "$(oc new-app installable:file)" ]
[ "$(oc new-app installable:file 2>&1 | grep 'requires that you grant the image access')" ]
[ "$(oc new-app installable:serviceaccount 2>&1 | grep "requires an 'installer' service account with project editor access")" ]
[ "$(oc new-app installable:file --grant-install-rights -o yaml | grep -F '/var/run/openshift.secret.token')" ]
[ "$(oc new-app installable:file --grant-install-rights -o yaml | grep -F 'activeDeadlineSeconds: 14400')" ]
[ "$(oc new-app installable:file --grant-install-rights -o yaml | grep -F 'openshift.io/generated-job: "true"')" ]
[ "$(oc new-app installable:file --grant-install-rights -o yaml | grep -F 'openshift.io/generated-job.for: installable:file')" ]
[ "$(oc new-app installable:token --grant-install-rights -o yaml | grep -F 'name: TOKEN_ENV')" ]
[ "$(oc new-app installable:token --grant-install-rights -o yaml | grep -F 'openshift/origin@sha256:')" ]
[ "$(oc new-app installable:serviceaccount --grant-install-rights -o yaml | grep -F 'serviceAccountName: installer')" ]
[ "$(oc new-app installable:serviceaccount --grant-install-rights -o yaml | grep -F 'fieldPath: metadata.namespace')" ]
[ "$(oc new-app installable:serviceaccount --grant-install-rights -o yaml A=B | grep -F 'name: A')" ]

echo "new-app: ok"