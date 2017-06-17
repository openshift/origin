package chug

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
)

type Entry struct {
	IsLager bool
	Raw     []byte
	Log     LogEntry
}

type LogEntry struct {
	Timestamp time.Time
	LogLevel  lager.LogLevel

	Source  string
	Message string
	Session string

	Error error
	Trace string

	Data lager.Data
}

func Chug(reader io.Reader, out chan<- Entry) {
	scanner := bufio.NewReader(reader)
	for {
		line, err := scanner.ReadBytes('\n')
		if line != nil {
			out <- entry(bytes.TrimSuffix(line, []byte{'\n'}))
		}
		if err != nil {
			break
		}
	}
	close(out)
}

func entry(raw []byte) (entry Entry) {
	copiedBytes := make([]byte, len(raw))
	copy(copiedBytes, raw)
	entry = Entry{
		IsLager: false,
		Raw:     copiedBytes,
	}

	rawString := string(raw)
	idx := strings.Index(rawString, "{")
	if idx == -1 {
		return
	}

	var lagerLog lager.LogFormat
	decoder := json.NewDecoder(strings.NewReader(rawString[idx:]))
	err := decoder.Decode(&lagerLog)
	if err != nil {
		return
	}

	entry.Log, entry.IsLager = convertLagerLog(lagerLog)

	return
}

func convertLagerLog(lagerLog lager.LogFormat) (LogEntry, bool) {
	timestamp, err := strconv.ParseFloat(lagerLog.Timestamp, 64)

	if err != nil {
		return LogEntry{}, false
	}

	data := lagerLog.Data

	var logErr error
	if lagerLog.LogLevel == lager.ERROR || lagerLog.LogLevel == lager.FATAL {
		dataErr, ok := lagerLog.Data["error"]
		if ok {
			errorString, ok := dataErr.(string)
			if !ok {
				return LogEntry{}, false
			}
			logErr = errors.New(errorString)
			delete(lagerLog.Data, "error")
		}
	}

	var logTrace string
	dataTrace, ok := lagerLog.Data["trace"]
	if ok {
		logTrace, ok = dataTrace.(string)
		if !ok {
			return LogEntry{}, false
		}
		delete(lagerLog.Data, "trace")
	}

	var logSession string
	dataSession, ok := lagerLog.Data["session"]
	if ok {
		logSession, ok = dataSession.(string)
		if !ok {
			return LogEntry{}, false
		}
		delete(lagerLog.Data, "session")
	}

	return LogEntry{
		Timestamp: time.Unix(0, int64(timestamp*1e9)),
		LogLevel:  lagerLog.LogLevel,
		Source:    lagerLog.Source,
		Message:   lagerLog.Message,
		Session:   logSession,

		Error: logErr,
		Trace: logTrace,

		Data: data,
	}, true
}
