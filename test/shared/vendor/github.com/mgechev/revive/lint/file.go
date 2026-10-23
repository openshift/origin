package lint

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"log/slog"
	"math"
	"regexp"
	"strings"
)

// File abstraction used for representing files.
type File struct {
	Name    string
	Pkg     *Package
	content []byte
	AST     *ast.File
	logger  *slog.Logger
}

// IsTest returns if the file contains tests.
func (f *File) IsTest() bool { return strings.HasSuffix(f.Name, "_test.go") }

// IsImportable returns if the symbols defined in this file can be imported in other packages.
//
// Symbols from the package `main` or test files are not exported, so they cannot be imported.
func (f *File) IsImportable() bool {
	if f.IsTest() {
		// Test files cannot be imported.
		return false
	}

	if f.Pkg.IsMain() {
		// The package `main` cannot be imported.
		return false
	}

	return true
}

// Content returns the file's content.
func (f *File) Content() []byte {
	return f.content
}

// NewFile creates a new file.
func NewFile(name string, content []byte, pkg *Package) (*File, error) {
	f, err := parser.ParseFile(pkg.fset, name, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return &File{
		Name:    name,
		content: content,
		Pkg:     pkg,
		AST:     f,
		logger:  slog.New(slog.DiscardHandler),
	}, nil
}

// ToPosition returns line and column for given position.
func (f *File) ToPosition(pos token.Pos) token.Position {
	return f.Pkg.fset.Position(pos)
}

// Render renders a node.
func (f *File) Render(x any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, f.Pkg.fset, x); err != nil {
		panic(err)
	}
	return buf.String()
}

// CommentMap builds a comment map for the file.
func (f *File) CommentMap() ast.CommentMap {
	return ast.NewCommentMap(f.Pkg.fset, f.AST, f.AST.Comments)
}

var basicTypeKinds = map[types.BasicKind]string{
	types.UntypedBool:    "bool",
	types.UntypedInt:     "int",
	types.UntypedRune:    "rune",
	types.UntypedFloat:   "float64",
	types.UntypedComplex: "complex128",
	types.UntypedString:  "string",
}

// IsUntypedConst reports whether expr is an untyped constant,
// and indicates what its default type is.
// Scope may be nil.
func (f *File) IsUntypedConst(expr ast.Expr) (defType string, ok bool) {
	// Re-evaluate expr outside its context to see if it's untyped.
	// (An expr evaluated within, for example, an assignment context will get the type of the LHS.)
	exprStr := f.Render(expr)
	tv, err := types.Eval(f.Pkg.fset, f.Pkg.TypesPkg(), expr.Pos(), exprStr)
	if err != nil {
		return "", false
	}
	if b, ok := tv.Type.(*types.Basic); ok {
		if dt, ok := basicTypeKinds[b.Kind()]; ok {
			return dt, true
		}
	}

	return "", false
}

func (f *File) isMain() bool {
	return f.AST.Name.Name == "main"
}

const (
	directiveSpecifyDisableReason = "specify-disable-reason"
	directiveSpecifyDisableRule   = "specify-disable-rule"
)

func (f *File) lint(rules []Rule, config Config, failures chan Failure) error {
	rulesConfig := config.Rules
	_, mustSpecifyDisableReason := config.Directives[directiveSpecifyDisableReason]
	_, mustSpecifyDisableRules := config.Directives[directiveSpecifyDisableRule]
	disabledIntervals := f.disabledIntervals(rules, mustSpecifyDisableReason, mustSpecifyDisableRules, failures)
	for _, currentRule := range rules {
		ruleConfig := rulesConfig[currentRule.Name()]
		if ruleConfig.MustExclude(f.Name) {
			continue
		}
		currentFailures := currentRule.Apply(f, ruleConfig.Arguments)
		filtered := currentFailures[:0]
		for _, failure := range currentFailures {
			// Log and skip internal failures: they signal a rule could not run on this file,
			// but other rules can still produce useful reports.
			if failure.IsInternal() {
				f.logger.Warn("rule skipped due to internal failure",
					"rule", currentRule.Name(),
					"file", f.Name,
					"failure", failure.Failure,
				)
				continue
			}

			if failure.RuleName == "" {
				failure.RuleName = currentRule.Name()
			}
			if failure.Node != nil {
				failure.Position = ToFailurePosition(failure.Node.Pos(), failure.Node.End(), f)
			}
			filtered = append(filtered, failure)
		}
		currentFailures = f.filterFailures(filtered, disabledIntervals)
		for _, failure := range currentFailures {
			if failure.Confidence >= config.Confidence {
				failures <- failure
			}
		}
	}
	return nil
}

type enableDisableConfig struct {
	enabled  bool
	position int
}

type disabledIntervalsMap = map[string][]DisabledInterval

const (
	directivePos = 1
	modifierPos  = 2
	rulesPos     = 3
	reasonPos    = 4
)

var directiveRegexp = regexp.MustCompile(`^//[\s]*revive:(enable|disable)(?:-(line|next-line))?(?::([^\s]+))?[\s]*(?: (.+))?$`)

func (f *File) disabledIntervals(rules []Rule, mustSpecifyDisableReason, mustSpecifyDisableRules bool, failures chan Failure) disabledIntervalsMap {
	enabledDisabledRulesMap := map[string][]enableDisableConfig{}

	getEnabledDisabledIntervals := func() disabledIntervalsMap {
		result := disabledIntervalsMap{}

		for ruleName, disabledArr := range enabledDisabledRulesMap {
			ruleResult := []DisabledInterval{}
			for i := range disabledArr {
				interval := DisabledInterval{
					RuleName: ruleName,
					From: token.Position{
						Filename: f.Name,
						Line:     disabledArr[i].position,
					},
					To: token.Position{
						Filename: f.Name,
						Line:     math.MaxInt32,
					},
				}
				if i%2 == 0 {
					ruleResult = append(ruleResult, interval)
				} else {
					ruleResult[len(ruleResult)-1].To.Line = disabledArr[i].position
				}
			}
			result[ruleName] = ruleResult
		}

		return result
	}

	// handleConfig records a state change for the rule and reports whether it changed anything.
	// A directive that repeats the current state (e.g. a second disable) is a no-op.
	handleConfig := func(isEnabled bool, line int, name string) bool {
		existing, ok := enabledDisabledRulesMap[name]
		if !ok {
			existing = []enableDisableConfig{}
			enabledDisabledRulesMap[name] = existing
		}
		currentlyEnabled := len(existing) == 0 || existing[len(existing)-1].enabled
		if currentlyEnabled == isEnabled {
			return false
		}
		enabledDisabledRulesMap[name] = append(existing, enableDisableConfig{
			enabled:  isEnabled,
			position: line,
		})
		return true
	}

	handleRules := func(modifier string, isEnabled bool, line int, ruleNames []string) {
		for _, name := range ruleNames {
			switch modifier {
			case "line":
				if handleConfig(isEnabled, line, name) {
					handleConfig(!isEnabled, line, name)
				}
			case "next-line":
				if handleConfig(isEnabled, line+1, name) {
					handleConfig(!isEnabled, line+1, name)
				}
			default:
				handleConfig(isEnabled, line, name)
			}
		}
	}

	handleComment := func(c *ast.CommentGroup, line int) {
		comments := c.List
		for _, c := range comments {
			match := directiveRegexp.FindStringSubmatch(c.Text)
			if len(match) == 0 {
				continue
			}
			ruleNames := []string{}

			for name := range strings.SplitSeq(match[rulesPos], ",") {
				name = strings.Trim(name, "\n")
				if name != "" {
					ruleNames = append(ruleNames, name)
				}
			}

			mustCheckDisablingReason := mustSpecifyDisableReason && match[directivePos] == "disable"
			if mustCheckDisablingReason && strings.Trim(match[reasonPos], " ") == "" {
				failures <- Failure{
					Category:   FailureCategoryComments,
					Confidence: 1,
					RuleName:   directiveSpecifyDisableReason,
					Failure:    "reason of lint disabling not found",
					Position:   ToFailurePosition(c.Pos(), c.End(), f),
					Node:       c,
				}
				continue // skip this linter disabling directive
			}

			mustCheckDisablingRules := mustSpecifyDisableRules && match[directivePos] == "disable"
			if mustCheckDisablingRules && len(ruleNames) == 0 {
				failures <- Failure{
					Category:   FailureCategoryComments,
					Confidence: 1,
					RuleName:   directiveSpecifyDisableRule,
					Failure:    "rule name for lint disabling not found",
					Position:   ToFailurePosition(c.Pos(), c.End(), f),
					Node:       c,
				}
				continue // skip this linter disabling directive
			}

			// TODO: optimize
			if len(ruleNames) == 0 {
				for _, rule := range rules {
					ruleNames = append(ruleNames, rule.Name())
				}
			}

			handleRules(match[modifierPos], match[directivePos] == "enable", line, ruleNames)
		}
	}

	for _, c := range f.AST.Comments {
		handleComment(c, f.ToPosition(c.End()).Line)
	}

	return getEnabledDisabledIntervals()
}

func (*File) filterFailures(failures []Failure, disabledIntervals disabledIntervalsMap) []Failure {
	result := []Failure{}
	for _, failure := range failures {
		fStart := failure.Position.Start.Line
		fEnd := failure.Position.End.Line
		intervals, ok := disabledIntervals[failure.RuleName]
		if !ok {
			result = append(result, failure)
			continue
		}

		include := true
		for _, interval := range intervals {
			intStart := interval.From.Line
			intEnd := interval.To.Line
			if (fStart >= intStart && fStart <= intEnd) ||
				(fEnd >= intStart && fEnd <= intEnd) {
				include = false
				break
			}
		}
		if include {
			result = append(result, failure)
		}
	}
	return result
}
