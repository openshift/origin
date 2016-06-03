package imageprogress

import (
	"encoding/json"
	"io"
	"regexp"
	"time"
)

const (
	defaultProgressTimeThreshhold = 30 * time.Second
	defaultStableThreshhold       = 10
)

// progressLine is a structure representation of a Docker pull progress line
type progressLine struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Detail *progressDetail `json:"progressDetail"`
}

// progressDetail is the progressDetail structure in a Docker pull progress line
type progressDetail struct {
	Current int64 `json:"current"`
	Total   int64 `json:"total"`
}

// layerDetail is layer information associated with a specific layerStatus
type layerDetail struct {
	Count   int
	Current int64
	Total   int64
}

// layerStatus is one of different possible status for layers detected by
// the ProgressWriter
type layerStatus int

const (
	statusPending layerStatus = iota
	statusDownloading
	statusExtracting
	statusComplete
	statusPushing
)

// layerStatusFromDockerString translates a string in a Docker status
// line to a layerStatus
func layerStatusFromDockerString(dockerStatus string) layerStatus {
	switch dockerStatus {
	case "Pushing":
		return statusPushing
	case "Downloading":
		return statusDownloading
	case "Extracting", "Verifying Checksum", "Download complete":
		return statusExtracting
	case "Pull complete", "Already exists", "Pushed":
		return statusComplete
	default:
		return statusPending
	}
}

type report map[layerStatus]*layerDetail

func (r report) count(status layerStatus) int {
	detail, ok := r[status]
	if !ok {
		return 0
	}
	return detail.Count
}

func (r report) percentProgress(status layerStatus) float32 {
	detail, ok := r[status]
	if !ok {
		return 0
	}
	if detail.Total == 0 {
		return 0
	}
	pct := float32(detail.Current) / float32(detail.Total) * 100.0
	if pct > 100.0 {
		pct = 100.0
	}
	return pct
}

func (r report) totalCount() int {
	cnt := 0
	for _, detail := range r {
		cnt += detail.Count
	}
	return cnt
}

// newWriter creates a writer that periodically reports
// on pull/push progress of a Docker image. It only reports when the state of the
// different layers has changed and uses time thresholds to limit the
// rate of the reports.
func newWriter(reportFn func(report), layersChangedFn func(report, report) bool) io.Writer {
	pipeIn, pipeOut := io.Pipe()
	writer := &imageProgressWriter{
		Writer:                 pipeOut,
		decoder:                json.NewDecoder(pipeIn),
		layerStatus:            map[string]progressLine{},
		reportFn:               reportFn,
		layersChangedFn:        layersChangedFn,
		progressTimeThreshhold: defaultProgressTimeThreshhold,
		stableThreshhold:       defaultStableThreshhold,
	}
	go func() {
		err := writer.readProgress()
		if err != nil {
			pipeIn.CloseWithError(err)
		}
	}()
	return writer
}

type imageProgressWriter struct {
	io.Writer
	decoder                *json.Decoder
	layerStatus            map[string]progressLine
	lastLayerCount         int
	stableLines            int
	stableThreshhold       int
	progressTimeThreshhold time.Duration
	lastReport             report
	lastReportTime         time.Time
	reportFn               func(report)
	layersChangedFn        func(report, report) bool
}

func (w *imageProgressWriter) readProgress() error {
	for {
		line := &progressLine{}
		err := w.decoder.Decode(line)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		err = w.processLine(line)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *imageProgressWriter) processLine(line *progressLine) error {
	// determine if it's a line we want to process
	if !islayerStatus(line) {
		return nil
	}

	w.layerStatus[line.ID] = *line

	// if the number of layers has not stabilized yet, return and wait for more
	// progress
	if !w.isStableLayerCount() {
		return nil
	}

	r := createReport(w.layerStatus)

	// check if the count of layers in each state has changed
	if w.layersChangedFn(w.lastReport, r) {
		w.lastReport = r
		w.lastReportTime = time.Now()
		w.reportFn(r)
		return nil
	}
	// If layer counts haven't changed, but enough time has passed (30 sec by default),
	// at least report on download/push progress
	if time.Since(w.lastReportTime) > w.progressTimeThreshhold {
		w.lastReport = r
		w.lastReportTime = time.Now()
		w.reportFn(r)
	}
	return nil
}

func (w *imageProgressWriter) isStableLayerCount() bool {
	// If the number of layers has changed since last status, we're not stable
	if w.lastLayerCount != len(w.layerStatus) {
		w.lastLayerCount = len(w.layerStatus)
		w.stableLines = 0
		return false
	}
	// Only proceed after we've received status for the same number
	// of layers at least stableThreshhold times. If not, they're still increasing
	w.stableLines++
	if w.stableLines < w.stableThreshhold {
		// We're not stable enough yet
		return false
	}

	return true
}

var layerIDRegexp = regexp.MustCompile("^[a-f,0-9]*$")

func islayerStatus(line *progressLine) bool {
	// ignore status lines with no layer id
	if len(line.ID) == 0 {
		return false
	}
	// ignore layer ids that are not hex string
	if !layerIDRegexp.MatchString(line.ID) {
		return false
	}
	return true
}

func createReport(dockerProgress map[string]progressLine) report {
	r := report{}
	for _, line := range dockerProgress {
		layerStatus := layerStatusFromDockerString(line.Status)
		detail, exists := r[layerStatus]
		if !exists {
			detail = &layerDetail{}
			r[layerStatus] = detail
		}
		detail.Count++
		if line.Detail != nil {
			detail.Current += line.Detail.Current
			detail.Total += line.Detail.Total
		}
	}
	return r
}
