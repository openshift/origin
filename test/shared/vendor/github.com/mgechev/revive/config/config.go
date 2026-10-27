// Package config implements revive's configuration data structures and related methods.
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/mgechev/revive/formatter"
	internalconfig "github.com/mgechev/revive/internal/config"
	"github.com/mgechev/revive/lint"
	"github.com/mgechev/revive/rule"
)

var defaultRules = []lint.Rule{
	&rule.VarDeclarationsRule{},
	&rule.PackageCommentsRule{},
	&rule.DotImportsRule{},
	&rule.BlankImportsRule{},
	&rule.ExportedRule{},
	&rule.VarNamingRule{},
	&rule.IndentErrorFlowRule{},
	&rule.RangeRule{},
	&rule.ErrorfRule{},
	&rule.ErrorNamingRule{},
	&rule.ErrorStringsRule{},
	&rule.ReceiverNamingRule{},
	&rule.IncrementDecrementRule{},
	&rule.ErrorReturnRule{},
	&rule.UnexportedReturnRule{},
	&rule.TimeNamingRule{},
	&rule.ContextKeysType{},
	&rule.ContextAsArgumentRule{},
	&rule.EmptyBlockRule{},
	&rule.SuperfluousElseRule{},
	&rule.UnusedParamRule{},
	&rule.UnreachableCodeRule{},
	&rule.RedefinesBuiltinIDRule{},
}

var allRules = append([]lint.Rule{
	&rule.ArgumentsLimitRule{},
	&rule.CyclomaticRule{},
	&rule.FileHeaderRule{},
	&rule.ConfusingNamingRule{},
	&rule.GetReturnRule{},
	&rule.ModifiesParamRule{},
	&rule.ConfusingResultsRule{},
	&rule.DeepExitRule{},
	&rule.AddConstantRule{},
	&rule.FlagParamRule{},
	&rule.UnnecessaryStmtRule{},
	&rule.StructTagRule{},
	&rule.ModifiesValRecRule{},
	&rule.ConstantLogicalExprRule{},
	&rule.BoolLiteralRule{},
	&rule.ImportsBlocklistRule{},
	&rule.FunctionResultsLimitRule{},
	&rule.MaxPublicStructsRule{},
	&rule.RangeValInClosureRule{},
	&rule.RangeValAddress{},
	&rule.WaitGroupByValueRule{},
	&rule.AtomicRule{},
	&rule.EmptyLinesRule{},
	&rule.LineLengthLimitRule{},
	&rule.CallToGCRule{},
	&rule.DuplicatedImportsRule{},
	&rule.ImportShadowingRule{},
	&rule.BareReturnRule{},
	&rule.UnusedReceiverRule{},
	&rule.UnhandledErrorRule{},
	&rule.CognitiveComplexityRule{},
	&rule.StringOfIntRule{},
	&rule.StringFormatRule{},
	&rule.EarlyReturnRule{},
	&rule.UnconditionalRecursionRule{},
	&rule.IdenticalBranchesRule{},
	&rule.DeferRule{},
	&rule.UnexportedNamingRule{},
	&rule.FunctionLength{},
	&rule.NestedStructs{},
	&rule.UselessBreak{},
	&rule.UncheckedTypeAssertionRule{},
	&rule.TimeEqualRule{},
	&rule.TimeDateRule{},
	&rule.BannedCharsRule{},
	&rule.OptimizeOperandsOrderRule{},
	&rule.UseAnyRule{},
	&rule.DataRaceRule{},
	&rule.CommentSpacingsRule{},
	&rule.IfReturnRule{},
	&rule.RedundantImportAlias{},
	&rule.ImportAliasNamingRule{},
	&rule.EnforceMapStyleRule{},
	&rule.EnforceRepeatedArgTypeStyleRule{},
	&rule.EnforceSliceStyleRule{},
	&rule.MaxControlNestingRule{},
	&rule.CommentsDensityRule{},
	&rule.FileLengthLimitRule{},
	&rule.FilenameFormatRule{},
	&rule.RedundantBuildTagRule{},
	&rule.UseErrorsNewRule{},
	&rule.RedundantTestMainExitRule{},
	&rule.UnnecessaryFormatRule{},
	&rule.UseFmtPrintRule{},
	&rule.EnforceSwitchStyleRule{},
	&rule.IdenticalSwitchConditionsRule{},
	&rule.IdenticalIfElseIfConditionsRule{},
	&rule.IdenticalIfElseIfBranchesRule{},
	&rule.IdenticalSwitchBranchesRule{},
	&rule.UselessFallthroughRule{},
	&rule.PackageDirectoryMismatchRule{},
	&rule.UseWaitGroupGoRule{},
	&rule.UnsecureURLSchemeRule{},
	&rule.InefficientMapLookupRule{},
	&rule.ForbiddenCallInWgGoRule{},
	&rule.UnnecessaryIfRule{},
	&rule.EpochNamingRule{},
	&rule.UseSlicesSort{},
	&rule.UseSlicesConcatRule{},
	&rule.PackageNamingRule{},
	&rule.MultilineIfInitRule{},
	&rule.MarshalReceiverRule{},
}, defaultRules...)

// AllRules returns a copy of the list of all rules registered in revive.
func AllRules() []lint.Rule {
	return slices.Clone(allRules)
}

// DefaultRules returns a copy of the list of rules that are enabled by default.
func DefaultRules() []lint.Rule {
	return slices.Clone(defaultRules)
}

// EnabledRules returns the rules that are enabled in the given configuration.
func EnabledRules(config *lint.Config) []lint.Rule {
	if config == nil {
		return nil
	}
	rulesByName := make(map[string]lint.Rule, len(allRules))
	for _, r := range allRules {
		rulesByName[r.Name()] = r
	}
	var rules []lint.Rule
	for name, c := range config.Rules {
		if c.Disabled {
			continue
		}
		if r, ok := rulesByName[actualRuleName(name)]; ok {
			rules = append(rules, r)
		}
	}
	return rules
}

// allFormatters is a list of all available formatters to output the linting results.
// Keep the list sorted and in sync with available formatters in README.md.
var allFormatters = []lint.Formatter{
	&formatter.Checkstyle{},
	&formatter.Default{},
	&formatter.Friendly{},
	&formatter.JSON{},
	&formatter.NDJSON{},
	&formatter.Plain{},
	&formatter.Sarif{},
	&formatter.Stylish{},
	&formatter.Unix{},
}

func getFormatters() map[string]lint.Formatter {
	result := map[string]lint.Formatter{}
	for _, f := range allFormatters {
		result[f.Name()] = f
	}
	return result
}

// GetLintingRules yields the linting rules that must be applied by the linter.
func GetLintingRules(config *lint.Config, extraRules []lint.Rule) ([]lint.Rule, error) {
	rulesMap := map[string]lint.Rule{}
	for _, r := range allRules {
		rulesMap[r.Name()] = r
	}
	for _, r := range extraRules {
		if _, ok := rulesMap[r.Name()]; ok {
			continue
		}
		rulesMap[r.Name()] = r
	}

	var lintingRules []lint.Rule
	for name, ruleConfig := range config.Rules {
		actualName := actualRuleName(name)
		r, ok := rulesMap[actualName]
		if !ok {
			return nil, fmt.Errorf("cannot find rule: %s", name)
		}

		if ruleConfig.Disabled {
			continue // skip disabled rules
		}

		if r, ok := r.(lint.ConfigurableRule); ok {
			if err := r.Configure(ruleConfig.Arguments); err != nil {
				return nil, fmt.Errorf("cannot configure rule: %q: %w", name, err)
			}
		}

		lintingRules = append(lintingRules, r)
	}

	return lintingRules, nil
}

func actualRuleName(name string) string {
	switch name {
	case "imports-blacklist":
		return "imports-blocklist"
	default:
		return name
	}
}

func parseConfig(data []byte, config *lint.Config) error {
	// Decode the top-level keys as primitives first so each option can be matched to its config field
	// regardless of the spelling used in the file (camelCase, kebab-case or lowercase).
	primitives := map[string]toml.Primitive{}
	md, err := toml.Decode(string(data), &primitives)
	if err != nil {
		return fmt.Errorf("cannot parse the config file: %w", err)
	}

	fields := configFieldsByNormalizedName(config)
	seen := make(map[string]string, len(primitives))
	for key, primitive := range primitives {
		normalized := internalconfig.NormalizeOption(key)
		field, ok := fields[normalized]
		if !ok {
			continue // ignore unknown options, as toml.Unmarshal does
		}
		if other, dup := seen[normalized]; dup {
			return fmt.Errorf("cannot parse the config file: options %q and %q refer to the same option", other, key)
		}
		seen[normalized] = key
		if err := md.PrimitiveDecode(primitive, field.Addr().Interface()); err != nil {
			return fmt.Errorf("cannot parse the config file: %w", err)
		}
	}

	for k, r := range config.Rules {
		err := r.Initialize()
		if err != nil {
			return fmt.Errorf("error in config of rule [%s] : [%w]", k, err)
		}
		config.Rules[k] = r
	}

	return nil
}

// configFieldsByNormalizedName maps the normalized name of each config option to the corresponding struct field,
// so an option can be looked up regardless of the casing or hyphenation used in the config file.
func configFieldsByNormalizedName(config *lint.Config) map[string]reflect.Value {
	v := reflect.ValueOf(config).Elem()
	t := v.Type()
	fields := make(map[string]reflect.Value, t.NumField())
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("toml")
		if tag == "" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		fields[internalconfig.NormalizeOption(name)] = v.Field(i)
	}
	return fields
}

func validateConfig(config *lint.Config) error {
	if config.EnableAllRules && config.EnableDefaultRules {
		return errors.New("config options enable-all-rules and enable-default-rules cannot be combined")
	}
	return nil
}

// Normalize fills in default rule entries (according to the EnableAllRules / EnableDefaultRules options)
// and propagates the configured severity to rules and directives that don't define their own.
func Normalize(config *lint.Config) {
	if len(config.Rules) == 0 {
		config.Rules = map[string]lint.RuleConfig{}
	}

	addRules := func(config *lint.Config, rules []lint.Rule) {
		for _, r := range rules {
			ruleName := r.Name()
			if _, ok := config.Rules[ruleName]; !ok {
				config.Rules[ruleName] = lint.RuleConfig{}
			}
		}
	}

	if config.EnableAllRules {
		addRules(config, allRules)
	} else if config.EnableDefaultRules {
		addRules(config, defaultRules)
	}

	severity := config.Severity
	if severity != "" {
		for k, v := range config.Rules {
			if v.Severity == "" {
				v.Severity = severity
			}
			config.Rules[k] = v
		}
		for k, v := range config.Directives {
			if v.Severity == "" {
				v.Severity = severity
			}
			config.Directives[k] = v
		}
	}
}

// DefaultConfidence is the default confidence level for revive's linter.
const DefaultConfidence = 0.8

// GetConfig yields the configuration.
func GetConfig(configPath string) (*lint.Config, error) {
	config := &lint.Config{}
	switch {
	case configPath != "":
		config.Confidence = DefaultConfidence
		data, err := os.ReadFile(configPath) //nolint:gosec // ignore G304: potential file inclusion via variable
		if err != nil {
			return nil, errors.New("cannot read the config file")
		}
		err = parseConfig(data, config)
		if err != nil {
			return nil, err
		}

	default: // no configuration provided
		config = Default()
	}

	if err := validateConfig(config); err != nil {
		return nil, err
	}

	Normalize(config)
	return config, nil
}

// GetFormatter yields the formatter for lint failures.
func GetFormatter(formatterName string) (lint.Formatter, error) {
	formatters := getFormatters()
	if formatterName == "" {
		return formatters["default"], nil
	}
	f, ok := formatters[formatterName]
	if !ok {
		return nil, fmt.Errorf("unknown formatter %v", formatterName)
	}
	return f, nil
}

// Default returns the default linter configuration, used when no configuration is provided.
func Default() *lint.Config {
	defaultConfig := lint.Config{
		Confidence: DefaultConfidence,
		Severity:   lint.SeverityWarning,
		Rules:      map[string]lint.RuleConfig{},
	}
	for _, r := range defaultRules {
		defaultConfig.Rules[r.Name()] = lint.RuleConfig{}
	}
	return &defaultConfig
}
