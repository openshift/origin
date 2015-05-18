package log

import (
	"bytes"
	"errors"
	"fmt"
	ct "github.com/daviddengcn/go-colortext"
	"io"
	"io/ioutil"
	"strings"
	"text/template"
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
	loggerType
	level        Level
	warningsSeen int
	errorsSeen   int
}

// Internal type to deal with different log formats
type loggerType interface {
	Write(LogEntry)
	Finish()
}

func NewLogger(setLevel int, setFormat string, out io.Writer) (*Logger, error) {
	var logger loggerType
	switch setFormat {
	case "json":
		logger = &jsonLogger{out: out}
	case "yaml":
		logger = &yamlLogger{out: out}
	case "text", "":
		logger = newTextLogger(out)
	default:
		return nil, errors.New("Output format must be one of: text, json, yaml")
	}

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
		loggerType: logger,
		level:      level,
	}, err
}

type Message struct {
	ID       string
	Template string

	// TemplateData is passed to template executor to complete the message
	TemplateData interface{}

	EvaluatedText string
}

func (m Message) String() string {
	if len(m.EvaluatedText) > 0 {
		return fmt.Sprintf("%s: %s", m.EvaluatedText)
	}

	if len(m.Template) == 0 {
		return fmt.Sprintf("%s: %s %#v", m.ID, m.Template, m.TemplateData)
	}

	// if given a template, convert it to text
	parsedTmpl, err := template.New(m.ID).Parse(m.Template)
	if err != nil {
		return fmt.Sprintf("%s: %s %#v: %v", m.ID, m.Template, m.TemplateData, err)
	}

	var buff bytes.Buffer
	err = parsedTmpl.Execute(&buff, m.TemplateData)
	if err != nil {
		return fmt.Sprintf("%s: %s %#v: %v", m.ID, m.Template, m.TemplateData, err)
	}

	return buff.String()
}

type LogEntry struct {
	Level Level
	Message
}

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
	ErrorLevel  = Level{4, "error", "ERROR: ", ct.Red, true}   // Something is definitely wrong
	WarnLevel   = Level{3, "warn", "WARN:  ", ct.Yellow, true} // Likely to be an issue but maybe not
	NoticeLevel = Level{2, "note", "[Note] ", ct.White, false} // Introductory / summary
	InfoLevel   = Level{1, "info", "Info:  ", ct.None, false}  // Just informational
	DebugLevel  = Level{0, "debug", "debug: ", ct.None, false} // Extra verbose
)

// Provide a summary at the end
func (l *Logger) Summary() {
	l.Notice("summary", "\nSummary of diagnostics execution:\n")
	if l.warningsSeen > 0 {
		l.Noticef("sumWarn", "Warnings seen: %d", l.warningsSeen)
	}
	if l.errorsSeen > 0 {
		l.Noticef("sumErr", "Errors seen: %d", l.errorsSeen)
	}
	if l.warningsSeen == 0 && l.errorsSeen == 0 {
		l.Notice("sumNone", "Completed with no errors or warnings seen.")
	}
}

func (l *Logger) LogMessage(level Level, message Message) {
	// if there's no logger, return silently
	if l == nil {
		return
	}

	// track how many of every type we've seen (probably unnecessary)
	if level.Level == ErrorLevel.Level {
		l.errorsSeen += 1
	} else if level.Level == WarnLevel.Level {
		l.warningsSeen += 1
	}

	if level.Level < l.level.Level {
		return
	}

	if len(message.Template) == 0 {
		l.Write(LogEntry{level, message})
		return
	}

	// if given a template, convert it to text
	parsedTmpl, err := template.New(message.ID).Parse(message.Template)
	if err != nil {
		templateErrorMessage := Message{
			ID: "templateParseErr",
			TemplateData: map[string]interface{}{
				"error":           err.Error(),
				"originalMessage": message,
			},
		}
		l.LogMessage(level, templateErrorMessage)
		return
	}

	var buff bytes.Buffer
	err = parsedTmpl.Execute(&buff, message.TemplateData)
	if err != nil {
		templateErrorMessage := Message{
			ID: "templateParseErr",
			TemplateData: map[string]interface{}{
				"error":           err.Error(),
				"originalMessage": message,
			},
		}
		l.LogMessage(level, templateErrorMessage)
		return

	}

	message.EvaluatedText = buff.String()
	l.Write(LogEntry{level, message})
}

// Convenience functions
func (l *Logger) Error(id string, text string) {
	l.Logp(ErrorLevel, id, text)
}
func (l *Logger) Errorf(id string, msg string, a ...interface{}) {
	l.Logpf(ErrorLevel, id, msg, a...)
}
func (l *Logger) Errorm(message Message) {
	l.LogMessage(ErrorLevel, message)
}
func (l *Logger) Warn(id string, text string) {
	l.Logp(WarnLevel, id, text)
}
func (l *Logger) Warnf(id string, msg string, a ...interface{}) {
	l.Logpf(WarnLevel, id, msg, a...)
}
func (l *Logger) Warnm(message Message) {
	l.LogMessage(WarnLevel, message)
}
func (l *Logger) Info(id string, text string) {
	l.Logp(InfoLevel, id, text)
}
func (l *Logger) Infof(id string, msg string, a ...interface{}) {
	l.Logpf(InfoLevel, id, msg, a...)
}
func (l *Logger) Infom(message Message) {
	l.LogMessage(InfoLevel, message)
}
func (l *Logger) Notice(id string, text string) {
	l.Logp(NoticeLevel, id, text)
}
func (l *Logger) Noticef(id string, msg string, a ...interface{}) {
	l.Logpf(NoticeLevel, id, msg, a...)
}
func (l *Logger) Noticem(message Message) {
	l.LogMessage(NoticeLevel, message)
}
func (l *Logger) Debug(id string, text string) {
	l.Logp(DebugLevel, id, text)
}
func (l *Logger) Debugf(id string, msg string, a ...interface{}) {
	l.Logpf(DebugLevel, id, msg, a...)
}
func (l *Logger) Debugm(message Message) {
	l.LogMessage(DebugLevel, message)
}

func (l *Logger) Logp(level Level, id string, text string) {
	l.LogMessage(level, Message{ID: id, EvaluatedText: text})
}
func (l *Logger) Logpf(level Level, id string, msg string, a ...interface{}) {
	l.Logp(level, id, fmt.Sprintf(msg, a...))
}

func (l *Logger) Finish() {
	l.loggerType.Finish()
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
