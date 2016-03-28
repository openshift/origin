#!/bin/bash

SERVICES=(atomic-openshift-master)
for SERVICE in "${SERVICES[@]}"
do
  echo "uninstalling system service ${SERVICE}..."
  chroot /host rm /etc/systemd/system/${SERVICE}.service
  chroot /host rm /etc/sysconfig/${SERVICE}
done

echo "Removing configuration files..."
DIRS=(/etc/origin /var/lib/origin)
for DIR in "${DIRS[@]}"
do
  chroot /host rm -rf ${DIR}
done

IMAGES=(openshift/origin openshift/origin-docker-registry cockpit/kubernetes)
#aweiteka/cockpit-registry

echo "Uninstallation complete."
echo "Stopped container and images have not been removed. To remove them manually run:"
echo "'sudo docker rm \$(sudo docker ps -qa)'"
echo "'sudo docker rmi ${IMAGES[*]}'"
