package pullprogress

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

const (
	defaultCountTimeThreshhold    = 10 * time.Second
	defaultProgressTimeThreshhold = 45 * time.Second
	defaultStableThreshhold       = 3
)

// NewPullProgressWriter creates a writer that periodically reports
// on pull progress of an image. It only reports when the state of the
// different layers has changed and uses time threshholds to limit the
// rate of the reports.
func NewPullProgressWriter(reportFn func(*ProgressReport)) io.Writer {
	pipeIn, pipeOut := io.Pipe()
	progressWriter := &pullProgressWriter{
		Writer:                 pipeOut,
		decoder:                json.NewDecoder(pipeIn),
		layerStatus:            map[string]Status{},
		reportFn:               reportFn,
		countTimeThreshhold:    defaultCountTimeThreshhold,
		progressTimeThreshhold: defaultProgressTimeThreshhold,
		stableThreshhold:       defaultStableThreshhold,
	}
	go func() {
		err := progressWriter.readProgress()
		if err != nil {
			pipeIn.CloseWithError(err)
		}
	}()
	return progressWriter
}

// Status is a structure representation of a Docker pull progress line
type Status struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail Detail `json:"progressDetail"`
	Progress       string `json:"progress"`
}

// Detail is the progressDetail structure in a Docker pull progress line
type Detail struct {
	Current int64 `json:"current"`
	Total   int64 `json:"total"`
}

// ProgressReport is a report of the progress of an image pull. It provides counts
// of layers in a given state. It also provides a percentage of downloaded data
// of those layers that are currently getting downloaded
type ProgressReport struct {
	Waiting     int
	Downloading int
	Extracting  int
	Complete    int

	DownloadPct float32
}

type statusType int

const (
	statusWaiting statusType = iota
	statusDownloading
	statusExtracting
	statusComplete
)

type pullProgressWriter struct {
	io.Writer
	decoder                *json.Decoder
	layerStatus            map[string]Status
	lastLayerCount         int
	stableLines            int
	stableThreshhold       int
	countTimeThreshhold    time.Duration
	progressTimeThreshhold time.Duration
	lastReport             *ProgressReport
	lastReportTime         time.Time
	reportFn               func(*ProgressReport)
}

func (w *pullProgressWriter) readProgress() error {
	for {
		status := &Status{}
		err := w.decoder.Decode(status)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		err = w.processStatus(status)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *pullProgressWriter) isStableLayerCount() bool {
	// If the number of layers has changed since last status, we're not stable
	if w.lastLayerCount != len(w.layerStatus) {
		w.lastLayerCount = len(w.layerStatus)
		w.stableLines = 0
		return false
	}
	// Only proceed after we've received status for the same number
	// of layers at least 3 times. If not, they're still increasing
	w.stableLines++
	if w.stableLines < w.stableThreshhold {
		// We're not stable enough yet
		return false
	}

	return true
}

func (w *pullProgressWriter) processStatus(status *Status) error {
	// determine if it's a status we want to process
	if !isLayerStatus(status) {
		return nil
	}

	w.layerStatus[status.ID] = *status

	// if the number of layers has not stabilized yet, return and wait for more
	// progress
	if !w.isStableLayerCount() {
		return nil
	}

	report := createProgressReport(w.layerStatus)

	// check if the count of layers in each state has changed
	if countsChanged(report, w.lastReport) {
		// only report on changed counts if the change occurs after
		// a predefined set of seconds (10 sec by default). This prevents
		// multiple reports in rapid succession
		if time.Since(w.lastReportTime) > w.countTimeThreshhold {
			w.lastReport = report
			w.lastReportTime = time.Now()
			w.reportFn(report)
		}
	} else {
		// If counts haven't changed, but enough time has passed (45 sec by default),
		// at least report on download progress
		if time.Since(w.lastReportTime) > w.progressTimeThreshhold {
			w.lastReport = report
			w.lastReportTime = time.Now()
			w.reportFn(report)
		}
	}
	return nil
}

func countsChanged(new, old *ProgressReport) bool {
	if old == nil {
		return true
	}
	return new.Waiting != old.Waiting ||
		new.Downloading != old.Downloading ||
		new.Extracting != old.Extracting ||
		new.Complete != old.Complete
}

func layerStatusToPullStatus(str string) statusType {
	switch str {
	case "Downloading":
		return statusDownloading
	case "Extracting", "Verifying Checksum", "Download complete":
		return statusExtracting
	case "Pull complete", "Already exists":
		return statusComplete
	default: // "Pull fs layer" or "Waiting"
		return statusWaiting
	}
}

func isLayerStatus(status *Status) bool {
	// ignore status lines with no layer id
	if len(status.ID) == 0 {
		return false
	}
	// ignore status lines with the initial named layer
	if strings.HasPrefix(status.Status, "Pulling from") {
		return false
	}

	return true
}

func createProgressReport(layerStatus map[string]Status) *ProgressReport {
	report := &ProgressReport{}
	var totalDownload, totalCurrent int64
	for _, status := range layerStatus {
		pullStatus := layerStatusToPullStatus(status.Status)
		switch pullStatus {
		case statusWaiting:
			report.Waiting++
		case statusDownloading:
			report.Downloading++
			totalDownload += status.ProgressDetail.Total
			totalCurrent += status.ProgressDetail.Current
		case statusExtracting:
			report.Extracting++
		case statusComplete:
			report.Complete++
		}
	}
	if totalDownload == 0 {
		report.DownloadPct = 0
	} else {
		report.DownloadPct = float32(totalCurrent) / float32(totalDownload) * 100.0
	}
	return report
}
