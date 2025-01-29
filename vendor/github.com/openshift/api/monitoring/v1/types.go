package v1

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
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=alertingrules,scope=Namespaced
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1406
// +openshift:file-pattern=cvoRunLevel=0000_50,operatorName=monitoring,operatorOrdering=01
// +kubebuilder:metadata:annotations="description=OpenShift Monitoring alerting rules"
type AlertingRule struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the desired state of this AlertingRule object.
	// +required
	Spec AlertingRuleSpec `json:"spec"`

	// status describes the current state of this AlertOverrides object.
	//
	// +optional
	Status AlertingRuleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlertingRuleList is a list of AlertingRule objects.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:openapi-gen=true
type AlertingRuleList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of AlertingRule objects.
	// +optional
	Items []AlertingRule `json:"items,omitempty"`
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
	// +required
	Groups []RuleGroup `json:"groups"`
}

// Duration is a valid prometheus time duration.
// Supported units: y, w, d, h, m, s, ms
// Examples: `30s`, `1m`, `1h20m15s`, `15d`
// +kubebuilder:validation:Pattern:="^(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$"
// +kubebuilder:validation:MaxLength=2048
type Duration string

// RuleGroup is a list of sequentially evaluated alerting rules.
//
// +k8s:openapi-gen=true
type RuleGroup struct {
	// name is the name of the group.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	Name string `json:"name"`

	// interval is how often rules in the group are evaluated.  If not specified,
	// it defaults to the global.evaluation_interval configured in Prometheus,
	// which itself defaults to 30 seconds.  You can check if this value has been
	// modified from the default on your cluster by inspecting the platform
	// Prometheus configuration:
	// The relevant field in that resource is: spec.evaluationInterval
	//
	// +optional
	Interval Duration `json:"interval,omitempty"`

	// rules is a list of sequentially evaluated alerting rules.  Prometheus may
	// process rule groups in parallel, but rules within a single group are always
	// processed sequentially, and all rules are processed.
	//
	// +kubebuilder:validation:MinItems:=1
	// +required
	Rules []Rule `json:"rules"`
}

// Rule describes an alerting rule.
// See Prometheus documentation:
// - https://www.prometheus.io/docs/prometheus/latest/configuration/alerting_rules
//
// +k8s:openapi-gen=true
type Rule struct {
	// alert is the name of the alert. Must be a valid label value, i.e. may
	// contain any Unicode character.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	Alert string `json:"alert"`

	// expr is the PromQL expression to evaluate. Every evaluation cycle this is
	// evaluated at the current time, and all resultant time series become pending
	// or firing alerts.  This is most often a string representing a PromQL
	// expression, e.g.: mapi_current_pending_csr > mapi_max_pending_csr
	// In rare cases this could be a simple integer, e.g. a simple "1" if the
	// intent is to create an alert that is always firing.  This is sometimes used
	// to create an always-firing "Watchdog" alert in order to ensure the alerting
	// pipeline is functional.
	//
	// +required
	Expr intstr.IntOrString `json:"expr"`

	// for is the time period after which alerts are considered firing after first
	// returning results.  Alerts which have not yet fired for long enough are
	// considered pending.
	//
	// +optional
	For Duration `json:"for,omitempty"`

	// labels to add or overwrite for each alert.  The results of the PromQL
	// expression for the alert will result in an existing set of labels for the
	// alert, after evaluating the expression, for any label specified here with
	// the same name as a label in that set, the label here wins and overwrites
	// the previous value.  These should typically be short identifying values
	// that may be useful to query against.  A common example is the alert
	// severity, where one sets `severity: warning` under the `labels` key:
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations to add to each alert.  These are values that can be used to
	// store longer additional information that you won't query on, such as alert
	// descriptions or runbook links.
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
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	Name string `json:"name"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// AlertRelabelConfig defines a set of relabel configs for alerts.
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=alertrelabelconfigs,scope=Namespaced
// +kubebuilder:subresource:status
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/1406
// +openshift:file-pattern=cvoRunLevel=0000_50,operatorName=monitoring,operatorOrdering=02
// +kubebuilder:metadata:annotations="description=OpenShift Monitoring alert relabel configurations"
type AlertRelabelConfig struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec describes the desired state of this AlertRelabelConfig object.
	// +required
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
	// +required
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
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AlertRelabelConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is a list of AlertRelabelConfigs.
	// +optional
	Items []AlertRelabelConfig `json:"items,omitempty"`
}

// LabelName is a valid Prometheus label name which may only contain ASCII
// letters, numbers, and underscores.
//
// +kubebuilder:validation:Pattern:="^[a-zA-Z_][a-zA-Z0-9_]*$"
// +kubebuilder:validation:MaxLength=2048
type LabelName string

// RelabelConfig allows dynamic rewriting of label sets for alerts.
// See Prometheus documentation:
// - https://prometheus.io/docs/prometheus/latest/configuration/configuration/#alert_relabel_configs
// - https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
//
// +kubebuilder:validation:XValidation:rule="self.action != 'HashMod' || self.modulus != 0",message="relabel action hashmod requires non-zero modulus"
// +kubebuilder:validation:XValidation:rule="(self.action != 'Replace' && self.action != 'HashMod') || has(self.targetLabel)",message="targetLabel is required when action is Replace or HashMod"
// +kubebuilder:validation:XValidation:rule="(self.action != 'LabelDrop' && self.action != 'LabelKeep') || !has(self.sourceLabels)",message="LabelKeep and LabelDrop actions require only 'regex', and no other fields (found sourceLabels)"
// +kubebuilder:validation:XValidation:rule="(self.action != 'LabelDrop' && self.action != 'LabelKeep') || !has(self.targetLabel)",message="LabelKeep and LabelDrop actions require only 'regex', and no other fields (found targetLabel)"
// +kubebuilder:validation:XValidation:rule="(self.action != 'LabelDrop' && self.action != 'LabelKeep') || !has(self.modulus)",message="LabelKeep and LabelDrop actions require only 'regex', and no other fields (found modulus)"
// +kubebuilder:validation:XValidation:rule="(self.action != 'LabelDrop' && self.action != 'LabelKeep') || !has(self.separator)",message="LabelKeep and LabelDrop actions require only 'regex', and no other fields (found separator)"
// +kubebuilder:validation:XValidation:rule="(self.action != 'LabelDrop' && self.action != 'LabelKeep') || !has(self.replacement)",message="LabelKeep and LabelDrop actions require only 'regex', and no other fields (found replacement)"
// +kubebuilder:validation:XValidation:rule="!has(self.modulus) || (has(self.modulus) && size(self.sourceLabels) > 0)",message="modulus requires sourceLabels to be present"
// +kubebuilder:validation:XValidation:rule="(self.action == 'LabelDrop' || self.action == 'LabelKeep') || has(self.sourceLabels)",message="sourceLabels is required for actions Replace, Keep, Drop, HashMod and LabelMap"
// +kubebuilder:validation:XValidation:rule="(self.action != 'Replace' && self.action != 'LabelMap') || has(self.replacement)",message="replacement is required for actions Replace and LabelMap"
// +k8s:openapi-gen=true
type RelabelConfig struct {
	// sourceLabels select values from existing labels. Their content is
	// concatenated using the configured separator and matched against the
	// configured regular expression for the 'Replace', 'Keep', and 'Drop' actions.
	// Not allowed for actions 'LabelKeep' and 'LabelDrop'.
	//
	// +optional
	SourceLabels []LabelName `json:"sourceLabels,omitempty"`

	// separator placed between concatenated source label values. When omitted,
	// Prometheus will use its default value of ';'.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Separator string `json:"separator,omitempty"`

	// targetLabel to which the resulting value is written in a 'Replace' action.
	// It is required for 'Replace' and 'HashMod' actions and forbidden for
	// actions 'LabelKeep' and 'LabelDrop'. Regex capture groups
	// are available.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	TargetLabel string `json:"targetLabel,omitempty"`

	// regex against which the extracted value is matched. Default is: '(.*)'
	// regex is required for all actions except 'HashMod'
	//
	// +optional
	// +kubebuilder:default=(.*)
	// +kubebuilder:validation:MaxLength=2048
	Regex string `json:"regex,omitempty"`

	// modulus to take of the hash of the source label values.  This can be
	// combined with the 'HashMod' action to set 'target_label' to the 'modulus'
	// of a hash of the concatenated 'source_labels'. This is only valid if
	// sourceLabels is not empty and action is not 'LabelKeep' or 'LabelDrop'.
	//
	// +optional
	Modulus uint64 `json:"modulus,omitempty"`

	// replacement value against which a regex replace is performed if the regular
	// expression matches. This is required if the action is 'Replace' or
	// 'LabelMap' and forbidden for actions 'LabelKeep' and 'LabelDrop'.
	// Regex capture groups are available. Default is: '$1'
	//
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Replacement string `json:"replacement,omitempty"`

	// action to perform based on regex matching. Must be one of: 'Replace', 'Keep',
	// 'Drop', 'HashMod', 'LabelMap', 'LabelDrop', or 'LabelKeep'. Default is: 'Replace'
	//
	// +kubebuilder:validation:Enum=Replace;Keep;Drop;HashMod;LabelMap;LabelDrop;LabelKeep
	// +kubebuilder:default=Replace
	// +optional
	Action string `json:"action,omitempty"`
}
