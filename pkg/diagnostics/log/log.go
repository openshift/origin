package log

import (
	"bytes"
	"errors"
	"fmt"
	ct "github.com/daviddengcn/go-colortext"
	"io"
	"strings"
	"text/template"
)

type Level struct {
	Level  int
	Name   string
	Prefix string
	Color  ct.Color
	Bright bool
}

type Logger struct {
	logger       loggerType
	level        Level
	warningsSeen int
	errorsSeen   int
}

// Internal type to deal with different log formats
type loggerType interface {
	Write(Level, Msg)
	Finish()
}

func NewLogger(setLevel int, setFormat string, out io.Writer) (*Logger, error) {

	var logger loggerType
	switch setFormat {
	case "json":
		logger = &jsonLogger{out: out}
	case "yaml":
		logger = &yamlLogger{out: out}
	case "text":
		logger = newTextLogger(out)
	default:
		return nil, errors.New("Output format must be one of: text, json, yaml")
	}

	var err error = nil
	level := DebugLevel
	switch setLevel {
	case 0:
		level = ErrorLevel
	case 1:
		level = WarnLevel
	case 2:
		level = NoticeLevel
	case 3:
		level = InfoLevel
	case 4:
		// Debug, also default for invalid numbers below
	default:
		err = errors.New("Invalid diagnostic level; must be 0-4")
	}
	return &Logger{
		logger: logger,
		level:  level,
	}, err
}

// a map message type to throw type safety and method signatures out the window:
type Msg map[string]interface{}

/* a Msg can be expected to have the following entries:
 * "id": an identifier unique to the message being logged, intended for json/yaml output
 *       so that automation can recognize specific messages without trying to parse them.
 * "text": human-readable message text
 * "tmpl": a template string as understood by text/template that can use any of the other
 *         entries in this Msg as inputs. This is removed, evaluated, and the result is
 *         placed in "text". If there is an error during evaluation, the error is placed
 *         in "templateErr", the original id of the message is stored in "templateId",
 *         and the Msg id is changed to "tmplErr". Of course, this should never happen
 *         if there are no mistakes in the calling code.
 */

var (
	ErrorLevel  = Level{0, "error", "ERROR: ", ct.Red, true}   // Something is definitely wrong
	WarnLevel   = Level{1, "warn", "WARN:  ", ct.Yellow, true} // Likely to be an issue but maybe not
	NoticeLevel = Level{2, "note", "[Note] ", ct.White, false} // Introductory / summary
	InfoLevel   = Level{3, "info", "Info:  ", ct.None, false}  // Just informational
	DebugLevel  = Level{4, "debug", "debug: ", ct.None, false} // Extra verbose
)

// Provide a summary at the end
func (l *Logger) Summary() {
	l.Notice("summary", "\nSummary of diagnostics execution:\n")
	if l.warningsSeen > 0 {
		l.Noticem("sumWarn", Msg{"tmpl": "Warnings seen: {{.num}}", "num": l.warningsSeen})
	}
	if l.errorsSeen > 0 {
		l.Noticem("sumErr", Msg{"tmpl": "Errors seen: {{.num}}", "num": l.errorsSeen})
	}
	if l.warningsSeen == 0 && l.errorsSeen == 0 {
		l.Notice("sumNone", "Completed with no errors or warnings seen.")
	}
}

func (l *Logger) Log(level Level, id string, msg Msg) {
	if level.Level > l.level.Level {
		return
	}
	msg["id"] = id // TODO: use to retrieve template from elsewhere
	// if given a template, convert it to text
	if tmpl, exists := msg["tmpl"]; exists {
		var buff bytes.Buffer
		if tmplString, assertion := tmpl.(string); !assertion {
			msg["templateErr"] = fmt.Sprintf("Invalid template type: %T", tmpl)
			msg["templateId"] = id
			msg["id"] = "tmplErr"
		} else {
			parsedTmpl, err := template.New(id).Parse(tmplString)
			if err != nil {
				msg["templateErr"] = err.Error()
				msg["templateId"] = id
				msg["id"] = "tmplErr"
			} else if err = parsedTmpl.Execute(&buff, msg); err != nil {
				msg["templateErr"] = err.Error()
				msg["templateId"] = id
				msg["id"] = "tmplErr"
			} else {
				msg["text"] = buff.String()
				delete(msg, "tmpl")
			}
		}
	}
	if level.Level == ErrorLevel.Level {
		l.errorsSeen += 1
	} else if level.Level == WarnLevel.Level {
		l.warningsSeen += 1
	}
	l.logger.Write(level, msg)
}

// Convenience functions
func (l *Logger) Error(id string, text string) {
	l.Log(ErrorLevel, id, Msg{"text": text})
}
func (l *Logger) Errorf(id string, msg string, a ...interface{}) {
	l.Error(id, fmt.Sprintf(msg, a...))
}
func (l *Logger) Errorm(id string, msg Msg) {
	l.Log(ErrorLevel, id, msg)
}
func (l *Logger) Warn(id string, text string) {
	l.Log(WarnLevel, id, Msg{"text": text})
}
func (l *Logger) Warnf(id string, msg string, a ...interface{}) {
	l.Warn(id, fmt.Sprintf(msg, a...))
}
func (l *Logger) Warnm(id string, msg Msg) {
	l.Log(WarnLevel, id, msg)
}
func (l *Logger) Info(id string, text string) {
	l.Log(InfoLevel, id, Msg{"text": text})
}
func (l *Logger) Infof(id string, msg string, a ...interface{}) {
	l.Info(id, fmt.Sprintf(msg, a...))
}
func (l *Logger) Infom(id string, msg Msg) {
	l.Log(InfoLevel, id, msg)
}
func (l *Logger) Notice(id string, text string) {
	l.Log(NoticeLevel, id, Msg{"text": text})
}
func (l *Logger) Noticef(id string, msg string, a ...interface{}) {
	l.Notice(id, fmt.Sprintf(msg, a...))
}
func (l *Logger) Noticem(id string, msg Msg) {
	l.Log(NoticeLevel, id, msg)
}
func (l *Logger) Debug(id string, text string) {
	l.Log(DebugLevel, id, Msg{"text": text})
}
func (l *Logger) Debugf(id string, msg string, a ...interface{}) {
	l.Debug(id, fmt.Sprintf(msg, a...))
}
func (l *Logger) Debugm(id string, msg Msg) {
	l.Log(DebugLevel, id, msg)
}

func (l *Logger) Finish() {
	l.logger.Finish()
}

func (l *Logger) ErrorsSeen() bool {
	return l.errorsSeen > 0
}

// turn excess lines into [...]
func LimitLines(msg string, n int) string {
	lines := strings.SplitN(msg, "\n", n+1)
	if len(lines) == n+1 {
		lines[n] = "[...]"
	}
	return strings.Join(lines, "\n")
}
