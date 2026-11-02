package auth_proxy

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubefake "k8s.io/client-go/kubernetes/fake"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func TestClusterProxyRequiresClient(t *testing.T) {
	t.Parallel()
	client := kubefake.NewSimpleClientset()
	_, err := ClusterProxy(context.Background(), nil, client.CoreV1())
	if err == nil || !strings.Contains(err.Error(), "proxies client must not be nil") {
		t.Fatalf("error = %v, want missing proxies client", err)
	}
	if len(client.Actions()) != 0 {
		t.Error("invalid arguments caused API access")
	}
}

func TestDeploymentProxyConfigValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		names          [3]string
		selector       corev1.ConfigMapKeySelector
		nilDeployments bool
		wantErr        string
	}{
		{name: "nil deployments", names: [3]string{"ns", "deployment", "container"}, nilDeployments: true, wantErr: "deployments client must not be nil"},
		{name: "empty namespace", names: [3]string{"", "deployment", "container"}, wantErr: "must be non-empty"},
		{name: "empty deployment", names: [3]string{"ns", "", "container"}, wantErr: "must be non-empty"},
		{name: "empty container", names: [3]string{"ns", "deployment", ""}, wantErr: "must be non-empty"},
		{name: "blank namespace", names: [3]string{" ", "deployment", "container"}, wantErr: "must be non-empty"},
		{name: "selector key only", names: [3]string{"ns", "deployment", "container"}, selector: corev1.ConfigMapKeySelector{Key: "ca.crt"}, wantErr: "must provide both name and key"},
		{
			name: "selector name only", names: [3]string{"ns", "deployment", "container"},
			selector: corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "proxy-ca"}}, wantErr: "must provide both name and key",
		},
		{
			name: "blank selector key", names: [3]string{"ns", "deployment", "container"},
			selector: corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "proxy-ca"}, Key: " "}, wantErr: "must provide both name and key",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := kubefake.NewSimpleClientset()
			var deployments appsclientv1.DeploymentsGetter = client.AppsV1()
			if tc.nilDeployments {
				deployments = nil
			}
			_, err := DeploymentProxyConfig(context.Background(), tc.names[0], tc.names[1], tc.names[2], tc.selector, deployments, client.CoreV1())
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want message containing %q", err, tc.wantErr)
			}
			if len(client.Actions()) != 0 {
				t.Error("invalid arguments caused API access")
			}
		})
	}
}

func TestDefaultCollectionsAreIndependent(t *testing.T) {
	t.Parallel()
	noProxy := DefaultNoProxy()
	noProxy.Insert("changed.example")
	noProxy.Delete("localhost")
	if got := DefaultNoProxy(); !got.Equal(sets.New(".cluster.local", ".svc", "127.0.0.1", "localhost")) {
		t.Errorf("modifying a returned set changed the defaults: %v", got)
	}
	labels := ComponentProxyTestLabels()
	labels["e2e-test"] = "changed"
	if got := ComponentProxyTestLabels()["e2e-test"]; got != "openshift-authentication-operator" {
		t.Errorf("modifying returned labels changed the defaults: %q", got)
	}
	config := EmptyProxyConfig()
	config.NoProxy.Insert("changed.example")
	if got := EmptyProxyConfig(); got.HasProxy() || len(got.UpstreamCACertPEM) != 0 || got.NoProxy.Len() != 0 {
		t.Errorf("modifying an empty config changed subsequent configs: %v", got)
	}
}
