package install

import (
	"strconv"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/intstr"
)

// newOpenShiftController returns a controller for openshift.
// TODO: pass tag
func newOpenShiftController(name string) *kapi.ReplicationController {
	labels := map[string]string{"name": name}
	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ReplicationControllerSpec{
			Replicas: 1,
			Selector: labels,
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels: labels,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "origin",
							Image: "openshift/origin:latest",
							Args: []string{
								"start",
								"master",
								"--config=/config/master-config.yaml",
							},
							Ports: []kapi.ContainerPort{
								{
									ContainerPort: 8443,
								},
							},
							VolumeMounts: []kapi.VolumeMount{
								{
									Name:      "config",
									ReadOnly:  true,
									MountPath: "/config",
								},
							},
							TerminationMessagePath: "/dev/termination-log",
							ImagePullPolicy:        kapi.PullIfNotPresent,
						},
					},
					DNSPolicy: kapi.DNSClusterFirst,
					Volumes: []kapi.Volume{
						{
							Name: "config",
							VolumeSource: kapi.VolumeSource{
								Secret: &kapi.SecretVolumeSource{
									SecretName: "openshift",
								},
							},
						},
					},
				},
			},
		},
	}
}

// newOpenShiftService returns a service for openshift
// TODO: need to handle clusters that dont have load balancers
func newOpenShiftService(name string) *kapi.Service {
	labels := map[string]string{"name": name}
	return &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ServiceSpec{
			Type:     kapi.ServiceTypeLoadBalancer,
			Selector: labels,
			Ports: []kapi.ServicePort{
				{
					Name:       "openshift",
					Port:       8443,
					TargetPort: intstr.FromInt(8443),
				},
			},
		},
	}
}

// newEtcdDiscoveryService returns a service for etcd-discovery
func newEtcdDiscoveryService(name string) *kapi.Service {
	labels := map[string]string{"name": name}
	return &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ServiceSpec{
			Type:     kapi.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []kapi.ServicePort{
				{
					Port:       2379,
					TargetPort: intstr.FromInt(2379),
				},
			},
		},
	}
}

// newEtcdDiscoveryService returns a controller for etcd-discovery
func newEtcdDiscoveryController(name string) *kapi.ReplicationController {
	labels := map[string]string{"name": name}
	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ReplicationControllerSpec{
			Replicas: 1,
			Selector: labels,
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels: labels,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "discovery",
							Image: "openshift/etcd-20-centos7",
							Args:  []string{"etcd-discovery.sh"},
							Ports: []kapi.ContainerPort{
								{
									Name:          "client",
									ContainerPort: 2379,
									Protocol:      kapi.ProtocolTCP,
								},
							},
							TerminationMessagePath: "/dev/termination-log",
							ImagePullPolicy:        kapi.PullIfNotPresent,
						},
					},
					DNSPolicy: kapi.DNSClusterFirst,
				},
			},
		},
	}
}

// newEtcdService returns a service for etcd
func newEtcdService(name string) *kapi.Service {
	labels := map[string]string{"name": name}
	return &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ServiceSpec{
			Type:     kapi.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []kapi.ServicePort{
				{
					Name:       "client",
					Port:       2379,
					TargetPort: intstr.FromInt(2379),
				},
				{
					Name:       "server",
					Port:       2380,
					TargetPort: intstr.FromInt(2380),
				},
			},
			SessionAffinity: kapi.ServiceAffinityNone,
		},
	}
}

// newEtcdController returns a controller for etcd
func newEtcdController(name string, numMembers int, clusterToken, discoveryToken string) *kapi.ReplicationController {
	labels := map[string]string{"name": name}
	return &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: kapi.ReplicationControllerSpec{
			Replicas: numMembers,
			Selector: labels,
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: kapi.ObjectMeta{
					Labels: labels,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "member",
							Image: "openshift/etcd-20-centos7",
							Args:  []string{"etcd-discovery.sh"},
							Ports: []kapi.ContainerPort{
								{
									ContainerPort: 2379,
									Protocol:      kapi.ProtocolTCP,
								},
								{
									ContainerPort: 2380,
									Protocol:      kapi.ProtocolTCP,
								},
							},
							TerminationMessagePath: "/dev/termination-log",
							ImagePullPolicy:        kapi.PullIfNotPresent,
							Env: []kapi.EnvVar{
								{
									Name:  "ETCD_NUM_MEMBERS", // maximum number of members to launch, must match number of replicas
									Value: strconv.Itoa(numMembers),
								},
								{
									Name:  "ETCD_INITIAL_CLUSTER_STATE",
									Value: "new",
								},
								{
									Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
									Value: etcdClusterToken,
								},
								{
									Name:  "ETCD_DISCOVERY_TOKEN",
									Value: etcdDiscoveryToken,
								},
								{
									Name:  "ETCD_DISCOVERY_URL",
									Value: "http://etcd-discovery:2379",
								},
								{
									Name:  "ETCDCTL_PEERS",
									Value: "http://etcd:2379",
								},
							},
						},
					},
					DNSPolicy: kapi.DNSClusterFirst,
				},
			},
		},
	}
}
