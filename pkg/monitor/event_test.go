package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// readTestFiles takes a filename of a kubeconfig file and events json file and returns
// raw events and a kubernetes.Clientset.  This is mostly used for interactive debugging.
// Get the kubeconfig by creating a file using the output of the KAAS tool or cluster-bot.
// NOTE: You can possible get panics due to timeout of KAAS kube configs.
// Get the json file for all events from artifacts/gather-extra.  Or by using using
// "oc -n aNamespace get event" after setting KUBECONFIG.
func readTestFiles(kubeconfig, jsonFile string) (corev1.EventList, *kubernetes.Clientset, error) {

	_, err := os.Stat(kubeconfig)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("File does not exist:", kubeconfig)
		return corev1.EventList{}, nil, err
	}
	_, err = os.Stat(jsonFile)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("File does not exist:", jsonFile)
		return corev1.EventList{}, nil, err
	}

	file, err := os.Open(jsonFile)
	if err != nil {
		fmt.Println("Error opening jsonFile:", err)
		return corev1.EventList{}, nil, err
	}
	defer file.Close()

	var kubeEvents corev1.EventList
	if err := json.NewDecoder(file).Decode(&kubeEvents); err != nil {
		fmt.Println("Error reading jsonFile")
		return corev1.EventList{}, nil, err
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println("Unable to setup *rest.Config:", err)
		return kubeEvents, nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Unable to setup kube client:", err)
		return kubeEvents, nil, err
	}

	return kubeEvents, clientSet, nil
}

func Test_recordAddOrUpdateEvent(t *testing.T) {

	// If we have files, we can use this to do a local debugging.
	// Without files, we use the single event tests.
	kubeEvents, clientSet, _ := readTestFiles("/tmp/test/k.txt", "/tmp/test/kube_events.json")
	smallKubeEvents := corev1.EventList{
		Items: []corev1.Event{
			{
				Count:         2,
				Reason:        "NodeHasNoDiskPressure",
				Message:       "sample message",
				LastTimestamp: metav1.Now(),
			},
		},
	}
	if len(kubeEvents.Items) == 0 {
		kubeEvents.Items = append(kubeEvents.Items, smallKubeEvents.Items...)
	}

	type args struct {
		ctx                    context.Context
		m                      *Monitor
		client                 kubernetes.Interface
		reMatchFirstQuote      *regexp.Regexp
		significantlyBeforeNow time.Time
		kubeEventList          corev1.EventList
	}

	now := time.Now()

	tests := []struct {
		name string
		args args
		want int
		skip bool
	}{
		{
			name: "Single Event test",
			args: args{
				ctx:                    context.TODO(),
				m:                      NewMonitorWithInterval(time.Second),
				client:                 nil,
				reMatchFirstQuote:      regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`),
				significantlyBeforeNow: now.UTC().Add(-15 * time.Minute),
				kubeEventList:          smallKubeEvents,
			},
			want: 1,
		},
		{
			name: "Multiple Event (from file) test",
			skip: true, // skip since we use this only for interactive debugging
			args: args{
				ctx:               context.TODO(),
				m:                 NewMonitorWithInterval(time.Second),
				client:            clientSet,
				reMatchFirstQuote: regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`),

				// Use the timestamp of the first corev1.Event
				significantlyBeforeNow: kubeEvents.Items[0].LastTimestamp.UTC().Add(-15 * time.Minute),
				kubeEventList:          kubeEvents,
			},
		},
	}
	for _, tt := range tests {
		if tt.skip {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			for _, event := range tt.args.kubeEventList.Items {
				recordAddOrUpdateEvent(tt.args.ctx, tt.args.m, tt.args.client, tt.args.reMatchFirstQuote, tt.args.significantlyBeforeNow, &event)
				len := len(tt.args.m.Intervals(now.Add(-10*time.Minute), now.Add(10*time.Minute)))
				if len != tt.want {
					t.Errorf("Wrong number of EventIntervals; got: %d expected: %d", len, tt.want)
				}
			}
		})
	}
}
