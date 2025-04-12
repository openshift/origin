package auditloganalyzer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CountForSecond struct {
	// NumberOfConcurrentRequestsBeingHandled is the sum of requests that were ongoing (minus watches I think, I'm not sure).
	// That includes
	// 1. requests that were started before this second and not completed until during or after this second
	// 2. requests that were started this second
	NumberOfConcurrentRequestsBeingHandled int

	NumberOfRequestsReceived int

	// NumberOfRequestsReceivedThatLaterGot500 is calculated based on when the request was received instead of completed
	// because different requests have different timeouts, so it's more useful to categorize based on received time
	NumberOfRequestsReceivedThatLaterGot500 int
}

type CountsForRun struct {
	NumberOfSeconds int

	// CountsForEachSecond is a count for every second of a job run.  Every index is one second long and the count starts
	// with the creation time of the ClusterVersion resource.
	CountsForEachSecond []CountForSecond

	EstimatedStartOfCluster metav1.Time
	LastAcceptedTime        metav1.Time
}

type countTracking struct {
	lock         sync.Mutex
	CountsForRun CountsForRun
}

func CountsOverTime(estimatedStartOfCluster metav1.Time) *countTracking {
	numberOfSecondsToTrack := 6 * 60 * 60 // six hours

	return &countTracking{
		CountsForRun: CountsForRun{
			NumberOfSeconds:         numberOfSecondsToTrack,
			CountsForEachSecond:     make([]CountForSecond, numberOfSecondsToTrack),
			EstimatedStartOfCluster: estimatedStartOfCluster,
			LastAcceptedTime:        metav1.Time{Time: estimatedStartOfCluster.Add(time.Duration(numberOfSecondsToTrack-1) * time.Second)},
		},
	}
}

func (s *countTracking) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	receivedIndex, completionIndex, httpStatusCode, include := s.CountsForRun.countIndexesFromAuditTime(auditEvent)
	if !include {
		return
	}

	s.CountsForRun.CountsForEachSecond[receivedIndex].NumberOfRequestsReceived++
	if httpStatusCode == 500 {
		s.CountsForRun.CountsForEachSecond[receivedIndex].NumberOfRequestsReceivedThatLaterGot500++
	}

	for i := receivedIndex; i <= completionIndex; i++ {
		s.CountsForRun.CountsForEachSecond[receivedIndex].NumberOfConcurrentRequestsBeingHandled++
	}
}

// returns received index, completion index, status code, and false if the index is out of bounds
func (c *CountsForRun) countIndexesFromAuditTime(in *auditv1.Event) (int, int, int, bool) {
	if in.Stage != auditv1.StageResponseComplete {
		return -1, -1, -1, false
	}
	if in.Verb == "watch" {
		return -1, -1, -1, false
	}
	if !strings.HasPrefix(in.RequestURI, "/api/") && !strings.HasPrefix(in.RequestURI, "/apis/") {
		return -1, -1, -1, false
	}
	if in.RequestReceivedTimestamp.BeforeTime(&c.EstimatedStartOfCluster) {
		return -1, -1, -1, false
	}
	if in.RequestReceivedTimestamp.After(c.LastAcceptedTime.Time) {
		return -1, -1, -1, false
	}

	receivedIndex := int(in.RequestReceivedTimestamp.Sub(c.EstimatedStartOfCluster.Time).Round(time.Second).Seconds())
	completionIndex := int(in.StageTimestamp.Sub(c.EstimatedStartOfCluster.Time).Round(time.Second).Seconds())
	if completionIndex >= c.NumberOfSeconds {
		completionIndex = c.NumberOfSeconds - 1
	}

	statusCode := 0
	if in.ResponseStatus != nil {
		statusCode = int(in.ResponseStatus.Code)
	}

	return receivedIndex, completionIndex, statusCode, true
}

func (c *CountsForRun) ToCSV() ([]byte, error) {
	out := &bytes.Buffer{}
	csvWriter := csv.NewWriter(out)
	if err := csvWriter.Write([]string{"seconds after cluster start", "number of requests being handled", "number of requests received", "number of requests received this second getting 500"}); err != nil {
		return nil, fmt.Errorf("failed writing headers: %w", err)
	}

	for i, curr := range c.CountsForEachSecond {
		record := []string{
			strconv.Itoa(i),
			strconv.Itoa(curr.NumberOfConcurrentRequestsBeingHandled),
			strconv.Itoa(curr.NumberOfRequestsReceived),
			strconv.Itoa(curr.NumberOfRequestsReceivedThatLaterGot500),
		}

		if err := csvWriter.Write(record); err != nil {
			return nil, fmt.Errorf("failed writing record %d: %w", i, err)
		}
	}
	csvWriter.Flush()

	return out.Bytes(), nil
}

func (c *CountsForRun) LastSecondWithData() int {
	lastIndexWithData := len(c.CountsForEachSecond) - 1
	for i := lastIndexWithData; i >= 0; i-- {
		if c.CountsForEachSecond[i].NumberOfConcurrentRequestsBeingHandled != 0 {
			return lastIndexWithData
		}
		if c.CountsForEachSecond[i].NumberOfRequestsReceived != 0 {
			return lastIndexWithData
		}
		if c.CountsForEachSecond[i].NumberOfRequestsReceivedThatLaterGot500 != 0 {
			return lastIndexWithData
		}
		lastIndexWithData = i
	}

	return lastIndexWithData
}

func (c *CountsForRun) TruncateDataAfterLastValue() {
	lastIndexWithData := c.LastSecondWithData()

	c.NumberOfSeconds = lastIndexWithData + 1
	c.LastAcceptedTime = metav1.Time{Time: c.EstimatedStartOfCluster.Add(time.Duration(lastIndexWithData) * time.Second)}
	c.CountsForEachSecond = c.CountsForEachSecond[0:lastIndexWithData]
}

func (c *CountsForRun) SubsetDataAtTime(endTime metav1.Time) *CountsForRun {
	if c == nil {
		return nil
	}
	if !endTime.After(c.EstimatedStartOfCluster.Time) {
		return nil
	}
	if endTime.After(c.LastAcceptedTime.Time) {
		endTime = c.LastAcceptedTime
	}
	lastIndex := int(endTime.Sub(c.EstimatedStartOfCluster.Time).Round(time.Second).Seconds())
	if lastIndex > c.NumberOfSeconds {
		lastIndex = c.NumberOfSeconds
	}

	ret := &CountsForRun{
		NumberOfSeconds:         lastIndex + 1,
		CountsForEachSecond:     c.CountsForEachSecond[0:lastIndex],
		EstimatedStartOfCluster: c.EstimatedStartOfCluster,
		LastAcceptedTime:        endTime,
	}

	return ret
}

func (c *CountsForRun) ToLineChart() (*plot.Plot, error) {
	p := plot.New()
	p.Title.Text = "Requests by Second of Cluster Life"
	p.Y.Label.Text = "Number of Requests in that Second"
	p.X.Label.Text = "Seconds of Cluster Life"
	p.X.Tick.Marker = plot.TimeTicks{}
	plotter.DefaultLineStyle.Width = vg.Points(1)

	concurrentRequestsBeingHandled := plotter.XYs{}
	requestReceived := plotter.XYs{}
	requestReceivedThatResultsIn500 := plotter.XYs{}
	for i, requestCounts := range c.CountsForEachSecond {
		timeOfSecond := c.EstimatedStartOfCluster.Add(time.Duration(i) * time.Second)
		concurrentRequestsBeingHandled = append(concurrentRequestsBeingHandled, plotter.XY{X: float64(timeOfSecond.Unix()), Y: float64(requestCounts.NumberOfConcurrentRequestsBeingHandled)})
		requestReceived = append(requestReceived, plotter.XY{X: float64(timeOfSecond.Unix()), Y: float64(requestCounts.NumberOfRequestsReceived)})
		requestReceivedThatResultsIn500 = append(requestReceivedThatResultsIn500, plotter.XY{X: float64(timeOfSecond.Unix()), Y: float64(requestCounts.NumberOfRequestsReceivedThatLaterGot500)})
	}

	lineOfConcurrentRequestsBeingHandled, err := plotter.NewLine(concurrentRequestsBeingHandled)
	if err != nil {
		return nil, err
	}
	lineOfConcurrentRequestsBeingHandled.LineStyle.Color = color.RGBA{R: 0, G: 0, B: 255, A: 255}
	p.Add(lineOfConcurrentRequestsBeingHandled)
	p.Legend.Add("Concurrent Requests", lineOfConcurrentRequestsBeingHandled)

	lineOfRequestReceived, err := plotter.NewLine(requestReceived)
	if err != nil {
		return nil, err
	}
	lineOfRequestReceived.LineStyle.Color = color.RGBA{R: 0, G: 255, B: 0, A: 255}
	p.Add(lineOfRequestReceived)
	p.Legend.Add("Requests Received", lineOfRequestReceived)

	lineOfRequestReceivedThatResultsIn500, err := plotter.NewLine(requestReceivedThatResultsIn500)
	if err != nil {
		return nil, err
	}
	lineOfRequestReceivedThatResultsIn500.LineStyle.Color = color.RGBA{R: 255, G: 0, B: 0, A: 255}
	p.Add(lineOfRequestReceivedThatResultsIn500)
	p.Legend.Add("Requests Received Ending in 500", lineOfRequestReceivedThatResultsIn500)

	return p, nil
}

func (c *CountsForRun) WriteContentToStorage(storageDir, name, timeSuffix string) error {
	csvContent, err := c.ToCSV()
	if err != nil {
		return err
	}
	requestCountsByTimeFile := path.Join(storageDir, fmt.Sprintf("%s_%s.csv", name, timeSuffix))
	if err := os.WriteFile(requestCountsByTimeFile, csvContent, 0644); err != nil {
		return err
	}
	requestCountsForEntireRunPlot, err := c.ToLineChart()
	if err != nil {
		return err
	}
	requestCountsByTimeGraphFile := path.Join(storageDir, fmt.Sprintf("%s_%s.png", name, timeSuffix))
	err = requestCountsForEntireRunPlot.Save(vg.Length(c.NumberOfSeconds), 500, requestCountsByTimeGraphFile)
	if err != nil {
		return err
	}

	return nil
}
