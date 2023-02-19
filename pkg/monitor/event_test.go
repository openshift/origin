package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_recordAddOrUpdateEvent(t *testing.T) {

	type KubeEventsItems struct {
		Items []corev1.Event `json:"items"`
	}

	// Get the kubeconfig by creating a file using the output of the KAAS tool or cluster-bot.
	// You can possible get panics due to timeout of KAAS kube configs.
	// Get the json file for all events from artifacts/gather-extra.  Or by using using
	// "oc -n aNamespace get event" after setting KUBECONFIG.
	kubeconfig := "/tmp/k.txt"
	jsonFile := "/tmp/kube_events.json"

	file, err := os.Open(jsonFile)
	if err != nil {
		fmt.Println("Error opening jsonFile:", err)
		return
	}
	defer file.Close()

	var kubeEvents KubeEventsItems
	if err := json.NewDecoder(file).Decode(&kubeEvents); err != nil {
		fmt.Println("Error reading jsonFile:", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Println("Unable to setup *rest.Config:", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Unable to setup kube client:", err)
	}

	smallKubeEvents := KubeEventsItems{
		Items: []corev1.Event{
			{
				Count:   2,
				Reason:  "NodeHasNoDiskPressure",
				Message: "sample message",
			},
		},
	}
	type args struct {
		ctx                    context.Context
		m                      Recorder
		client                 kubernetes.Interface
		reMatchFirstQuote      *regexp.Regexp
		significantlyBeforeNow time.Time
		kubeEventList          KubeEventsItems
	}

	tests := []struct {
		name string
		args args
		skip bool
	}{
		{
			name: "Single Event test",
			args: args{
				ctx:                    context.TODO(),
				m:                      NewMonitorWithInterval(time.Second),
				client:                 nil,
				reMatchFirstQuote:      regexp.MustCompile(`"([^"]+)"( in (\d+(\.\d+)?(s|ms)$))?`),
				significantlyBeforeNow: time.Now().UTC().Add(-15 * time.Minute),
				kubeEventList:          smallKubeEvents,
			},
		},
		{
			name: "Multiple Event (from file) test",
			skip: true, // skip in case we don't have a file
			args: args{
				ctx:               context.TODO(),
				m:                 NewMonitorWithInterval(time.Second),
				client:            client,
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
			}
		})
	}
}
