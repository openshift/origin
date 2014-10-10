package config

import (
	"fmt"
	"os"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	oclient "github.com/openshift/origin/pkg/client"
)

func NewOpenShiftClient() *oclient.Client {
	config := &kclient.Config{
		Host:     GetHost(),
		Username: "",
		Password: "",
	}
	cli, err := oclient.New(config)
	if err != nil {
		fmt.Errorf("Error: %s", err)
		os.Exit(1)
	}
	return cli
}

func NewKubeClient() *kclient.Client {
	config := &kclient.Config{
		Host:     GetHost(),
		Username: "",
		Password: "",
	}
	cli, err := kclient.New(config)
	if err != nil {
		fmt.Errorf("Error: %s", err)
		os.Exit(1)
	}
	return cli
}

func GetHost() string {
	if hostEnv := os.Getenv("OPENSHIFT_SERVER"); len(hostEnv) > 0 {
		return hostEnv
	} else {
		return "http://localhost:8080"
	}
}
