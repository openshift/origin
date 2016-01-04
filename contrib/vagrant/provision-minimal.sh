#!/bin/bash

function setup {
	set -euo pipefail
	IFS=$'\n\t'

	sed -i s/^Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers

	# patch incompatible with fail-over DNS setup
	SCRIPT='/etc/NetworkManager/dispatcher.d/fix-slow-dns'
	if [[ -f "${SCRIPT}" ]]; then
	    echo "Removing ${SCRIPT}..."
	    rm "${SCRIPT}"
	    sed -i -e '/^options.*$/d' /etc/resolv.conf
	fi
	unset SCRIPT

	if [ -f /usr/bin/generate_openshift_service ]
	then
	  sudo /usr/bin/generate_openshift_service
	fi
}

function installOpenShift {
<<<<<<< HEAD

        cd /data/src/github.com/openshift/origin
	 
	# make it first.  this might take a while.
	# so, we check if an installation is available already.
	if which openshift >/dev/null; then
	    echo "Openshift found, not building."
    	else	
	    echo "-- Starting build, memory:"
	    free -mh
	    make clean build
	fi

	echo "Starting openshift"
	sudo `which openshift` start --loglevel=5 --public-master=localhost &> openshift.log &
=======
	cd /data/src/github.com/openshift/origin
	echo "-- Starting build, memory:"
	free -mh
	# make clean build
	echo "Starting openshift"
	sudo `which openshift` start --public-master=localhost &> openshift.log &
>>>>>>> d4486238f79e7aa2c0e518ee53486cd3bc3388d0
	echo "-- Now starting as new user..."
	#oc logout
	#yes "j" | oc login
	echo "-- now checking who i am and creating project!"
	echo "sleeping to avoid openshift.local... kubeconfig missing error..."
	sleep 5
	sudo -u vagrant whoami
	sudo chmod +r openshift.local.config/master/openshift-registry.kubeconfig
	sudo chmod +r openshift.local.config/master/admin.kubeconfig
	echo "Creating registry.  Sleeping a while first..."
	sleep 1
	oadm registry --create --credentials=openshift.local.config/master/openshift-registry.kubeconfig --config=openshift.local.config/master/admin.kubeconfig
	echo "now creating project"
	oc="/data/src/github.com/openshift/origin/_output/local/bin/linux/amd64/oc"

	# warning this project needs to be manually deleted after your first 'vagrant up', or it will persist even after destroying vms.
	# purge the openshift.local.etcd created on your host.
	sudo -u vagrant $oc login https://localhost:8443 -u=admin -p=admin --config=/data/src/github.com/openshift/origin/openshift.local.config/master/openshift-registry.kubeconfig
	#sudo -u vagrant $oc login localhost:8443 -u=admin -p=admin --config=/data/src/github.com/openshift/origin/openshift.local.config/master/openshift-registry.kubeconfig
	sudo -u vagrant $oc new-project project1 --config=/data/src/github.com/openshift/origin/openshift.local.config/master/openshift-registry.kubeconfig || true
	
	echo "Now starting the examples!!!"

	# Make for sure that admin can do things as user vagrant.
	sudo chmod 755 /openshift.local.config/master/admin.kubeconfig
	sudo -u vagrant mkdir /home/vagrant/.kube/
	sudo -u vagrant cp /openshift.local.config/master/admin.kubeconfig /home/vagrant/.kube/config	
	sudo -u vagrant ls /home/vagrant/.kube/
	sudo -u vagrant $oc login https://localhost:8443 -u=admin -p=admin --insecure-skip-tls-verify=true --config=/home/vagrant/.kube/config && $oc --config=/home/vagrant/.kube/config create -f /data/src/github.com/openshift/origin/examples/image-streams/image-streams-centos7.json -n project1

	sudo -u vagrant $oc get nodes --config=/data/src/github.com/openshift/origin/openshift.local.config/master/admin.kubeconfig
}
setup
installOpenShift
