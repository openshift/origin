package prometheus

import (
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	dto "github.com/prometheus/client_model/go"

	"k8s.io/apimachinery/pkg/util/sets"
)

// exceptionMetrics is the set of metric names that are known to violate
// promlint rules. These are pre-existing violations that were present when the
// linting test was introduced.
//
// New metrics MUST NOT be added to this list — fix the violation instead. If a
// metric intentionally deviates from conventions (e.g. node-exporter metrics
// reflecting procfs entries), document the reason in a comment.
//
// To regenerate this list from a cluster run the promlint test with an empty
// set, collect the failing metric names from the test output, and add them
// here sorted alphabetically.
var exceptionMetrics = sets.New[string]()

// Problem wraps promlint.Problem.
type Problem struct {
	Metric string
	Text   string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s", p.Metric, p.Text)
}

// Linter wraps promlint.Linter with OpenShift exception filtering.
type Linter struct {
	linter *promlint.Linter
}

// NewLinterWithMetricFamilies creates a Linter from MetricFamily protos.
func NewLinterWithMetricFamilies(families []*dto.MetricFamily) *Linter {
	return &Linter{
		linter: promlint.NewWithMetricFamilies(families),
	}
}

// Lint runs promlint and filters out excepted metrics.
func (l *Linter) Lint() ([]Problem, error) {
	upstream, err := l.linter.Lint()
	if err != nil {
		return nil, err
	}

	var problems []Problem
	for _, p := range upstream {
		if exceptionMetrics.Has(p.Metric) {
			continue
		}
		problems = append(problems, Problem{
			Metric: p.Metric,
			Text:   p.Text,
		})
	}

	return problems, nil
}

// FormatExceptionEntries returns sorted Go source code for the exception set
// derived from the given problems, suitable for pasting into the
// exceptionMetrics declaration.
func FormatExceptionEntries(problems []Problem) string {
	seen := sets.New[string]()
	type entry struct {
		name string
		text string
	}
	var entries []entry
	for _, p := range problems {
		if seen.Has(p.Metric) {
			continue
		}
		seen.Insert(p.Metric)
		entries = append(entries, entry{name: p.Metric, text: p.Text})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "\t%q, // %s\n", e.name, e.text)
	}
	return b.String()
}
