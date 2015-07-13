#!/bin/bash

# Usage: plugin-migrate.sh [multitenant | subnet]

new_plugin=$1
old_plugin=

if [ $new_plugin == "multitenant" ]; then
  old_plugin=subnet
elif [ $new_plugin == "subnet" ]; then
  old_plugin=multitenant
else
  echo ERR: plugin $new_plugin unknown
  exit 1
fi

echo "#### Restarting master with $new_plugin"
vagrant ssh master -c "
sudo sed -ie s/openshift-ovs-$old_plugin/openshift-ovs-$new_plugin/g /lib/systemd/system/openshift-master.service 2>/dev/null
sudo systemctl daemon-reload 2>/dev/null
sudo systemctl restart openshift-master 2>/dev/null
"

for i in `seq 1 $OPENSHIFT_NUM_MINIONS`; do
  node_name=minion-$i
  vagrant ssh $node_name -c "
sudo sed -ie s/openshift-ovs-$old_plugin/openshift-ovs-$new_plugin/g /openshift.local.config/node-openshift-$node_name/node-config.yaml 2>/dev/null
sudo systemctl daemon-reload 2>/dev/null
"
  echo "### Restarting $node_name with $new_plugin"
  vagrant ssh $node_name -c "
openshift admin manage-node openshift-$node_name --schedulable=false 2>/dev/null
sudo ip link del lbr0 2>/dev/null
sudo systemctl restart openshift-node 2>/dev/null
openshift admin manage-node openshift-$node_name --schedulable=true 2>/dev/null
"
done
