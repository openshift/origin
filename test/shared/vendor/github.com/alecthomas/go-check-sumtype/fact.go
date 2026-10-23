package gochecksumtype

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// sumTypeFact is a package-level fact that declares sum types and their variants.
// It is exported by a package that contains a //sumtype:decl annotation and
// imported by downstream packages to enable cross-package exhaustiveness checking.
type sumTypeFact struct {
	Definitions []sumTypeDefinitionFact
}

type sumTypeDefinitionFact struct {
	TypeName string
	Variants []string
}

var _ analysis.Fact = (*sumTypeFact)(nil)

// AFact implements [analysis.Fact].
func (*sumTypeFact) AFact() {}

func (f *sumTypeFact) String() string {
	definitions := make([]string, 0, len(f.Definitions))
	for _, definition := range f.Definitions {
		definitions = append(definitions, fmt.Sprintf("%s %v", definition.TypeName, definition.Variants))
	}
	return fmt.Sprintf("sumTypeFact{%s}", strings.Join(definitions, "; "))
}
