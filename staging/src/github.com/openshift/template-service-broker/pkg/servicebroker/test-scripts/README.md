oc create -f clusterrole.yaml

./catalog.sh
./provision.sh
./lastoperation-provision.sh
./bind.sh
./unbind.sh
./deprovision.sh
