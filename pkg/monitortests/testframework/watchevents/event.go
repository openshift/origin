package watchevents

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var reMatchFirstQuote = regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`)

func startEventMonitoring(ctx context.Context, m monitorapi.RecorderWriter, adminRESTConfig *rest.Config, client kubernetes.Interface) {

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
				recordAddOrUpdateEvent(ctx, m, adminRESTConfig, client, significantlyBeforeNow, event)
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
				recordAddOrUpdateEvent(ctx, m, adminRESTConfig, client, significantlyBeforeNow, event)
				processedEventUIDs[event.UID] = event.ResourceVersion
			}
			return nil
		},
	}
	reflector := cache.NewReflector(listWatch, &corev1.Event{}, customStore, 0)
	go reflector.Run(ctx.Done())
}

var allRepeatedEventPatterns = combinedDuplicateEventPatterns()

// TODO: delete this
func combinedDuplicateEventPatterns() *regexp.Regexp {
	s := ""
	for _, r := range pathologicaleventlibrary.AllowedRepeatedEventPatterns {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	for _, r := range pathologicaleventlibrary.AllowedUpgradeRepeatedEventPatterns {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	for _, r := range pathologicaleventlibrary.KnownEventsBugs {
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
func checkAllowedRepeatedEventOKFns(adminRESTConfig *rest.Config, event monitorapi.Interval, times int32) bool {
	for _, isRepeatedEventOKFuncList := range [][]pathologicaleventlibrary.IsRepeatedEventOKFunc{pathologicaleventlibrary.AllowedRepeatedEventFns, pathologicaleventlibrary.AllowedSingleNodeRepeatedEventFns} {
		for _, allowRepeatedEventFn := range isRepeatedEventOKFuncList {
			allowed, err := allowRepeatedEventFn(event, adminRESTConfig, int(times))
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
	recorder monitorapi.RecorderWriter,
	adminRESTConfig *rest.Config,
	client kubernetes.Interface,
	significantlyBeforeNow time.Time,
	obj *corev1.Event) {

	recorder.RecordResource("events", obj)

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

	message := monitorapi.NewMessage().HumanMessage(obj.Message)
	if obj.Count > 1 {
		message = message.WithAnnotation(monitorapi.AnnotationCount, string(obj.Count))
	}

	if obj.InvolvedObject.Kind == "Node" {
		if node, err := client.CoreV1().Nodes().Get(ctx, obj.InvolvedObject.Name, metav1.GetOptions{}); err == nil {
			message = message.WithAnnotation(monitorapi.AnnotationRoles, nodeRoles(node))
		}
	}
	if obj.Reason != "" {
		message = message.Reason(monitorapi.IntervalReason(obj.Reason))
	}

	// special case some very common events
	switch obj.Reason {
	case "":
	case "Killing":
		if obj.InvolvedObject.Kind == "Pod" {
			if containerName, ok := eventForContainer(obj.InvolvedObject.FieldPath); ok {
				message = message.WithAnnotation(monitorapi.AnnotationContainer, containerName)
				break
			}
		}
	case "Pulling", "Pulled":
		if obj.InvolvedObject.Kind == "Pod" {
			if containerName, ok := eventForContainer(obj.InvolvedObject.FieldPath); ok {
				if m := reMatchFirstQuote.FindStringSubmatch(obj.Message); m != nil {
					if len(m) > 3 {
						if d, err := time.ParseDuration(m[3]); err == nil {
							message = message.WithAnnotation(monitorapi.AnnotationContainer, containerName)
							message = message.WithAnnotation(monitorapi.AnnotationDuration, fmt.Sprintf("%.3fs", d.Seconds()))
							message = message.WithAnnotation(monitorapi.AnnotationImage, m[1])
							break
						}
					}
					message = message.WithAnnotation(monitorapi.AnnotationContainer, containerName)
					message = message.WithAnnotation(monitorapi.AnnotationImage, m[1])
					break
				}
			}
		}
	default:
	}

	level := monitorapi.Info
	if obj.Type == corev1.EventTypeWarning {
		level = monitorapi.Warning
	}

	pathoFrom := obj.LastTimestamp.Time
	if pathoFrom.IsZero() {
		pathoFrom = obj.EventTime.Time
	}
	if pathoFrom.IsZero() {
		pathoFrom = obj.CreationTimestamp.Time
	}
	if pathoFrom.Before(significantlyBeforeNow) {
		if osEvent {
			logrus.Infof("OS update event filtered for being too old: %s - %s - %s",
				obj.Reason, obj.InvolvedObject.Name, obj.LastTimestamp.Format(time.RFC3339))
		}
		return
	}
	// We start with to equal to from, the majority of kube event intervals had this, and these get filtered out
	// when generating spyglass html. For interesting/pathological events, we're adding a second, which causes them
	// to get included in the html.
	// TODO: Probably should switch to using Display() and use a consistent time + 1s for the To.
	to := pathoFrom // we may override later for some events we want to have a duration and get charted
	locator := monitorapi.NewLocator().KubeEvent(obj)

	// TODO: kill it with fire
	// The matching here needs to mimic what is being done in the synthetictests/testDuplicatedEvents function.
	//eventDisplayMessage := fmt.Sprintf("%s - %s", event.Locator, event.Message)
	// TODO: accommodate upgrade events as well, and see if we need to pass kube config
	isInteresting, _ := pathologicaleventlibrary.MatchesAny(pathologicaleventlibrary.AllowedRepeatedEvents,
		locator, message.Build(), nil)
	/*
		isInteresting := allRepeatedEventPatterns.MatchString(eventDisplayMessage) ||
			checkAllowedRepeatedEventOKFns(adminRESTConfig, event, obj.Count)

	*/

	if obj.Count > 1 {

		if isInteresting {
			// This is a repeated event that we know about
			message = message.WithAnnotation(monitorapi.AnnotationInteresting, "true")
		}

		isPathological := obj.Count > pathologicaleventlibrary.DuplicateEventThreshold
		if isPathological {
			// This is a repeated event that exceeds threshold
			message = message.WithAnnotation(monitorapi.AnnotationPathological, "true")
		}

		to = pathoFrom.Add(1 * time.Second)
	} else if strings.Contains(obj.Message, "pod sandbox") {
		// TODO: gross hack until we port these to structured intervals. serialize.go EventsIntervalsToJSON will filter out
		// any intervals where from == to, which kube events normally do other than interesting/pathological above,
		// which is only applied to things with a count. to get pod sandbox intervals charted we have to get a little creative.
		// structured intervals should come here soon.

		// TODO: fake interesting flag, accomodate flagging this interesting in the new duplicated events patterns definitions
		message = message.WithAnnotation(monitorapi.AnnotationInteresting, "true")

		// make sure we add 1 second to the to timestamp so it doesn't get filtered out when creating the spyglass html
		to = pathoFrom.Add(1 * time.Second)
	}

	interval := monitorapi.NewInterval(monitorapi.SourceKubeEvent, level).
		Locator(locator).
		Message(message).Build(pathoFrom, to)

	logrus.WithField("event", *obj).Info("processed event")
	logrus.WithField("locator", interval.StructuredLocator).Info("resulting interval locator")
	logrus.WithField("message", interval.StructuredMessage).Info("resulting interval message")

	recorder.AddIntervals(interval)
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

// TODO decide whether we want to allow "random" locator keys.  deads2k is -1 on random locator keys and thinks we should enumerate every possible key we special case.
func locateEvent(event *corev1.Event) string {
	switch {
	case event.InvolvedObject.Kind == "Namespace":
		// namespace better match the event itself.
		return monitorapi.NewLocator().LocateNamespace(event.InvolvedObject.Name).OldLocator()

	case event.InvolvedObject.Kind == "Node":
		return monitorapi.NewLocator().NodeFromName(event.InvolvedObject.Name).OldLocator()

	case len(event.InvolvedObject.Namespace) == 0:
		if len(event.Source.Host) > 0 && event.Source.Component == "kubelet" {
			return fmt.Sprintf("%s/%s node/%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.Source.Host)
		}
		return fmt.Sprintf("%s/%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)

	default:
		// involved object is namespaced
		if len(event.Source.Host) > 0 && event.Source.Component == "kubelet" {
			return fmt.Sprintf("ns/%s %s/%s node/%s", event.InvolvedObject.Namespace, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.Source.Host)
		}
		return fmt.Sprintf("ns/%s %s/%s", event.InvolvedObject.Namespace, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
	}
}

func nodeRoles(node *corev1.Node) string {
	const roleLabel = "node-role.kubernetes.io"
	var roles []string
	for label := range node.Labels {
		if strings.Contains(label, roleLabel) {
			roles = append(roles, label[len(roleLabel)+1:])
		}
	}

	sort.Strings(roles)
	return strings.Join(roles, ",")
}
