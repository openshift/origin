#!/bin/bash


CMD="chroot /host docker exec -it origin-master"

PODS=(docker-registry cockpit-kube)
for POD in "${PODS[@]}"
do
  echo "Scaling down ${POD}"
  $CMD oc scale dc ${POD} --replicas=0
done

SERVICES=(atomic-openshift-master)
for SERVICE in "${SERVICES[@]}"
do
  echo "Stopping and disabling system service ${SERVICE}..."
  chroot /host systemctl stop $SERVICE.service
  chroot /host systemctl disable $SERVICE.service
done

