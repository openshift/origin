// +build integration,!no-etcd

package integration

import (
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierror "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func TestAuthenticatedUsersAgainstOpenshiftNamespace(t *testing.T) {
	startConfig, err := StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, openshiftClientConfig, err := startConfig.GetOpenshiftClient()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	valerieClientConfig := *openshiftClientConfig
	valerieClientConfig.Username = ""
	valerieClientConfig.Password = ""
	valerieClientConfig.BearerToken = ""
	valerieClientConfig.CertFile = ""
	valerieClientConfig.KeyFile = ""
	valerieClientConfig.CertData = nil
	valerieClientConfig.KeyData = nil

	accessToken, err := tokencmd.RequestToken(&valerieClientConfig, nil, "valerie", "security!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valerieClientConfig.BearerToken = accessToken
	valerieOpenshiftClient, err := client.New(&valerieClientConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	openshiftSharedResourcesNamespace := "openshift"

	if _, err := valerieOpenshiftClient.Templates(openshiftSharedResourcesNamespace).List(labels.Everything(), labels.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.Templates(kapi.NamespaceDefault).List(labels.Everything(), labels.Everything()); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageRepositories(openshiftSharedResourcesNamespace).List(labels.Everything(), labels.Everything()); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageRepositories(kapi.NamespaceDefault).List(labels.Everything(), labels.Everything()); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := valerieOpenshiftClient.ImageRepositoryTags(openshiftSharedResourcesNamespace).Get("name", "tag"); !kapierror.IsNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := valerieOpenshiftClient.ImageRepositoryTags(kapi.NamespaceDefault).Get("name", "tag"); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("unexpected error: %v", err)
	}
}
