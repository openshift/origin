#!/bin/bash
# OpenShift namespaced objects:
# oc get --raw /oapi/v1/ |  python -c 'import json,sys ; resources = "\n".join([o["name"] for o in json.load(sys.stdin)["resources"] if o["namespaced"] and "create" in o["verbs"] and "delete" in o["verbs"] ]) ; print resources'
# Kubernetes namespaced objects:
# oc get --raw /api/v1/ |  python -c 'import json,sys ; resources = "\n".join([o["name"] for o in json.load(sys.stdin)["resources"] if o["namespaced"] and "create" in o["verbs"] and "delete" in o["verbs"] ]) ; print resources'

set -eo pipefail

die(){
  echo "$1"
  exit $2
}

usage(){
  echo "$0 <projectname>"
  echo "  projectname  The OCP project to be exported"
  echo "Examples:"
  echo "    $0 myproject"
}

ns(){
  echo "Exporting namespace to ${PROJECT}/ns.json"
  oc get --export -o=json ns/${PROJECT} | jq '
    del(.status,
      .metadata.uid,
      .metadata.selfLink,
      .metadata.resourceVersion,
      .metadata.creationTimestamp,
      .metadata.generation
      )' > ${PROJECT}/ns.json
}

rolebindings(){
  echo "Exporting rolebindings to ${PROJECT}/rolebindings.json"
  oc get --export -o=json rolebindings -n ${PROJECT} | jq '.items[] |
  del(.metadata.uid,
      .metadata.selfLink,
      .metadata.resourceVersion,
      .metadata.creationTimestamp
      )' > ${PROJECT}/rolebindings.json
}

serviceaccounts(){
  echo "Exporting serviceaccounts to ${PROJECT}/serviceaccounts.json"
  oc get --export -o=json serviceaccounts -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp
        )' > ${PROJECT}/serviceaccounts.json
}

secrets(){
  echo "Exporting secrets to ${PROJECT}/secrets.json"
  oc get --export -o=json secrets -n ${PROJECT} | jq '.items[] |
    select(.type!="kubernetes.io/service-account-token") |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.annotations."kubernetes.io/service-account.uid"
        )' > ${PROJECT}/secrets.json
}

dcs(){
  echo "Exporting deploymentconfigs to ${PROJECT}/dc_*.json"
  DCS=$(oc get dc -n ${PROJECT} -o jsonpath="{.items[*].metadata.name}")
  for dc in ${DCS}; do
    oc get --export -o=json dc ${dc} -n ${PROJECT} | jq '
      del(.status,
          .metadata.uid,
          .metadata.selfLink,
          .metadata.resourceVersion,
          .metadata.creationTimestamp,
          .metadata.generation,
          .spec.triggers[].imageChangeParams.lastTriggeredImage
          )' > ${PROJECT}/dc_${dc}.json
    if [ !$(cat ${PROJECT}/dc_${dc}.json | jq '.spec.triggers[].type' | grep -q "ImageChange") ]; then
      for container in $(cat ${PROJECT}/dc_${dc}.json | jq -r '.spec.triggers[] | select(.type == "ImageChange") .imageChangeParams.containerNames[]'); do
        echo "Patching DC..."
        OLD_IMAGE=$(cat ${PROJECT}/dc_${dc}.json | jq --arg cname ${container} -r '.spec.template.spec.containers[] | select(.name == $cname)| .image')
        NEW_IMAGE=$(cat ${PROJECT}/dc_${dc}.json | jq -r '.spec.triggers[] | select(.type == "ImageChange") .imageChangeParams.from.name // empty')
        sed -e "s#$OLD_IMAGE#$NEW_IMAGE#g" ${PROJECT}/dc_${dc}.json >> ${PROJECT}/dc_${dc}_patched.json
      done
    fi
  done
}

bcs(){
  echo "Exporting buildconfigs to ${PROJECT}/bcs.json"
  oc get --export -o=json bc -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .spec.triggers[].imageChangeParams.lastTriggeredImage
        )' > ${PROJECT}/bcs.json
}

builds(){
  echo "Exporting builds to ${PROJECT}/builds.json"
  oc get --export -o=json builds -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/builds.json
}

is(){
  echo "Exporting imagestreams to ${PROJECT}/iss.json"
  oc get --export -o=json is -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .metadata.annotations."openshift.io/image.dockerRepositoryCheck"
        )' > ${PROJECT}/iss.json
}

rcs(){
  echo "Exporting replicationcontrollers to ${PROJECT}/rcs.json"
  oc get --export -o=json rc -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/rcs.json
}

svcs(){
  echo "Exporting services to ${PROJECT}/svc_*.json"
  SVCS=$(oc get svc -n ${PROJECT} -o jsonpath="{.items[*].metadata.name}")
  for svc in ${SVCS}; do
    oc get --export -o=json svc ${svc} -n ${PROJECT} | jq '
      del(.status,
            .metadata.uid,
            .metadata.selfLink,
            .metadata.resourceVersion,
            .metadata.creationTimestamp,
            .metadata.generation,
            .spec.clusterIP
            )' > ${PROJECT}/svc_${svc}.json
    if [[ $(cat ${PROJECT}/svc_${svc}.json | jq -e '.spec.selector.app') == "null" ]]; then
      oc get --export -o json endpoints ${svc} -n ${PROJECT}| jq '
        del(.status,
            .metadata.uid,
            .metadata.selfLink,
            .metadata.resourceVersion,
            .metadata.creationTimestamp,
            .metadata.generation
            )' > ${PROJECT}/endpoint_${svc}.json
    fi
  done
}

pods(){
  echo "Exporting pods to ${PROJECT}/pods.json"
  oc get --export -o=json pod -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/pods.json
}

cms(){
  echo "Exporting configmaps to ${PROJECT}/cms.json"
  oc get --export -o=json configmaps -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/cms.json
}

pvcs(){
  echo "Exporting pvcs to ${PROJECT}/pvcs.json"
  oc get --export -o=json pvc -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .metadata.annotations["pv.kubernetes.io/bind-completed"],
        .metadata.annotations["pv.kubernetes.io/bound-by-controller"],
        .metadata.annotations["volume.beta.kubernetes.io/storage-provisioner"],
        .spec.volumeName
        )' > ${PROJECT}/pvcs.json
}

pvcs_attachment(){
  echo "Exporting pvcs (with attachment included data) to ${PROJECT}/pvcs_attachment.json"
  oc get --export -o=json pvc -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/pvcs_attachment.json
}

routes(){
  echo "Exporting routes to ${PROJECT}/routes.json"
  oc get --export -o=json routes -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/routes.json
}

templates(){
  echo "Exporting templates to ${PROJECT}/templates.json"
  oc get --export -o=json templates -n ${PROJECT} | jq '.items[] |
    del(.status,
        .metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation
        )' > ${PROJECT}/templates.json
}

egressnetworkpolicies(){
  echo "Exporting egressnetworkpolicies to ${PROJECT}/egressnetworkpolicies.json"
  oc get --export -o=json egressnetworkpolicies -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp
        )' > ${PROJECT}/egressnetworkpolicies.json
}

imagestreamtags(){
  echo "Exporting imagestreamtags to ${PROJECT}/imagestreamtags.json"
  oc get --export -o=json imagestreamtags -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .tag.generation
        )' > ${PROJECT}/imagestreamtags.json
}

rolebindingrestrictions(){
  echo "Exporting rolebindingrestrictions to ${PROJECT}/rolebindingrestrictions.json"
  oc get --export -o=json rolebindingrestrictions -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp
        )' > ${PROJECT}/rolebindingrestrictions.json
}

limitranges(){
  echo "Exporting limitranges to ${PROJECT}/limitranges.json"
  oc get --export -o=json limitranges -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp
        )' > ${PROJECT}/limitranges.json
}

resourcequotas(){
  echo "Exporting resourcequotas to ${PROJECT}/resourcequotas.json"
  oc get --export -o=json resourcequotas -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .status
        )' > ${PROJECT}/resourcequotas.json
}

podpreset(){
  echo "Exporting podpreset to ${PROJECT}/podpreset.json"
  oc get --export -o=json podpreset -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp
        )' > ${PROJECT}/podpreset.json
}

cronjobs(){
  echo "Exporting cronjobs to ${PROJECT}/cronjobs.json"
  oc get --export -o=json cronjobs -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .status
        )' > ${PROJECT}/cronjobs.json
}

statefulsets(){
  echo "Exporting statefulsets to ${PROJECT}/statefulsets.json"
  oc get --export -o=json statefulsets -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .status
        )' > ${PROJECT}/statefulsets.json
}

hpas(){
  echo "Exporting hpas to ${PROJECT}/hpas.json"
  oc get --export -o=json hpa -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .status
        )' > ${PROJECT}/hpas.json
}

deployments(){
  echo "Exporting deployments to ${PROJECT}/deployments.json"
  oc get --export -o=json deploy -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .status
        )' > ${PROJECT}/deployments.json
}

replicasets(){
  echo "Exporting replicasets to ${PROJECT}/replicasets.json"
  oc get --export -o=json replicasets -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .status,
        .ownerReferences.uid
        )' > ${PROJECT}/replicasets.json
}

poddisruptionbudget(){
  echo "Exporting poddisruptionbudget to ${PROJECT}/poddisruptionbudget.json"
  oc get --export -o=json poddisruptionbudget -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .status
        )' > ${PROJECT}/poddisruptionbudget.json
}

daemonset(){
  echo "Exporting daemonset to ${PROJECT}/daemonset.json"
  oc get --export -o=json daemonset -n ${PROJECT} | jq '.items[] |
    del(.metadata.uid,
        .metadata.selfLink,
        .metadata.resourceVersion,
        .metadata.creationTimestamp,
        .metadata.generation,
        .status
        )' > ${PROJECT}/daemonset.json
}

if [[ ( $@ == "--help") ||  $@ == "-h" ]]
then
  usage
  exit 0
fi

if [[ $# -lt 1 ]]
then
  usage
  die "projectname not provided" 2
fi

for i in jq oc
do
  command -v $i >/dev/null 2>&1 || die "$i required but not found" 3
done

PROJECT=${1}

mkdir -p ${PROJECT}

ns
rolebindings
serviceaccounts
secrets
dcs
bcs
builds
is
imagestreamtags
rcs
svcs
pods
podpreset
cms
egressnetworkpolicies
rolebindingrestrictions
limitranges
resourcequotas
pvcs
pvcs_attachment
routes
templates
cronjobs
statefulsets
hpas
deployments
replicasets
poddisruptionbudget
daemonset

echo "Removing empty files"
find "${PROJECT}" -type f -empty -delete

exit 0
