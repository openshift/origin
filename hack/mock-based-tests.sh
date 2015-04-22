#!/bin/bash

openshift admin new-project mock --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"

osc project mock

# upload the template
osc create -f examples/mock-app-template/mock-template.json -n mock

# create the config

# create the new app
osc new-app mock-app-sample -p MOCK_NAME=mock1

# wait for
echo "[INFO] Waiting pod mock"
wait_for_command "osc get -n mock pods -l name=mock-app-pod | grep -i Running" $((60*TIME_SEC))

echo "[INFO] Waiting service mock"
wait_for_command "osc get -n mock services | grep mock-service" $((20*TIME_SEC))
MOCK_IP=$(osc get -n mock -o template --template="{{ .spec.portalIP }}" service mock-service)
echo "[INFO] IP Mock Service $MOCK_IP ."

echo "[INFO] Setting in-memory value for the container "
# put "value1" for key "key1"
wait_for_command '[[ "$(curl -s http://${MOCK_IP}:5018/put/key1/value1)" = "ok" ]]' $((60*TIME_SEC))
# asserting value is well "value1"
wait_for_command '[[ "$(curl -s http://${MOCK_IP}:5018/get/key1)" = "value1" ]]' $((60*TIME_SEC))

get_container_id () {
    echo $(osc get -o template pod $1 --template='{{range .status.containerStatuses}}{{.containerID}}{{end}}')
}

# Kill the only pod container
POD_NAME=$(curl -s http://${MOCK_IP}:5018/hostname)
CONTAINER_ID=$(get_container_id ${POD_NAME})
# $(osc get -o template pod ${POD_NAME} --template="{{range .status.containerStatuses}}{{.containerID}}{{end}}")
echo "Pod Name ${POD_NAME}"
echo "Container Id ${CONTAINER_ID}"

wait_for_command '[[ "$(curl -s http://${MOCK_IP}:5018/shutdown)" = "ok" ]]' $((60*TIME_SEC))

# asserting the containerid have changed

wait_for_command '[[ "$(get_container_id $POD_NAME)" != "${CONTAINER_ID}" ]]' $((60*TIME_SEC))

# asserting value for "key1" is not "value1" anymore as the container is dead.
wait_for_command '[[ "$(curl -s http://${MOCK_IP}:5018/get/key1)" = "" ]]' $((60*TIME_SEC))

# resize the rC mock-rc to 3 
osc scale -n mock --replicas=3 replicationcontrollers mock-rc

# we wait the effective resizing
mockrc_size=$(osc get -n mock pods -l name=mock-app-pod | grep mock-rc | wc -l)
wait_for_command '[[ "$mockrc_size" = "3" ]]' $((60*TIME_SEC))


# Test inter communication pod
# getting endoints like "172.17.0.62:8080,172.17.0.63:8080,172.17.0.64:8080"

# we need that endpoints have 3 ips,
# we'll count the ':' char, we need to have 3 of them
endpointstring=""
number_of_occurrences=0
cnt=0
until [ $number_of_occurrences = 3 ]; do

    endpointstring=$(osc get -n mock endpoints | grep mock-service |  awk '{print $2}')

    # we're looking of the number of char ":" in endpointstring
    number_of_occurrences=$(grep -o ":" <<< "$endpointstring" | wc -l) || true
    cnt=$((cnt+1))
    echo "[INFO] Waiting all 3 endpoints present. Actual = $number_of_occurrences."
    echo "[INFO]  $endpointstring"
    if [ $cnt = 19 ];
    then
        
        exit 2
        #error ${LINENO} "Too much time for endpoints to appear." 2
    fi
    
    sleep 3
done


declare -a endpoints=()
hostnames=""
echo "[INFO] endpointstring => $endpointstring"
i=0


# build this
#mock-rc-55lsa
#mock-rc-t0lb9
#mock-rc-zet3o


# Preparing Test intra pod communication
# mock-app allows chain communication between containers
# if we provide a special string, like this one
# #http://172.17.0.62:8080/chainredirect/http/172.17.0.62_8080_hostname-172.17.0.63_8080_hostname-172.17.0.64_8080_hostname

magic_string_suffix=""


# In the loop we 
# 1) asking each mock-app container in the pod its hostname directly via the endpoints
# 2) construct the string for the intra pod communication
for m in $(echo $endpointstring | tr "," "\n") ;
do
    endpoints[$i]=$m
    echo "[INFO] endpoint => ${endpoints[$i]}"
    
    # all 3 containers have "mock1" for name
    # asserting that
    wait_for_command '[[ "$(curl -s http://${endpoints[$i]}/name)" = "mock1" ]]' $((60*TIME_SEC))

    sep1=$'\n'
    sep2="-"
    if [[ $i == "0" ]];
    then
        sep1=""
        sep2=""
    fi
    # 1) concatening the 3 hostname directly asking them
    hostnames+="$sep1$(curl -s http://${endpoints[$i]}/hostname)"

    # 2) concatening the string for the intra pod commuication
    magic_string_suffix+="${sep2}${m/:/_}_hostname"
    
    i=$((i+1))
done

magic_string_prefix="http://${endpoints[0]}/chainredirect/http/"

magic_string="$magic_string_prefix$magic_string_suffix"

echo "[INFO] Chain call : $magic_string"

hostnames_via_intra_communication=$(curl -s $magic_string)

echo "[INFO] "
echo "[INFO] Hostnames : $hostnames ."
echo "[INFO] Hostnames : $hostnames_via_intra_communication ."

wait_for_command '[[ "$hostnames_via_intra_communication" = "$hostnames" ]]'
