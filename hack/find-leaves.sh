#!/bin/bash

# this is all the dependencies we have
#go list -f '{{.Deps}}'  github.com/openshift/origin/pkg/cmd/openshift-apiserver | xargs -n 1 echo | grep 'github.com/openshift/origin' | grep -v 'github.com/openshift/origin/vendor' | sort
all_packages=$(go list -f '{{.Deps}}' $1 | xargs -n 1 echo | grep 'github.com/openshift/origin' | grep -v 'github.com/openshift/origin/vendor' | sort)

for package in ${all_packages}
do
    curr_packages=$(go list -f '{{.Deps}}'  ${package} | xargs -n 1 echo | grep 'github.com/openshift/origin' | grep -v 'github.com/openshift/origin/vendor' | sort)
    if [ -z "${curr_packages}" ]
    then
        echo "${package} should move"
    else
        echo "${package} has deps"
    fi
done
