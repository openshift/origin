package systemd

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

const (
	sdLogReadErr = `Diagnostics failed to query journalctl for the '%s' unit logs.
This should be very unusual, so please report this error:
%s`
)

// AnalyzeLogs
type AnalyzeLogs struct {
	SystemdUnits map[string]types.SystemdUnit
}

func (d AnalyzeLogs) Name() string {
	return "AnalyzeLogs"
}

func (d AnalyzeLogs) Description() string {
	return "Check for recent problems in systemd service logs"
}

func (d AnalyzeLogs) CanRun() (bool, error) {
	return true, nil
}

func (d AnalyzeLogs) Check() *types.DiagnosticResult {
	r := types.NewDiagnosticResult("AnalyzeLogs")

	for _, unit := range unitLogSpecs {
		if svc := d.SystemdUnits[unit.Name]; svc.Enabled || svc.Active {
			r.Infof("sdCheckLogs", "Checking journalctl logs for '%s' service", unit.Name)

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
				r.Errorf("sdLogReadErr", err, sdLogReadErr, unit.Name, errStr(err))
				return r
			}
			defer func() { // close out pipe once done reading
				reader.Close()
				cmd.Wait()
			}()
			timeLimit := time.Now().Add(-time.Hour)                     // if it didn't happen in the last hour, probably not too relevant
			matchCopy := append([]logMatcher(nil), unit.LogMatchers...) // make a copy, will remove matchers after they match something
			lineCount := 0                                              // each log entry is a line
			for lineReader.Scan() {
				lineCount += 1
				if len(matchCopy) == 0 { // if no rules remain to match
					break // don't waste time reading more log entries
				}
				bytes, entry := lineReader.Bytes(), logEntry{}
				if err := json.Unmarshal(bytes, &entry); err != nil {
					r.Debugf("sdLogBadJSON", "Couldn't read the JSON for this log message:\n%s\nGot error %s", string(bytes), errStr(err))
				} else {
					if lineCount > 500 && stampTooOld(entry.TimeStamp, timeLimit) {
						r.Debugf("sdLogTrunc", "Stopped reading %s log: timestamp %s too old", unit.Name, entry.TimeStamp)
						break // if we've analyzed at least 500 entries, stop when age limit reached (don't scan days of logs)
					}
					if unit.StartMatch.MatchString(entry.Message) {
						break // saw log message for unit startup; don't analyze previous logs
					}
					for index, match := range matchCopy { // match log message against provided matchers
						if strings := match.Regexp.FindStringSubmatch(entry.Message); strings != nil {
							// if matches: print interpretation, remove from matchCopy, and go on to next log entry
							keep := match.KeepAfterMatch // generic keep logic
							if match.Interpret != nil {  // apply custom match logic
								currKeep, result := match.Interpret(&entry, strings)
								keep = currKeep
								r.Append(result)
							} else { // apply generic match processing
								template := "Found '{{.unit}}' journald log message:\n  {{.logMsg}}\n{{.interpretation}}"
								templateData := log.Hash{"unit": unit.Name, "logMsg": entry.Message, "interpretation": match.Interpretation}

								switch match.Level {
								case log.DebugLevel:
									r.Debugt(match.Id, template, templateData)
								case log.InfoLevel:
									r.Infot(match.Id, template, templateData)
								case log.WarnLevel:
									r.Warnt(match.Id, nil, template, templateData)
								case log.ErrorLevel:
									r.Errort(match.Id, nil, template, templateData)
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

	return r
}

func stampTooOld(stamp string, timeLimit time.Time) bool {
	if epochns, err := strconv.ParseInt(stamp, 10, 64); err == nil {
		return time.Unix(epochns/1000000, 0).Before(timeLimit)
	}
	return true // something went wrong, stop looking...
}
