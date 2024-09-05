package terminationlog

import "regexp"

type lateConnectionLogEntry struct {
	*LogEntry
	RequestURI string
	SourceIP   string
	UserAgent  string
}

func newLateConnectionsSummary() *lateConnectionSummary {
	return &lateConnectionSummary{ByNode: map[string][]*lateConnectionLogEntry{}}
}

type lateConnectionSummary struct {
	ByNode map[string][]*lateConnectionLogEntry
}

// lateConnectionMsgRe captures requestURI, sourceIP, and userAgent from the
// message: Request to %q (source %q, user agent %q) through a connection created
// very late in the graceful termination process (more than 80% has passed),
// possibly a sign for a broken load balancer setup.
var lateConnectionMsgRe = regexp.MustCompile(`(?m)Request to "(?P<requestURI>[^"]*)" \(source IP (?P<sourceIP>[^,:]*)(?::[0-9]+)?. user agent "(?P<userAgent>[^"\\]*(?:\\.[^"\\]*)*)".*connection created very late in the graceful termination process.*`)

func (h *lateConnectionSummary) ProcessTerminationLogEntry(entry *LogEntry) {
	// ignore log lines that did not have a timestamp
	if entry.TS.IsZero() {
		return
	}
	match := lateConnectionMsgRe.FindStringSubmatch(entry.Msg)
	if match == nil {
		return
	}
	entries := h.ByNode[entry.Node]
	entries = append(entries, &lateConnectionLogEntry{
		LogEntry:   entry,
		RequestURI: match[1],
		SourceIP:   match[2],
		UserAgent:  match[3],
	})
	h.ByNode[entry.Node] = entries
}
