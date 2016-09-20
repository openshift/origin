#!/bin/bash

# This script will find any network connectivity issues with OpenShift SDN.
# It performs: pod<->pod, pod<->node, pod<->service, pod<->external checks.
# Assumes: SSH access to openshift master and openshift nodes.

# die() echoes args to stderr and exits
function die() {
    echo "$@" 1>&2
    exit 1
}
readonly -f die

# echo_and_eval() echoes the command provided as $@ and then runs it
function echo_and_eval() {
    echo "> $@"
    echo ""
    eval "$@"
}
readonly -f echo_and_eval

# try_eval() runs the command provided as $@, and either returns silently with
# status 0 or else logs an error message with the command's output
function try_eval() {
    local tmpfile status

    tmpfile="$(mktemp)"
    if ! eval "$@" >& "${tmpfile}"; then
        status=1
        echo "ERROR: Could not run '$@':"
        sed -e 's/^/  /' "${tmpfile}"
        echo ""
    else
        status=0
    fi

    rm -f "${tmpfile}"
    return ${status}
}
readonly -f try_eval

# filter_env() will purge sensitive information like passwords or private keys
function filter_env() {
    awk '
        / env:$/ {
                    indent = index($0, "e");
                    skipping = 1;
                    next
                 }
                 !skipping {
                    print;
                 } skipping {
                    ch = substr($0, indent, 1);
                    if (ch != " " && ch != "-") {
                        skipping = 0;
                        print
                    }
                }
    '
}

function log_service() {
    local logpath=$1
    local service=$2
    local start_args=$3

    echo_and_eval  journalctl -u "${service}" ${JOURNALCTL_OPTIONS}   &> "${logpath}/journal-${service}"
    echo_and_eval  systemctl show "${service}"                        &> "${logpath}/systemctl-show-${service}"

    local config_file
    config_file="$(get_config_path_from_service "${start_args}" "${service}")"
    if [[ -f "${config_file}" ]]; then
        echo_and_eval  cat "${config_file}" &> "${logpath}/CONFIG-${service}"
    fi
}

function log_system() {
    local logpath=$1

    echo_and_eval  journalctl --boot ${JOURNALCTL_OPTIONS}            &> "${logpath}/journal-boot"
    echo_and_eval  nmcli --nocheck -f all dev show                    &> "${logpath}/nmcli-dev"
    echo_and_eval  nmcli --nocheck -f all con show                    &> "${logpath}/nmcli-con"
    echo_and_eval  head -1000 /etc/sysconfig/network-scripts/ifcfg-*  &> "${logpath}/ifcfg"
    echo_and_eval  ip addr show                                       &> "${logpath}/addresses"
    echo_and_eval  ip route show                                      &> "${logpath}/routes"
    echo_and_eval  ip neighbor show                                   &> "${logpath}/arp"
    echo_and_eval  iptables-save                                      &> "${logpath}/iptables"
    echo_and_eval  cat /etc/hosts                                     &> "${logpath}/hosts"
    echo_and_eval  cat /etc/resolv.conf                               &> "${logpath}/resolv.conf"
    echo_and_eval  lsmod                                              &> "${logpath}/modules"
    echo_and_eval  sysctl -a                                          &> "${logpath}/sysctl"

    echo_and_eval  oc version                                         &> "${logpath}/version"
    echo                                                             &>> "${logpath}/version"
    echo_and_eval  docker version                                    &>> "${logpath}/version"
    echo                                                             &>> "${logpath}/version"
    echo_and_eval  cat /etc/system-release-cpe                       &>> "${logpath}/version"
}

function remove_formatting() {
    template=$1
    echo "$(echo "${template}" | tr -d '\n' | sed -e 's/} \+/}/g')"
}

function do_master() {
    local nodes
    nodes="$(oc get nodes --template '{{range .items}}{{.spec.externalID}} {{end}}')"
    if [[ -z "${nodes}" ]]; then
        echo "No nodes found"
    fi

    mkdir -p "${MASTER_DIR}"

    # Grab master service logs and config files
    if [[ -n "${AOS_MASTER_SERVICE}" ]]; then
        log_service "${MASTER_DIR}" "${AOS_MASTER_SERVICE}" "master"
    fi
    if [[ -n "${AOS_MASTER_CONTROLLERS_SERVICE}" ]]; then
        log_service "${MASTER_DIR}" "${AOS_MASTER_CONTROLLERS_SERVICE}" "master controllers"
    fi
    if [[ -n "${AOS_MASTER_API_SERVICE}" ]]; then
        log_service "${MASTER_DIR}" "${AOS_MASTER_API_SERVICE}" "master api"
    fi

    # Log the generic system stuff
    log_system "${MASTER_DIR}"

    # And the master specific stuff
    echo_and_eval  oc get nodes                      -o yaml               &> "${MASTER_DIR}/nodes"
    echo_and_eval  oc get pods      --all-namespaces -o yaml  | filter_env &> "${MASTER_DIR}/pods"
    echo_and_eval  oc get services  --all-namespaces -o yaml               &> "${MASTER_DIR}/services"
    echo_and_eval  oc get endpoints --all-namespaces -o yaml               &> "${MASTER_DIR}/endpoints"
    echo_and_eval  oc get routes    --all-namespaces -o yaml               &> "${MASTER_DIR}/aos_routes"
    echo_and_eval  oc get clusternetwork             -o yaml               &> "${MASTER_DIR}/clusternetwork"
    echo_and_eval  oc get hostsubnets                -o yaml               &> "${MASTER_DIR}/hostsubnets"
    echo_and_eval  oc get netnamespaces              -o yaml               &> "${MASTER_DIR}/netnamespaces"

    local reg_ip resolv_ip
    read -d '' template <<EOF
    {{range .status.addresses}}
        {{if eq .type "InternalIP"}}
            {{.address}}
        {{end}}
    {{end}}
EOF
    template="$(remove_formatting "${template}")"
    for node in ${nodes}; do
        reg_ip="$(oc get node "${node}" --template "${template}")"
        if [[ -z "${reg_ip}" ]]; then
            echo "Node ${node}: no IP address in OpenShift"
            continue
        fi

        resolv_ip="$(getent ahostsv4 "${node}" | awk '/STREAM/ { print $1; exit; }')"

        if [[ "${reg_ip}" != "${resolv_ip}" ]]; then
            echo "Node ${node}: the IP in OpenShift (${reg_ip}) does not match DNS/hosts (${resolv_ip})"
        fi

        try_eval ping -c1 -W2 "${node}"
    done

    # Outputs a list of nodes in the form "nodename IP"
    read -d '' node_template <<EOF
    {{range .items}}
        {{\$name := .metadata.name}}
        {{range .status.addresses}}
            {{if eq .type "InternalIP"}}
                {{\$name}}{{printf " "}}
                {{.address}}
                {{"\\\n"}}
            {{end}}
        {{end}}
    {{end}}
EOF
    node_template="$(remove_formatting "${node_template}")"
    oc get nodes --template "${node_template}" > "${META_DIR}/nodeinfo"

    # Outputs a list of pods in the form "minion-1 172.17.0.1 mypod namespace 10.1.0.2 e4f1d61b"
    read -d '' pod_template <<EOF
    {{range .items}}
        {{if .status.containerStatuses}}
            {{if (index .status.containerStatuses 0).ready}}
                {{if not .spec.hostNetwork}}
                    {{.spec.nodeName}}{{printf " "}}
                    {{.status.hostIP}}{{printf " "}}
                    {{.metadata.name}}{{printf " "}}
                    {{.metadata.namespace}}{{printf " "}}
                    {{.status.podIP}}{{printf " %.21s" (index .status.containerStatuses 0).containerID}}
                    {{"\\\n"}}
                {{end}}
            {{end}}
        {{end}}
    {{end}}
EOF
    pod_template="$(remove_formatting "${pod_template}")"
    oc get pods --all-namespaces --template "${pod_template}" | sed -e 's|docker://||' > "${META_DIR}/podinfo"

    # Outputs a list of services in the form "myservice namespace 172.30.0.99 tcp 5454"
    read -d '' service_template <<EOF
    {{range .items}}
        {{if ne .spec.clusterIP "None"}}
            {{.metadata.name}}{{printf " "}}
            {{.metadata.namespace}}{{printf " "}}
            {{.spec.clusterIP}}{{printf " "}}
            {{(index .spec.ports 0).protocol}}{{printf " "}}
            {{(index .spec.ports 0).port}}
            {{"\\\n"}}
        {{end}}
    {{end}}
EOF
    service_template="$(remove_formatting "${service_template}")"
    oc get services --all-namespaces --template "${service_template}" | sed -e 's/ TCP / tcp /g' -e 's/ UDP / udp /g' > "${META_DIR}/serviceinfo"
}

function get_port_for_addr() {
    local addr=$1
    local lognode=$2

    # The nw_src line works with all current installs. The nw_dst line is needed for
    # older installs using the original single-tenant rules.
    sed -n -e "s/.*in_port=\([0-9]*\).*nw_src=${addr}.*/\1/p" \
           -e "s/.*nw_dst=${addr}.*output://p" "${lognode}/flows" | head -1
}

function get_vnid_for_addr() {
    local addr=$1
    local lognode=$2

    # On multitenant, the sed will match, and output something like "xd1", which we prefix
    # with "0" to get "0xd1". On non-multitenant, the sed won't match, and outputs nothing,
    # which we prefix with "0" to get "0". So either way, $base_pod_vnid is correct.
    echo "0$(sed -ne "s/.*reg0=0\(x[^,]*\),.*nw_dst=${addr}.*/\1/p" "${lognode}/flows" | head -1)"
}

function do_pod_to_pod_connectivity_check() {
    local where=$1
    local namespace=$2
    local base_pod_name=$3
    local base_pod_addr=$4
    local base_pod_pid=$5
    local base_pod_port=$6
    local base_pod_vnid=$7
    local base_pod_ether=$8
    local other_pod_name=$9
    local other_pod_addr=${10}
    local other_pod_nodeaddr=${11}
    local lognode=${12}

    echo "${where} pod, ${namespace} namespace:" | tr '[a-z]' '[A-Z]'
    echo ""

    local other_pod_port other_pod_vnid in_spec
    other_pod_port="$(get_port_for_addr "${other_pod_addr}" "${lognode}")"
    if [[ -n "${other_pod_port}" ]]; then
        other_pod_vnid="$(get_vnid_for_addr "${other_pod_addr}" "${lognode}")"
        in_spec="in_port=${other_pod_port}"
    else
        case "${namespace}" in
            default)
                other_pod_vnid=0
                ;;
            same)
                other_pod_vnid=${base_pod_vnid}
                ;;
            different)
                # VNIDs 1-10 are currently unused, so this is always different from $base_pod_vnid
                other_pod_vnid=6
                ;;
        esac
        in_spec="in_port=1,tun_src=${other_pod_nodeaddr},tun_id=${other_pod_vnid}"
    fi

    echo "${base_pod_name} -> ${other_pod_name}"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=${other_pod_addr}"
    echo ""
    echo "${other_pod_name} -> ${base_pod_name}"
    echo_and_eval ovs-appctl ofproto/trace br0 "${in_spec},ip,nw_src=${other_pod_addr},nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    if nsenter -n -t "${base_pod_pid}" -- ping -c 1 -W 2 "${other_pod_addr}"  &> /dev/null; then
        echo "ping ${other_pod_addr} -> success"
    else
        echo "ping ${other_pod_addr} -> failed"
    fi

    echo ""
    echo ""
}

function do_pod_external_connectivity_check() {
    local base_pod_name=$1
    local base_pod_addr=$2
    local base_pod_pid=$3
    local base_pod_port=$4
    local base_pod_vnid=$5
    local base_pod_ether=$6

    echo "EXTERNAL TRAFFIC:"
    echo ""
    echo "${base_pod_name} -> example.com"
    # This address is from a range which is reserved for documentation examples
    # (RFC 5737) and not allowed to be used in private networks, so it should be
    # guaranteed to only match the default route.
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=198.51.100.1"
    echo ""
    echo "example.com -> ${base_pod_name}"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=2,ip,nw_src=198.51.100.1,nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    if nsenter -n -t "${base_pod_pid}" -- ping -c 1 -W 2 "www.google.com"  &> /dev/null; then
        echo "ping www.google.com -> success"
    else
        echo "ping www.google.com -> failed"
    fi
}

function do_pod_service_connectivity_check() {
    local namespace=$1
    local base_pod_name=$2
    local base_pod_addr=$3
    local base_pod_pid=$4
    local base_pod_port=$5
    local base_pod_vnid=$6
    local base_pod_ether=$7
    local service_name=$8
    local service_addr=$9
    local service_proto=${10}
    local service_port=${11}

    echo "service, ${namespace} namespace:" | tr '[a-z]' '[A-Z]'
    echo ""

    echo "${base_pod_name} -> ${service_name}"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},${service_proto},nw_src=${base_pod_addr},nw_dst=${service_addr},${service_proto}_dst=${service_port}"
    echo ""
    echo "${service_name} -> ${base_pod_name}"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=2,${service_proto},nw_src=${service_addr},nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    # In bash, redirecting to /dev/tcp/HOST/PORT or /dev/udp/HOST/PORT opens a connection
    # to that HOST:PORT. Use this to test connectivity to the service; we can't use ping
    # like in the pod connectivity check because only connections to the correct port
    # get redirected by the iptables rules.
    if nsenter -n -t "${base_pod_pid}" -- timeout 1 bash -c "echo -n '' > /dev/${service_proto}/${service_addr}/${service_port} 2>/dev/null"; then
        echo "connect ${service_addr}:${service_port} -> success"
    else
        echo "connect ${service_addr}:${service_port} -> failed"
    fi

    echo ""
    echo ""
}

function get_config_path_from_service() {
    # 'node', 'master', 'master api', 'master controllers'
    local service_type=$1
    # 'atomic-openshift-node.service', 'atomic-openshift-master-controllers.service', etc
    local service_name=$2
    local config varfile

    config="$(ps wwaux | grep -v grep | sed -ne "s/.*openshift start ${service_type} --.*config=\([^ ]*\.yaml\).*/\1/p")"
    if [[ -z "${config}" ]]; then
        config="$(systemctl show -p ExecStart "${service_name}" | sed -ne 's/.*--config=\([^ ]*\).*/\1/p')"
        if [[ "${config}" == "\${CONFIG_FILE}" ]]; then 
            varfile="$(systemctl show "${service_name}" | grep EnvironmentFile | sed -ne 's/EnvironmentFile=\([^ ]*\).*/\1/p')"
            if [[ -f "${varfile}" ]]; then
                config="$(cat "${varfile}" | sed -ne 's/CONFIG_FILE=//p')"
            fi
        fi
    fi
    echo "${config}"
}

function do_node() {
    local config_file

    config_file="$(get_config_path_from_service "node" "${AOS_NODE_SERVICE}")"
    if [[ -z "${config_file}" ]]; then
        die "Could not find node-config.yaml from 'ps' or 'systemctl show'"
    fi

    local node
    node=$(sed -ne 's/^nodeName: //p' "${config_file}")
    if [[ -z "${node}" ]]; then
        die "Could not find node name in ${config_file}"
    fi

    local lognode="${NODE_DIR}/${node}"
    mkdir -p "${lognode}"

    # Grab node service logs and config file
    log_service "${lognode}" "${AOS_NODE_SERVICE}" "node"

    # Log the generic system stuff
    log_system "${lognode}"

    # Log some node-only information
    echo_and_eval  brctl show                              &> "${lognode}/bridges"
    echo_and_eval  docker ps -a                            &> "${lognode}/docker-ps"
    echo_and_eval  ovs-ofctl -O OpenFlow13 dump-flows br0  &> "${lognode}/flows"
    echo_and_eval  ovs-ofctl -O OpenFlow13 show br0        &> "${lognode}/ovs-show"
    echo_and_eval  tc qdisc show                           &> "${lognode}/tc-qdisc"
    echo_and_eval  tc class show                           &> "${lognode}/tc-class"
    echo_and_eval  tc filter show                          &> "${lognode}/tc-filter"
    echo_and_eval  systemctl cat docker.service            &> "${lognode}/docker-unit-file"
    echo_and_eval  cat "$(systemctl cat docker.service | grep EnvironmentFile.\*openshift-sdn | awk -F=- '{print $2}')" \
                                                           &> "${lognode}/docker-network-file"

    # Iterate over all pods on this node, and log some data about them.
    # Remember the name, address, namespace, and pid of the first pod we find on
    # this node which is not in the default namespace
    local base_pod_addr
    while read pod_node pod_nodeaddr pod_name pod_ns pod_addr pod_id; do
        if [[ "${pod_node}" != ${node} ]]; then
            continue
        fi

        local logpod="${lognode}/pods/${pod_name}"
        mkdir -p "${logpod}"

        local pid
        pid="$(docker inspect -f '{{.State.Pid}}' "${pod_id}")"
        if [[ -z "${pid}" ]]; then
            echo "${node}:${pod_name}: could not find pid of ${pod}"
            continue
        fi

        echo_and_eval nsenter -n -t "${pid}" -- ip addr  show  &> "${logpod}/addresses"
        echo_and_eval nsenter -n -t "${pid}" -- ip route show  &> "${logpod}/routes"

        # If we haven't found a local pod yet, or if we have, but it's
        # in the default namespace, then make this the new base pod.
        if [[ -z "${base_pod_addr}" || "${base_pod_ns}" = "default" ]]; then
            base_pod_addr="${pod_addr}"
            base_pod_ns="${pod_ns}"
            base_pod_name="${pod_name}"
            base_pod_pid="${pid}"
        fi
    done < "${META_DIR}/podinfo"

    if [[ "${SKIP_TESTS}" = "true" ]]; then
        echo "Skipping all pod, service and external connectivity tests as indicated"
        return
    fi

    if [[ -z "${base_pod_addr}" ]]; then
        echo "No pods on ${node}, so no connectivity tests"
        return
    fi

    local base_pod_port
    base_pod_port="$(get_port_for_addr "${base_pod_addr}" "${lognode}")"
    if [[ -z "${base_pod_port}" ]]; then
        echo "Could not find port for ${base_pod_addr}!"
        return
    fi
    local base_pod_vnid
    base_pod_vnid="$(get_vnid_for_addr "${base_pod_addr}" "${lognode}")"
    if [[ -z "${base_pod_vnid}" ]]; then
        echo "Could not find VNID for ${base_pod_addr}!"
        return
    fi
    local base_pod_ether
    base_pod_ether="$(nsenter -n -t "${base_pod_pid}" -- ip a | sed -ne "s/.*link.ether \([^ ]*\) .*/\1/p")"
    if [[ -z "${base_pod_ether}" ]]; then
        echo "Could not find MAC address for ${base_pod_addr}!"
        return
    fi

    unset did_local_default   did_local_same   did_local_different
    unset did_remote_default  did_remote_same  did_remote_different
    unset did_service_default did_service_same did_service_different
    if [[ "${base_pod_ns}" = "default" ]]; then
        # These would be redundant with the "default" tests
        did_local_same=1
        did_remote_same=1
        did_service_same=1
    fi

    # Now find other pods of various types to test connectivity against
    touch "${lognode}/pod-connectivity"
    local where namespace
    while read pod_node pod_nodeaddr pod_name pod_ns pod_addr pod_id; do
        if [[ "${pod_addr}" = "${base_pod_addr}" ]]; then
            continue
        fi

        if [[ "${pod_node}" = "${node}" ]]; then
            where="local"
        else
            where="remote"
        fi
        if [[ "${pod_ns}" = "default" ]]; then
            namespace="default"
        elif [[ "${pod_ns}" = "${base_pod_ns}" ]]; then
            namespace="same"
        else
            namespace="different"
        fi

        if [[ "$(eval echo \$did_${where}_${namespace})" = 1 ]]; then
            continue
        fi

        do_pod_to_pod_connectivity_check "${where}" "${namespace}" \
                         "${base_pod_name}" "${base_pod_addr}" \
                         "${base_pod_pid}" "${base_pod_port}" \
                         "${base_pod_vnid}" "${base_pod_ether}" \
                         "${pod_name}" "${pod_addr}" "${pod_nodeaddr}" \
                         "${lognode}" &>> "${lognode}/pod-connectivity"
        eval did_${where}_${namespace}=1
    done < "${META_DIR}/podinfo"

    do_pod_external_connectivity_check "${base_pod_name}" "${base_pod_addr}" \
                       "${base_pod_pid}" "${base_pod_port}" \
                       "${base_pod_vnid}" "${base_pod_ether}" \
                       &>> "${lognode}/pod-connectivity"

    # And now for services
    touch "${lognode}/service-connectivity"
    while read service_name service_ns service_addr service_proto service_port; do
        if [[ "${service_ns}" = "default" ]]; then
            namespace="default"
        elif [[ "${service_ns}" = "${base_pod_ns}" ]]; then
            namespace="same"
        else
            namespace="different"
        fi

        if [[ "$(eval echo \$did_service_${namespace})" = 1 ]]; then
            continue
        fi

        do_pod_service_connectivity_check "${namespace}" \
                          "${base_pod_name}" "${base_pod_addr}" \
                          "${base_pod_pid}" "${base_pod_port}" \
                          "${base_pod_vnid}" "${base_pod_ether}" \
                          "${service_name}" "${service_addr}" "${service_proto}" "${service_port}" \
                          &>> "${lognode}/service-connectivity"
        eval did_service_${namespace}=1
    done < "${META_DIR}/serviceinfo"
}

function run_self_via_ssh() {
    local mode=$1
    local host=$2
    local args=$3

    if ! try_eval ssh ${SSH_OPTS} "root@${host}" /bin/true; then
        return 1
    fi

    if ! try_eval ssh ${SSH_OPTS} "root@${host}" mkdir -m 0700 -p "${LOGDIR}"; then
        return 1
    fi

    if ! try_eval scp -pr ${SSH_OPTS} "${META_DIR}" "root@${host}:${LOGDIR}"; then
        return 1
    fi

    ssh ${SSH_OPTS} "root@${host}" /bin/bash "${META_DIR}/$(basename "${PROGRAM_PATH}") ${mode} ${args}"
}

function do_master_and_nodes() {
    echo "Analyzing master"

    if [[ -z "${HOST}" ]]; then
        do_master
    else
        if run_self_via_ssh --master "${HOST}" ${CMD_OPTIONS} < /dev/null; then
            try_eval scp ${SSH_OPTS} -pr "root@${HOST}:${LOG_DIR}" "${LOGDIR}"
        else
            return 1
        fi
    fi

    while read name addr; do
        echo ""
        echo "Analyzing ${name} (${addr})"

        if ip addr show | grep -q "inet ${addr}/"; then
            # Running on master which is also a node
            /bin/bash "${META_DIR}/$(basename "${PROGRAM_PATH}")" --node ${CMD_OPTIONS}
        else
            run_self_via_ssh --node "${addr}" ${CMD_OPTIONS} < /dev/null && \
            try_eval scp ${SSH_OPTS} -pr "root@$addr:${NODE_DIR}" "${LOGDIR}"
        fi
    done < "${META_DIR}/nodeinfo"
}

function check_openshift_services() {
    for systemd_dir in /etc/systemd/system /usr/lib/systemd/system; do
        for name in openshift origin atomic-openshift; do
            if [[ -f "${systemd_dir}/${name}-master.service" ]]; then
                AOS_MASTER_SERVICE="${name}-master.service"
            fi
            if [[ -f "${systemd_dir}/${name}-master-controllers.service" ]]; then
                AOS_MASTER_CONTROLLERS_SERVICE="${name}-master-controllers.service"
            fi
            if [[ -f "${systemd_dir}/${name}-master-api.service" ]]; then
                AOS_MASTER_API_SERVICE="${name}-master-api.service"
            fi
            if [[ -f "${systemd_dir}/${name}-node.service" ]]; then
                AOS_NODE_SERVICE="${name}-node.service"
            fi
        done
    done
}

function get_program_path() {
    local pgm_path

    case "$PROGRAM" in
        /*)
            pgm_path=$PROGRAM
            ;;
        *)
            pgm_path="$(pwd)/$PROGRAM"
            ;;
    esac
    echo "${pgm_path}"
}

function init_parse_arguments() {
    AOS_MASTER_SERVICE=""
    AOS_MASTER_CONTROLLERS_SERVICE=""
    AOS_MASTER_API_SERVICE=""
    AOS_NODE_SERVICE=""
    HOST=""

    CHECK_NODE=false
    CHECK_MASTER=false
    SKIP_TESTS=false
    JOURNALCTL_OPTIONS="-r -n 4000"

    # Propagate --skip-tests, --no-limit options to all the nodes
    CMD_OPTIONS=""

    local num_args=$#
    for((i=1; i <= num_args; i++)); do
        case "$1" in
            --node)
                CHECK_NODE=true
                ;;
            --master)
                CHECK_MASTER=true
                ;;
            --skip-tests)
                SKIP_TESTS=true
                CMD_OPTIONS="--skip-tests $CMD_OPTIONS"
                ;;
            --no-limit)
                JOURNALCTL_OPTIONS=""
                CMD_OPTIONS="--no-limit $CMD_OPTIONS"
                ;;
            *)
                # Master or node address
                HOST=$1
                ;;
        esac
        shift
    done
}

function create_dumpfile() {
    local dumpname="openshift-sdn-debug-$(date --iso-8601).tgz"
    (cd "${LOGDIR}"; tar -cf - --transform='s/^\./openshift-sdn-debug/' .) | gzip -c > "${dumpname}"
    echo ""
    echo "Output: ${dumpname}"
}

function set_master_node_logdir() {
    META_DIR="${LOGDIR}/${META}"
    MASTER_DIR="${LOGDIR}/master"
    NODE_DIR="${LOGDIR}/nodes"
}

######## Main program starts here

readonly PROGRAM=$0
readonly PROGRAM_PATH="$(get_program_path)"
readonly SSH_OPTS="-o StrictHostKeyChecking=no -o PasswordAuthentication=no"

# openshift services
declare AOS_MASTER_SERVICE AOS_MASTER_CONTROLLERS_SERVICE AOS_MASTER_API_SERVICE AOS_NODE_SERVICE
# network debug dirs
declare LOGDIR META_DIR MASTER_DIR NODE_DIR
# network debug script command line options
declare CHECK_NODE CHECK_MASTER SKIP_TESTS JOURNALCTL_OPTIONS HOST CMD_OPTIONS

META="meta"

init_parse_arguments "$@"
check_openshift_services

if [[ "${CHECK_NODE}" = "true" || "${CHECK_MASTER}" = "true" ]]; then
    LOGDIR="$(dirname $PROGRAM | sed -e "s|\/"${META}"$||")"
    set_master_node_logdir

    if [[ "${CHECK_NODE}" = "true" ]]; then
        do_node
    elif [[ "${CHECK_MASTER}" = "true" ]]; then
        do_master
    fi
    exit 0
fi

if [[ -z "${AOS_MASTER_SERVICE}" ]]; then
    echo "Usage:"
    echo "  [from master]"
    echo "    $PROGRAM"
    echo "  Gathers data on the master and then connects to each node via ssh"
    echo ""
    echo "  [from any other machine]"
    echo "    $PROGRAM MASTER-NAME"
    echo "  Connects to MASTER-NAME via ssh and then connects to each node via ssh"
    echo ""
    echo "  The machine you run from must be able to ssh to each other machine"
    echo "  via ssh with no password."
    exit 1
fi

LOGDIR="$(mktemp --tmpdir -d openshift-sdn-debug-XXXXXXXXX)"
set_master_node_logdir
mkdir "${META_DIR}" "${MASTER_DIR}" "${NODE_DIR}"
cp "${PROGRAM_PATH}" "${META_DIR}"
do_master_and_nodes |& tee "${LOGDIR}/log"
create_dumpfile
