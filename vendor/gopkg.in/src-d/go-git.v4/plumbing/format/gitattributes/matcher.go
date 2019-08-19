package gitattributes

// Matcher defines a global multi-pattern matcher for gitattributes patterns
type Matcher interface {
	// Match matches patterns in the order of priorities.
	Match(path []string, attributes []string) (map[string]Attribute, bool)
}

type MatcherOptions struct{}

// NewMatcher constructs a new matcher. Patterns must be given in the order of
// increasing priority. That is the most generic settings files first, then the
// content of the repo .gitattributes, then content of .gitattributes down the
// path.
func NewMatcher(stack []MatchAttribute) Matcher {
	m := &matcher{stack: stack}
	m.init()

	return m
}

type matcher struct {
	stack  []MatchAttribute
	macros map[string]MatchAttribute
}

func (m *matcher) init() {
	m.macros = make(map[string]MatchAttribute)

	for _, attr := range m.stack {
		if attr.Pattern == nil {
			m.macros[attr.Name] = attr
		}
	}
}

// Match matches path against the patterns in gitattributes files and returns
// the attributes associated with the path.
//
// Specific attributes can be specified otherwise all attributes are returned.
//
// Matched is true if any path was matched to a rule, even if the results map
// is empty.
func (m *matcher) Match(path []string, attributes []string) (results map[string]Attribute, matched bool) {
	results = make(map[string]Attribute, len(attributes))

	n := len(m.stack)
	for i := n - 1; i >= 0; i-- {
		if len(attributes) > 0 && len(attributes) == len(results) {
			return
		}

		pattern := m.stack[i].Pattern
		if pattern == nil {
			continue
		}

		if match := pattern.Match(path); match {
			matched = true
			for _, attr := range m.stack[i].Attributes {
				if attr.IsSet() {
					m.expandMacro(attr.Name(), results)
				}
				results[attr.Name()] = attr
			}
		}
	}
	return
}

func (m *matcher) expandMacro(name string, results map[string]Attribute) bool {
	if macro, ok := m.macros[name]; ok {
		for _, attr := range macro.Attributes {
			results[attr.Name()] = attr
		}
	}
	return false
}
