package rule

import (
	"go/token"
	"regexp"
	"strings"

	"github.com/mgechev/revive/internal/config"
	"github.com/mgechev/revive/lint"
)

func srcLine(src []byte, p token.Position) string {
	// Run to end of line in both directions if not at line start/end.
	lo, hi := p.Offset, p.Offset+1
	for lo > 0 && src[lo-1] != '\n' {
		lo--
	}
	for hi < len(src) && src[hi-1] != '\n' {
		hi++
	}
	return string(src[lo:hi])
}

// isRuleOption returns true if arg and name are the same after normalization.
func isRuleOption(arg, name string) bool {
	return config.NormalizeOption(arg) == config.NormalizeOption(name)
}

var normalizePathReplacer = strings.NewReplacer("-", "", "_", "", ".", "")

// normalizePath removes hyphens, underscores, and dots from the name
//
// Example: normalizePath("foo.bar-_buz") -> "foobarbuz".
func normalizePath(name string) string {
	return normalizePathReplacer.Replace(name)
}

// isVersionPath checks if a directory name is a version directory (v1, V2, etc.)
func isVersionPath(name string) bool {
	if len(name) < 2 || (name[0] != 'v' && name[0] != 'V') {
		return false
	}

	for i := 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}

	return true
}

var directiveCommentRE = regexp.MustCompile("^//(line |extern |export |[a-z0-9]+:[a-z0-9])") // see https://go-review.googlesource.com/c/website/+/442516/1..2/_content/doc/comment.md#494

func isDirectiveComment(line string) bool {
	return directiveCommentRE.MatchString(line)
}

// newInternalFailureError returns a slice of Failure with a single internal failure in it.
func newInternalFailureError(e error) []lint.Failure {
	return []lint.Failure{lint.NewInternalFailure(e.Error())}
}
