package kubeletlogcollector

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/staticpodinstall/kubeletlogparser"
	"github.com/openshift/origin/test/extended/util/image"

	"k8s.io/client-go/kubernetes"
)

func intervalsFromNodeLogs(ctx context.Context, kubeClient kubernetes.Interface, beginning, end time.Time) (monitorapi.Intervals, error) {
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
			nodeLogs, err := GetNodeLog(ctx, kubeClient, nodeName, "kubelet")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node logs from %s: %s", nodeName, err.Error())
				errCh <- err
				return
			}
			newEvents := eventsFromKubeletLogs(nodeName, nodeLogs)

			ovsVswitchdLogs, err := GetNodeLog(ctx, kubeClient, nodeName, "ovs-vswitchd")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node ovs-vswitchd logs from %s: %s", nodeName, err.Error())
				errCh <- err
				return
			}
			newOVSEvents := intervalsFromOVSVswitchdLogs(nodeName, ovsVswitchdLogs)

			networkManagerLogs, err := GetNodeLog(ctx, kubeClient, nodeName, "NetworkManager")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node NetworkManager logs from %s: %s", nodeName, err.Error())
				errCh <- err
				return
			}
			newNetworkManagerIntervals := intervalsFromNetworkManagerLogs(nodeName, networkManagerLogs)

			systemdCoreDumpLogs, err := GetNodeLog(ctx, kubeClient, nodeName, "systemd-coredump")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting node systemd-coredump logs from %s: %s", nodeName, err.Error())
				errCh <- err
				return
			}
			newSystemdCoreDumpIntervals := intervalsFromSystemdCoreDumpLogs(nodeName, systemdCoreDumpLogs)

			lock.Lock()
			defer lock.Unlock()
			ret = append(ret, newEvents...)
			ret = append(ret, newOVSEvents...)
			ret = append(ret, newNetworkManagerIntervals...)
			ret = append(ret, newSystemdCoreDumpIntervals...)
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

// eventsFromKubeletLogs returns the produced intervals.  Any errors during this creation are logged, but
// not returned because this is a best effort step
func eventsFromKubeletLogs(nodeName string, kubeletLog []byte) monitorapi.Intervals {
	nodeLocator := monitorapi.NewLocator().NodeFromName(nodeName)
	ret := monitorapi.Intervals{}

	parse := kubeletlogparser.NewEtcdStaticPodEventsFromKubelet()

	scanner := bufio.NewScanner(bytes.NewBuffer(kubeletLog))
	for scanner.Scan() {
		currLine := scanner.Text()
		ret = append(ret, readinessFailure(nodeName, currLine)...)
		ret = append(ret, readinessError(nodeName, currLine)...)
		ret = append(ret, statusHttpClientConnectionLostError(nodeName, currLine)...)
		ret = append(ret, reflectorHttpClientConnectionLostError(nodeName, currLine)...)
		ret = append(ret, kubeletNodeHttpClientConnectionLostError(nodeName, currLine)...)
		ret = append(ret, startupProbeError(nodeName, currLine)...)
		ret = append(ret, errParsingSignature(nodeName, currLine)...)
		ret = append(ret, failedToDeleteCGroupsPath(nodeLocator, currLine)...)
		ret = append(ret, anonymousCertConnectionError(nodeLocator, currLine)...)
		ret = append(ret, leaseUpdateError(nodeLocator, currLine)...)
		ret = append(ret, leaseFailBackOff(nodeLocator, currLine)...)
		ret = append(ret, parse(nodeName, currLine)...)
	}

	return ret
}

// intervalsFromOVSVswitchdLogs returns the produced intervals.  Any errors during this creation are logged, but
// not returned because this is a best effort step
func intervalsFromOVSVswitchdLogs(nodeName string, ovsLogs []byte) monitorapi.Intervals {
	nodeLocator := monitorapi.NewLocator().NodeFromName(nodeName)
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
func unreasonablyLongPollInterval(logLine string, nodeLocator monitorapi.Locator) monitorapi.Intervals {
	if !strings.Contains(logLine, "Unreasonably long") {
		return nil
	}

	toTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())

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
		monitorapi.NewInterval(monitorapi.SourceOVSVswitchdLog, monitorapi.Warning).Locator(
			nodeLocator).Message(monitorapi.NewMessage().HumanMessage(message)).
			Display().Build(fromTime, toTime),
	}
}

var unreasonablyLongPollIntervalRE = regexp.MustCompile(`Unreasonably long (\d+)ms poll interval`)

// intervalsFromSystemdCoreDumpLogs returns the produced intervals.  Any errors during this creation are logged, but
// not returned because this is a best effort step
func intervalsFromSystemdCoreDumpLogs(nodeName string, coreDumpLogs []byte) monitorapi.Intervals {
	nodeLocator := monitorapi.NewLocator().NodeFromName(nodeName)
	ret := monitorapi.Intervals{}

	scanner := bufio.NewScanner(bytes.NewBuffer(coreDumpLogs))
	for scanner.Scan() {
		currLine := scanner.Text()
		ret = append(ret, processCoreDump(currLine, nodeLocator)...)
	}

	return ret
}

// processCoreDump searches for core dump events with process information
//
// Process 7798 (haproxy) of user 1000680000 dumped core.
func processCoreDump(logLine string, nodeLocator monitorapi.Locator) monitorapi.Intervals {
	if !strings.Contains(logLine, "dumped core") {
		return nil
	}

	logTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())

	// Extract the process name from within parentheses
	var processName string
	match := coreDumpProcessRE.FindStringSubmatch(logLine)
	if match != nil && len(match) > 1 {
		processName = match[1]
	}

	message := logLine[strings.Index(logLine, "Process"):]

	// Build the message with process annotation if we extracted it
	messageBuilder := monitorapi.NewMessage().HumanMessage(message).Reason(monitorapi.ReasonProcessDumpedCore)
	if processName != "" {
		messageBuilder = messageBuilder.WithAnnotation("process", processName)
	}

	interval := monitorapi.NewInterval(monitorapi.SourceSystemdCoreDumpLog, monitorapi.Warning).Locator(
		nodeLocator).Message(messageBuilder).
		Display().Build(logTime, logTime.Add(1*time.Second))

	return monitorapi.Intervals{interval}
}

var coreDumpProcessRE = regexp.MustCompile(`Process \d+ \(([^)]+)\) of user \d+ dumped core`)

// intervalsFromNetworkManagerLogs returns the produced intervals.  Any errors during this creation are logged, but
// not returned because this is a best effort step
func intervalsFromNetworkManagerLogs(nodeName string, ovsLogs []byte) monitorapi.Intervals {
	locator := monitorapi.NewLocator().NodeFromName(nodeName)
	ret := monitorapi.Intervals{}

	scanner := bufio.NewScanner(bytes.NewBuffer(ovsLogs))
	for scanner.Scan() {
		currLine := scanner.Text()
		ret = append(ret, tooManyNetlinkEvents(currLine, locator)...)
	}

	return ret
}

// tooManyNetlinkEvents searches for a failure associated with https://issues.redhat.com/browse/OCPBUGS-11591
//
// Apr 12 11:49:49.188086 ci-op-xs3rnrtc-2d4c7-4mhm7-worker-b-dwc7w NetworkManager[1155]:
// <info> [1681300187.8326] platform-linux: netlink[rtnl]: read: too many netlink events.
// Need to resynchronize platform cache
func tooManyNetlinkEvents(logLine string, nodeLocator monitorapi.Locator) monitorapi.Intervals {
	if !strings.Contains(logLine, "too many netlink events. Need to resynchronize platform cache") {
		return nil
	}

	logTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())

	message := logLine[strings.Index(logLine, "NetworkManager"):]
	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceNetworkManagerLog, monitorapi.Warning).Locator(
			nodeLocator).Message(monitorapi.NewMessage().HumanMessage(message)).
			Display().Build(logTime, logTime.Add(1*time.Second)),
	}
}

type kubeletLogLineEventCreator func(logLine string) monitorapi.Intervals

var lineToEvents = []kubeletLogLineEventCreator{}

func readinessFailure(nodeName, logLine string) monitorapi.Intervals {
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
	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())
	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(containerRef).
			Message(monitorapi.NewMessage().Reason(monitorapi.ContainerReasonReadinessFailed).Node(nodeName).HumanMessage(message)).
			Display().
			Build(failureTime, failureTime),
	}
}

func readinessError(nodeName, logLine string) monitorapi.Intervals {
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
	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())
	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(containerRef).
			Message(
				monitorapi.NewMessage().
					Reason(monitorapi.ContainerReasonReadinessErrored).
					Node(nodeName).
					HumanMessage(message),
			).
			Display().
			Build(failureTime, failureTime),
	}
}

func errParsingSignature(nodeName, logLine string) monitorapi.Intervals {
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
	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())
	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(containerRef).
			Message(
				monitorapi.NewMessage().
					Reason(monitorapi.ContainerErrImagePull).
					Cause(monitorapi.ContainerUnrecognizedSignatureFormat).
					Node(nodeName),
			).
			Display().
			Build(failureTime, failureTime),
	}
}

// startupProbeError extracts locator information from kubelet logs of the form:
// "Probe failed" probeType="Startup" pod="<some_ns>/<some_pod" podUID="<some_pod_uid>" containerName="<some_container_name>" .* output="<some_output>"
// and returns an Interval (which will show up in the junit/events...json file and the intervals chart).
func startupProbeError(nodeName, logLine string) monitorapi.Intervals {
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
	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())
	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(containerRef).
			Message(
				monitorapi.NewMessage().
					Reason(monitorapi.ContainerReasonStartupProbeFailed).
					Node(nodeName).
					HumanMessage(message),
			).
			Display().
			Build(failureTime, failureTime),
	}
}

var imagePullContainerRefRegex = regexp.MustCompile(`err=.*for \\"(?P<CONTAINER>[a-z0-9.-]+)\\".*pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)" podUID="(?P<PODUID>[a-z0-9.-]+)"`)

var containerRefRegex = regexp.MustCompile(`pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)" podUID="(?P<PODUID>[a-z0-9.-]+)" containerName="(?P<CONTAINER>[a-z0-9.-]+)"`)
var readinessFailureOutputRegex = regexp.MustCompile(`"Probe failed" probeType="Readiness".*output="(?P<OUTPUT>.+)"`)
var readinessErrorOutputRegex = regexp.MustCompile(`"Probe errored" err="(?P<OUTPUT>.+)" probeType="Readiness"`)
var startupFailureOutputRegex = regexp.MustCompile(`"Probe failed" probeType="Startup".*output="(?P<OUTPUT>.+)"`)

// Some logs end with "probeResult=failure output=<" and the output continues on the next log line.
// Since we're parsing one line at a time, we won't get the output -- but we will match on the pattern
// so we won't miss the event.
var startupFailureMultiLineOutputRegex = regexp.MustCompile(`"Probe failed" probeType="Startup".*output=\<(?P<OUTPUT>.*)`)

func errImagePullToContainerReference(logLine string) monitorapi.Locator {
	// err="failed to \"StartContainer\" for \"oauth-proxy\"
	return regexToContainerReference(logLine, imagePullContainerRefRegex)
}

func probeProblemToContainerReference(logLine string) monitorapi.Locator {
	return regexToContainerReference(logLine, containerRefRegex)
}

func regexToContainerReference(logLine string, containerReferenceMatch *regexp.Regexp) monitorapi.Locator {
	containerReferenceMatch.MatchString(logLine)
	if !containerReferenceMatch.MatchString(logLine) {
		return monitorapi.Locator{}
	}

	subMatches := containerReferenceMatch.FindStringSubmatch(logLine)
	subNames := containerReferenceMatch.SubexpNames()
	namespace := ""
	podName := ""
	uid := ""
	containerName := ""
	for i, name := range subNames {
		switch name {
		case "NS":
			namespace = subMatches[i]
		case "POD":
			podName = subMatches[i]
		case "PODUID":
			uid = subMatches[i]
		case "CONTAINER":
			containerName = subMatches[i]
		}
	}

	return monitorapi.NewLocator().ContainerFromNames(namespace, podName, uid, containerName)
}

var reflectorRefRegex = regexp.MustCompile(`object-"(?P<NS>[a-z0-9.-]+)"\/"(?P<POD>[a-z0-9.-]+)"`)
var reflectorOutputRegex = regexp.MustCompile(`error on the server \("(?P<OUTPUT>.+)"\)`)

func reflectorHttpClientConnectionLostError(nodeName, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `http2: client connection lost`) {
		return nil
	}

	if !strings.Contains(logLine, `watch of`) {
		return nil
	}

	return commonErrorInterval(nodeName, logLine, reflectorOutputRegex, monitorapi.HttpClientConnectionLost, func() monitorapi.Locator {
		containerRef := regexToContainerReference(logLine, reflectorRefRegex)
		return containerRef
	})
}

var statusRefRegex = regexp.MustCompile(`podUID="(?P<PODUID>[a-z0-9.-]+)" pod="(?P<NS>[a-z0-9.-]+)\/(?P<POD>[a-z0-9.-]+)"`)
var statusOutputRegex = regexp.MustCompile(`err="(?P<OUTPUT>.+)"`)

func statusHttpClientConnectionLostError(nodeName, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `http2: client connection lost`) {
		return nil
	}

	if !strings.Contains(logLine, `Failed to get status for pod`) {
		return nil
	}

	return commonErrorInterval(nodeName, logLine, nodeOutputRegex, monitorapi.HttpClientConnectionLost, func() monitorapi.Locator {
		containerRef := regexToContainerReference(logLine, statusRefRegex)
		return containerRef
	})
}

func failedToDeleteCGroupsPath(nodeLocator monitorapi.Locator, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, "Failed to delete cgroup paths") {
		return nil
	}

	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())

	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Error).
			Locator(nodeLocator).
			Message(monitorapi.NewMessage().Reason(monitorapi.FailedToDeleteCGroupsPath).HumanMessage(logLine)).
			Display().
			Build(failureTime, failureTime.Add(1*time.Second)),
	}
}

func anonymousCertConnectionError(nodeLocator monitorapi.Locator, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, "User \"system:anonymous\"") {
		return nil
	}

	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())

	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Error).
			Locator(nodeLocator).
			Message(monitorapi.NewMessage().Reason(monitorapi.FailedToAuthenticateWithOpenShiftUser).
				HumanMessage(logLine)).
			Display().
			Build(failureTime, failureTime.Add(1*time.Second)),
	}
}

// lower 'f'ailed and 'error'
var failedLeaseUpdateErrorRegex = regexp.MustCompile(`failed to update lease, error: Put \"(?P<URL>[a-z0-9.-:\/\-\?\=]+)\": (?P<MSG>[^\"]+)`)

// upper 'F'ailed and 'err'
var failedLeaseUpdateErrRegex = regexp.MustCompile(`Failed to update lease\" err\=\"Put \\\"(?P<URL>[a-z0-9.-:\/\-\?\=]+)\\\": (?P<MSG>[^\"]+)`)

var failedLeaseFiveTimes = regexp.MustCompile(`failed to update lease using latest lease, fallback to ensure lease`)

func leaseUpdateError(nodeLocator monitorapi.Locator, logLine string) monitorapi.Intervals {

	// Two cases, one upper F the other lower so substring match without the leading f
	if !strings.Contains(logLine, "ailed to update lease") {
		return nil
	}

	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())
	url := ""
	msg := ""

	subMatches := failedLeaseUpdateErrorRegex.FindStringSubmatch(logLine)
	subNames := failedLeaseUpdateErrorRegex.SubexpNames()

	if len(subMatches) == 0 {
		subMatches = failedLeaseUpdateErrRegex.FindStringSubmatch(logLine)
		subNames = failedLeaseUpdateErrRegex.SubexpNames()
	}

	if len(subNames) > len(subMatches) {
		return nil
	}

	for i, name := range subNames {
		switch name {
		case "URL":
			url = subMatches[i]
		case "MSG":
			msg = subMatches[i]
		}
	}

	if len(url) == 0 && len(msg) == 0 {
		return nil
	}

	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(nodeLocator).
			Message(
				monitorapi.NewMessage().Reason(monitorapi.NodeFailedLease).HumanMessage(fmt.Sprintf("%s - %s", url, msg)),
			).
			Display().
			Build(failureTime, failureTime.Add(1*time.Second)),
	}
}

func leaseFailBackOff(nodeLocator monitorapi.Locator, logLine string) monitorapi.Intervals {

	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())

	subMatches := failedLeaseFiveTimes.FindStringSubmatch(logLine)

	if len(subMatches) == 0 {
		return nil
	}

	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(nodeLocator).
			Message(
				monitorapi.NewMessage().Reason(monitorapi.NodeFailedLeaseBackoff).HumanMessage("detected multiple lease failures"),
			).
			Display().
			Build(failureTime, failureTime.Add(1*time.Second)),
	}
}

// Our tests will flag an error if leases are failing more than 3 times in 33 seconds.
// So we will find the first lease failure and then see if more than 3 failures around leases happen
// If that is the case, we will flag that lease failure as important and fail the test.
// This will take all the intervals and return only the intervals that we want to report as a failure
func findLeaseIntervalsImportant(intervals monitorapi.Intervals) monitorapi.Intervals {
	const leaseInterval = 33 * time.Second
	// Get all intervals that have NodeLease errors
	nodeLeaseIntervals := intervals.Filter(monitorapi.NodeLeaseBackoff)
	if len(nodeLeaseIntervals) == 0 {
		return monitorapi.Intervals(nil)
	}
	intervalsByNode := make(map[string]monitorapi.Intervals)

	for _, val := range nodeLeaseIntervals {
		nodeName := val.Condition.Locator.Keys["node"]
		if len(intervalsByNode[nodeName]) == 0 {
			intervalsByNode[nodeName] = append(intervalsByNode[nodeName], val)
		}
		// We have a lot of events that have the same To and From.
		// We will assume that intervals that have the same node and occur at the same time
		// are duplicated events.
		previousInterval := intervalsByNode[nodeName][len(intervalsByNode[nodeName])-1]
		if val.From != previousInterval.From && val.To != previousInterval.To {
			intervalsByNode[nodeName] = append(intervalsByNode[nodeName], val)
		}
	}
	// Let's sort by node name so this is deterministic.
	var keys []string
	for node := range intervalsByNode {
		keys = append(keys, node)
	}
	sort.Strings(keys)
	importantIntervals := monitorapi.Intervals(nil)
	for _, node := range keys {
		nodeInterval := intervalsByNode[node]
		if len(nodeInterval) < 2 {
			continue
		}
		for i, interval := range nodeInterval {
			if i >= len(nodeInterval)-1 {
				continue
			}
			if nodeInterval[i+1].To.Before(interval.To.Add(leaseInterval)) {
				importantIntervals = append(importantIntervals, interval)
			}
		}
	}
	return importantIntervals
}

func findLeaseBackOffs(intervals monitorapi.Intervals) monitorapi.Intervals {
	nodeLeaseIntervals := intervals.Filter(monitorapi.NodeLeaseBackoff)
	if len(nodeLeaseIntervals) == 0 {
		return monitorapi.Intervals(nil)
	}
	return nodeLeaseIntervals
}

var nodeRefRegex = regexp.MustCompile(`error getting node \\"(?P<NODEID>[a-z0-9.-]+)\\"`)
var nodeOutputRegex = regexp.MustCompile(`err="(?P<OUTPUT>.+)"`)

func kubeletNodeHttpClientConnectionLostError(nodeName, logLine string) monitorapi.Intervals {
	if !strings.Contains(logLine, `http2: client connection lost`) {
		return nil
	}

	if !strings.Contains(logLine, `Error updating node status`) {
		return nil
	}

	return commonErrorInterval(nodeName, logLine, statusOutputRegex, monitorapi.HttpClientConnectionLost, func() monitorapi.Locator {
		return monitorapi.NewLocator().NodeFromName(nodeName)
	})

}

func commonErrorInterval(nodeName, logLine string, messageExp *regexp.Regexp, reason monitorapi.IntervalReason, locator func() monitorapi.Locator) monitorapi.Intervals {
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

	failureTime := utility.SystemdJournalLogTime(logLine, time.Now().Year())
	return monitorapi.Intervals{
		monitorapi.NewInterval(monitorapi.SourceKubeletLog, monitorapi.Info).
			Locator(locator()).
			Message(
				monitorapi.NewMessage().Reason(reason).Node(nodeName).HumanMessage(message),
			).
			Display().
			Build(failureTime, failureTime),
	}
}

// GetNodeLog returns logs for a particular systemd service on a given node.
// We're count on these logs to fit into some reasonable memory size.
// If the kubelet API is unavailable, it falls back to using a debug pod.
func GetNodeLog(ctx context.Context, client kubernetes.Interface, nodeName, systemdServiceName string) ([]byte, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix("journal").URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")
	req.Param("since", "-1d")
	req.Param("unit", systemdServiceName)

	in, err := req.Stream(ctx)
	if err != nil {
		// Kubelet API might be down, try fallback method using debug pod
		fmt.Fprintf(os.Stderr, "Failed to get logs via kubelet API for %s on node %s: %v. Trying fallback method...\n",
			systemdServiceName, nodeName, err)
		return getNodeLogViaDebugPod(ctx, client, nodeName, systemdServiceName)
	}
	defer in.Close()

	return ioutil.ReadAll(in)
}

// getNodeLogViaDebugPod collects systemd service logs using a privileged debug pod.
// This method works even when kubelet is down because it directly accesses the host filesystem.
func getNodeLogViaDebugPod(ctx context.Context, client kubernetes.Interface, nodeName, systemdServiceName string) ([]byte, error) {
	const debugNamespace = "openshift-e2e-node-log-collector"

	// Ensure namespace exists
	if err := ensureNamespace(ctx, client, debugNamespace); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace %s: %w", debugNamespace, err)
	}

	// Create debug pod
	podName := fmt.Sprintf("node-log-%s-%d", nodeName, time.Now().Unix())
	_, err := createDebugPod(ctx, client, debugNamespace, podName, nodeName, systemdServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug pod: %w", err)
	}

	// Ensure pod cleanup
	defer func() {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client.CoreV1().Pods(debugNamespace).Delete(deleteCtx, podName, metav1.DeleteOptions{})
	}()

	// Wait for pod to complete
	if err := waitForPodCompletion(ctx, client, debugNamespace, podName); err != nil {
		return nil, fmt.Errorf("pod did not complete: %w", err)
	}

	// Get logs from the pod
	logs, err := getPodLogs(ctx, client, debugNamespace, podName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}

	return logs, nil
}

// ensureNamespace creates the namespace if it doesn't exist
func ensureNamespace(ctx context.Context, client kubernetes.Interface, namespace string) error {
	_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	// Create namespace with privileged labels
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce":         "privileged",
				"pod-security.kubernetes.io/audit":           "privileged",
				"pod-security.kubernetes.io/warn":            "privileged",
				"security.openshift.io/scc.podSecurityLabelSync": "false",
			},
		},
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// createDebugPod creates a privileged pod that runs journalctl to collect logs
func createDebugPod(ctx context.Context, client kubernetes.Interface, namespace, podName, nodeName, systemdServiceName string) (*corev1.Pod, error) {
	privileged := true
	hostPathDirectory := corev1.HostPathDirectory
	zero := int64(0)

	// Build journalctl command to collect logs from the last 24 hours
	command := fmt.Sprintf("chroot /host journalctl --utc --no-pager -u %s --since=-1d", systemdServiceName)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "node-log-collector",
			},
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			HostPID:       true,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "log-collector",
					Image: image.ShellImage(),
					Command: []string{
						"/bin/bash",
						"-c",
						command,
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
						RunAsUser:  &zero,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host",
							MountPath: "/host",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/",
							Type: &hostPathDirectory,
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				{
					Operator: corev1.TolerationOpExists,
				},
			},
		},
	}

	return client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// waitForPodCompletion waits for the pod to reach completed or failed state
func waitForPodCompletion(ctx context.Context, client kubernetes.Interface, namespace, podName string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return true, nil
		case corev1.PodFailed:
			return false, fmt.Errorf("pod failed with reason: %s, message: %s",
				pod.Status.Reason, pod.Status.Message)
		case corev1.PodPending, corev1.PodRunning:
			// Check if container has terminated
			if len(pod.Status.ContainerStatuses) > 0 {
				containerStatus := pod.Status.ContainerStatuses[0]
				if containerStatus.State.Terminated != nil {
					if containerStatus.State.Terminated.ExitCode == 0 {
						return true, nil
					}
					return false, fmt.Errorf("container exited with code %d: %s",
						containerStatus.State.Terminated.ExitCode,
						containerStatus.State.Terminated.Message)
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("unexpected pod phase: %s", pod.Status.Phase)
		}
	})
}

// getPodLogs retrieves the logs from a completed pod
func getPodLogs(ctx context.Context, client kubernetes.Interface, namespace, podName string) ([]byte, error) {
	req := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
