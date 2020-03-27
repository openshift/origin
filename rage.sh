#!/bin/bash
run_func(){
 ./_output/local/bin/darwin/amd64/openshift-tests run all --dry-run | grep -e "should create a ResourceQuota and capture the life of a secret" | ./_output/local/bin/darwin/amd64/openshift-tests run -f -
}

for i in {1..50}
do
 run_func $i &
done

wait
echo "done"
