package synthetictests

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
		"cloud-credential",
		"cluster-autoscaler",
		"config-operator",
		"console",
		"csi-snapshot-controller",
		"dns",
		"etcd",
		"ingress",
		"image-registry",
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
		"openshift-apiserver",
		"openshift-controller-manager",
		"openshift-samples",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"operator-lifecycle-manager-packageserver",
		"service-ca",
		"storage",
	)

	operatorToBugzillaComponent = map[string]string{}
)

func init() {
	utilruntime.Must(addOperatorMapping("authentication", "apiserver-auth"))
	utilruntime.Must(addOperatorMapping("cloud-credential", "Cloud Credential Operator"))
	utilruntime.Must(addOperatorMapping("cluster-autoscaler", "Cloud Compute"))
	utilruntime.Must(addOperatorMapping("config-operator", "config-operator"))
	utilruntime.Must(addOperatorMapping("console", "Management Console"))
	utilruntime.Must(addOperatorMapping("csi-snapshot-controller", "Storage"))
	utilruntime.Must(addOperatorMapping("dns", "DNS"))
	utilruntime.Must(addOperatorMapping("etcd", "Etcd"))
	utilruntime.Must(addOperatorMapping("ingress", "Routing"))
	utilruntime.Must(addOperatorMapping("image-registry", "Image Registry"))
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
