package commatrix

var generalStaticEntries = `
[
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "22",
        "nodeRole": "worker",
        "service": "sshd",
        "namespace": "system",
        "pod": "system",
        "container": "system",
        "optional": true
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9637",
        "nodeRole": "master",
        "service": "kube-rbac-proxy",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9637",
        "nodeRole": "worker",
        "service": "kube-rbac-proxy",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10250",
        "nodeRole": "worker",
        "service": "kubelet",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9107",
        "nodeRole": "worker",
        "service": "egressip-node-healthcheck",
        "namespace": "openshift-ovn-kubernetes",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "111",
        "nodeRole": "worker",
        "service": "rpcbind",
        "namespace": "system",
        "pod": "system",
        "container": "system",
        "optional": true
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10256",
        "nodeRole": "master",
        "service": "openshift-sdn",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10256",
        "nodeRole": "worker",
        "service": "openshift-sdn",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": true
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9001",
        "nodeRole": "worker",
        "service": "machine-config-daemon",
        "namespace": "openshift-machine-config-operator",
        "pod": "machine-config-daemon",
        "container": "kube-rbac-proxy",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9537",
        "nodeRole": "master",
        "service": "crio-metrics",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9537",
        "nodeRole": "worker",
        "service": "crio-metrics",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10250",
        "nodeRole": "master",
        "service": "kubelet",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9107",
        "nodeRole": "master",
        "service": "egressip-node-healthcheck",
        "namespace": "openshift-ovn-kubernetes",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "111",
        "nodeRole": "master",
        "service": "rpcbind",
        "namespace": "system",
        "pod": "system",
        "container": "system",
        "optional": true
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "22",
        "nodeRole": "master",
        "service": "sshd",
        "namespace": "system",
        "pod": "system",
        "container": "system",
        "optional": true
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9192",
        "nodeRole": "master",
        "service": "machine-approver",
        "namespace": "openshift-cluster-machine-approver",
        "pod": "machine-approver",
        "container": "kube-rbac-proxy",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9258",
        "nodeRole": "master",
        "service": "machine-approver",
        "namespace": "openshift-cloud-controller-manager-operator",
        "pod": "cluster-cloud-controller-manager",
        "container": "cluster-cloud-controller-manager",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9099",
        "nodeRole": "master",
        "service": "cluster-version-operator",
        "namespace": "openshift-cluster-version",
        "pod": "cluster-version-operator",
        "container": "cluster-version-operator",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9980",
        "nodeRole": "master",
        "service": "etcd",
        "namespace": "openshift-etcd",
        "pod": "etcd",
        "container": "etcd",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9979",
        "nodeRole": "master",
        "service": "etcd",
        "namespace": "openshift-etcd",
        "pod": "etcd",
        "container": "etcd-metrics",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9978",
        "nodeRole": "master",
        "service": "etcd",
        "namespace": "openshift-etcd",
        "pod": "etcd-metrics",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10357",
        "nodeRole": "master",
        "service": "cluster-policy-controller-apiserver-healthz",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "17697",
        "nodeRole": "master",
        "service": "no-service",
        "namespace": "openshift-kube-apiserver",
        "pod": "kube-apiserve",
        "container": "kube-apiserver-check-endpoints",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "2380",
        "nodeRole": "master",
        "service": "healthz",
        "namespace": "etcd",
        "pod": "etcd",
        "container": "etcd",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "2379",
        "nodeRole": "master",
        "service": "etcd",
        "namespace": "openshift-etcd",
        "pod": "etcd",
        "container": "etcdctl",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "6080",
        "nodeRole": "master",
        "service": "no-service",
        "namespace": "openshift-kube-apiserver",
        "pod": "kube-apiserver",
        "container": "kube-apiserver-insecure-readyz",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "22624",
        "nodeRole": "master",
        "service": "machine-config-server",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "22623",
        "nodeRole": "master",
        "service": "machine-config-server",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    }
]
`

var baremetalStaticEntries = `
[
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "53",
        "nodeRole": "master",
        "service": "dns-default",
        "namespace": "openshift-dns",
        "pod": "dnf-default",
        "container": "dns",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "53",
        "nodeRole": "worker",
        "service": "none",
        "namespace": "openshift-dns",
        "pod": "dnf-default",
        "container": "dns",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "5050",
        "nodeRole": "master",
        "service": "metal3",
        "namespace": "openshift-machine-api",
        "pod": "ironic-proxy",
        "container": "ironic-proxy",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9444",
        "nodeRole": "master",
        "service": "openshift-kni-infra-haproxy-haproxy",
        "namespace": "openshift-kni-infra",
        "pod": "haproxy",
        "container": "haproxy",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9445",
        "nodeRole": "master",
        "service": "haproxy-openshift-dsn-internal-loadbalancer",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9191",
        "nodeRole": "master",
        "service": "machine-approver",
        "namespace": "machine-approver",
        "pod": "machine-approver",
        "container": "machine-approver-controller",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "6385",
        "nodeRole": "master",
        "service": "no-service",
        "namespace": "openshift-machine-api",
        "pod": "ironic-proxy",
        "container": "ironic-proxy",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "29445",
        "nodeRole": "master",
        "service": "haproxy-openshift-dsn",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": true
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "18080",
        "service": "openshift-kni-infra-coredns",
        "nodeRole": "worker",
        "namespace": "openshift-kni-infra",
        "pod": "coredns",
        "container": "coredns",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "18080",
        "nodeRole": "master",
        "service": "openshift-kni-infra-coredns",
        "namespace": "openshift-kni-infra",
        "pod": "corend",
        "container": "coredns",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "9447",
        "nodeRole": "master",
        "service": "baremetal-operator-webhook-baremetal provisioning",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    }
]
`

var awsCloudStaticEntries = `
[
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "8080",
        "nodeRole": "master",
        "service": "cluster-network",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10260",
        "nodeRole": "master",
        "service": "aws-cloud-controller",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10258",
        "nodeRole": "master",
        "service": "aws-cloud-controller",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10304",
        "nodeRole": "master",
        "service": "csi-node-driver",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10304",
        "nodeRole": "worker",
        "service": "csi-node-driver",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10300",
        "nodeRole": "master",
        "service": "csi-livenessprobe",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    },
    {
        "direction": "ingress",
        "protocol": "TCP",
        "port": "10300",
        "nodeRole": "worker",
        "service": "csi-livenessprobe",
        "namespace": "",
        "pod": "",
        "container": "",
        "optional": false
    }
]
`
