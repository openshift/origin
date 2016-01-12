package systemd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

const (
	AnalyzeLogsName = "AnalyzeLogs"

	sdLogReadErr = `Diagnostics failed to query journalctl for the '%s' unit logs.
This should be very unusual, so please report this error:
%s`
)

// HasJournalctl checks that journalctl exists, and is usable on this system.
func HasJournalctl() bool {
	if runtime.GOOS == "linux" {
		journalctlErr := exec.Command("journalctl", "-n", "1").Run()
		if journalctlErr == nil {
			return true
		}
	}
	return false
}

// AnalyzeLogs is a Diagnostic to check for recent problems in systemd service logs
type AnalyzeLogs struct {
	SystemdUnits map[string]types.SystemdUnit
}

func (d AnalyzeLogs) Name() string {
	return AnalyzeLogsName
}

func (d AnalyzeLogs) Description() string {
	return "Check for recent problems in systemd service logs"
}

func (d AnalyzeLogs) CanRun() (bool, error) {
	if HasJournalctl() {
		return true, nil
	}
	return false, errors.New("journalctl is not present/functional on this host")
}

func (d AnalyzeLogs) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(AnalyzeLogsName)

	for _, unit := range unitLogSpecs {
		for _, unitName := range unit.Names {
			if svc := d.SystemdUnits[unitName]; svc.Enabled || svc.Active {
				r.Info("DS0001", fmt.Sprintf("Checking journalctl logs for '%s' service", unitName))

				cmd := exec.Command("journalctl", "-ru", unitName, "--output=json")
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
					r.Error("DS0002", err, fmt.Sprintf(sdLogReadErr, unitName, errStr(err)))
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
						r.Debug("DS0003", fmt.Sprintf("Couldn't read the JSON for this log message:\n%s\nGot error %s", string(bytes), errStr(err)))
					} else {
						if lineCount > 500 && stampTooOld(entry.TimeStamp, timeLimit) {
							r.Debug("DS0004", fmt.Sprintf("Stopped reading %s log: timestamp %s too old", unitName, entry.TimeStamp))
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
									currKeep := match.Interpret(&entry, strings, r)
									keep = currKeep
								} else { // apply generic match processing
									text := fmt.Sprintf("Found '%s' journald log message:\n  %s\n%s", unitName, entry.Message, match.Interpretation)
									switch match.Level {
									case log.DebugLevel:
										r.Debug(match.Id, text)
									case log.InfoLevel:
										r.Info(match.Id, text)
									case log.WarnLevel:
										r.Warn(match.Id, nil, text)
									case log.ErrorLevel:
										r.Error(match.Id, nil, text)
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
	}

	return r
}

func stampTooOld(stamp string, timeLimit time.Time) bool {
	if epochns, err := strconv.ParseInt(stamp, 10, 64); err == nil {
		return time.Unix(epochns/1000000, 0).Before(timeLimit)
	}
	return true // something went wrong, stop looking...
}
