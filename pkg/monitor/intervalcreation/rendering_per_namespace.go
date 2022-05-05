package intervalcreation

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type relatedNamespaces struct {
	name       string
	namespaces sets.String
}

// wellKnownNamespaceGroups is a set I randomly assigned to keep related-ish pods together.
func wellKnownNamespaceGroups() []relatedNamespaces {
	return []relatedNamespaces{
		{
			name: "kube-control-plane",
			namespaces: sets.NewString(
				"openshift-cluster-version",
				"openshift-config-operator",
				"openshift-kube-apiserver-operator", "openshift-kube-apiserver",
				"openshift-etcd-operator", "openshift-etcd",
				"openshift-kube-controller-manager-operator", "openshift-kube-controller-manager",
				"openshift-kube-scheduler-operator", "openshift-kube-scheduler",
				"openshift-kube-storage-version-migrator-operator", "openshift-kube-storage-version-migrator",
			),
		},
		{
			name: "openshift-control-plane",
			namespaces: sets.NewString(
				"openshift-apiserver-operator", "openshift-apiserver",
				"openshift-authentication-operator", "openshift-authentication", "openshift-oauth-apiserver",
				"openshift-controller-manager-operator", "openshift-controller-manager",
				"openshift-image-registry",
				"openshift-service-ca-operator", "openshift-service-ca",
			),
		},
		{
			name: "openshift-machines",
			namespaces: sets.NewString(
				"openshift-cloud-controller-manager-operator", "openshift-cloud-controller-manager",
				"openshift-cloud-credential-operator",
				"openshift-cluster-machine-approver",
				"openshift-cluster-node-tuning-operator",
				"openshift-kni-infra",
				"openshift-machine-api",
				"openshift-machine-config-operator",
				"openshift-openstack-infra",
				"openshift-ovirt-infra",
				"openshift-vsphere-infra",
			),
		},
		{
			name: "openshift-networking",
			namespaces: sets.NewString(
				"openshift-cloud-network-config-controller",
				"openshift-dns-operator", "openshift-dns",
				"openshift-host-network",
				"openshift-ingress", "openshift-ingress-canary", "openshift-ingress-operator",
				"openshift-network-operator", "openshift-multus", "openshift-network-diagnostics", "openshift-sdn",
			),
		},
		{
			name:       "openshift-storage",
			namespaces: sets.NewString("openshift-cluster-storage-operator", "openshift-cluster-csi-drivers"),
		},
		{
			name: "openshift-support-stuff",
			namespaces: sets.NewString(
				"openshift-console-operator", "openshift-console",
				"openshift-cluster-samples-operator",
				"openshift-insights",
				"openshift-marketplace",
				"openshift-operator-lifecycle-manager",
				"openshift-operators",
			),
		},
		{
			name: "openshift-monitoring",
			namespaces: sets.NewString(
				"openshift-monitoring",
				"openshift-user-workload-monitoring",
			),
		},
	}
}

type podRendering struct {
	name string
}

func NewPodEventIntervalRenderer() podRendering {
	return podRendering{}
}

func (r podRendering) WriteEventData(artifactDir string, events monitorapi.Intervals, timeSuffix string) error {
	allNamespaces := sets.NewString()
	for _, interval := range events {
		allNamespaces.Insert(monitorapi.NamespaceFromLocator(interval.Locator))
	}

	namespaceGroups := wellKnownNamespaceGroups()
	e2eNamespaces := relatedNamespaces{
		name: "e2e-namespaces", namespaces: sets.String{},
	}
	allTheOtherNamespaces := relatedNamespaces{
		name: "everything-else", namespaces: sets.String{},
	}
	for _, namespace := range allNamespaces.List() {
		collected := false
		for _, nsGroup := range namespaceGroups {
			if nsGroup.namespaces.Has(namespace) {
				collected = true
			}
		}
		if collected {
			continue
		}

		if strings.HasPrefix(namespace, "e2e-") {
			e2eNamespaces.namespaces.Insert(namespace)
			continue
		}
		allTheOtherNamespaces.namespaces.Insert(namespace)
	}
	namespaceGroups = append(namespaceGroups, e2eNamespaces, allTheOtherNamespaces)

	errs := []error{}
	for _, namespaceGroup := range namespaceGroups {
		writer := NewNonSpyglassEventIntervalRenderer(namespaceGroup.name,
			func(eventInterval monitorapi.EventInterval) bool {
				if !IsPodLifecycle(eventInterval) {
					return false
				}
				if isInterestingNamespace(eventInterval, namespaceGroup.namespaces) {
					return true
				}
				return false
			})
		if err := writer.WriteEventData(artifactDir, events, timeSuffix); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}
