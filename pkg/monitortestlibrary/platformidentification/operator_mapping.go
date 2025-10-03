package platformidentification

import (
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	ValidBugzillaComponents = sets.NewString(
		"apiserver-auth",
		"assisted-installer",
		"Bare Metal Hardware Provisioning",
		"Build",
		"Cloud Compute",
		"Cloud Credential Operator",
		"Cluster Loader",
		"Cluster Version Operator",
		"CNF Variant Validation",
		"Compliance Operator",
		"config-operator",
		"Console Kubevirt Plugin",
		"Console Metal3 Plugin",
		"Console Storage Plugin",
		"Containers",
		"crc",
		"Dev Console",
		"DNS",
		"Documentation",
		"Etcd",
		"Federation",
		"File Integrity Operator",
		"Fuse",
		"Hawkular",
		"ibm-roks-toolkit",
		"Image",
		"Image Registry",
		"Insights Operator",
		"Installer",
		"ISV Operators",
		"Jenkins",
		"kata-containers",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-scheduler",
		"kube-storage-version-migrator",
		"Logging",
		"Machine Config Operator",
		"Management Console",
		"Metering Operator",
		"Migration Tooling",
		"Monitoring",
		"Multi-Arch",
		"Multi-cluster-management",
		"Networking",
		"Node",
		"Node Feature Discovery Operator",
		"Node Tuning Operator",
		"oauth-apiserver",
		"oauth-proxy",
		"oc",
		"OLM",
		"openshift-apiserver",
		"openshift-controller-manager",
		"Operator SDK",
		"Performance Addon Operator",
		"Reference Architecture",
		"Registry Console",
		"Release",
		"RHCOS",
		"RHMI Monitoring",
		"Routing",
		"Samples",
		"Security",
		"Service Broker",
		"Service Catalog",
		"service-ca",
		"Special Resources Operator",
		"Storage",
		"Templates",
		"Test Infrastructure",
		"Unknown",
		"Windows Containers",
	)

	// nothing fancy, I just copied the listing
	KnownOperators = sets.NewString(
		"authentication",
		"baremetal",
		"cloud-controller-manager",
		"cloud-credential",
		"cluster-autoscaler",
		"config-operator",
		"console",
		"control-plane-machine-set",
		"csi-snapshot-controller",
		"dns",
		"etcd",
		"image-registry",
		"ingress",
		"insights",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-scheduler",
		"kube-storage-version-migrator",
		"machine-api",
		"machine-approver",
		"machine-config",
		"marketplace",
		"monitoring",
		"network",
		"node-tuning",
		"olm",
		"openshift-apiserver",
		"openshift-controller-manager",
		"openshift-samples",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"operator-lifecycle-manager-packageserver",
		"service-ca",
		"storage",
	)

	NamespaceOther  = "all the other namespaces"
	KnownNamespaces = sets.String{}

	operatorToBugzillaComponent  = map[string]string{}
	namespaceToBugzillaComponent = map[string]string{}
)

func init() {
	utilruntime.Must(addOperatorMapping("authentication", "apiserver-auth"))
	utilruntime.Must(addOperatorMapping("baremetal", "Bare Metal Hardware Provisioning"))
	utilruntime.Must(addOperatorMapping("cloud-controller-manager", "Cloud Compute"))
	utilruntime.Must(addOperatorMapping("cloud-credential", "Cloud Credential Operator"))
	utilruntime.Must(addOperatorMapping("cluster-autoscaler", "Cloud Compute"))
	utilruntime.Must(addOperatorMapping("config-operator", "config-operator"))
	utilruntime.Must(addOperatorMapping("console", "Management Console"))
	utilruntime.Must(addOperatorMapping("control-plane-machine-set", "Cloud Compute"))
	utilruntime.Must(addOperatorMapping("csi-snapshot-controller", "Storage"))
	utilruntime.Must(addOperatorMapping("dns", "DNS"))
	utilruntime.Must(addOperatorMapping("etcd", "Etcd"))
	utilruntime.Must(addOperatorMapping("image-registry", "Image Registry"))
	utilruntime.Must(addOperatorMapping("ingress", "Routing"))
	utilruntime.Must(addOperatorMapping("insights", "Insights Operator"))
	utilruntime.Must(addOperatorMapping("kube-apiserver", "kube-apiserver"))
	utilruntime.Must(addOperatorMapping("kube-controller-manager", "kube-controller-manager"))
	utilruntime.Must(addOperatorMapping("kube-scheduler", "kube-scheduler"))
	utilruntime.Must(addOperatorMapping("kube-storage-version-migrator", "kube-storage-version-migrator"))
	utilruntime.Must(addOperatorMapping("machine-api", "Cloud Compute"))
	utilruntime.Must(addOperatorMapping("machine-approver", "Cloud Compute"))
	utilruntime.Must(addOperatorMapping("machine-config", "Machine Config Operator"))
	utilruntime.Must(addOperatorMapping("marketplace", "OLM"))
	utilruntime.Must(addOperatorMapping("monitoring", "Monitoring"))
	utilruntime.Must(addOperatorMapping("network", "Networking"))
	utilruntime.Must(addOperatorMapping("node-tuning", "Node Tuning Operator"))
	utilruntime.Must(addOperatorMapping("olm", "OLM"))
	utilruntime.Must(addOperatorMapping("openshift-apiserver", "openshift-apiserver"))
	utilruntime.Must(addOperatorMapping("openshift-controller-manager", "openshift-controller-manager"))
	utilruntime.Must(addOperatorMapping("openshift-samples", "Samples"))
	utilruntime.Must(addOperatorMapping("operator-lifecycle-manager", "OLM"))
	utilruntime.Must(addOperatorMapping("operator-lifecycle-manager-catalog", "OLM"))
	utilruntime.Must(addOperatorMapping("operator-lifecycle-manager-packageserver", "OLM"))
	utilruntime.Must(addOperatorMapping("service-ca", "service-ca"))
	utilruntime.Must(addOperatorMapping("storage", "Storage"))

	for _, name := range KnownOperators.List() {
		if bz := GetBugzillaComponentForOperator(name); bz == "Unknown" {
			panic(fmt.Sprintf("%q missing a bugzilla mapping", name))
		}
	}

	// nothing fancy, just listed from must-gather
	utilruntime.Must(addNamespaceMapping("default", "Unknown"))
	utilruntime.Must(addNamespaceMapping("kube-system", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-apiserver", "openshift-apiserver"))
	utilruntime.Must(addNamespaceMapping("openshift-apiserver-operator", "openshift-apiserver"))
	utilruntime.Must(addNamespaceMapping("openshift-authentication", "apiserver-auth"))
	utilruntime.Must(addNamespaceMapping("openshift-authentication-operator", "apiserver-auth"))
	utilruntime.Must(addNamespaceMapping("openshift-cloud-controller-manager", "Cloud Compute"))
	utilruntime.Must(addNamespaceMapping("openshift-cloud-controller-manager-operator", "Cloud Compute"))
	utilruntime.Must(addNamespaceMapping("openshift-cloud-credential-operator", "Cloud Credential Operator"))
	utilruntime.Must(addNamespaceMapping("openshift-cloud-network-config-controller", "Networking"))
	utilruntime.Must(addNamespaceMapping("openshift-cluster-csi-drivers", "Storage"))
	utilruntime.Must(addNamespaceMapping("openshift-cluster-machine-approver", "Cloud Compute"))
	utilruntime.Must(addNamespaceMapping("openshift-cluster-node-tuning-operator", "Node Tuning Operator"))
	utilruntime.Must(addNamespaceMapping("openshift-cluster-samples-operator", "Samples"))
	utilruntime.Must(addNamespaceMapping("openshift-cluster-storage-operator", "Storage"))
	utilruntime.Must(addNamespaceMapping("openshift-cluster-version", "Cluster Version Operator"))
	utilruntime.Must(addNamespaceMapping("openshift-config", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-config-managed", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-config-operator", "config-operator"))
	utilruntime.Must(addNamespaceMapping("openshift-console", "Management Console"))
	utilruntime.Must(addNamespaceMapping("openshift-console-operator", "Management Console"))
	utilruntime.Must(addNamespaceMapping("openshift-controller-manager", "openshift-controller-manager"))
	utilruntime.Must(addNamespaceMapping("openshift-controller-manager-operator", "openshift-controller-manager"))
	utilruntime.Must(addNamespaceMapping("openshift-dns", "DNS"))
	utilruntime.Must(addNamespaceMapping("openshift-dns-operator", "DNS"))
	utilruntime.Must(addNamespaceMapping("openshift-etcd", "Etcd"))
	utilruntime.Must(addNamespaceMapping("openshift-etcd-operator", "Etcd"))
	utilruntime.Must(addNamespaceMapping("openshift-host-network", "Networking"))
	utilruntime.Must(addNamespaceMapping("openshift-image-registry", "Image Registry"))
	utilruntime.Must(addNamespaceMapping("openshift-ingress", "Routing"))
	utilruntime.Must(addNamespaceMapping("openshift-ingress-canary", "Routing"))
	utilruntime.Must(addNamespaceMapping("openshift-ingress-operator", "Routing"))
	utilruntime.Must(addNamespaceMapping("openshift-insights", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-kni-infra", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-apiserver", "kube-apiserver"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-apiserver-operator", "kube-apiserver"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-controller-manager", "kube-controller-manager"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-controller-manager-operator", "kube-controller-manager"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-scheduler", "kube-scheduler"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-scheduler-operator", "kube-scheduler"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-storage-version-migrator", "kube-storage-version-migrator"))
	utilruntime.Must(addNamespaceMapping("openshift-kube-storage-version-migrator-operator", "kube-storage-version-migrator"))
	utilruntime.Must(addNamespaceMapping("openshift-machine-api", "Cloud Compute"))
	utilruntime.Must(addNamespaceMapping("openshift-machine-config-operator", "Machine Config Operator"))
	utilruntime.Must(addNamespaceMapping("openshift-marketplace", "OLM"))
	utilruntime.Must(addNamespaceMapping("openshift-monitoring", "Monitoring"))
	utilruntime.Must(addNamespaceMapping("openshift-multus", "Networking"))
	utilruntime.Must(addNamespaceMapping("openshift-network-diagnostics", "Networking"))
	utilruntime.Must(addNamespaceMapping("openshift-network-operator", "Networking"))
	utilruntime.Must(addNamespaceMapping("openshift-nutanix-infra", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-oauth-apiserver", "oauth-apiserver"))
	utilruntime.Must(addNamespaceMapping("openshift-openstack-infra", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-operator-lifecycle-manager", "OLM"))
	utilruntime.Must(addNamespaceMapping("openshift-operators", "OLM"))
	utilruntime.Must(addNamespaceMapping("openshift-ovirt-infra", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-ovn-kubernetes", "Networking"))
	utilruntime.Must(addNamespaceMapping("openshift-service-ca", "service-ca"))
	utilruntime.Must(addNamespaceMapping("openshift-service-ca-operator", "service-ca"))
	utilruntime.Must(addNamespaceMapping("openshift-user-workload-monitoring", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-vsphere-infra", "Unknown"))
	utilruntime.Must(addNamespaceMapping("openshift-infra", "Unknown"))

	KnownNamespaces = sets.StringKeySet(namespaceToBugzillaComponent)
}

func GetBugzillaComponentForOperator(operator string) string {
	ret, ok := operatorToBugzillaComponent[operator]
	if !ok {
		return "Unknown"
	}
	return ret
}

func addOperatorMapping(operator, bugzillaComponent string) error {
	if !ValidBugzillaComponents.Has(bugzillaComponent) {
		return fmt.Errorf("%q is not a valid bugzilla component", bugzillaComponent)
	}
	operatorToBugzillaComponent[operator] = bugzillaComponent
	return nil
}

func addNamespaceMapping(namespace, bugzillaComponent string) error {
	if !ValidBugzillaComponents.Has(bugzillaComponent) {
		return fmt.Errorf("%q is not a valid bugzilla component", bugzillaComponent)
	}
	namespaceToBugzillaComponent[namespace] = bugzillaComponent
	return nil
}

func GetNamespacesToBugzillaComponents() map[string]string {
	ret := map[string]string{}
	for k, v := range namespaceToBugzillaComponent {
		ret[k] = v
	}
	return ret
}
