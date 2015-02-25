package server

import (
	"testing"

	"github.com/openshift/origin/pkg/cmd/util"
)

func TestMasterPublicAddressDefaulting(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetMasterPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestMasterPublicAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.MasterPublicAddr.Set(expected)

	actual, err := cfg.GetMasterPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressDefaultToKubernetesAddress(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.KubernetesAddr.Set(expected)
	cfg.MasterPublicAddr.Set("unexpectedpublicmaster")
	cfg.MasterAddr.Set("unexpectedmaster")

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressDefaultToPublicMasterAddress(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterPublicAddr.Set(expected)
	cfg.MasterAddr.Set("unexpectedmaster")

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressDefaultToMasterAddress(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.KubernetesAddr.Set("http://internal.com:9013")
	cfg.MasterPublicAddr.Set("http://internal.com:9014")
	cfg.KubernetesPublicAddr.Set(expected)

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesAddressDefaulting(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.KubernetesAddr.Set(expected)

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressDefaulting(t *testing.T) {
	expected := "http://example.com:4001"
	master := "https://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(master)

	actual, err := cfg.GetEtcdAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.EtcdAddr.Set(expected)

	actual, err := cfg.GetEtcdAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestMasterAddressDefaultingToBindValues(t *testing.T) {
	defaultIP, err := util.DefaultLocalIP4()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expected := "http://" + defaultIP.String() + ":9012"

	cfg := NewDefaultConfig()
	cfg.StartMaster = true
	cfg.BindAddr.Set("http://0.0.0.0:9012")

	actual, err := cfg.GetMasterAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestMasterAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetMasterAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}
