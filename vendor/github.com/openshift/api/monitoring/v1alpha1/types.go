package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// AlertingRule represents a set of user-defined Prometheus rule groups containing
// alerting rules.  This resource is the supported method for cluster admins to
// create alerts based on metrics recorded by the platform monitoring stack in
// OpenShift, i.e. the Prometheus instance deployed to the openshift-monitoring
// namespace.  You might use this to create custom alerting rules not shipped with
// OpenShift based on metrics from components such as the node_exporter, which
// provides machine-level metrics such as CPU usage, or kube-state-metrics, which
// provides metrics on Kubernetes usage.
//
// The API is mostly compatible with the upstream PrometheusRule type from the
// prometheus-operator.  The primary difference being that recording rules are not
// allowed here -- only alerting rules.  For each AlertingRule resource created, a
// corresponding PrometheusRule will be created in the openshift-monitoring
// namespace.  OpenShift requires admins to use the AlertingRule resource rather
// than the upstream type in order to allow better OpenShift specific defaulting
// and validation, while not modifying the upstream APIs directly.
//
// You can find upstream API documentation for PrometheusRule resources here:
//
// https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type AlertingRule struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the desired state of this AlertingRule object.
	Spec AlertingRuleSpec `json:"spec"`

	// status describes the current state of this AlertOverrides object.
	//
	// +optional
	Status AlertingRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlertingRuleList is a list of AlertingRule objects.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +k8s:openapi-gen=true
type AlertingRuleList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of AlertingRule objects.
	Items []AlertingRule `json:"items"`
}

// AlertingRuleSpec is the desired state of an AlertingRule resource.
//
// +k8s:openapi-gen=true
type AlertingRuleSpec struct {
	// groups is a list of grouped alerting rules.  Rule groups are the unit at
	// which Prometheus parallelizes rule processing.  All rules in a single group
	// share a configured evaluation interval.  All rules in the group will be
	// processed together on this interval, sequentially, and all rules will be
	// processed.
	//
	// It's common to group related alerting rules into a single AlertingRule
	// resources, and within that resource, closely related alerts, or simply
	// alerts with the same interval, into individual groups.  You are also free
	// to create AlertingRule resources with only a single rule group, but be
	// aware that this can have a performance impact on Prometheus if the group is
	// extremely large or has very complex query expressions to evaluate.
	// Spreading very complex rules across multiple groups to allow them to be
	// processed in parallel is also a common use-case.
	//
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems:=1
	Groups []RuleGroup `json:"groups"`
}

// RuleGroup is a list of sequentially evaluated alerting rules.
//
// +k8s:openapi-gen=true
type RuleGroup struct {
	// name is the name of the group.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// interval is how often rules in the group are evaluated.  If not specified,
	// it defaults to the global.evaluation_interval configured in Prometheus,
	// which itself defaults to 30 seconds.  You can check if this value has been
	// modified from the default on your cluster by inspecting the platform
	// Prometheus configuration:
	//
	// $ oc -n openshift-monitoring describe prometheus k8s
	//
	// The relevant field in that resource is: spec.evaluationInterval
	//
	// This is represented as a Prometheus duration, e.g. 1d, 1h30m, 5m, 10s.  You
	// can find the upstream documentation here:
	//
	// https://prometheus.io/docs/prometheus/latest/configuration/configuration/#duration
	//
	// +kubebuilder:validation:Pattern:="^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$"
	// +optional
	Interval string `json:"interval,omitempty"`

	// rules is a list of sequentially evaluated alerting rules.  Prometheus may
	// process rule groups in parallel, but rules within a single group are always
	// processed sequentially, and all rules are processed.
	//
	// +kubebuilder:validation:MinItems:=1
	Rules []Rule `json:"rules"`
}

// Rule describes an alerting rule.
// See Prometheus documentation:
// - https://www.prometheus.io/docs/prometheus/latest/configuration/alerting_rules
//
// +k8s:openapi-gen=true
type Rule struct {
	// alert is the name of the alert. Must be a valid label value, i.e. only
	// contain ASCII letters, numbers, and underscores.
	//
	// +kubebuilder:validation:Pattern:="^[a-zA-Z_][a-zA-Z0-9_]*$"
	// +required
	Alert string `json:"alert"`

	// expr is the PromQL expression to evaluate. Every evaluation cycle this is
	// evaluated at the current time, and all resultant time series become pending
	// or firing alerts.  This is most often a string representing a PromQL
	// expression, e.g.:
	//
	//   mapi_current_pending_csr > mapi_max_pending_csr
	//
	// In rare cases this could be a simple integer, e.g. a simple "1" if the
	// intent is to create an alert that is always firing.  This is sometimes used
	// to create an always-firing "Watchdog" alert in order to ensure the alerting
	// pipeline is functional.
	//
	// +required
	Expr intstr.IntOrString `json:"expr"`

	// for is the time period after which alerts are considered firing after first
	// returning results.  Alerts which have not yet fired for long enough are
	// considered pending. This is represented as a Prometheus duration, for
	// details on the format see:
	// - https://prometheus.io/docs/prometheus/latest/configuration/configuration/#duration
	//
	// +kubebuilder:validation:Pattern:="^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$"
	// +optional
	For string `json:"for,omitempty"`

	// labels to add or overwrite for each alert.  The results of the PromQL
	// expression for the alert will result in an existing set of labels for the
	// alert, after evaluating the expression, for any label specified here with
	// the same name as a label in that set, the label here wins and overwrites
	// the previous value.  These should typically be short identifying values
	// that may be useful to query against.  A common example is the alert
	// severity:
	//
	//   labels:
	//     severity: warning
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations to add to each alert.  These are values that can be used to
	// store longer additional information that you won't query on, such as alert
	// descriptions or runbook links, e.g.:
	//
	//   annotations:
	//     summary: HAProxy reload failure
	//     description: |
	//       This alert fires when HAProxy fails to reload its
	//       configuration, which will result in the router not picking up
	//       recently created or modified routes.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// AlertingRuleStatus is the status of an AlertingRule resource.
type AlertingRuleStatus struct {
	// observedGeneration is the last generation change you've dealt with.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// prometheusRule is the generated PrometheusRule for this AlertingRule.  Each
	// AlertingRule instance results in a generated PrometheusRule object in the
	// same namespace, which is always the openshift-monitoring namespace.
	//
	// +optional
	PrometheusRule PrometheusRuleRef `json:"prometheusRule,omitempty"`
}

// PrometheusRuleRef is a reference to an existing PrometheusRule object.  Each
// AlertingRule instance results in a generated PrometheusRule object in the same
// namespace, which is always the openshift-monitoring namespace.  This is used to
// point to the generated PrometheusRule object in the AlertingRule status.
type PrometheusRuleRef struct {
	// This is a struct so that we can support future expansion of fields within
	// the reference should we ever need to.

	// name of the referenced PrometheusRule.
	Name string `json:"name"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// AlertRelabelConfig defines a set of relabel configs for alerts.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +k8s:openapi-gen=true
type AlertRelabelConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the desired state of this AlertRelabelConfig object.
	Spec AlertRelabelConfigSpec `json:"spec"`

	// status describes the current state of this AlertRelabelConfig object.
	//
	// +optional
	Status AlertRelabelConfigStatus `json:"status,omitempty"`
}

// AlertRelabelConfigsSpec is the desired state of an AlertRelabelConfig resource.
//
// +k8s:openapi-gen=true
type AlertRelabelConfigSpec struct {
	// configs is a list of sequentially evaluated alert relabel configs.
	//
	// +kubebuilder:validation:MinItems:=1
	Configs []RelabelConfig `json:"configs"`
}

// AlertRelabelConfigStatus is the status of an AlertRelabelConfig resource.
type AlertRelabelConfigStatus struct {
	// conditions contains details on the state of the AlertRelabelConfig, may be
	// empty.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	// AlertRelabelConfigReady is the condition type indicating readiness.
	AlertRelabelConfigReady string = "Ready"
)

// AlertRelabelConfigList is a list of AlertRelabelConfigs.
//
// Compatibility level 4: No compatibility is provided, the API can change at any point for any reason. These capabilities should not be used by applications needing long term support.
// +openshift:compatibility-gen:level=4
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AlertRelabelConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of AlertRelabelConfigs.
	Items []*AlertRelabelConfig `json:"items"`
}

// LabelName is a valid Prometheus label name which may only contain ASCII
// letters, numbers, and underscores.
//
// +kubebuilder:validation:Pattern:="^[a-zA-Z_][a-zA-Z0-9_]*$"
type LabelName string

// RelabelConfig allows dynamic rewriting of label sets for alerts.
// See Prometheus documentation:
// - https://prometheus.io/docs/prometheus/latest/configuration/configuration/#alert_relabel_configs
// - https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
//
// +k8s:openapi-gen=true
type RelabelConfig struct {
	// sourceLabels select values from existing labels. Their content is
	// concatenated using the configured separator and matched against the
	// configured regular expression for the Replace, Keep, and Drop actions.
	//
	// +optional
	SourceLabels []LabelName `json:"sourceLabels,omitempty"`

	// separator placed between concatenated source label values. When omitted,
	// Prometheus will use its default value of ';'.
	//
	// +optional
	Separator string `json:"separator,omitempty"`

	// targetLabel to which the resulting value is written in a 'Replace' action.
	// It is mandatory for 'Replace' and 'HashMod' actions. Regex capture groups
	// are available.
	//
	// +optional
	TargetLabel string `json:"targetLabel,omitempty"`

	// regex against which the extracted value is matched. Default is: '(.*)'
	//
	// +optional
	Regex string `json:"regex,omitempty"`

	// modulus to take of the hash of the source label values.  This can be
	// combined with the 'HashMod' action to set 'target_label' to the 'modulus'
	// of a hash of the concatenated 'source_labels'.
	//
	// +optional
	Modulus uint64 `json:"modulus,omitempty"`

	// replacement value against which a regex replace is performed if the regular
	// expression matches. This is required if the action is 'Replace' or
	// 'LabelMap'. Regex capture groups are available. Default is: '$1'
	//
	// +optional
	Replacement string `json:"replacement,omitempty"`

	// action to perform based on regex matching. Must be one of: Replace, Keep,
	// Drop, HashMod, LabelMap, LabelDrop, or LabelKeep.  Default is: 'Replace'
	//
	// +kubebuilder:validation:Enum=Replace;Keep;Drop;HashMod;LabelMap;LabelDrop;LabelKeep
	// +kubebuilder:default=Replace
	// +optional
	Action string `json:"action,omitempty"`
}
