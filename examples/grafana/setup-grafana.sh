#!/bin/bash

setoauth=0
node_exporter=0
datasource_name=''
prometheus_namespace=''
sa_reader=''
graph_granularity=''
yaml=''
protocol="https://"

while getopts 'n:s:p:g:y:ae' flag; do
  case "${flag}" in
    n) datasource_name="${OPTARG}" ;;
    s) sa_reader="${OPTARG}" ;;
    p) prometheus_namespace="${OPTARG}" ;;
    g) graph_granularity="${OPTARG}" ;;
    y) yaml="${OPTARG}" ;;
    a) setoauth=1 ;;
    e) node_exporter=1;;
    *) error "Unexpected option ${flag}" ;;
  esac
done

usage() {
echo "
USAGE
 setup-grafana.sh -n <datasource_name> -a [optional: -p <prometheus_namespace> -s <prometheus_serviceaccount> -g <graph_granularity> -y <yaml> -e]

 switches:
   -n: grafana datasource name
   -s: prometheus serviceaccount name
   -p: existing prometheus name e.g openshift-metrics
   -g: specifiy granularity
   -y: specifies the grafana yaml
   -a: deploy oauth proxy for grafana - otherwise skip it (for preconfigured deployment)
   -e: deploy node exporter

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

set::oauth() {
htpasswd -c /etc/origin/master/htpasswd grafana
sed -ie 's|AllowAllPasswordIdentityProvider|HTPasswdPasswordIdentityProvider\n      file: /etc/origin/master/htpasswd|' /etc/origin/master/master-config.yaml
oc adm policy add-cluster-role-to-user cluster-reader grafana
systemctl restart atomic-openshift-master-api.service
}

# deploy node exporter
node::exporter(){
oc annotate ns kube-system openshift.io/node-selector= --overwrite
sed -i.bak "s/Xs/${graph_granularity}/" "${dashboard_file}"
sed -i.bak "s/\${DS_PR}/${datasource_name}/" "${dashboard_file}"
curl -H "Content-Type: application/json" -u admin:admin "${grafana_host}/api/dashboards/db" -X POST -d "@./node-exporter-full-dashboard.json"
mv "${dashboard_file}.bak" "${dashboard_file}"
}

[[ -n ${datasource_name} ]] || usage
[[ -n ${sa_reader} ]] || sa_reader="prometheus"
[[ -n ${prometheus_namespace} ]] || get::namespace
[[ -n ${graph_granularity} ]]  || graph_granularity="2m"
[[ -n ${yaml} ]] || yaml="grafana.yaml"
((setoauth)) && set::oauth || echo "skip oauth"

oc new-project grafana
oc process -f "${yaml}" |oc create -f -
oc rollout status deployment/grafana
oc adm policy add-role-to-user view -z grafana -n "${prometheus_namespace}"

payload="$( mktemp )"
cat <<EOF >"${payload}"
{
"name": "${datasource_name}",
"type": "prometheus",
"typeLogoUrl": "",
"access": "proxy",
"url": "https://$( oc get route prometheus -n "${prometheus_namespace}" -o jsonpath='{.spec.host}' )",
"basicAuth": false,
"withCredentials": false,
"jsonData": {
    "tlsSkipVerify":true,
    "token":"$( oc sa get-token "${sa_reader}" -n "${prometheus_namespace}" )"
}
}
EOF

# setup grafana data source
grafana_host="${protocol}$( oc get route grafana -o jsonpath='{.spec.host}' )"
curl -H "Content-Type: application/json" -u admin:admin "${grafana_host}/api/datasources" -X POST -d "@${payload}"

# deploy openshift dashboard
dashboard_file="./openshift-cluster-monitoring.json"
sed -i.bak "s/Xs/${graph_granularity}/" "${dashboard_file}"
sed -i.bak "s/\${DS_PR}/${datasource_name}/" "${dashboard_file}"
curl -H "Content-Type: application/json" -u admin:admin "${grafana_host}/api/dashboards/db" -X POST -d "@${dashboard_file}"
mv "${dashboard_file}.bak" "${dashboard_file}"

((node_exporter)) && node::exporter || echo "skip node exporter"

exit 0
