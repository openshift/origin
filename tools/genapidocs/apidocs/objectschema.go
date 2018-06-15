package apidocs

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-openapi/spec"
)

type line struct {
	Prefix string
	Name   string
	Schema *spec.Schema
	Lines  []*line
}

func newLine(prefix, name string, schema *spec.Schema) *line {
	return &line{
		Prefix: prefix,
		Name:   name,
		Schema: schema,
	}
}

func (l *line) Tooltip() string {
	tooltipText := fmt.Sprintf("(%s)", FriendlyTypeName(l.Schema))
	if l.Schema.Description != "" {
		tooltipText += " " + l.Schema.Description
	}
	return tooltipText
}

// Open controls which lines have the details displayed
func (l *line) Open() bool {
	return l.Prefix == "" && (l.Name == "metadata" || l.Name == "spec")
}

func buildLines(s *spec.Swagger, schema spec.Schema, prefix string) (lines []*line) {
	for _, name := range SortedKeys(schema.Properties, reflect.TypeOf(sort.StringSlice{})).(sort.StringSlice) {
		property := schema.Properties[name]

		l := newLine(prefix, name, &property)
		switch {
		case property.Type.Contains("array"):
			if property.Items.Schema.Ref.String() != "" {
				l.Lines = buildLines(s, s.Definitions[RefType(property.Items.Schema)], prefix+"- ")
			} else {
				l.Lines = []*line{newLine(prefix+"- ", "["+property.Items.Schema.Type[0]+"]", property.Items.Schema)}
			}

		case property.Type.Contains("object"):
			l.Lines = []*line{newLine(prefix+"  ", "[string]", &spec.Schema{SchemaProps: spec.SchemaProps{Type: []string{"string"}}})}

		case RefType(&property) != "" && len(s.Definitions[RefType(&property)].Properties) > 0:
			l.Lines = buildLines(s, s.Definitions[RefType(&property)], prefix+"  ")
		}
		lines = append(lines, l)
		prefix = strings.Replace(prefix, "-", " ", -1)
	}

	return
}
