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
[ "$(oc new-app mysql -o yaml | grep mysql-55-centos7)" ]

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
[ "$(oc new-app --search mysql | grep mysql-55-centos7)" ]
[ "$(oc new-app --search ruby-helloworld-sample | grep ruby-helloworld-sample)" ]
# check search - partial matches
[ "$(oc new-app --search ruby-hellow | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search --template=ruby-hel | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search --template=ruby-helloworld-sam -o yaml | grep ruby-helloworld-sample)" ]
[ "$(oc new-app --search rub | grep openshift/ruby-20-centos7)" ]
[ "$(oc new-app --search --image-stream=rub | grep openshift/ruby-20-centos7)" ]
# check search - check correct usage of filters
[ ! "$(oc new-app --search --image-stream=ruby-heloworld-sample | grep application-template-stibuild)" ]
[ ! "$(oc new-app --search --template=mongodb)" ]
[ ! "$(oc new-app --search --template=php)" ]
[ ! "$(oc new-app -S --template=nodejs)" ]
[ ! "$(oc new-app -S --template=perl)" ]
# check search - filtered, exact matches
[ "$(oc new-app --search --image-stream=mongodb | grep openshift/mongodb-24-centos7)" ]
[ "$(oc new-app --search --image-stream=mysql | grep openshift/mysql-55-centos7)" ]
[ "$(oc new-app --search --image-stream=nodejs | grep openshift/nodejs-010-centos7)" ]
[ "$(oc new-app --search --image-stream=perl | grep openshift/perl-516-centos7)" ]
[ "$(oc new-app --search --image-stream=php | grep openshift/php-55-centos7)" ]
[ "$(oc new-app --search --image-stream=postgresql | grep openshift/postgresql-92-centos7)" ]
[ "$(oc new-app -S --image-stream=python | grep openshift/python-33-centos7)" ]
[ "$(oc new-app -S --image-stream=ruby | grep openshift/ruby-20-centos7)" ]
[ "$(oc new-app -S --image-stream=wildfly | grep openshift/wildfly-81-centos7)" ]
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
oc new-app ruby-helloworld-sample -l app=helloworld
oc delete all -l app=helloworld
# create from template with code explicitly set is not supported
[ ! "$(oc new-app ruby-helloworld-sample~git@github.com/mfojtik/sinatra-app-example)" ]
oc delete template ruby-helloworld-sample
# override component names
[ "$(oc new-app mysql --name=db | grep db)" ]
oc new-app https://github.com/openshift/ruby-hello-world -l app=ruby
oc delete all -l app=ruby

# allow use of non-existent image
[ "$(oc new-app  openshift/bogusImage https://github.com/openshift/ruby-hello-world.git -o yaml 2>&1 | grep "no image or template matched")" ]
[ "$(oc new-app  openshift/bogusImage https://github.com/openshift/ruby-hello-world.git -o yaml --allow-missing-images)" ]

# ensure a local-only image gets a proper imagestream created, it used to be getting a :tag appended to the end, incorrectly.
tmp=$(mktemp -d)
pushd $tmp
cat <<EOF >> Dockerfile
FROM scratch
EXPOSE 80
EOF
docker build -t test/scratchimage .
popd
rm -rf $tmp
[ "$(oc new-app  test/scratchimage https://github.com/openshift/ruby-hello-world.git -o yaml 2>&1 | grep -E "dockerImageRepository: test/scratchimage$")" ]

echo "new-app: ok"
