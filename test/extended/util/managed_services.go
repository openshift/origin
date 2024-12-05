package util

import "k8s.io/apimachinery/pkg/util/sets"

// ManagedServiceNamespaces is the set of namespaces used by managed service platforms
// like ROSA, ARO, etc. These are typically exempt from the requirements we impose on
// core platform namespaces.
var ManagedServiceNamespaces = sets.New[string](
	"openshift-addon-operator",
	"openshift-backplane",
	"openshift-backplane-srep",
	"openshift-cloud-ingress-operator",
	"openshift-custom-domains-operator",
	"openshift-deployment-validation-operator",
	"openshift-managed-node-metadata-operator",
	"openshift-managed-upgrade-operator",
	"openshift-marketplace",
	"openshift-must-gather-operator",
	"openshift-observability-operator",
	"openshift-ocm-agent-operator",
	"openshift-osd-metrics",
	"openshift-package-operator",
	"openshift-rbac-permissions",
	"openshift-route-monitor-operator",
	"openshift-security",
	"openshift-splunk-forwarder-operator",
	"openshift-sre-pruning",
	"openshift-validation-webhook",
	"openshift-velero",
)
