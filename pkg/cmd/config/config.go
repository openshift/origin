package config

import (
	"fmt"
	"os"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	oclient "github.com/openshift/origin/pkg/client"
)

func NewOpenShiftClient() *oclient.Client {
	cli, err := oclient.New(GetHost(), GetVersion(), GetAuthInfo())
	if err != nil {
		fmt.Errorf("Error: %s", err)
		os.Exit(1)
	}
	return cli
}

func NewKubeClient() *kclient.Client {
	cli, err := kclient.New(GetHost(), GetVersion(), GetAuthInfo())
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

func GetVersion() string {
	return ""
}

func GetAuthInfo() *kclient.AuthInfo {
	return nil
}
