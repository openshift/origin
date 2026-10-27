package formatter

import (
	"bytes"
	"encoding/xml"
	plain "text/template"

	"github.com/mgechev/revive/lint"
)

// Checkstyle is an implementation of the [lint.Formatter] interface
// which formats the errors to Checkstyle-like format.
type Checkstyle struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter.
func (*Checkstyle) Name() string {
	return "checkstyle"
}

type issue struct {
	Line       int
	Col        int
	What       string
	Confidence float64
	Severity   lint.Severity
	RuleName   string
}

// Format formats the failures gotten from the lint.
func (*Checkstyle) Format(failures <-chan lint.Failure, config lint.Config) (string, error) {
	issues := map[string][]issue{}
	for failure := range failures {
		iss := issue{
			Line:       failure.Position.Start.Line,
			Col:        failure.Position.Start.Column,
			What:       failure.Failure,
			Confidence: failure.Confidence,
			Severity:   failure.SeverityFor(&config),
			RuleName:   failure.RuleName,
		}
		fn := failure.Filename()
		if issues[fn] == nil {
			issues[fn] = []issue{}
		}
		issues[fn] = append(issues[fn], iss)
	}

	t, err := plain.New("revive").Funcs(plain.FuncMap{"escape": xmlEscape}).Parse(checkstyleTemplate)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)

	err = t.Execute(buf, issues)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// xmlEscape escapes s so that it can be safely interpolated
// into the XML attributes of the checkstyle template.
// It is registered as the "escape" function of the template.
func xmlEscape(s string) string {
	buf := new(bytes.Buffer)
	xml.Escape(buf, []byte(s))
	return buf.String()
}

const checkstyleTemplate = `<?xml version='1.0' encoding='UTF-8'?>
<checkstyle version="5.0">
{{- range $k, $v := . }}
    <file name="{{ escape $k }}">
      {{- range $i, $issue := $v }}
      <error line="{{ $issue.Line }}" column="{{ $issue.Col }}" message="{{ escape $issue.What }} (confidence {{ $issue.Confidence}})" {{ "" -}}
        severity="{{ $issue.Severity }}" source="revive/{{ escape $issue.RuleName }}"/>
      {{- end }}
    </file>
{{- end }}
</checkstyle>`
