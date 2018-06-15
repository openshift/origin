# Openshift Grafana Dashboards

This example creates a custom Grafana instance preconfigured to gather Prometheus openshift metrics.
It uses "OAuth" token to login openshift Prometheus.

## Available Dashboards
- openshift cluster metrics
- node exporter metrics

## To run grafana and deploy dashboards
Note: make sure to have openshift prometheus deployed (possibly, with node exporter).
(https://github.com/openshift/origin/tree/master/examples/prometheus)

### Run the deployment script
``` 
./setup-grafana.sh -n <any_datasorce_name> -a -e
```
for more info ```./setup-grafana.sh -h```

#### Manual deployment for oauth proxy:
Note: when using oauth make sure your user has permission to browse grafana.
- add a openshift user htpasswd ```htpasswd -c /etc/origin/master/htpasswd gfadmin```
- use the HTPasswdPasswordIdentityProvider as described here - https://docs.openshift.com/enterprise/3.0/admin_guide/configuring_authentication.html 
- make sure point the provider file to /etc/origin/master/htpasswd.
  or using this example cmd:
  ```
  sed -ie 's|AllowAllPasswordIdentityProvider|HTPasswdPasswordIdentityProvider\n      file: /etc/origin/master/htpasswd|' /etc/origin/master/master-config.yaml
  ```
- add view role to user ```oc adm policy add-cluster-role-to-user cluster-reader gfadmin```
- restart master api ```systemctl restart atomic-openshift-master-api.service```
- get the grafana url by ```oc get route```
- discover your openshift dashboard.

#### Pull standalone docker grafana instance
to build standalone docker instance see
https://github.com/mrsiano/grafana-ocp

#### Resources 
- example video https://youtu.be/srCApR_J3Os
- deploy openshift prometheus https://github.com/openshift/origin/tree/master/examples/prometheus 
