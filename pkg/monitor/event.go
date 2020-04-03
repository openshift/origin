package monitor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

func startEventMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	reMatchFirstQuote := regexp.MustCompile(`"([^"]+)"`)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			events, err := client.CoreV1().Events("").List(metav1.ListOptions{Limit: 1})
			if err != nil {
				continue
			}
			rv := events.ResourceVersion

			for expired := false; !expired; {
				w, err := client.CoreV1().Events("").Watch(metav1.ListOptions{ResourceVersion: rv})
				if err != nil {
					if errors.IsResourceExpired(err) {
						break
					}
					continue
				}
				w = watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
					// TODO: gathering all events results in a 4x increase in e2e.log size, but is is
					//       valuable enough to gather that the cost is worth it
					// return in, filterToSystemNamespaces(in.Object)
					return in, true
				})
				func() {
					defer w.Stop()
					for event := range w.ResultChan() {
						switch event.Type {
						case watch.Added, watch.Modified:
							obj, ok := event.Object.(*corev1.Event)
							if !ok {
								continue
							}
							message := obj.Message
							if obj.Count > 1 {
								message += fmt.Sprintf(" (%d times)", obj.Count)
							}
							// special case some very common events
							switch obj.Reason {
							case "":
							case "Scheduled":
								if obj.InvolvedObject.Kind == "Pod" {
									if strings.HasPrefix(message, "Successfully assigned ") {
										if i := strings.Index(message, " to "); i != -1 {
											node := message[i+4:]
											message = fmt.Sprintf("node/%s reason/%s", node, obj.Reason)
											break
										}
									}
								}
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							case "Started", "Created", "Killing":
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
											message = fmt.Sprintf("container/%s reason/%s image/%s", containerName, obj.Reason, m[1])
											break
										}
									}
								}
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							default:
								message = fmt.Sprintf("reason/%s %s", obj.Reason, message)
							}
							condition := Condition{
								Level:   Info,
								Locator: locateEvent(obj),
								Message: message,
							}
							if obj.Type == corev1.EventTypeWarning {
								condition.Level = Warning
							}
							m.Record(condition)
						case watch.Error:
							var message string
							if status, ok := event.Object.(*metav1.Status); ok {
								if err := errors.FromObject(status); err != nil && errors.IsResourceExpired(err) {
									expired = true
									return
								}
								message = status.Message
							} else {
								message = fmt.Sprintf("event object was not a Status: %T", event.Object)
							}
							m.Record(Condition{
								Level:   Info,
								Locator: "kube-apiserver",
								Message: fmt.Sprintf("received an error while watching events: %s", message),
							})
							return
						default:
						}
					}
				}()
			}
		}
	}()
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
