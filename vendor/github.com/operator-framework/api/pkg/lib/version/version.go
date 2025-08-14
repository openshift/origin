package version

import (
	"encoding/json"

	semver "github.com/blang/semver/v4"
)

// +k8s:openapi-gen=true
// OperatorVersion is a wrapper around semver.Version which supports correct
// marshaling to YAML and JSON.
// +kubebuilder:validation:Type=string
type OperatorVersion struct {
	semver.Version `json:"-"`
}

// DeepCopyInto creates a deep-copy of the Version value.
func (v *OperatorVersion) DeepCopyInto(out *OperatorVersion) {
	out.Major = v.Major
	out.Minor = v.Minor
	out.Patch = v.Patch

	if v.Pre != nil {
		pre := make([]semver.PRVersion, len(v.Pre))
		copy(pre, v.Pre)
		out.Pre = pre
	}

	if v.Build != nil {
		build := make([]string, len(v.Build))
		copy(build, v.Build)
		out.Build = build
	}
}

// MarshalJSON implements the encoding/json.Marshaler interface.
func (v OperatorVersion) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// UnmarshalJSON implements the encoding/json.Unmarshaler interface.
func (v *OperatorVersion) UnmarshalJSON(data []byte) (err error) {
	var versionString string

	if err = json.Unmarshal(data, &versionString); err != nil {
		return
	}

	version := semver.Version{}
	version, err = semver.ParseTolerant(versionString)
	if err != nil {
		return err
	}
	v.Version = version
	return
}

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
//
// See: https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
func (_ OperatorVersion) OpenAPISchemaType() []string { return []string{"string"} }

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
// "semver" is not a standard openapi format but tooling may use the value regardless
func (_ OperatorVersion) OpenAPISchemaFormat() string { return "semver" }
