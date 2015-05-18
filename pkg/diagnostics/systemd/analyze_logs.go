package systemd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
	"github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
)

// AnalyzeLogs
type AnalyzeLogs struct {
	SystemdUnits map[string]types.SystemdUnit

	Log *log.Logger
}

func (d AnalyzeLogs) Description() string {
	return "Check for problems in systemd service logs since each service last started"
}
func (d AnalyzeLogs) CanRun() (bool, error) {
	return true, nil
}
func (d AnalyzeLogs) Check() (bool, []log.Message, []error, []error) {
	infos := []log.Message{}
	warnings := []error{}
	errors := []error{}

	for _, unit := range unitLogSpecs {
		if svc := d.SystemdUnits[unit.Name]; svc.Enabled || svc.Active {
			checkMessage := log.Message{ID: "sdCheckLogs", EvaluatedText: fmt.Sprintf("Checking journalctl logs for '%s' service", unit.Name)}
			d.Log.LogMessage(log.InfoLevel, checkMessage)
			infos = append(infos, checkMessage)

			cmd := exec.Command("journalctl", "-ru", unit.Name, "--output=json")
			// JSON comes out of journalctl one line per record
			lineReader, reader, err := func(cmd *exec.Cmd) (*bufio.Scanner, io.ReadCloser, error) {
				stdout, err := cmd.StdoutPipe()
				if err == nil {
					lineReader := bufio.NewScanner(stdout)
					if err = cmd.Start(); err == nil {
						return lineReader, stdout, nil
					}
				}
				return nil, nil, err
			}(cmd)

			if err != nil {
				diagnosticError := diagnostic.NewDiagnosticError("sdLogReadErr", fmt.Sprintf(sdLogReadErr, unit.Name, errStr(err)), err)
				d.Log.Error(diagnosticError.ID, diagnosticError.Explanation)
				errors = append(errors, diagnosticError)

				return false, infos, warnings, errors
			}
			defer func() { // close out pipe once done reading
				reader.Close()
				cmd.Wait()
			}()
			entryTemplate := logEntry{Message: `json:"MESSAGE"`}
			matchCopy := append([]logMatcher(nil), unit.LogMatchers...) // make a copy, will remove matchers after they match something
			for lineReader.Scan() {                                     // each log entry is a line
				if len(matchCopy) == 0 { // if no rules remain to match
					break // don't waste time reading more log entries
				}
				bytes, entry := lineReader.Bytes(), entryTemplate
				if err := json.Unmarshal(bytes, &entry); err != nil {
					badJSONMessage := log.Message{ID: "sdLogBadJSON", EvaluatedText: fmt.Sprintf("Couldn't read the JSON for this log message:\n%s\nGot error %s", string(bytes), errStr(err))}
					d.Log.LogMessage(log.DebugLevel, badJSONMessage)

				} else {
					if unit.StartMatch.MatchString(entry.Message) {
						break // saw the log message where the unit started; done looking.
					}
					for index, match := range matchCopy { // match log message against provided matchers
						if strings := match.Regexp.FindStringSubmatch(entry.Message); strings != nil {
							// if matches: print interpretation, remove from matchCopy, and go on to next log entry
							keep := match.KeepAfterMatch
							if match.Interpret != nil {
								currKeep, currInfos, currWarnings, currErrors := match.Interpret(d.Log, &entry, strings)
								keep = currKeep
								infos = append(infos, currInfos...)
								warnings = append(warnings, currWarnings...)
								errors = append(errors, currErrors...)

							} else {
								text := fmt.Sprintf("Found '%s' journald log message:\n  %s\n", unit.Name, entry.Message) + match.Interpretation
								message := log.Message{ID: match.Id, EvaluatedText: text, TemplateData: map[string]string{"unit": unit.Name, "logMsg": entry.Message}}
								d.Log.LogMessage(match.Level, message)
								diagnosticError := diagnostic.NewDiagnosticError(match.Id, text, nil)

								switch match.Level {
								case log.InfoLevel, log.NoticeLevel:
									infos = append(infos, message)

								case log.WarnLevel:
									warnings = append(warnings, diagnosticError)

								case log.ErrorLevel:
									errors = append(errors, diagnosticError)

								}

							}

							if !keep { // remove matcher once seen
								matchCopy = append(matchCopy[:index], matchCopy[index+1:]...)
							}
							break
						}
					}
				}
			}

		}
	}

	return (len(errors) == 0), infos, warnings, errors
}

const (
	sdLogReadErr = `Diagnostics failed to query journalctl for the '%s' unit logs.
This should be very unusual, so please report this error:
%s`
)
