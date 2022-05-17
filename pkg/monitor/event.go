package monitor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

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
	m.RecordAt(t, condition)

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
