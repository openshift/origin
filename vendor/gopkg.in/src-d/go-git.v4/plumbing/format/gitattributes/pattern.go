package gitattributes

import (
	"path/filepath"
	"strings"
)

const (
	patternDirSep  = "/"
	zeroToManyDirs = "**"
)

// Pattern defines a gitattributes pattern.
type Pattern interface {
	// Match matches the given path to the pattern.
	Match(path []string) bool
}

type pattern struct {
	domain  []string
	pattern []string
}

// ParsePattern parses a gitattributes pattern string into the Pattern
// structure.
func ParsePattern(p string, domain []string) Pattern {
	return &pattern{
		domain:  domain,
		pattern: strings.Split(p, patternDirSep),
	}
}

func (p *pattern) Match(path []string) bool {
	if len(path) <= len(p.domain) {
		return false
	}
	for i, e := range p.domain {
		if path[i] != e {
			return false
		}
	}

	if len(p.pattern) == 1 {
		// for a simple rule, .gitattribute matching rules differs from
		// .gitignore and only the last part of the path is considered.
		path = path[len(path)-1:]
	} else {
		path = path[len(p.domain):]
	}

	pattern := p.pattern
	var match, doublestar bool
	var err error
	for _, part := range path {
		// skip empty
		if pattern[0] == "" {
			pattern = pattern[1:]
		}

		// eat doublestar
		if pattern[0] == zeroToManyDirs {
			pattern = pattern[1:]
			if len(pattern) == 0 {
				return true
			}
			doublestar = true
		}

		switch {
		case strings.Contains(pattern[0], "**"):
			return false

		// keep going down the path until we hit a match
		case doublestar:
			match, err = filepath.Match(pattern[0], part)
			if err != nil {
				return false
			}

			if match {
				doublestar = false
				pattern = pattern[1:]
			}

		default:
			match, err = filepath.Match(pattern[0], part)
			if err != nil {
				return false
			}
			if !match {
				return false
			}
			pattern = pattern[1:]
		}
	}

	if len(pattern) > 0 {
		return false
	}
	return match
}
