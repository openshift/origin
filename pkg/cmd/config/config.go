package config

import (
	"fmt"
	"os"

	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/openshift/origin/pkg/client"
)

func NewClient() *client.Client {
	cli, err := client.New(GetHost(), GetVersion(), GetAuthInfo())
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

func GetAuthInfo() *kubeclient.AuthInfo {
	return nil
}
