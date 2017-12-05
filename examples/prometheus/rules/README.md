# Prometheus and Alertmanager Rules

## Loading Rules

With this deployment method all files in the rules directory are mounted into the pod as a configmap.

1. Create a configmap of the rules directory

        oc create configmap base-rules --from-file=rules/
1. Attach the configmap to the prometheus statefulset as a volume

        oc volume statefulset/prometheus --add \
           --configmap-name=base-rules --name=base-rules -t configmap \
           --mount-path=/etc/prometheus/rules
1. Delete pod to restart with new configuration

        oc delete $(oc get pods -o name --selector='app=prometheus')

## Updating Rules

1. Edit or add a local rules file
1. Validate the rules directory. ('promtool' may be downloaded from the [Prometheus web site](https://prometheus.io/download/).)

        promtool check rules rules/*.rules
1. Update the configmap

        oc create configmap base-rules --from-file=rules/ --dry-run -o yaml | oc apply -f -
1. Delete pod to restart with new configuration

        oc delete $(oc get pods -o name --selector='app=prometheus')

