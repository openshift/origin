# grafana-ocp

This template creates a custom Grafana instance preconfigured to gather Prometheus openshift metrics.
It is uses OAuth token to login openshift Prometheus.

# Examples Dashboards
#### Use node export with Grafana:
```
curl -H "Content-Type: application/json" -u admin:admin "${grafana_host}/api/dashboards/db" -X POST -d "@./node-exporter-full-dashboard.json"
```


## To deploy grafana
Note: make sure to have openshift prometheus deployed.
(https://github.com/openshift/origin/tree/master/examples/prometheus)

1. ```oc create namespace grafana```
2. ```oc new-app -f grafana-ocp.yaml```
3. grab the grafana url ``` oc get route |awk 'NR==2 {print $2}' ```
4. grab the ocp token, from openshift master run: ```oc sa get-token prometheus -n kube-system```
5. browse to grafana datasource's and add new prometheus datasource. 
6. grab the prometheus url via ```oc get route -n kube-system prometheus |awk 'NR==2 {print $2}'``` and paste the prometheus url e.g https://prometheus-kube-system.apps.example.com
7. paste the token string at the token field.
8. checkout the TLS checkbox.
9. save & test and make sure all green.

### Pull standalone docker grafana instance
to build standalone docker instance see
https://github.com/mrsiano/grafana-ocp

#### Resources 
- example video https://youtu.be/srCApR_J3Os
- deploy openshift prometheus https://github.com/openshift/origin/tree/master/examples/prometheus 
