package log

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	txttemplate "text/template"

	ct "github.com/daviddengcn/go-colortext"

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

type Entry struct {
	ID      string
	Origin  string
	Level   Level
	Message string
}

var (
	ErrorLevel  = Level{4, "error", "ERROR: ", ct.Red, true}   // Something is definitely wrong
	WarnLevel   = Level{3, "warn", "WARN:  ", ct.Yellow, true} // Likely to be an issue but maybe not
	NoticeLevel = Level{2, "note", "[Note] ", ct.White, false} // Introductory / summary
	InfoLevel   = Level{1, "info", "Info:  ", ct.None, false}  // Just informational
	DebugLevel  = Level{0, "debug", "debug: ", ct.None, false} // Extra verbose
)

// Provide a summary at the end
func (l *Logger) Summary() {
	l.Notice("DL0001", fmt.Sprintf("Summary of diagnostics execution (version %v):\n", version.Get()))
	if l.warningsSeen > 0 {
		l.Notice("DL0002", fmt.Sprintf("Warnings seen: %d", l.warningsSeen))
	}
	if l.errorsSeen > 0 {
		l.Notice("DL0003", fmt.Sprintf("Errors seen: %d", l.errorsSeen))
	}
	if l.warningsSeen == 0 && l.errorsSeen == 0 {
		l.Notice("DL0004", "Completed with no errors or warnings seen.")
	}
}

func (l *Logger) ErrorsSeen() bool {
	return l.errorsSeen > 0
}

func (l *Logger) LogEntry(entry Entry) {
	if l == nil { // if there's no logger, return silently
		return
	}
	if entry.Level == ErrorLevel {
		l.errorsSeen++
	}
	if entry.Level == WarnLevel {
		l.warningsSeen++
	}
	if entry.Level.Level < l.level.Level { // logging level says skip this entry
		return
	}
	l.Write(entry)
}

// Convenience functions
func (l *Logger) Error(id string, text string) {
	l.LogEntry(Entry{id, origin(1), ErrorLevel, text})
}
func (l *Logger) Warn(id string, text string) {
	l.LogEntry(Entry{id, origin(1), WarnLevel, text})
}
func (l *Logger) Info(id string, text string) {
	l.LogEntry(Entry{id, origin(1), InfoLevel, text})
}
func (l *Logger) Notice(id string, text string) {
	l.LogEntry(Entry{id, origin(1), NoticeLevel, text})
}
func (l *Logger) Debug(id string, text string) {
	l.LogEntry(Entry{id, origin(1), DebugLevel, text})
}

func origin(skip int) string {
	if _, file, _, ok := runtime.Caller(skip + 1); ok {
		paths := strings.SplitAfter(file, "github.com/")
		return "controller " + paths[len(paths)-1]
	} else {
		return "unknown"
	}
}

// Utilities related to output

// turn excess lines into [...]
func LimitLines(msg string, n int) string {
	lines := strings.SplitN(msg, "\n", n+1)
	if len(lines) == n+1 {
		lines[n] = "[...]"
	}
	return strings.Join(lines, "\n")
}

type Hash map[string]interface{} // convenience/cosmetic type
func EvalTemplate(id string, template string, data map[string]interface{}) string {
	if len(template) == 0 {
		return fmt.Sprintf("%s: %s %#v", id, template, data)
	}
	// if given a template, convert it to text
	parsedTmpl, err := txttemplate.New(id).Parse(template)
	if err != nil { // if the template is broken ...
		return fmt.Sprintf("%s: %s %#v: %v", id, template, data, err)
	}
	var buff bytes.Buffer
	err = parsedTmpl.Execute(&buff, data)
	if err != nil { // if execution choked ...
		return fmt.Sprintf("%s: %s %#v: %v", id, template, data, err)
	}
	return buff.String()
}
