package operators

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName is the group name used in this package.
	GroupName = "operators.coreos.com"
	// GroupVersion is the group version used in this package.
	GroupVersion = runtime.APIVersionInternal

	// LEGACY: Exported kind names, remove after major version bump

	// ClusterServiceVersionKind is the kind name for ClusterServiceVersion resources.
	ClusterServiceVersionKind = "ClusterServiceVersion"
	// CatalogSourceKind is the kind name for CatalogSource resources.
	CatalogSourceKind = "CatalogSource"
	// InstallPlanKind is the kind name for InstallPlan resources.
	InstallPlanKind = "InstallPlan"
	// SubscriptionKind is the kind name for Subscription resources.
	SubscriptionKind = "Subscription"
	// OperatorKind is the kind name for Operator resources.
	OperatorKind = "Operator"
	// OperatorGroupKind is the kind name for OperatorGroup resources.
	OperatorGroupKind = "OperatorGroup"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
