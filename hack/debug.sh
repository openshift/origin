#!/bin/bash

# echoes args to stderr and exits
die () {
    echo "$*" 1>&2
    exit 1
}

# echoes the command provided as $@ and then runs it
echo_and_eval () {
    echo "> $*"
    echo ""
    eval "$@"
}

# runs the command provided as $@, and either returns silently with
# status 0 or else logs an error message with the command's output
try_eval () {
    tmpfile=`mktemp`
    if ! eval "$@" >& $tmpfile; then
	status=1
	echo "ERROR: Could not run '$*':"
	sed -e 's/^/  /' $tmpfile
	echo ""
    else
	status=0
    fi
    rm -f $tmpfile
    return $status
}

do_master () {
    if ! nodes=$(oc get nodes -t '{{range .items}}{{.spec.externalID}} {{end}}'); then
	die "Could not get list of nodes"
    fi

    logmaster=$logdir/master
    mkdir $logmaster
    echo_and_eval journalctl --boot >& $logmaster/journal-full
    echo_and_eval journalctl -u openshift-master.service >& $logmaster/journal-openshift
    echo_and_eval systemctl show openshift-master.service >& $logmaster/systemctl-show
    echo_and_eval nmcli --nocheck -f all dev show >& $logmaster/nmcli-dev
    echo_and_eval nmcli --nocheck -f all con show >& $logmaster/nmcli-con
    echo_and_eval head -1000 /etc/sysconfig/network-scripts/ifcfg-* >& $logmaster/ifcfg
    echo_and_eval ip addr show >& $logmaster/addresses
    echo_and_eval ip route show >& $logmaster/routes
    echo_and_eval iptables-save >& $logmaster/iptables
    echo_and_eval cat /etc/hosts >& $logmaster/hosts
    echo_and_eval oc get nodes -o json >& $logmaster/nodes
    echo_and_eval oc get pods --all-namespaces -o json >& $logmaster/pods

    for node in $nodes; do
	reg_ip=$(oc get node $node -t '{{range .status.addresses}}{{if eq .type "InternalIP"}}{{.address}}{{end}}{{end}}')
	if [ -z "$reg_ip" ]; then
	    echo "Node $node: no IP address in OpenShift"
	    continue
	fi

	resolv_ip=$(awk '/\s'$node'$/ { print $1; exit; }' /etc/hosts)
	if [ -z "$resolv_ip" ]; then
	    resolv_ip=$(host $node 2>/dev/null)
	    if [ -z "$resolv_ip" ]; then
		echo "Node $node: no IP address in either DNS or /etc/hosts"
	    fi
	fi

	if [ "$reg_ip" != "$resolv_ip" ]; then
	    echo "Node $node: the IP in OpenShift ($reg_ip) does not match DNS/hosts ($resolv_ip)"
	fi

	try_eval ping -c1 -W2 $node
    done

    oc get nodes -t '{{range .items}}{{range .status.addresses}}{{if eq .type "InternalIP"}}{{.address}} {{end}}{{end}}{{end}}' | tr ' ' '\012' > $logmaster/node-ips
}

# Returns a list of pods in the form "minion-1:mypod:namespace:10.1.0.2:e4f1d61b"
get_pods () {
    if ! pods=$(oc get pods --all-namespaces -t '{{range .items}}{{if .status.containerStatuses}}{{.spec.nodeName}}:{{.metadata.name}}:{{.metadata.namespace}}:{{.status.podIP}}:{{printf "%.21s" (index .status.containerStatuses 0).containerID}} {{end}}{{end}}'); then
	die "Could not get list of pods"
    fi
    echo $pods | sed -e 's/docker:\/\///g'
}

# Given the name of a variable containing a "podspec" like
# "minion-1:mypod:namespace:10.1.0.2:e4f1d61b", split into pieces
split_podspec () {
    prefix=$1
    spec=$(eval echo \${$prefix})

    array=(${spec//:/ })
    eval ${prefix}_node=${array[0]}
    eval ${prefix}_name=${array[1]}
    eval ${prefix}_ns=${array[2]}
    eval ${prefix}_addr=${array[3]}
    eval ${prefix}_id=${array[4]}
}

do_node_connectivity_check () {
    base_pod_addr=$1
    base_pod_name=$2
    base_pod_pid=$3
    local_same_addr=$4
    local_same_name=$5
    local_different_addr=$6
    local_different_name=$7
    remote_same_addr=$8
    remote_same_name=$9
    remote_different_addr=${10}
    remote_different_name=${11}

    base_pod_port=$(sed -ne "s/.*nw_dst=${base_pod_addr}.*output://p" $lognode/flows)
    if [ -z "$base_pod_port" ]; then
	echo "Could not find port for $base_pod_addr in dump-flow output"
	return
    fi
    # On multitenant, the sed will match and output, eg "xd1", which we prefix with "0"
    # to get "0xd1". On non-multitenant, the sed won't match, and outputs nothing, which
    # we prefix with "0" to get "0". So either way, $base_pod_vnid is correct.
    base_pod_vnid=0$(sed -ne "s/.*reg0=0\(x[^,]*\),.*nw_dst=${base_pod_addr}.*/\1/p" $lognode/flows)

    base_pod_ether=$(nsenter -n -t $base_pod_pid ip a | sed -ne "s/.*link.ether \([^ ]*\) .*/\1/p")
    if [ -z "$base_pod_port" ]; then
	echo "Could not find MAC address for $base_pod in 'ip addr' output"
	return
    fi

    if [ -n "$local_same_addr" ]; then
	echo "LOCAL, SAME NAMESPACE:"
	echo ""
	other_pod_port=$(sed -ne "s/.*nw_dst=${local_same_addr}.*output://p" $lognode/flows)
	if [ -n "$other_pod_port" ]; then
	    echo "$base_pod_name -> $local_same_name"
	    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=${local_same_addr}"
	    echo ""
	    echo "$local_same_name -> $base_pod_name"
	    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${other_pod_port},reg0=${base_pod_vnid},ip,nw_src=${local_same_addr},nw_dst=${base_pod_addr}"
	else
	    echo "Could not find port for ${local_same_addr}!"
	fi
	echo ""

	if nsenter -n -t $base_pod_pid ping -c 1 -W 2 $local_same_addr >& /dev/null; then
	    echo "ping $local_same_addr  ->  success"
	else
	    echo "ping $local_same_addr  ->  failed"
	fi

	echo ""
	echo ""
    fi

    if [ -n "$local_different_addr" ]; then
	echo "LOCAL, DIFFERENT NAMESPACE:"
	echo ""
	other_pod_port=$(sed -ne "s/.*nw_dst=${local_different_addr}.*output://p" $lognode/flows)
	other_pod_vnid=0$(sed -ne "s/.*reg0=0\(x[^,]*\),.*nw_dst=${local_different_addr}.*/\1/p" $lognode/flows)
	if [ -n "$other_pod_port" ]; then
	    echo "$base_pod_name -> $local_different_name"
	    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=${local_different_addr}"
	    echo ""
	    echo "$local_different_name -> $base_pod_name"
	    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${other_pod_port},reg0=${other_pod_vnid},ip,nw_src=${local_different_addr},nw_dst=${base_pod_addr}"
	else
	    echo "Could not find port for ${local_different_addr}!"
	fi
	echo ""

	if nsenter -n -t $base_pod_pid ping -c 1 -W 2 $local_different_addr >& /dev/null; then
	    echo "ping $local_different_addr  ->  success"
	else
	    echo "ping $local_different_addr  ->  failed"
	fi

	echo ""
	echo ""
    fi

    if [ -n "$remote_same_addr" ]; then
	echo "REMOTE, SAME NAMESPACE:"
	echo ""
	echo "$base_pod_name -> $remote_same_name"
	echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=${remote_same_addr}"
	echo ""
	echo "$remote_same_name -> $base_pod_name"
	echo_and_eval ovs-appctl ofproto/trace br0 "in_port=1,tun_id=${base_pod_vnid},ip,nw_src=${remote_same_addr},nw_dst=${base_pod_addr}"
	echo ""

	if nsenter -n -t $base_pod_pid ping -c 1 -W 2 $remote_same_addr >& /dev/null; then
	    echo "ping $remote_same_addr  ->  success"
	else
	    echo "ping $remote_same_addr  ->  failed"
	fi

	echo ""
	echo ""
    fi

    if [ -n "$remote_different_addr" ]; then
	echo "REMOTE, DIFFERENT NAMESPACE:"
	echo ""
	echo "$base_pod_name -> $remote_different_name"
	echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=${remote_different_addr}"
	echo ""
	# VNIDs 1-10 are currently unused
	echo "$remote_different_name -> $base_pod_name"
	echo_and_eval ovs-appctl ofproto/trace br0 "in_port=1,tun_id=6,ip,nw_src=${remote_different_addr},nw_dst=${base_pod_addr}"
	echo ""

	if nsenter -n -t $base_pod_pid ping -c 1 -W 2 $remote_different_addr >& /dev/null; then
	    echo "ping $remote_different_addr  ->  success"
	else
	    echo "ping $remote_different_addr  ->  failed"
	fi

	echo ""
	echo ""
    fi

    echo "EXTERNAL TRAFFIC:"
    echo ""
    echo "$base_pod_name -> example.com"
    # This address is from a range which is reserved for documentation examples
    # (RFC 5737) and not allowed to be used in private networks, so it should be
    # guaranteed to only match the default route.
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=198.51.100.1"
    echo ""
    echo "example.com -> $base_pod_name"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=2,ip,nw_src=198.51.100.1,nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    if nsenter -n -t $base_pod_pid ping -c 1 -W 2 www.redhat.com >& /dev/null; then
	echo "ping www.redhat.com  ->  success"
    else
	echo "ping www.redhat.com  ->  failed"
    fi
}

do_node () {
    config=$(systemctl show -p ExecStart openshift-node.service | sed -ne 's/.*--config=\([^ ]*\).*/\1/p')
    if [ -z "$config" ]; then
	die "Could not find node-config.yaml from systemctl status"
    fi
    node=$(sed -ne 's/^nodeName: //p' $config)
    if [ -z "$node" ]; then
	die "Could not find node name in $config"
    fi

    logdir=$(dirname $0)
    lognode=$logdir/nodes/$node
    mkdir -p $lognode
    echo_and_eval journalctl --boot >& $lognode/journal-full
    echo_and_eval journalctl -u openshift-node.service >& $lognode/journal-openshift
    echo_and_eval systemctl show openshift-node.service >& $lognode/systemctl-show
    echo_and_eval nmcli --nocheck -f all dev show >& $lognode/nmcli-dev
    echo_and_eval nmcli --nocheck -f all con show >& $lognode/nmcli-con
    echo_and_eval head -1000 /etc/sysconfig/network-scripts/ifcfg-* >& $lognode/ifcfg
    echo_and_eval ip addr show >& $lognode/addresses
    echo_and_eval ip route show >& $lognode/routes
    echo_and_eval iptables-save >& $lognode/iptables
    echo_and_eval cat /etc/hosts >& $logmaster/hosts
    echo_and_eval brctl show >& $lognode/bridges
    echo_and_eval ovs-ofctl -O OpenFlow13 dump-flows br0 >& $lognode/flows
    echo_and_eval ovs-ofctl -O OpenFlow13 show br0 >& $lognode/ovs-show

    first_pod_addr=
    local_same_addr=
    local_different_addr=
    remote_same_addr=
    remote_different_addr=

    # Iterate over all pods on this node, and log some data about them.
    # Remember the name, address, namespace, and pid of the first pod we find on
    # this node, and (if possible), the name and address of two additional pods
    # on this node, one in the same namespace and one in a different namespace.
    for pod in $(get_pods); do
	split_podspec pod
	if [ "$pod_node" != "$node" ]; then
	    continue
	fi

	logpod=$lognode/pods/$pod_name
	mkdir -p $logpod

	pid=$(docker inspect -f '{{.State.Pid}}' $pod_id)
	if [ -z "$pid" ]; then
	    echo "$node:$pod_name: could not find pid of ($pod)"
	    continue
	fi

	echo_and_eval nsenter -n -t $pid ip addr show >& $logpod/addresses
	echo_and_eval nsenter -n -t $pid ip route show >& $logpod/routes

	if [ -z "$first_pod_addr" ]; then
	    first_pod_addr=$pod_addr
	    first_pod_ns=$pod_ns
	    first_pod_name=$pod_name
	    first_pod_pid=$pid
	elif [ "$pod_ns" = "$first_pod_ns" -a -z "$local_same_addr" ]; then
	    local_same_addr=$pod_addr
	    local_same_name=$pod_name
	elif [ "$pod_ns" != "$first_pod_ns" -a -z "$local_different_addr" ]; then
	    local_different_addr=$pod_addr
	    local_different_name=$pod_name
	fi
    done

    if [ -z "$first_pod_addr" ]; then
	echo "No pods on $node, so no connectivity tests"
	return
    fi

    # Now find some remote pods to test against
    for pod in $pods; do
	split_podspec pod

	if [ "$pod_node" != "$node" ]; then
	    if [ "$pod_ns" = "$first_pod_ns" -a -z "$remote_same_addr" ]; then
		remote_same_addr=$pod_addr
		remote_same_name=$pod_name
	    elif [ "$pod_ns" != "$first_pod_ns" -a -z "$remote_different_addr" ]; then
		remote_different_addr=$pod_addr
		remote_different_name=$pod_name
	    fi
	fi
    done

    do_node_connectivity_check "$first_pod_addr" "$first_pod_name" "$first_pod_pid" \
			       "$local_same_addr" "$local_same_name" \
			       "$local_different_addr" "$local_different_name" \
			       "$remote_same_addr" "$remote_same_name" \
			       "$remote_different_addr" "$remote_different_name" \
			       >& $lognode/connectivity
}

run_self_via_ssh () {
    host=$1
    args=$2

    if ! try_eval ssh -o PasswordAuthentication=no root@$host /bin/true; then
	return
    fi

    if ! try_eval ssh root@$host mkdir -m 0700 $logdir; then
	return
    fi

    if ! try_eval scp $self root@$host:$logdir/debug.sh; then
	return
    fi

    extra_env=""
    if ! try_eval ssh root@$host oc get pods; then
	if [ -z "$KUBECONFIG" ]; then
	    return
	fi

	echo "Retrying with local kubeconfig"
	if ! try_eval scp $KUBECONFIG root@$host:$logdir/.kubeconfig; then
	    return
	fi
	extra_env="env KUBECONFIG=$logdir/.kubeconfig"
	if ! try_eval ssh root@$host $extra_env oc get pods; then
	    return
	fi
    fi

    ssh root@$host $extra_env $logdir/debug.sh $args
}

do_master_and_nodes ()
{
    master="$1"

    echo "Analyzing master"

    if [ -z "$master" ]; then
	do_master
    else
	run_self_via_ssh $master --master
	try_eval scp -pr root@$master:$logdir/master $logdir/
    fi

    nodes=$(cat $logdir/master/node-ips)
    for node in $nodes; do
	echo ""
	echo "Analyzing $node"

	run_self_via_ssh $node --node
	try_eval scp -pr root@$node:$logdir/nodes $logdir/
    done
}

########

case "$1" in
    --node)
	logdir=$(dirname $0)
	do_node
	exit 0
	;;

    --master)
	logdir=$(dirname $0)
	do_master
	exit 0
	;;

    "")
	if systemctl show -p LoadState openshift-master | grep -q 'not-found'; then
	    echo "Usage:"
	    echo "  [from master]"
	    echo "    $0"
	    echo "  Gathers data on the master and then connects to each node via ssh"
	    echo ""
	    echo "  [from any other machine]"
	    echo "    $0 MASTER-NAME"
	    echo "  Connects to MASTER-NAME via ssh and then connects to each node via ssh"
	    echo ""
	    echo "  The machine you run from must be able to ssh to each other machine"
	    echo "  via ssh with no password."
	    exit 1
	fi
	;;
esac

case "$0" in
    /*)
	self=$0
	;;
    *)
	self=$(pwd)/$0
	;;
esac

logdir=$(mktemp --tmpdir -d openshift-sdn-debug-XXXXXXXXX)
do_master_and_nodes "$1" |& tee $logdir/log

dumpname=openshift-sdn-debug-$(date --iso-8601).tgz
(cd $logdir; tar -cf - --transform='s/^\./openshift-sdn-debug/' .) | gzip -c > $dumpname
echo ""
echo "Output is in $dumpname"
