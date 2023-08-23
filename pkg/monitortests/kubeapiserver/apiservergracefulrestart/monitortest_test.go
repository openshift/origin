package apiservergracefulrestart

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func TestBuilder(t *testing.T) {
	oldLocator := "node/node-name ns/openshift-kube-apiserver pod/pod-name"
	podRef := monitorapi.PodFrom(oldLocator)
	nodeName, _ := monitorapi.NodeFromLocator(oldLocator)

	interval := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Info).
		Locator(monitorapi.NewLocator().
			LocateServer(namespaceToServer[podRef.Namespace], nodeName, podRef.Namespace, podRef.Name, true),
		).
		Message(monitorapi.NewMessage().
			Constructed("graceful-shutdown-analyzer").
			Reason(monitorapi.GracefulAPIServerShutdown),
		).
		Display().
		Build(time.Time{}, time.Time{})

	if interval.Locator != "namespace/openshift-kube-apiserver node/node-name pod/pod-name server/kube-apiserver shutdown/apiserver" {
		t.Fatal(interval.Locator)
	}
}
