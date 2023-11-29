package metadata

var (
	Required = []string{"owner"}
	All      = []Requirement{NewOwnerRequirement(), NewDescriptionRequirement()}
)
