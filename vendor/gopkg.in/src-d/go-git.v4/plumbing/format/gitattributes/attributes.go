package gitattributes

import (
	"errors"
	"io"
	"io/ioutil"
	"strings"
)

const (
	commentPrefix = "#"
	eol           = "\n"
	macroPrefix   = "[attr]"
)

var (
	ErrMacroNotAllowed      = errors.New("macro not allowed")
	ErrInvalidAttributeName = errors.New("Invalid attribute name")
)

type MatchAttribute struct {
	Name       string
	Pattern    Pattern
	Attributes []Attribute
}

type attributeState byte

const (
	attributeUnknown     attributeState = 0
	attributeSet         attributeState = 1
	attributeUnspecified attributeState = '!'
	attributeUnset       attributeState = '-'
	attributeSetValue    attributeState = '='
)

type Attribute interface {
	Name() string
	IsSet() bool
	IsUnset() bool
	IsUnspecified() bool
	IsValueSet() bool
	Value() string
	String() string
}

type attribute struct {
	name  string
	state attributeState
	value string
}

func (a attribute) Name() string {
	return a.name
}

func (a attribute) IsSet() bool {
	return a.state == attributeSet
}

func (a attribute) IsUnset() bool {
	return a.state == attributeUnset
}

func (a attribute) IsUnspecified() bool {
	return a.state == attributeUnspecified
}

func (a attribute) IsValueSet() bool {
	return a.state == attributeSetValue
}

func (a attribute) Value() string {
	return a.value
}

func (a attribute) String() string {
	switch a.state {
	case attributeSet:
		return a.name + ": set"
	case attributeUnset:
		return a.name + ": unset"
	case attributeUnspecified:
		return a.name + ": unspecified"
	default:
		return a.name + ": " + a.value
	}
}

// ReadAttributes reads patterns and attributes from the gitattributes format.
func ReadAttributes(r io.Reader, domain []string, allowMacro bool) (attributes []MatchAttribute, err error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(data), eol) {
		attribute, err := ParseAttributesLine(line, domain, allowMacro)
		if err != nil {
			return attributes, err
		}
		if len(attribute.Name) == 0 {
			continue
		}

		attributes = append(attributes, attribute)
	}

	return attributes, nil
}

// ParseAttributesLine parses a gitattribute line, extracting path pattern and
// attributes.
func ParseAttributesLine(line string, domain []string, allowMacro bool) (m MatchAttribute, err error) {
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, commentPrefix) || len(line) == 0 {
		return
	}

	name, unquoted := unquote(line)
	attrs := strings.Fields(unquoted)
	if len(name) == 0 {
		name = attrs[0]
		attrs = attrs[1:]
	}

	var macro bool
	macro, name, err = checkMacro(name, allowMacro)
	if err != nil {
		return
	}

	m.Name = name
	m.Attributes = make([]Attribute, 0, len(attrs))

	for _, attrName := range attrs {
		attr := attribute{
			name:  attrName,
			state: attributeSet,
		}

		// ! and - prefixes
		state := attributeState(attr.name[0])
		if state == attributeUnspecified || state == attributeUnset {
			attr.state = state
			attr.name = attr.name[1:]
		}

		kv := strings.SplitN(attrName, "=", 2)
		if len(kv) == 2 {
			attr.name = kv[0]
			attr.value = kv[1]
			attr.state = attributeSetValue
		}

		if !validAttributeName(attr.name) {
			return m, ErrInvalidAttributeName
		}
		m.Attributes = append(m.Attributes, attr)
	}

	if !macro {
		m.Pattern = ParsePattern(name, domain)
	}
	return
}

func checkMacro(name string, allowMacro bool) (macro bool, macroName string, err error) {
	if !strings.HasPrefix(name, macroPrefix) {
		return false, name, nil
	}
	if !allowMacro {
		return true, name, ErrMacroNotAllowed
	}

	macroName = name[len(macroPrefix):]
	if !validAttributeName(macroName) {
		return true, name, ErrInvalidAttributeName
	}
	return true, macroName, nil
}

func validAttributeName(name string) bool {
	if len(name) == 0 || name[0] == '-' {
		return false
	}

	for _, ch := range name {
		if !(ch == '-' || ch == '.' || ch == '_' ||
			('0' <= ch && ch <= '9') ||
			('a' <= ch && ch <= 'z') ||
			('A' <= ch && ch <= 'Z')) {
			return false
		}
	}
	return true
}

func unquote(str string) (string, string) {
	if str[0] != '"' {
		return "", str
	}

	for i := 1; i < len(str); i++ {
		switch str[i] {
		case '\\':
			i++
		case '"':
			return str[1:i], str[i+1:]
		}
	}
	return "", str
}
