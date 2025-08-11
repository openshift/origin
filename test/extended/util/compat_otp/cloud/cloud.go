package cloud

import (
	"context"
	"fmt"
	"io/ioutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/test/extended/util/compat_otp/azure"
)

// LoadConfig uses the cluster to setup the cloud provider config.
func LoadConfig() (string, *e2e.CloudConfig, error) {
	// LoadClientset but don't set the UserAgent to include the current test name because
	// we don't run any test yet and this call panics
	coreClient, err := e2e.LoadClientset(true)
	if err != nil {
		return "", nil, err
	}
	// LoadConfig but don't set the UserAgent to include the current test name because
	// we don't run any test yet and this call panics
	clientConfig, err := e2e.LoadConfig(true)
	if err != nil {
		return "", nil, err
	}
	client := configclient.NewForConfigOrDie(clientConfig)

	infra, err := client.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}
	p := infra.Status.PlatformStatus
	if p == nil {
		return "", nil, fmt.Errorf("status.platformStatus must be set")
	}
	if p.Type == configv1.NonePlatformType {
		return "", nil, nil
	}

	masters, err := coreClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=",
	})
	if err != nil {
		return "", nil, err
	}
	zones := sets.NewString()
	for _, node := range masters.Items {
		zones.Insert(node.Labels["failure-domain.beta.kubernetes.io/zone"])
	}
	zones.Delete("")

	nonMasters, err := coreClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "!node-role.kubernetes.io/master",
	})
	if err != nil {
		return "", nil, err
	}

	cloudConfig := &e2e.CloudConfig{
		MultiMaster: len(masters.Items) > 1,
		MultiZone:   zones.Len() > 1,
	}
	if zones.Len() > 0 {
		cloudConfig.Zone = zones.List()[0]
	}
	if len(nonMasters.Items) == 0 {
		cloudConfig.NumNodes = len(nonMasters.Items)
	} else {
		cloudConfig.NumNodes = len(masters.Items)
	}

	var provider string
	switch {
	case p.AWS != nil:
		provider = "aws"
		cloudConfig.Region = p.AWS.Region

	case p.GCP != nil:
		provider = "gce"
		cloudConfig.ProjectID = p.GCP.ProjectID
		cloudConfig.Region = p.GCP.Region

	case p.Azure != nil:
		provider = "azure"

		data, err := azure.LoadConfigFile()
		if err != nil {
			return "", nil, err
		}
		tmpFile, err := ioutil.TempFile("", "e2e-*")
		if err != nil {
			return "", nil, err
		}
		tmpFile.Close()
		if err := ioutil.WriteFile(tmpFile.Name(), data, 0600); err != nil {
			return "", nil, err
		}
		cloudConfig.ConfigFile = tmpFile.Name()
	}

	return provider, cloudConfig, nil
}
