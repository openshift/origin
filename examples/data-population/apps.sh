#!/bin/sh

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating apps"

export KUBECONFIG=${OPENSHIFT_ADMIN_CONFIG}

gitrepos=(
  https://github.com/openshift/hello-world.git
)
sources=($(oc get istag -n openshift -o template --template='{{ range .items }}{{ .metadata.name }}{{ "\n" }}{{ end }}'))
num_sources=${#sources[@]}

for ((i=1; i <=$NUM_APPS; i++))
do
  number=$RANDOM
  let "number %= $NUM_PROJECTS"
  oc project ${PROJECT_NAME_PREFIX}${number}

  repo=""
  if [[ $RANDOM -gt 20000 ]]; then
    number=$RANDOM
    let "number %= ${#gitrepos[@]}"
    repo="~${gitrepos[$number]}"
  fi

  number=$RANDOM
  let "number %= $num_sources"
  oc new-app --name=app-${i} ${sources[$number]}${repo}
done

echo "Done"