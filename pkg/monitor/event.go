package monitor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startEventMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	reMatchFirstQuote := regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`)

	// filter out events written "now" but with significantly older start times (events
	// created in test jobs are the most common)
	significantlyBeforeNow := time.Now().UTC().Add(-15 * time.Minute)

	// map event UIDs to the last resource version we observed, used to skip recording resources
	// we've already recorded.
	processedEventUIDs := map[types.UID]string{}

	listWatch := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "events", "", fields.Everything())
	customStore := &cache.FakeCustomStore{
		// ReplaceFunc called when we do our initial list on starting the reflector. With no resync period,
		// it should not get called again.
		ReplaceFunc: func(items []interface{}, rv string) error {
			for _, obj := range items {
				event, ok := obj.(*corev1.Event)
				if !ok {
					continue
				}
				if processedEventUIDs[event.UID] != event.ResourceVersion {
					m.RecordResource("events", event)
					processedEventUIDs[event.UID] = event.ResourceVersion
				}
			}
			return nil
		},
		AddFunc: func(obj interface{}) error {
			event, ok := obj.(*corev1.Event)
			if !ok {
				return nil
			}
			if processedEventUIDs[event.UID] != event.ResourceVersion {
				recordAddOrUpdateEvent(ctx, m, client, reMatchFirstQuote, significantlyBeforeNow, event)
				processedEventUIDs[event.UID] = event.ResourceVersion
			}
			return nil
		},
		UpdateFunc: func(obj interface{}) error {
			event, ok := obj.(*corev1.Event)
			if !ok {
				return nil
			}
			if processedEventUIDs[event.UID] != event.ResourceVersion {
				recordAddOrUpdateEvent(ctx, m, client, reMatchFirstQuote, significantlyBeforeNow, event)
				processedEventUIDs[event.UID] = event.ResourceVersion
			}
			return nil
		},
	}
	reflector := cache.NewReflector(listWatch, &corev1.Event{}, customStore, 0)
	go reflector.Run(ctx.Done())
}

var allRepeatedEventPatterns = combinedDuplicateEventPatterns()

func combinedDuplicateEventPatterns() *regexp.Regexp {
	s := ""
	for _, r := range duplicateevents.AllowedRepeatedEventPatterns {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	for _, r := range duplicateevents.AllowedUpgradeRepeatedEventPatterns {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	for _, r := range duplicateevents.KnownEventsBugs {
		if s != "" {
			s += "|"
		}
		s += r.Regexp.String()
	}
	return regexp.MustCompile(s)
}

// checkAllowedRepeatedEventOKFns loops through all of the IsRepeatedEventOKFunc funcs
// and returns true if this event is an event we already know about.  Some functions
// require a kubeconfig, but also handle the kubeconfig being nil (in case we cannot
// successfully get a kubeconfig).
func checkAllowedRepeatedEventOKFns(event monitorapi.EventInterval, times int32) bool {

	kubeClient, err := GetMonitorRESTConfig()
	if err != nil {
		// If we couldn't get a kube client, we won't be able to run functions that require a kube config
		// but we can still pass a nil so that the rest of the functions can run.
		fmt.Printf("error getting kubeclient: [%v] when processing event %s - %s\n", err, event.Locator, event.Message)
		kubeClient = nil
	}
	for _, isRepeatedEventOKFuncList := range [][]duplicateevents.IsRepeatedEventOKFunc{duplicateevents.AllowedRepeatedEventFns, duplicateevents.AllowedSingleNodeRepeatedEventFns} {
		for _, allowRepeatedEventFn := range isRepeatedEventOKFuncList {
			allowed, err := allowRepeatedEventFn(event, kubeClient, int(times))
			if err != nil {
				// for errors, we'll default to no match.
				fmt.Printf("Error processing pathological event: %v\n", err)
				return false
			}
			if allowed {
				return true
			}
		}
	}
	return false
}

func recordAddOrUpdateEvent(
	ctx context.Context,
	m Recorder,
	client kubernetes.Interface,
	reMatchFirstQuote *regexp.Regexp,
	significantlyBeforeNow time.Time,
	obj *corev1.Event) {

	m.RecordResource("events", obj)

	// Temporary hack by dgoodwin, we're missing events here that show up later in
	// gather-extra/events.json. Adding some output to see if we can isolate what we saw
	// and where it might have been filtered out.
	// TODO: monitor for occurrences of this string, may no longer be needed given the
	// new rv handling logic added above.
	osEvent := false
	if obj.Reason == "OSUpdateStaged" || obj.Reason == "OSUpdateStarted" {
		osEvent = true
		fmt.Printf("Watch received OS update event: %s - %s - %s\n",
			obj.Reason, obj.InvolvedObject.Name, obj.LastTimestamp.Format(time.RFC3339))

	}
	t := obj.LastTimestamp.Time
	if t.IsZero() {
		t = obj.EventTime.Time
	}
	if t.IsZero() {
		t = obj.CreationTimestamp.Time
	}
	if t.Before(significantlyBeforeNow) {
		if osEvent {
			fmt.Printf("OS update event filtered for being too old: %s - %s - %s (now: %s)\n",
				obj.Reason, obj.InvolvedObject.Name, obj.LastTimestamp.Format(time.RFC3339),
				time.Now().Format(time.RFC3339))
		}
		return
	}

	message := obj.Message
	if obj.Count > 1 {
		message += fmt.Sprintf(" (%d times)", obj.Count)
	}

	if obj.InvolvedObject.Kind == "Node" {
		if node, err := client.CoreV1().Nodes().Get(ctx, obj.InvolvedObject.Name, metav1.GetOptions{}); err == nil {
			message = fmt.Sprintf("roles/%s %s", nodeRoles(node), message)
		}
	}

	// special case some very common events
	switch obj.Reason {
	case "":
	case "Killing":
		if obj.InvolvedObject.Kind == "Pod" {
			if containerName, ok := eventForContainer(obj.InvolvedObject.FieldPath); ok {
				message = fmt.Sprintf("container/%s reason/%s", containerName, obj.Reason)
				break
			}
		}
		message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
	case "Pulling", "Pulled":
		if obj.InvolvedObject.Kind == "Pod" {
			if containerName, ok := eventForContainer(obj.InvolvedObject.FieldPath); ok {
				if m := reMatchFirstQuote.FindStringSubmatch(obj.Message); m != nil {
					if len(m) > 3 {
						if d, err := time.ParseDuration(m[3]); err == nil {
							message = fmt.Sprintf("container/%s reason/%s duration/%.3fs image/%s", containerName, obj.Reason, d.Seconds(), m[1])
							break
						}
					}
					message = fmt.Sprintf("container/%s reason/%s image/%s", containerName, obj.Reason, m[1])
					break
				}
			}
		}
		message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
	default:
		message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
	}
	condition := monitorapi.Condition{
		Level:   monitorapi.Info,
		Locator: locateEvent(obj),
		Message: message,
	}
	if obj.Type == corev1.EventTypeWarning {
		condition.Level = monitorapi.Warning
	}

	pathoFrom := obj.LastTimestamp.Time
	if pathoFrom.IsZero() {
		pathoFrom = obj.EventTime.Time
	}
	if pathoFrom.IsZero() {
		pathoFrom = obj.CreationTimestamp.Time
	}
	to := pathoFrom.Add(1 * time.Second)

	event := monitorapi.EventInterval{
		Condition: condition,
	}

	if obj.Count > 1 {

		// The matching here needs to mimic what is being done in the synthetictests/testDuplicatedEvents function.
		eventDisplayMessage := fmt.Sprintf("%s - %s", event.Locator, event.Message)

		updatedMessage := event.Message
		if allRepeatedEventPatterns.MatchString(eventDisplayMessage) || checkAllowedRepeatedEventOKFns(event, obj.Count) {
			// This is a repeated event that we know about
			updatedMessage = fmt.Sprintf("%s %s", duplicateevents.InterestingMark, updatedMessage)
		}

		if obj.Count > duplicateevents.DuplicateEventThreshold && duplicateevents.EventCountExtractor.MatchString(eventDisplayMessage) {
			// This is a repeated event that exceeds threshold
			updatedMessage = fmt.Sprintf("%s %s", duplicateevents.PathologicalMark, updatedMessage)
		}

		condition.Message = updatedMessage
		if strings.Contains(condition.Message, duplicateevents.InterestingMark) || strings.Contains(condition.Message, duplicateevents.PathologicalMark) {

			// Remove the "(n times)" portion of the message, and get the first 10 characters of the hash of the message
			// so we can add it to the locator. This incorporates the message into the locator without the resulting
			// string being too much longer and makes it so that the spyglass chart shows locators that incorporate the message.
			removeNTimes := regexp.MustCompile(`\s+\(\d+ times\)`)
			newMessage := removeNTimes.ReplaceAllString(condition.Message, "")

			hash := sha256.Sum256([]byte(newMessage))
			hashStr := fmt.Sprintf("%x", hash)[:10]

			condition.Locator = fmt.Sprintf("%s hmsg/%s", condition.Locator, hashStr)
		}

		fmt.Printf("processed event: %+v\nresulting new interval: %s from: %s to %s\n", *obj, message, pathoFrom, to)

		// Add the interval.
		inter := m.StartInterval(pathoFrom, condition)
		m.EndInterval(inter, to)

	} else {
		m.RecordAt(t, condition)
	}
}

func eventForContainer(fieldPath string) (string, bool) {
	if !strings.HasSuffix(fieldPath, "}") {
		return "", false
	}
	fieldPath = strings.TrimSuffix(fieldPath, "}")
	switch {
	case strings.HasPrefix(fieldPath, "spec.containers{"):
		return strings.TrimPrefix(fieldPath, "spec.containers{"), true
	case strings.HasPrefix(fieldPath, "spec.initContainers{"):
		return strings.TrimPrefix(fieldPath, "spec.initContainers{"), true
	default:
		return "", false
	}
}
