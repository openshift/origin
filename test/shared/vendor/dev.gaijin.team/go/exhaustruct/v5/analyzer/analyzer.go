package analyzer

import (
	"flag"
	"sync"

	"dev.gaijin.team/go/golib/e"
	"dev.gaijin.team/go/golib/fields"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"

	"dev.gaijin.team/go/exhaustruct/v5/internal/astutil"
	"dev.gaijin.team/go/exhaustruct/v5/internal/directive"
	"dev.gaijin.team/go/exhaustruct/v5/internal/pattern"
	"dev.gaijin.team/go/exhaustruct/v5/internal/structure"
)

// NewAnalyzer returns an analyzer configured exclusively through command-line
// flags, intended for CLI drivers (singlechecker, go vet -vettool). The
// processor is built lazily on the first run, after the driver has parsed the
// flags.
func NewAnalyzer() *analysis.Analyzer {
	config := &Config{}

	lazyProcessor := sync.OnceValues(func() (*structure.Processor, error) {
		return newProcessor(config)
	})

	a := newBaseAnalyzer(func(pass *analysis.Pass) (any, error) {
		processor, err := lazyProcessor()
		if err != nil {
			return nil, err
		}

		run(pass, config, processor)

		return nil, nil //nolint:nilnil
	})

	a.Flags.Init("", flag.PanicOnError)
	config.bindToFlagSet(&a.Flags)

	return a
}

// NewAnalyzerWithConfig returns an analyzer configured programmatically,
// intended for library consumers such as golangci-lint. The configuration is
// copied and validated immediately; it exposes no flags, and later mutations
// of the passed Config have no effect.
func NewAnalyzerWithConfig(config Config) (*analysis.Analyzer, error) {
	processor, err := newProcessor(&config)
	if err != nil {
		return nil, err
	}

	return newBaseAnalyzer(func(pass *analysis.Pass) (any, error) {
		run(pass, &config, processor)

		return nil, nil //nolint:nilnil
	}), nil
}

func newBaseAnalyzer(run func(*analysis.Pass) (any, error)) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "exhaustruct",
		Doc:      "Checks if all structure fields are initialized",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func newProcessor(config *Config) (*structure.Processor, error) {
	enforce, err := pattern.NewList(config.EnforcePatterns...)
	if err != nil {
		return nil, e.NewFrom("compile enforce patterns", err, fields.F("flag", "enforce-rx"))
	}

	ignore, err := pattern.NewList(config.IgnorePatterns...)
	if err != nil {
		return nil, e.NewFrom("compile ignore patterns", err, fields.F("flag", "ignore-rx"))
	}

	optional, err := pattern.NewList(config.OptionalPatterns...)
	if err != nil {
		return nil, e.NewFrom("compile optional patterns", err, fields.F("flag", "optional-rx"))
	}

	allowEmpty, err := pattern.NewList(config.AllowEmptyPatterns...)
	if err != nil {
		return nil, e.NewFrom("compile allow-empty patterns", err, fields.F("flag", "allow-empty-rx"))
	}

	fp := astutil.NewFileParser()

	return structure.NewProcessor(
		directive.NewScanner(fp),
		structure.WithEnforce(enforce),
		structure.WithIgnore(ignore),
		structure.WithOptional(optional),
		structure.WithAllowEmpty(allowEmpty),
	), nil
}

func run(pass *analysis.Pass, config *Config, processor *structure.Processor) {
	for _, diag := range processor.Directives().ProcessFiles(pass.Fset, pass.Files...) {
		pass.Report(diag)
	}

	newMissingFieldsVisitor(pass, config, processor).run()
	newTagMigrationVisitor(pass, processor).run()
}
