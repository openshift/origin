## Overview

This guide enables the integration, deployment, and management of GlusterFS containerized storage nodes in a Kubernetes cluster. This enables Kubernetes administrators to provide their users with reliable, shared storage.

Another important resource on this topic is the
[gluster-kubernetes](https://github.com/gluster/gluster-kubernetes) project.
It is focused on the deployment of GlusterFS within a Kubernetes cluster,
and provides streamlined tools to accomplish this task.
It includes a [setup guide](https://github.com/gluster/gluster-kubernetes/blob/master/docs/setup-guide.md).
It also includes a [Hello World](https://github.com/gluster/gluster-kubernetes/tree/master/docs/examples/hello_world)
featuring an example web server pod using a dynamically-provisioned
GlusterFS volume for storage. For those looking to test or learn more about
this topic, follow the Quick-start instructions found in the main
[README](https://github.com/gluster/gluster-kubernetes) for gluster-kubernetes

This guide is intended to demonstrate a minimal example of Heketi managing
Gluster in a Kubernetes environment. It is meant to highlight the major
components of this configuration and is not suitable for production.

## Infrastructure Requirements

* A running Kubernetes cluster with at least three Kubernetes worker nodes
  that each have at least one available raw block device attached
  (like an EBS Volume or a local disk).
* The three Kubernetes nodes intended to run the GlusterFS Pods must have the appropriate ports opened for GlusterFS communication. Run the following commands on each of the nodes.
```
iptables -N HEKETI
iptables -A HEKETI -p tcp -m state --state NEW -m tcp --dport 24007 -j ACCEPT
iptables -A HEKETI -p tcp -m state --state NEW -m tcp --dport 24008 -j ACCEPT
iptables -A HEKETI -p tcp -m state --state NEW -m tcp --dport 2222 -j ACCEPT
iptables -A HEKETI -p tcp -m state --state NEW -m multiport --dports 49152:49251 -j ACCEPT
service iptables save
```

## Client Setup

Heketi provides a CLI that provides users with a means to administer
the deployment and configuration of GlusterFS in Kubernetes.
[Download and install the heketi-cli](https://github.com/heketi/heketi/releases)
on your client machine.

## Kubernetes Deployment
All the following files are located under `extras/kubernetes`.

* Deploy the GlusterFS DaemonSet

```
$ kubectl create -f glusterfs-daemonset.json
```

* Get node names by running:

```
$ kubectl get nodes
```

* Deploy gluster container onto a specified node by setting the
  label `storagenode=glusterfs` on that node.

```
$ kubectl label node <...node...> storagenode=glusterfs
```

Repeat as needed. Verify that the pods are running on the nodes, at least
three pods should be running.

```
$ kubectl get pods
```

* Next we will create a service account for Heketi:

```
$ kubectl create -f heketi-service-account.json
```

* We must now establish the ability for that service account to control
  the gluster pods. We do this by creating a cluster role binding for
  our newly created service account.

```
$ kubectl create clusterrolebinding heketi-gluster-admin --clusterrole=edit --serviceaccount=default:heketi-service-account
```

* Now we need to create a Kubernetes secret that will hold the configuration
  of our Heketi instance. The configuration file must be set to use the
  `kubernetes` executor in order for the Heketi server to control the
  gluster pods. Beyond that, feel free to experiment with the configuration
  options.

```
$ kubectl create secret generic heketi-config-secret --from-file=./heketi.json
```

* Next we need to deploy an initial Pod and a Service to access that pod.
  In the repo you cloned, there will be a heketi-bootstrap.json file.

Submit the file and verify everything is running properly as demonstrated below:

```
# kubectl create -f heketi-bootstrap.json
service "deploy-heketi" created
deployment "deploy-heketi" created

# kubectl get pods
NAME                                                      READY     STATUS    RESTARTS   AGE
deploy-heketi-1211581626-2jotm                            1/1       Running   0          35m
glusterfs-ip-172-20-0-217.ec2.internal-1217067810-4gsvx   1/1       Running   0          1h
glusterfs-ip-172-20-0-218.ec2.internal-2001140516-i9dw9   1/1       Running   0          1h
glusterfs-ip-172-20-0-219.ec2.internal-2785213222-q3hba   1/1       Running   0          1h
```

* Now that the Bootstrap Heketi Service is running we are going to configure port-forwarding so that we can communicate with the service using the Heketi CLI. Using the name of the Heketi pod, run the command below:

`kubectl port-forward deploy-heketi-1211581626-2jotm :8080`

If local port 8080 is free on the system where you are running the commands,
you can run the port-forward command so that it binds to 8080 for convenience:

`kubectl port-forward deploy-heketi-1211581626-2jotm 8080:8080`

Now verify that port forwarding is working by running a sample query
against the Heketi Service. The command should have printed the local port
that it will be forwarding from. Incorporate that into a URL to test the
service, as demonstrated below:

```
curl http://localhost:57598/hello
Handling connection for 57598
Hello from Heketi
```

Lastly, set an environment variable for the Heketi CLI client so that it knows how to reach the Heketi Server.

`export HEKETI_CLI_SERVER=http://localhost:57598`

* Next we are going to provide Heketi with information about the GlusterFS
  cluster it is to manage. We provide this information via
  [a topology file](./topology.md). There is a sample topology file within
  the repo you cloned called topology-sample.json. Topologies specify what
  Kubernetes Nodes the GlusterFS containers are running on as well as the
  corresponding raw block device for each of the nodes.

  Make sure that `hostnames/manage` points to the exact name as shown
  under `kubectl get nodes`, and `hostnames/storage` is the ip address
  of the storage network.

  **IMPORTANT**: At this time, the topology file must be loaded using a version
  of heketi-cli that matches the server version. As a last resort the Heketi
  container comes with a copy of heketi-cli that can be accessed via
  `kubectl exec ...`.

  Modify the topology file to reflect the choices you have made and then
  deploy it as demonstrated below:

```
heketi-client/bin/heketi-cli topology load --json=topology-sample.json
Handling connection for 57598
	Found node ip-172-20-0-217.ec2.internal on cluster e6c063ba398f8e9c88a6ed720dc07dd2
		Adding device /dev/xvdg ... OK
	Found node ip-172-20-0-218.ec2.internal on cluster e6c063ba398f8e9c88a6ed720dc07dd2
		Adding device /dev/xvdg ... OK
	Found node ip-172-20-0-219.ec2.internal on cluster e6c063ba398f8e9c88a6ed720dc07dd2
		Adding device /dev/xvdg ... OK
```

* Next we are going to use Heketi to provision a volume for it to store its database:

```
# heketi-client/bin/heketi-cli setup-openshift-heketi-storage
# kubectl create -f heketi-storage.json
```

> Pitfall: If heketi-cli reports an error of "No space"
  when running the setup-openshift-heketi-storage subcommand you may
  have inadvertently run `topology load` with mismatched versions of the
  server and heketi-cli. Stop the running Heketi pod
  (`kubectl scale deployment deploy-heketi --replicas=0`), manually remove any
  signatures from the storage block devices and then resume running a
  Heketi pod (`kubectl scale deployment deploy-heketi --replicas=1`). Then
  reload the topology with a matching version of heketi-cli and retry the step.

* Wait until the job is complete then delete the bootstrap Heketi:

```
# kubectl delete all,service,jobs,deployment,secret --selector="deploy-heketi"
```

* Create the long-term Heketi instance:

```
# kubectl create -f heketi-deployment.json
service "heketi" created
deployment "heketi" created
```

* Now that this is done the Heketi db will persist in a GlusterFS volume
  and will not reset every time the Heketi pod is restarted.

  Use commands such as `heketi-cli cluster list` and `heketi-cli volume list`
  to confirm that the cluster established earlier exists and that
  Heketi is aware of the db storage volume created during the bootstrapping
  phase.

# Usage Example

There are two ways to provision storage. The common way is to setup a
StorageClass which lets Kubernetes automatically provision storage for a
PersistentVolumeClaim submitted. Alternatively one can manually create and
manage volumes (PVs) through Kubernetes or work directly with volumes
from heketi-cli.

Please refer to the [gluster-kubernetes hello world example](https://github.com/gluster/gluster-kubernetes/blob/master/docs/examples/hello_world/README.md)
for more information on storageClass configuration.
