#!/bin/bash

datasource_name=$1
prometheus_namespace=$2
graph_granularity=$3
yaml=$4

protocol="https://"

usage() {
echo "
USAGE
 setup-grafana.sh <datasource_name> [optional: <prometheus_namespace> <graph_granularity> <yaml>]

 args:
   datasource_name: grafana datasource name
   prometheus_namespace: existing prometheus name e.g openshift-metrics
   graph_granularity: specifiy granularity
   yaml: specifies the grafana yaml

 note:
    - the project must have view permissions for kube-system
    - the script allow to use high granularity by adding '30s' arg, but it needs tuned scrape prometheus
"
exit 1
}

get::namespace(){
if [ -z "$(oc projects |grep openshift-metrics)" ]; then
    prometheus_namespace="kube-system"
else
    prometheus_namespace="openshift-metrics"
fi
}

[[ -n ${datasource_name} ]] || usage
[[ -n ${graph_granularity} ]] || graph_granularity="2m"
[[ -n ${yaml} ]] || yaml="grafana.yaml"
[[ -n ${prometheus_namespace} ]] || get::namespace

oc new-project grafana
oc process -f "${yaml}" |oc create -f -
oc rollout status deployment/grafana
oc adm policy add-role-to-user view -z grafana -n "${prometheus_namespace}"
oc adm pod-network join-projects --to=${prometheus_namespace}

payload="$( mktemp )"
cat <<EOF >"${payload}"
{
"name": "${datasource_name}",
"type": "prometheus",
"typeLogoUrl": "",
"access": "proxy",
"url": "https://prometheus",
"basicAuth": false,
"withCredentials": false,
"jsonData": {
    "tlsSkipVerify":true,
    "token":"$( oc sa get-token grafana )"
}
}
EOF

grafana_host="${protocol}$( oc get route grafana -o jsonpath='{.spec.host}' )"
curl -H "Content-Type: application/json" -u admin:admin "${grafana_host}/api/datasources" -X POST -d "@${payload}"

dashboard_file="./openshift-cluster-monitoring.json"
sed -i.bak "s/Xs/${graph_granularity}/" "${dashboard_file}"
sed -i.bak "s/\${DS_PR}/${datasource_name}/" "${dashboard_file}"
curl -H "Content-Type: application/json" -u admin:admin "${grafana_host}/api/dashboards/db" -X POST -d "@${dashboard_file}"
mv "${dashboard_file}.bak" "${dashboard_file}"

rm -f ${payload}

exit 0
