## SDN solutions for Openshift

# WORK IN PROGRESS
# DO NOT USE THIS YET

#### Build and Install

	$ git clone https://github.com/openshift/openshift-sdn
	$ cd openshift-sdn
	$ make clean        # optional
	$ make              # build
	$ make install      # installs in /usr/bin

#### Try it out

##### Use vagrant, pre-define a cluster, and bring it up

Create an openshift cluster on your desktop using vagrant:

	$ git clone https://github.com/openshift/origin
	$ cd origin
	$ make clean
	$ export OPENSHIFT_DEV_CLUSTER=1
	$ export OPENSHIFT_NUM_MINIONS=2
	$ export OPENSHIFT_SDN=ovs-simple
	$ vagrant up

##### Manually add minions to a master

Steps to create manually create an OpenShift cluster with openshift-sdn. This requires that each machine (master, minions) have compiled openshift and openshift-sdn already. Check [here](https://github.com/openshift/origin) for OpenShift instructions. Also ensure 'openvswitch' is installed and running (yum install -y openvswitch && systemctl enable openvswitch && systemctl start openvswitch).

On OpenShift master,

	$ openshift start master  # start the master openshift server (also starts the etcd server by default)
	$ openshift-sdn           # assumes etcd is running at localhost:4001

To add a node to the cluster, do the following on the node:

	$ openshift-sdn -etcd-endpoints=http://openshift-master:4001 -minion -public-ip=<10.10....> -hostname <hostname>
	where, 
		-etcd-endpoints	: reach the etcd db here
		-minion 	: run it in minion mode (will watch etcd servers for new minion subnets)
		-public-ip	: use this field for suggesting the publicly reachable IP address of this minion
		-hostname	: the name that will be used to register the minion with openshift-master
	$ openshift start node --master=http://openshift-master:8080

Back on the master, to finally register the node:

	Create a json file for the new minion resource
        $ cat <<EOF > mininon-1.json
	{
		"kind":"Minion", 
		"id":"openshift-minion-1",
	 	"apiVersion":"v1beta1"
	}
	EOF
	where, openshift-minion-1 is a hostname that is resolvable from the master (or, create an entry in /etc/hosts and point it to the public-ip of the minion).
	$ openshift cli create -f minion-1.json

Done. Repeat last two pieces to add more nodes. Create new pods from the master (or just docker containers on the minions), and see that the pods are indeed reachable from each other. 

#### Performance Note

The current design has a long path for packets directed for the overlay network.
There are two veth-pairs, a linux bridge, and then the OpenVSwitch, that cause a drop in performance of about 40%

Hand-crafted solutions that eliminate the long-path to just a single veth-pair bring the performance close to the wire. The performance has been measured using sockperf.

  | openshift-sdn | openshift-sdn (optimized) | without overlay
--- | --------- | ------- | ------
Latency | 112us | 84us | 82us

#### TODO

 - Add more options, so that users can choose the subnet to give to the cluster. The default is hardcoded today to "10.1.0.0/16"
 - Performance enhancements, as discussed above
 - Usability without depending on openshift

