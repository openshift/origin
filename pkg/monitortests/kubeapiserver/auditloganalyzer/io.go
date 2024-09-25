package auditloganalyzer

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/apimachinery/pkg/util/sets"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type eventWithCounter struct {
	event             *auditv1.Event
	count             int64
	statusCodeToCount map[int32]int64
	totalDuration     time.Duration
}

func newEventWithCounter(event *auditv1.Event) *eventWithCounter {
	return &eventWithCounter{
		event:             event,
		count:             1,
		statusCodeToCount: map[int32]int64{},
		totalDuration:     event.StageTimestamp.Time.Sub(event.RequestReceivedTimestamp.Time),
	}
}

func (e *eventWithCounter) addEvent(event *auditv1.Event) {
	e.count++
	e.totalDuration += event.StageTimestamp.Time.Sub(event.RequestReceivedTimestamp.Time)
	if event.ResponseStatus != nil {
		e.statusCodeToCount[event.ResponseStatus.Code] = e.statusCodeToCount[event.ResponseStatus.Code] + 1
	}
}

func PrintAuditEvents(writer io.Writer, events []*auditv1.Event) {
	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	//
	for _, event := range events {
		duration := event.StageTimestamp.Time.Sub(event.RequestReceivedTimestamp.Time)
		code := int32(0)
		if event.ResponseStatus != nil {
			code = event.ResponseStatus.Code
		}
		if _, err := fmt.Fprintf(w, "%s [%6s][%12s] [%3d]\t %s\t [%s]\n",
			event.RequestReceivedTimestamp.UTC().Format("15:04:05"),
			strings.ToUpper(event.Verb),
			duration,
			code,
			event.RequestURI,
			event.User.Username); err != nil {
			panic(err)
		}
	}
}

func PrintAuditEventsWithCount(writer io.Writer, events []*eventWithCounter) {
	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	//
	for _, event := range events {
		averageDuration := time.Duration(int64(event.totalDuration) / event.count)
		codeStrings := []string{}
		for code, count := range event.statusCodeToCount {
			codeStrings = append(codeStrings, fmt.Sprintf("%v-%v", code, count))
		}
		sort.Strings(codeStrings)
		if _, err := fmt.Fprintf(w, "%8s [%12s] [%v]\t %s\t [%s]\n",
			fmt.Sprintf("%dx", event.count),
			averageDuration,
			strings.Join(codeStrings, ","),
			event.event.RequestURI,
			event.event.User.Username); err != nil {
			panic(err)
		}
	}
}

func PrintAuditEventsWide(writer io.Writer, events []*auditv1.Event) {
	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	for _, event := range events {
		duration := event.StageTimestamp.Time.Sub(event.RequestReceivedTimestamp.Time)
		code := int32(0)
		if event.ResponseStatus != nil {
			code = event.ResponseStatus.Code
		}
		if _, err := fmt.Fprintf(w, "%s (%v) [%s][%s] [%d]\t %s\t [%s]\n",
			event.RequestReceivedTimestamp.UTC().Format("15:04:05"),
			event.AuditID,
			strings.ToUpper(event.Verb),
			duration,
			code,
			event.RequestURI,
			event.User.Username); err != nil {
			panic(err)
		}
	}
}

func PrintTopByUserAuditEvents(writer io.Writer, numToDisplay int, events []*auditv1.Event) {
	countUsers := map[string][]*auditv1.Event{}

	for _, event := range events {
		countUsers[event.User.Username] = append(countUsers[event.User.Username], event)
	}

	type userWithCount struct {
		name  string
		count int
	}
	result := []userWithCount{}

	for username, userEvents := range countUsers {
		result = append(result, userWithCount{name: username, count: len(userEvents)})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].count >= result[j].count
	})

	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	if len(result) > numToDisplay {
		result = result[0:numToDisplay]
	}

	for _, r := range result {
		fmt.Fprintf(w, "%dx\t %s\n", r.count, r.name)
	}
}

func PrintTopByResourceAuditEvents(writer io.Writer, numToDisplay int, events []*auditv1.Event) {
	result := map[string]int64{}

	for _, event := range events {
		noParamsUri := strings.Split(event.RequestURI, "?")
		uri := strings.Split(strings.TrimPrefix(noParamsUri[0], "/"), "/")
		if len(uri) == 0 {
			continue
		}

		switch uri[0] {
		// kube api
		case "api":
			switch len(uri) {
			case 1, 2:
				continue
			case 3:
				// /api/v1/nodes -> v1/nodes
				result[strings.Join(uri[1:3], "/")]++
			default:
				// /api/v1/namespaces/foo/secrets -> v1/secrets
				if uri[2] == "namespaces" && len(uri) >= 5 {
					result[uri[1]+"/"+uri[4]]++
					continue
				}
				result[strings.Join(uri[1:3], "/")]++
			}
		case "apis":
			switch len(uri) {
			case 1, 2, 3:
				continue
			case 4:
				result[strings.Join(uri[1:4], "/")]++
			default:
				if uri[3] == "namespaces" && len(uri) >= 6 {
					result[uri[1]+"/"+uri[5]]++
					continue
				}
				result[strings.Join(uri[1:4], "/")]++
			}
		}
	}

	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	type sortedResultItem struct {
		resource string
		count    int64
	}

	sortedResult := []sortedResultItem{}

	for resource, count := range result {
		sortedResult = append(sortedResult, sortedResultItem{resource: resource, count: count})
	}
	sort.Slice(sortedResult, func(i, j int) bool {
		return sortedResult[i].count >= sortedResult[j].count
	})
	if len(sortedResult) > numToDisplay {
		sortedResult = sortedResult[:numToDisplay]
	}

	for _, item := range sortedResult {
		fmt.Fprintf(w, "%dx\t %s\n", item.count, item.resource)
	}
}

func PrintTopByVerbAuditEvents(writer io.Writer, numToDisplay int, events []*auditv1.Event) {
	countVerbs := map[string][]*auditv1.Event{}

	for _, event := range events {
		countVerbs[event.Verb] = append(countVerbs[event.Verb], event)
	}

	result := map[string][]*eventWithCounter{}
	resultCounts := map[string]int{}

	for verb, eventList := range countVerbs {
		resultCounts[verb] = len(eventList)
		countedEvents := []*eventWithCounter{}
		for _, event := range eventList {
			found := false
			for i, countedEvent := range countedEvents {
				if IsEquivalentAuditURI(countedEvent.event.RequestURI, event.RequestURI) && countedEvent.event.User.Username == event.User.Username {
					countedEvents[i].addEvent(event)
					found = true
					break
				}
			}
			if !found {
				countedEvents = append(countedEvents, newEventWithCounter(event))
			}
		}

		sort.Slice(countedEvents, func(i, j int) bool {
			return countedEvents[i].count >= countedEvents[j].count
		})
		if len(countedEvents) <= numToDisplay {
			result[verb] = countedEvents
			continue
		}
		result[verb] = countedEvents[0:numToDisplay]
	}

	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	for _, verb := range sets.StringKeySet(result).List() {
		eventWithCounter := result[verb]
		fmt.Fprintf(w, "\nTop %d %q (of %d total hits):\n", numToDisplay, strings.ToUpper(verb), resultCounts[verb])
		PrintAuditEventsWithCount(writer, eventWithCounter)
	}
}

func GetEvents(auditFilenames ...string) ([]*auditv1.Event, error) {
	ret, readFailures, err := getEventFromManyFiles(auditFilenames...)
	if readFailures > 0 {
		fmt.Fprintf(os.Stderr, "had %d line read failures\n", readFailures)
	}

	// sort events by time
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].RequestReceivedTimestamp.Time.Before(ret[j].RequestReceivedTimestamp.Time)
	})

	return ret, err
}

func getEventFromManyFiles(auditFilenames ...string) ([]*auditv1.Event, int, error) {
	ret := []*auditv1.Event{}
	failures := 0
	for _, auditFilename := range auditFilenames {
		stat, err := os.Stat(auditFilename)
		if err != nil {
			return nil, 0, err
		}
		if !stat.IsDir() {
			var localEvents []*auditv1.Event
			var localFailures int
			var localErr error
			if strings.HasSuffix(auditFilename, ".gz") {
				localEvents, localFailures, localErr = getEventsFromZip(auditFilename)
			} else {
				localEvents, localFailures, localErr = getEventsFromFile(auditFilename)
			}
			if localErr != nil {
				return nil, 0, localErr
			}
			failures += localFailures
			ret = append(ret, localEvents...)
			continue
		}

		localEvents, localFailures, localErr := getEventsFromDirectory(auditFilename)
		if err != nil {
			return nil, 0, localErr
		}
		failures += localFailures
		ret = append(ret, localEvents...)
	}

	return ret, failures, nil
}

func getEventsFromZip(auditFilename string) ([]*auditv1.Event, int, error) {
	// TODO figure out if the zip is of an individual file or a directory
	return getEventsFromZipFile(auditFilename)
}

func getEventsFromFile(auditFilename string) ([]*auditv1.Event, int, error) {
	if strings.HasSuffix(auditFilename, ".gz") {
		return nil, 0, fmt.Errorf("is a zip")
	}

	stat, err := os.Stat(auditFilename)
	if err != nil {
		return nil, 0, err
	}
	if stat.IsDir() {
		return nil, 0, fmt.Errorf("wasn a directory")
	}
	file, err := os.Open(auditFilename)
	if err != nil {
		return nil, 0, err
	}

	scanner := bufio.NewScanner(file)
	return getEventsFromScanner(auditFilename, scanner)
}

func getEventsFromZipFile(auditFilename string) ([]*auditv1.Event, int, error) {
	if !strings.HasSuffix(auditFilename, ".gz") {
		return nil, 0, fmt.Errorf("not a zip")
	}

	stat, err := os.Stat(auditFilename)
	if err != nil {
		return nil, 0, err
	}
	if stat.IsDir() {
		return nil, 0, fmt.Errorf("wasn a directory")
	}
	file, err := os.Open(auditFilename)
	if err != nil {
		return nil, 0, err
	}

	zw, err := gzip.NewReader(file)
	if err != nil {
		return nil, 0, err
	}
	scanner := bufio.NewScanner(zw)
	return getEventsFromScanner(auditFilename, scanner)
}

func getEventsFromScanner(auditFilename string, scanner *bufio.Scanner) ([]*auditv1.Event, int, error) {
	ret := []*auditv1.Event{}
	failures := 0

	// each line in audit file use following format: `hostname {JSON}`, we are not interested in hostname,
	// so lets parse out the events.
	line := 0
	for scanner.Scan() {
		line++
		auditBytes := scanner.Bytes()
		if len(auditBytes) > 0 {
			if string(auditBytes[0]) != "{" {
				// strip the hostname part
				hostnameEndPos := bytes.Index(auditBytes, []byte(" "))
				if hostnameEndPos == -1 {
					// oops something is wrong in the file?
					continue
				}

				auditBytes = auditBytes[hostnameEndPos:]
			}
		}

		// shame, shame shame... we have to copy out the apiserver/apis/audit/v1alpha1.Event because adding it as dependency
		// will cause mess in flags...
		eventObj := &auditv1.Event{}
		if err := json.Unmarshal(auditBytes, eventObj); err != nil {
			failures++
			//klog.V(1).Infof("unable to decode %q line %d: %s to audit event: %v\n", auditFilename, line, string(auditBytes), err)
			continue
		}

		// Add to index
		ret = append(ret, eventObj)
	}
	return ret, failures, nil
}
func getEventsFromDirectory(auditFilename string) ([]*auditv1.Event, int, error) {
	ret := []*auditv1.Event{}
	failures := 0
	stat, err := os.Stat(auditFilename)
	if err != nil {
		return nil, 0, err
	}
	if !stat.IsDir() {
		return nil, 0, fmt.Errorf("wasn't a directory")
	}

	localLock := sync.Mutex{}
	waiters := sync.WaitGroup{}
	errs := []error{}
	// it was a directory, recurse.
	err = filepath.Walk(auditFilename,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Name() == stat.Name() {
				return nil
			}
			waiters.Add(1)
			go func() {
				defer waiters.Done()
				newEvents, readFailures, err := getEventFromManyFiles(path)

				localLock.Lock()
				defer localLock.Unlock()
				failures += readFailures
				ret = append(ret, newEvents...)
				errs = append(errs, err)
			}()
			return nil
		})
	waiters.Wait()
	if err != nil {
		return ret, failures, err
	}
	if len(errs) > 0 {
		return ret, failures, errors.NewAggregate(errs)
	}

	return ret, failures, nil
}

func PrintTopByHTTPStatusCodeAuditEvents(writer io.Writer, numToDisplay int, events []*auditv1.Event) {
	countHTTPStatusCode := map[int32][]*auditv1.Event{}

	for _, event := range events {
		if event.ResponseStatus == nil {
			countHTTPStatusCode[-1] = append(countHTTPStatusCode[-1], event)
			continue
		}
		countHTTPStatusCode[event.ResponseStatus.Code] = append(countHTTPStatusCode[event.ResponseStatus.Code], event)
	}

	result := map[int32][]*eventWithCounter{}
	resultCounts := map[int32]int{}

	for httpStatusCode, eventList := range countHTTPStatusCode {
		resultCounts[httpStatusCode] = len(eventList)
		countedEvents := []*eventWithCounter{}
		for _, event := range eventList {
			found := false
			for i, countedEvent := range countedEvents {
				if IsEquivalentAuditURI(countedEvent.event.RequestURI, event.RequestURI) && countedEvent.event.User.Username == event.User.Username {
					countedEvents[i].addEvent(event)
					found = true
					break
				}
			}
			if !found {
				countedEvents = append(countedEvents, newEventWithCounter(event))
			}
		}

		sort.Slice(countedEvents, func(i, j int) bool {
			return countedEvents[i].count >= countedEvents[j].count
		})
		if len(countedEvents) <= numToDisplay {
			result[httpStatusCode] = countedEvents
			continue
		}
		result[httpStatusCode] = countedEvents[0:numToDisplay]
	}

	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	for _, httpStatusCode := range sets.Int32KeySet(result).List() {
		eventWithCounter := result[httpStatusCode]
		fmt.Fprintf(w, "\nTop %d %d (of %d total hits):\n", numToDisplay, httpStatusCode, resultCounts[httpStatusCode])
		PrintAuditEventsWithCount(writer, eventWithCounter)
	}
}

func PrintTopByNamespace(writer io.Writer, numToDisplay int, events []*auditv1.Event) {
	countNamespaces := map[string][]*auditv1.Event{}

	for _, event := range events {
		// for a cluster scoped resource namespace will be empty.
		namespace, _, _, _ := URIToParts(event.RequestURI)
		countNamespaces[namespace] = append(countNamespaces[namespace], event)
	}

	type namespaceWithCount struct {
		name  string
		count int
	}
	result := []namespaceWithCount{}

	for namespace, namespaceEvents := range countNamespaces {
		result = append(result, namespaceWithCount{name: namespace, count: len(namespaceEvents)})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].count >= result[j].count
	})

	w := tabwriter.NewWriter(writer, 20, 0, 0, ' ', tabwriter.DiscardEmptyColumns)
	defer w.Flush()

	if len(result) > numToDisplay {
		result = result[0:numToDisplay]
	}

	for _, r := range result {
		fmt.Fprintf(w, "%dx\t %s\n", r.count, r.name)
	}
}

func PrintSummary(w io.Writer, events []*auditv1.Event) {
	if len(events) == 0 {
		return
	}

	first := events[0]
	last := events[len(events)-1]
	duration := last.RequestReceivedTimestamp.Time.Sub(first.RequestReceivedTimestamp.Time)

	fmt.Fprintf(w, "count: %d, first: %s, last: %s, duration: %s\n", len(events),
		first.RequestReceivedTimestamp.Time.Format(time.RFC3339), last.RequestReceivedTimestamp.Time.Format(time.RFC3339), duration.String())
}

// IsEquivalentAuditURI is fuzzy matcher that allows equivalence on non-exact matches.  This is important for watches and
// for lists since they can pass a resourceversion and timeout which always diffs, but is rarely importantly different
func IsEquivalentAuditURI(lhs, rhs string) bool {
	if lhs == rhs {
		return true
	}
	lhsURL, err := url.Parse("https://example.com" + lhs)
	if err != nil {
		panic(err)
	}
	rhsURL, err := url.Parse("https://example.com" + rhs)
	if err != nil {
		panic(err)
	}
	if lhsURL.Path != rhsURL.Path {
		return false
	}
	lhsQueries := lhsURL.Query()
	rhsQueries := lhsURL.Query()

	lhsKeys := sets.StringKeySet(lhsQueries)
	rhsKeys := sets.StringKeySet(rhsQueries)
	lhsKeys.Delete("timeout", "timeoutSeconds", "resourceVersion", "continue")
	rhsKeys.Delete("timeout", "timeoutSeconds", "resourceVersion", "continue")
	if !lhsKeys.Equal(rhsKeys) {
		return false
	}

	for _, queryName := range lhsKeys.List() {
		lhsQueryValue := lhsQueries[queryName]
		rhsQueryValue := rhsQueries[queryName]
		if !reflect.DeepEqual(lhsQueryValue, rhsQueryValue) {
			return false
		}
	}

	return true
}
