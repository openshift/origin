package metadata

var (
	Required = []Requirement{NewOwnerRequirement()}
	All      = []Requirement{NewOwnerRequirement(), NewDescriptionRequirement()}
)
