package intervalcreation

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitor/nodedetails"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
)

const (
	msgPhaseDrain          = "reason/NodeUpdate phase/Drain roles/%s drained node"
	msgPhaseOSUpdate       = "reason/NodeUpdate phase/OperatingSystemUpdate roles/%s updated operating system"
	msgPhaseReboot         = "reason/NodeUpdate phase/Reboot roles/%s rebooted and kubelet started"
	msgPhaseNeverCompleted = "reason/NodeUpdate phase/%s roles/%s phase never completed"
)

func IntervalsFromEvents_NodeChanges(events monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	var intervals monitorapi.Intervals
	nodeChangeToLastStart := map[string]map[string]time.Time{}
	nodeNameToRoles := map[string]string{}

	openInterval := func(state map[string]time.Time, name string, from time.Time) bool {
		if _, ok := state[name]; !ok {
			state[name] = from
			return false
		}
		return true
	}
	closeInterval := func(state map[string]time.Time, name string, level monitorapi.EventLevel, locator, message string, to time.Time) {
		from, ok := state[name]
		if !ok {
			return
		}
		intervals = append(intervals, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   level,
				Locator: locator,
				Message: message,
			},
			From: from,
			To:   to,
		})
		delete(state, name)
	}

	for _, event := range events {
		node, ok := monitorapi.NodeFromLocator(event.Locator)
		if !ok {
			continue
		}
		if !strings.HasPrefix(event.Message, "reason/") {
			continue
		}
		reason := strings.SplitN(strings.TrimPrefix(event.Message, "reason/"), " ", 2)[0]

		roles := monitorapi.GetNodeRoles(event)
		nodeNameToRoles[node] = roles
		state, ok := nodeChangeToLastStart[node]
		if !ok {
			state = make(map[string]time.Time)
			nodeChangeToLastStart[node] = state
		}

		// Use the four key reasons set by the MCD in events - since events are best effort these
		// could easily be incorrect or hide problems like double invocations. For now use these as
		// timeline indicators only, and use the stronger observed state events from the node object.
		// A separate test should look for anomalies in these intervals if the need is identified.
		//
		// The current structure will hold events open until they are closed, so you would see
		// excessively long intervals if the events are missing (because openInterval does not create
		// a new interval).
		switch reason {
		case "MachineConfigChange":
			openInterval(state, "Update", event.From)
		case "MachineConfigReached":
			message := strings.ReplaceAll(event.Message, "reason/MachineConfigReached ", "reason/NodeUpdate phase/Update ") + " roles/" + roles
			closeInterval(state, "Update", monitorapi.Info, event.Locator, message, event.From)
		case "Cordon", "Drain":
			openInterval(state, "Drain", event.From)
		case "OSUpdateStarted":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseDrain, roles), event.From)
			openInterval(state, "OperatingSystemUpdate", event.From)
		case "Reboot":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseDrain, roles), event.From)
			closeInterval(state, "OperatingSystemUpdate", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseOSUpdate, roles), event.From)
			openInterval(state, "Reboot", event.From)
		case "Starting":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseDrain, roles), event.From)
			closeInterval(state, "OperatingSystemUpdate", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseOSUpdate, roles), event.From)
			closeInterval(state, "Reboot", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseReboot, roles), event.From)
		}
	}

	for nodeName, state := range nodeChangeToLastStart {
		for name := range state {
			closeInterval(state, name, monitorapi.Warning, monitorapi.NodeLocator(nodeName), fmt.Sprintf(msgPhaseNeverCompleted, name, nodeNameToRoles[nodeName]), end)
		}
	}

	return intervals
}

func IntervalsFromNodeLogs(ctx context.Context, kubeClient kubernetes.Interface, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}

	allNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ret, err
	}

	collectionStart := time.Now()
	lock := sync.Mutex{}
	errCh := make(chan error, len(allNodes.Items))
	wg := sync.WaitGroup{}
	for _, node := range allNodes.Items {
		wg.Add(1)
		go func(ctx context.Context, nodeName string) {
			defer wg.Done()

			// TODO limit by begin/end here instead of post-processing
			nodeLogs, err := nodedetails.GetNodeLog(ctx, kubeClient, nodeName, "kubelet")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node logs from %s: %s", nodeName, err.Error())
				errCh <- err
				return
			}
			newEvents := eventsFromKubeletLogs(nodeName, nodeLogs)

			ovsVswitchdLogs, err := nodedetails.GetNodeLog(ctx, kubeClient, nodeName, "ovs-vswitchd")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node ovs-vswitchd logs from %s: %s", nodeName, err.Error())
				errCh <- err
				return
			}
			newOVSEvents := eventsFromOVSVswitchdLogs(nodeName, ovsVswitchdLogs)

			lock.Lock()
			defer lock.Unlock()
			ret = append(ret, newEvents...)
			ret = append(ret, newOVSEvents...)
		}(ctx, node.Name)
	}
	wg.Wait()
	collectionEnd := time.Now()
	fmt.Fprintf(os.Stdout, "Collection of node logs and analysis took: %v\n", collectionEnd.Sub(collectionStart))

	errs := []error{}
	for len(errCh) > 0 {
		err := <-errCh
		errs = append(errs, err)
	}

	return ret, utilerrors.NewAggregate(errs)
}

func IntervalsFromAuditLogs(ctx context.Context, kubeClient kubernetes.Interface, beginning, end time.Time) (*nodedetails.AuditLogSummary, monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}

	// TODO honor begin and end times.  maybe
	auditLogSummary, err := nodedetails.GetKubeAuditLogSummary(ctx, kubeClient)
	if err != nil {
		// TODO report the error AND the best possible summary we have
		return auditLogSummary, nil, err
	}

	return auditLogSummary, ret, nil
}

// eventsFromKubeletLogs returns the produced intervals.  Any errors during this creation are logged, but
// not returned because this is a best effort step
func eventsFromKubeletLogs(nodeName string, kubeletLog []byte) monitorapi.Intervals {
	nodeLocator := monitorapi.NodeLocator(nodeName)
	ret := monitorapi.Intervals{}

	scanner := bufio.NewScanner(bytes.NewBuffer(kubeletLog))
	for scanner.Scan() {
		currLine := scanner.Text()
		ret = append(ret, readinessFailure(currLine)...)
		ret = append(ret, readinessError(currLine)...)
		ret = append(ret, statusHttpClientConnectionLostError(currLine)...)
		ret = append(ret, reflectorHttpClientConnectionLostError(currLine)...)
		ret = append(ret, kubeletNodeHttpClientConnectionLostError(currLine)...)
		ret = append(ret, startupProbeError(currLine)...)
		ret = append(ret, errParsingSignature(currLine)...)
		ret = append(ret, failedToDeleteCGroupsPath(nodeLocator, currLine)...)
		ret = append(ret, anonymousCertConnectionError(nodeLocator, currLine)...)
	}

	return ret
}

// eventsFromOVSVswitchdLogs returns the produced intervals.  Any errors during this creation are logged, but
// not returned because this is a best effort step
func eventsFromOVSVswitchdLogs(nodeName string, ovsLogs []byte) monitorapi.Intervals {
	nodeLocator := monitorapi.NodeLocator(nodeName)
	ret := monitorapi.Intervals{}

	scanner := bufio.NewScanner(bytes.NewBuffer(ovsLogs))
	for scanner.Scan() {
		currLine := scanner.Text()
		ret = append(ret, unreasonablyLongPollInterval(currLine, nodeLocator)...)
	}

	return ret
}

// unreasonablyLongPollInterval searches for a failure associated with https://issues.redhat.com/browse/OCPBUGS-11591
//
// Apr 12 11:53:51.395838 ci-op-xs3rnrtc-2d4c7-4mhm7-worker-b-dwc7w ovs-vswitchd[1124]:
// ovs|00002|timeval(urcu4)|WARN|Unreasonably long 109127ms poll interval (0ms user, 0ms system)
func unreasonablyLongPollInterval(logLine, nodeLocator string) monitorapi.Intervals {
	if !strings.Contains(logLine, "Unreasonably long") {
		return nil
	}

	toTime := systemdJournalLogTime(logLine)

	// Extract the number of millis and use it for the interval, starting from the point we logged
	// and looking backwards.
	fromTime := toTime
	match := unreasonablyLongPollIntervalRE.FindStringSubmatch(logLine)
	if match == nil {
		fmt.Fprintf(os.Stderr, "Failure extracting milliseconds from log line we should have been able to parse: %s\n", logLine)
	} else {
		millis, err := strconv.Atoi(match[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error converting extracted millis to int for %s\n", match[1])
		}
		fromTime = toTime.Add(-time.Millisecond * time.Duration(millis))
	}

	message := logLine[strings.Index(logLine, "ovs-vswitchd"):]
	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Warning,
				Locator: nodeLocator,
				Message: message,
			},
			From: fromTime,
			To:   toTime,
		},
	}
}

var unreasonablyLongPollIntervalRE = regexp.MustCompile(`Unreasonably long (\d+)ms poll interval`)

type kubeletLogLineEventCreator func(logLine string) monitorapi.Intervals

var lineToEvents = []kubeletLogLineEventCreator{}

func readinessFailure(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `Probe failed`) {
		return nil
	}
	if !strings.Contains(logLine, `probeType="Readiness"`) {
		return nil
	}

	readinessFailureOutputRegex.MatchString(logLine)
	if !readinessFailureOutputRegex.MatchString(logLine) {
		return nil
	}
	outputSubmatches := readinessFailureOutputRegex.FindStringSubmatch(logLine)
	message := outputSubmatches[1]
	// message contains many \", this removes the escaping to result in message containing "
	// if we have an error, just use the original message, we don't really care that much.
	if unquotedMessage, err := strconv.Unquote(`"` + message + `"`); err == nil {
		message = unquotedMessage
	}

	containerRef := probeProblemToContainerReference(logLine)
	failureTime := systemdJournalLogTime(logLine)
	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: containerRef.ToLocator(),
				Message: monitorapi.ReasonedMessage(monitorapi.ContainerReasonReadinessFailed, message),
			},
			From: failureTime,
			To:   failureTime,
		},
	}
}

func readinessError(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `Probe errored`) {
		return nil
	}
	if !strings.Contains(logLine, `probeType="Readiness"`) {
		return nil
	}

	readinessErrorOutputRegex.MatchString(logLine)
	if !readinessErrorOutputRegex.MatchString(logLine) {
		return nil
	}
	outputSubmatches := readinessErrorOutputRegex.FindStringSubmatch(logLine)
	message := outputSubmatches[1]
	message, _ = strconv.Unquote(`"` + message + `"`)

	containerRef := probeProblemToContainerReference(logLine)
	failureTime := systemdJournalLogTime(logLine)
	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: containerRef.ToLocator(),
				Message: monitorapi.ReasonedMessage(monitorapi.ContainerReasonReadinessErrored, message),
			},
			From: failureTime,
			To:   failureTime,
		},
	}
}

func errParsingSignature(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, "StartContainer") {
		return nil
	}
	if !strings.Contains(logLine, "ErrImagePull") {
		return nil
	}
	if !strings.Contains(logLine, "unrecognized signature format") {
		return nil
	}

	containerRef := errImagePullToContainerReference(logLine)
	failureTime := systemdJournalLogTime(logLine)
	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: containerRef.ToLocator(),
				Message: monitorapi.ReasonedMessage(monitorapi.ContainerErrImagePull, monitorapi.ContainerUnrecognizedSignatureFormat),
			},
			From: failureTime,
			To:   failureTime,
		},
	}
}

// startupProbeError extracts locator information from kubelet logs of the form:
// "Probe failed" probeType="Startup" pod="<some_ns>/<some_pod" podUID=<some_pod_uid> containerName="<some_container_name>" .* output="<some_output>"
// and returns an Interval (which will show up in the junit/events...json file and the intervals chart).
func startupProbeError(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `Probe failed`) {
		return nil
	}
	if !strings.Contains(logLine, `probeType="Startup"`) {
		return nil
	}

	// Match one of the two types of logs for Startup Probe failures.
	lineMatch := startupFailureOutputRegex.MatchString(logLine)
	multiLineMatch := startupFailureMultiLineOutputRegex.MatchString(logLine)
	if !lineMatch && !multiLineMatch {
		return nil
	}
	var outputSubmatches []string
	if lineMatch {
		outputSubmatches = startupFailureOutputRegex.FindStringSubmatch(logLine)
	} else {
		outputSubmatches = startupFailureMultiLineOutputRegex.FindStringSubmatch(logLine)
	}
	message := outputSubmatches[1]
	// message contains many \", this removes the escaping to result in message containing "
	// if we have an error, just use the original message, we don't really care that much.
	if unquotedMessage, err := strconv.Unquote(`"` + message + `"`); err == nil {
		message = unquotedMessage
	}

	containerRef := probeProblemToContainerReference(logLine)
	failureTime := systemdJournalLogTime(logLine)
	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: containerRef.ToLocator(),
				Message: monitorapi.ReasonedMessage(monitorapi.ContainerReasonStartupProbeFailed, message),
			},
			From: failureTime,
			To:   failureTime,
		},
	}
}

var imagePullContainerRefRegex = regexp.MustCompile(`err=.*for \\"(?P<CONTAINER>[a-z0-9.-]+)\\".*pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)" podUID=(?P<PODUID>[a-z0-9.-]+)`)

var containerRefRegex = regexp.MustCompile(`pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)" podUID=(?P<PODUID>[a-z0-9.-]+) containerName="(?P<CONTAINER>[a-z0-9.-]+)"`)
var readinessFailureOutputRegex = regexp.MustCompile(`"Probe failed" probeType="Readiness".*output="(?P<OUTPUT>.+)"`)
var readinessErrorOutputRegex = regexp.MustCompile(`"Probe errored" err="(?P<OUTPUT>.+)" probeType="Readiness"`)
var startupFailureOutputRegex = regexp.MustCompile(`"Probe failed" probeType="Startup".*output="(?P<OUTPUT>.+)"`)

// Some logs end with "probeResult=failure output=<" and the output continues on the next log line.
// Since we're parsing one line at a time, we won't get the output -- but we will match on the pattern
// so we won't miss the event.
var startupFailureMultiLineOutputRegex = regexp.MustCompile(`"Probe failed" probeType="Startup".*output=\<(?P<OUTPUT>.*)`)

func errImagePullToContainerReference(logLine string) monitorapi.ContainerReference {
	// err="failed to \"StartContainer\" for \"oauth-proxy\"
	return regexToContainerReference(logLine, imagePullContainerRefRegex)
}

func probeProblemToContainerReference(logLine string) monitorapi.ContainerReference {
	return regexToContainerReference(logLine, containerRefRegex)
}

func regexToContainerReference(logLine string, containerReferenceMatch *regexp.Regexp) monitorapi.ContainerReference {
	ret := monitorapi.ContainerReference{}
	containerReferenceMatch.MatchString(logLine)
	if !containerReferenceMatch.MatchString(logLine) {
		return ret
	}

	subMatches := containerReferenceMatch.FindStringSubmatch(logLine)
	subNames := containerReferenceMatch.SubexpNames()
	for i, name := range subNames {
		switch name {
		case "NS":
			ret.Pod.Namespace = subMatches[i]
		case "POD":
			ret.Pod.Name = subMatches[i]
		case "PODUID":
			ret.Pod.UID = subMatches[i]
		case "CONTAINER":
			ret.ContainerName = subMatches[i]
		}
	}

	return ret
}

var reflectorRefRegex = regexp.MustCompile(`object-"(?P<NS>[a-z0-9.-]+)"\/"(?P<POD>[a-z0-9.-]+)"`)
var reflectorOutputRegex = regexp.MustCompile(`error on the server \("(?P<OUTPUT>.+)"\)`)

func reflectorHttpClientConnectionLostError(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `http2: client connection lost`) {
		return nil
	}

	if !strings.Contains(logLine, `watch of`) {
		return nil
	}

	return commonErrorInterval(logLine, reflectorOutputRegex, "HttpClientConnectionLost", func() string {
		containerRef := regexToContainerReference(logLine, reflectorRefRegex)
		return containerRef.ToLocator()
	})
}

var statusRefRegex = regexp.MustCompile(`podUID=(?P<PODUID>[a-z0-9.-]+) pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)"`)
var statusOutputRegex = regexp.MustCompile(`err="(?P<OUTPUT>.+)"`)

func statusHttpClientConnectionLostError(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `http2: client connection lost`) {
		return nil
	}

	if !strings.Contains(logLine, `Failed to get status for pod`) {
		return nil
	}

	return commonErrorInterval(logLine, nodeOutputRegex, "HttpClientConnectionLost", func() string {
		containerRef := regexToContainerReference(logLine, statusRefRegex)
		return containerRef.ToLocator()
	})
}

func failedToDeleteCGroupsPath(nodeLocator, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, "Failed to delete cgroup paths") {
		return nil
	}

	failureTime := systemdJournalLogTime(logLine)

	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: nodeLocator,
				Message: monitorapi.ReasonedMessage("FailedToDeleteCGroupsPath", logLine),
			},
			From: failureTime,
			To:   failureTime.Add(1 * time.Second),
		},
	}
}

func anonymousCertConnectionError(nodeLocator, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, "User \"system:anonymous\"") {
		return nil
	}

	failureTime := systemdJournalLogTime(logLine)

	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: nodeLocator,
				Message: monitorapi.ReasonedMessage("FailedToAuthenticateWithOpenShiftUser", logLine),
			},
			From: failureTime,
			To:   failureTime.Add(1 * time.Second),
		},
	}
}

var nodeRefRegex = regexp.MustCompile(`error getting node \\"(?P<NODEID>[a-z0-9.-]+)\\"`)
var nodeOutputRegex = regexp.MustCompile(`err="(?P<OUTPUT>.+)"`)

func kubeletNodeHttpClientConnectionLostError(logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `http2: client connection lost`) {
		return nil
	}

	if !strings.Contains(logLine, `Error updating node status`) {
		return nil
	}

	return commonErrorInterval(logLine, statusOutputRegex, "HttpClientConnectionLost", func() string {
		nodeRefRegex.MatchString(logLine)
		if !nodeRefRegex.MatchString(logLine) {
			return ""
		}
		return "node/" + nodeRefRegex.FindStringSubmatch(logLine)[1]
	})

}

func commonErrorInterval(logLine string, messageExp *regexp.Regexp, reason string, locator func() string) monitorapi.Intervals {
	messageExp.MatchString(logLine)
	if !messageExp.MatchString(logLine) {
		return nil
	}
	outputSubmatches := messageExp.FindStringSubmatch(logLine)
	message := outputSubmatches[1]
	// message contains many \", this removes the escaping to result in message containing "
	// if we have an error, just use the original message, we don't really care that much.
	if unquotedMessage, err := strconv.Unquote(`"` + message + `"`); err == nil {
		message = unquotedMessage
	}

	failureTime := systemdJournalLogTime(logLine)
	return monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: locator(),
				Message: monitorapi.ReasonedMessage(reason, message),
			},
			From: failureTime,
			To:   failureTime,
		},
	}
}

var kubeletTimeRegex = regexp.MustCompile(`^(?P<MONTH>\S+)\s(?P<DAY>\S+)\s(?P<TIME>\S+)`)

// systemdJournalLogTime returns Now if there is trouble reading the time.  This will stack the event intervals without
// parsable times at the end of the run, which will be more clearly visible as a problem than not reporting them.
func systemdJournalLogTime(logLine string) time.Time {
	kubeletTimeRegex.MatchString(logLine)
	if !kubeletTimeRegex.MatchString(logLine) {
		return time.Now()
	}

	month := ""
	day := ""
	year := fmt.Sprintf("%d", time.Now().Year())
	timeOfDay := ""
	subMatches := kubeletTimeRegex.FindStringSubmatch(logLine)
	subNames := kubeletTimeRegex.SubexpNames()
	for i, name := range subNames {
		switch name {
		case "MONTH":
			month = subMatches[i]
		case "DAY":
			day = subMatches[i]
		case "TIME":
			timeOfDay = subMatches[i]
		}
	}

	timeString := fmt.Sprintf("%s %s %s %s UTC", day, month, year, timeOfDay)
	ret, err := time.Parse("02 Jan 2006 15:04:05.999999999 MST", timeString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failure parsing time format: %v for %q\n", err, timeString)
		return time.Now()
	}

	return ret
}
