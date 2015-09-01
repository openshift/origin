package log

import (
	"bytes"
	"errors"
	"fmt"
	ct "github.com/daviddengcn/go-colortext"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	"text/template"

	"github.com/openshift/origin/pkg/version"
)

type LoggerOptions struct {
	Level  int
	Format string
	Out    io.Writer
}

func (o *LoggerOptions) NewLogger() (*Logger, error) {
	out := o.Out
	if out == nil {
		out = ioutil.Discard
	}

	ret, err := NewLogger(o.Level, o.Format, out)
	return ret, err
}

type Level struct {
	Level  int
	Name   string
	Prefix string
	Color  ct.Color
	Bright bool
}

type Logger struct {
	loggerInterface
	level        Level
	warningsSeen int
	errorsSeen   int
}

// Internal interface to implement logging
type loggerInterface interface {
	Write(Entry)
}

func NewLogger(setLevel int, setFormat string, out io.Writer) (*Logger, error) {
	logger := newTextLogger(out)

	var err error = nil
	level := DebugLevel
	switch setLevel {
	case ErrorLevel.Level:
		level = ErrorLevel
	case WarnLevel.Level:
		level = WarnLevel
	case NoticeLevel.Level:
		level = NoticeLevel
	case InfoLevel.Level:
		level = InfoLevel
	case DebugLevel.Level:
		// Debug, also default for invalid numbers below
	default:
		err = errors.New("Invalid diagnostic level; must be 0-4")
	}

	return &Logger{
		loggerInterface: logger,
		level:           level,
	}, err
}

type Message struct {
	// ID: an identifier unique to the message being logged
	ID string
	// Template: a template string as understood by text/template that can use any of the
	//           TemplateData entries in this Message as inputs.
	Template string
	// TemplateData is passed to template executor to complete the message
	TemplateData interface{}
	// EvaluatedText: human-readable message text
	EvaluatedText string
}

type Hash map[string]interface{} // convenience/cosmetic type

func (m Message) String() string {
	if len(m.EvaluatedText) > 0 {
		return m.EvaluatedText
	}

	if len(m.Template) == 0 {
		return fmt.Sprintf("%s: %s %#v", m.ID, m.Template, m.TemplateData)
	}

	// if given a template, convert it to text
	parsedTmpl, err := template.New(m.ID).Parse(m.Template)
	if err != nil { // unless the template is broken of course
		return fmt.Sprintf("%s: %s %#v: %v", m.ID, m.Template, m.TemplateData, err)
	}

	var buff bytes.Buffer
	err = parsedTmpl.Execute(&buff, m.TemplateData)
	if err != nil {
		return fmt.Sprintf("%s: %s %#v: %v", m.ID, m.Template, m.TemplateData, err)
	}

	return buff.String()
}

type Entry struct {
	ID     string
	Origin string
	Level  Level
	Message
}

var (
	ErrorLevel  = Level{4, "error", "ERROR: ", ct.Red, true}   // Something is definitely wrong
	WarnLevel   = Level{3, "warn", "WARN:  ", ct.Yellow, true} // Likely to be an issue but maybe not
	NoticeLevel = Level{2, "note", "[Note] ", ct.White, false} // Introductory / summary
	InfoLevel   = Level{1, "info", "Info:  ", ct.None, false}  // Just informational
	DebugLevel  = Level{0, "debug", "debug: ", ct.None, false} // Extra verbose
)

// Provide a summary at the end
func (l *Logger) Summary(warningsSeen int, errorsSeen int) {
	l.Noticef("DL0001", "\nSummary of diagnostics execution (version %v):\n", version.Get())
	if warningsSeen > 0 {
		l.Noticet("DL0002", "Warnings seen: {{.warnings}}", Hash{"warnings": warningsSeen})
	}
	if errorsSeen > 0 {
		l.Noticet("DL0003", "Errors seen: {{.errors}}", Hash{"errors": errorsSeen})
	}
	if warningsSeen == 0 && errorsSeen == 0 {
		l.Notice("DL0004", "Completed with no errors or warnings seen.")
	}
}

func (l *Logger) LogEntry(entry Entry) {
	if l == nil { // if there's no logger, return silently
		return
	}
	if entry.Level.Level < l.level.Level { // logging level says skip this entry
		return
	}

	if msg := &entry.Message; msg.EvaluatedText == "" && msg.Template != "" {
		// if given a template instead of text, convert it to text
		parsedTmpl, err := template.New(msg.ID).Parse(msg.Template)
		if err != nil {
			entry.Message = Message{
				ID: "templateParseErr",
				TemplateData: Hash{
					"error":           err.Error(),
					"originalMessage": msg,
				},
				EvaluatedText: fmt.Sprintf("Error parsing template for %s:\n%s=== Error was:\n%v\nOriginal message:\n%#v", msg.ID, msg.Template, err, msg),
			}
			entry.ID = entry.Message.ID
			l.Write(entry)
			return
		}

		var buff bytes.Buffer
		err = parsedTmpl.Execute(&buff, msg.TemplateData)
		if err != nil {
			entry.Message = Message{
				ID: "templateExecErr",
				TemplateData: Hash{
					"error":           err.Error(),
					"originalMessage": msg,
				},
				EvaluatedText: fmt.Sprintf("Error executing template for %s:\n%s=== Error was:\n%v\nOriginal message:\n%#v", msg.ID, msg.Template, err, msg),
			}
			entry.ID = entry.Message.ID
			l.Write(entry)
			return
		}

		msg.EvaluatedText = buff.String()
	}

	l.Write(entry)
}

// Convenience functions
func (l *Logger) Error(id string, text string) {
	l.logp(ErrorLevel, id, text)
}
func (l *Logger) Errorf(id string, msg string, a ...interface{}) {
	l.logf(ErrorLevel, id, msg, a...)
}
func (l *Logger) Errort(id string, template string, data interface{}) {
	l.logt(ErrorLevel, id, template, data)
}
func (l *Logger) Warn(id string, text string) {
	l.logp(WarnLevel, id, text)
}
func (l *Logger) Warnf(id string, msg string, a ...interface{}) {
	l.logf(WarnLevel, id, msg, a...)
}
func (l *Logger) Info(id string, text string) {
	l.logp(InfoLevel, id, text)
}
func (l *Logger) Infof(id string, msg string, a ...interface{}) {
	l.logf(InfoLevel, id, msg, a...)
}
func (l *Logger) Notice(id string, text string) {
	l.logp(NoticeLevel, id, text)
}
func (l *Logger) Noticef(id string, msg string, a ...interface{}) {
	l.logf(NoticeLevel, id, msg, a...)
}
func (l *Logger) Noticet(id string, template string, data interface{}) {
	l.logt(NoticeLevel, id, template, data)
}
func (l *Logger) Debug(id string, text string) {
	l.logp(DebugLevel, id, text)
}
func (l *Logger) Debugf(id string, msg string, a ...interface{}) {
	l.logf(DebugLevel, id, msg, a...)
}

func origin(skip int) string {
	if _, file, _, ok := runtime.Caller(skip + 1); ok {
		paths := strings.SplitAfter(file, "github.com/")
		return "controller " + paths[len(paths)-1]
	} else {
		return "unknown"
	}
}
func (l *Logger) logp(level Level, id string, text string) {
	l.LogEntry(Entry{id, origin(2), level, Message{ID: id, EvaluatedText: text}})
}
func (l *Logger) logf(level Level, id string, msg string, a ...interface{}) {
	l.LogEntry(Entry{id, origin(2), level, Message{ID: id, EvaluatedText: fmt.Sprintf(msg, a...)}})
}
func (l *Logger) logt(level Level, id string, template string, data interface{}) {
	l.LogEntry(Entry{id, origin(2), level, Message{ID: id, Template: template, TemplateData: data}})
}

// turn excess lines into [...]
func LimitLines(msg string, n int) string {
	lines := strings.SplitN(msg, "\n", n+1)
	if len(lines) == n+1 {
		lines[n] = "[...]"
	}
	return strings.Join(lines, "\n")
}
