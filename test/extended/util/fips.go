package util

import (
	"context"
	"fmt"

	yaml "gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	installConfigName = "cluster-config-v1"
)

// installConfig The subset of openshift-install's InstallConfig we parse for this test
type installConfig struct {
	FIPS bool `json:"fips,omitempty"`
}

func IsFIPS(client corev1client.ConfigMapsGetter) (bool, error) {
	// this currently uses an install config because it has a lower dependency threshold than going directly to the node.
	installConfig, err := installConfigFromCluster(client)
	if err != nil {
		return false, err
	}
	return installConfig.FIPS, nil
}

func installConfigFromCluster(client corev1client.ConfigMapsGetter) (*installConfig, error) {
	cm, err := client.ConfigMaps("kube-system").Get(context.Background(), installConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	data, ok := cm.Data["install-config"]
	if !ok {
		return nil, fmt.Errorf("no install-config found in kube-system/%s", installConfigName)
	}
	config := &installConfig{}
	if err := yaml.Unmarshal([]byte(data), config); err != nil {
		return nil, err
	}
	return config, nil
}
