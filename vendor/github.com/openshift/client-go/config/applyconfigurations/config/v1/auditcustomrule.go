// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	configv1 "github.com/openshift/api/config/v1"
)

// AuditCustomRuleApplyConfiguration represents a declarative configuration of the AuditCustomRule type for use
// with apply.
type AuditCustomRuleApplyConfiguration struct {
	Group   *string                    `json:"group,omitempty"`
	Profile *configv1.AuditProfileType `json:"profile,omitempty"`
}

// AuditCustomRuleApplyConfiguration constructs a declarative configuration of the AuditCustomRule type for use with
// apply.
func AuditCustomRule() *AuditCustomRuleApplyConfiguration {
	return &AuditCustomRuleApplyConfiguration{}
}

// WithGroup sets the Group field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Group field is set to the value of the last call.
func (b *AuditCustomRuleApplyConfiguration) WithGroup(value string) *AuditCustomRuleApplyConfiguration {
	b.Group = &value
	return b
}

// WithProfile sets the Profile field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Profile field is set to the value of the last call.
func (b *AuditCustomRuleApplyConfiguration) WithProfile(value configv1.AuditProfileType) *AuditCustomRuleApplyConfiguration {
	b.Profile = &value
	return b
}
