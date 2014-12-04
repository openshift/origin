package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	ktools "github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/auth"

	// instantiators that need initing
	_ "github.com/openshift/origin/pkg/cmd/auth/challengehandlers/basic"
	_ "github.com/openshift/origin/pkg/cmd/auth/granthandlers/auto"
	_ "github.com/openshift/origin/pkg/cmd/auth/granthandlers/empty"
	_ "github.com/openshift/origin/pkg/cmd/auth/granthandlers/prompt"
	_ "github.com/openshift/origin/pkg/cmd/auth/identitymappers/autocreate"
	_ "github.com/openshift/origin/pkg/cmd/auth/passwordauthenticators/basic"
	_ "github.com/openshift/origin/pkg/cmd/auth/passwordauthenticators/empty"
	_ "github.com/openshift/origin/pkg/cmd/auth/redirecthandlers/empty"
	_ "github.com/openshift/origin/pkg/cmd/auth/redirecthandlers/login"
	_ "github.com/openshift/origin/pkg/cmd/auth/redirecthandlers/oauth"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/basic"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/bearer"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/session"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/xremoteuser"
	_ "github.com/openshift/origin/pkg/cmd/auth/tokenauthenticators/csv"
	_ "github.com/openshift/origin/pkg/cmd/auth/tokenauthenticators/etcd"
)

func NewTestEtcd(client ktools.EtcdClient) ktools.EtcdHelper {
	return ktools.EtcdHelper{client, latest.Codec, ktools.RuntimeVersionAdapter{latest.ResourceVersioner}}
}

func TestExampleConfig(t *testing.T) {
	envInfo := &auth.EnvInfo{"masteraddr", nil, nil, NewTestEtcd(ktools.NewFakeEtcdClient(t)), http.NewServeMux()}

	jsonBytes, err := ioutil.ReadFile("auth-config.json")
	if err != nil {
		t.Errorf("Failed to read file due to %v", err)
	}

	var configInfo auth.AuthConfigInfo
	err = json.Unmarshal(jsonBytes, &configInfo)
	if err != nil {
		t.Errorf("Failed to parse authConfig: %v", err)
	}

	fmt.Printf("AuthConfigInfo is\n%#v\n", configInfo)

	authConfig, err := auth.InstantiateAuthConfig(configInfo, envInfo)
	if err != nil {
		t.Errorf("Failed to instantiate: %v", err)
	}

	fmt.Printf("AuthConfig is\n%#v\n", authConfig)
}
