package resourcegraph

import (
	"fmt"
)

type simpleSource struct {
	coordinates ResourceCoordinates
	note        string
	nested      []Resource
	sources     []Resource
}

func (r *simpleSource) Coordinates() ResourceCoordinates {
	return r.coordinates
}

func (s *simpleSource) Add(resources Resources) Resource {
	resources.Add(s)
	return s
}

func (s *simpleSource) From(source Resource) Resource {
	s.sources = append(s.sources, source)
	return s
}

func (s *simpleSource) Note(note string) Resource {
	s.note = note
	return s
}

func (s *simpleSource) String() string {
	return fmt.Sprintf("%v%s", s.coordinates, s.note)
}

func (s *simpleSource) GetNote() string {
	return s.note
}

func (s *simpleSource) Sources() []Resource {
	return s.sources
}

func (r *simpleSource) Dump(indentDepth int) []string {
	lines := []string{}
	lines = append(lines, indent(indentDepth, r.String()))

	for _, nested := range r.nested {
		lines = append(lines, nested.Dump(indentDepth+1)...)
	}

	return lines
}

func (r *simpleSource) DumpSources(indentDepth int) []string {
	lines := []string{}
	lines = append(lines, indent(indentDepth, r.String()))

	for _, source := range r.sources {
		lines = append(lines, source.DumpSources(indentDepth+1)...)
	}

	return lines
}

func indent(depth int, in string) string {
	indent := ""
	for i := 0; i < depth; i++ {
		indent = indent + "    "
	}
	return indent + in
}
