package cloud

import (
	"fmt"
	"io/ioutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/test/extended/util/azure"
)

type ClusterConfiguration struct {
	ProviderName string

	// These fields chosen to match the e2e configuration we fill
	ProjectID       string
	Region          string
	Zone            string
	NumNodes        int
	MultiMaster     bool
	MultiZone       bool
	CloudConfigFile string

	NetworkPlugin string
}

// LoadConfig uses the cluster to setup the cloud provider config.
func LoadConfig(clientConfig *rest.Config) (*ClusterConfiguration, error) {
	// LoadClientset but don't set the UserAgent to include the current test name because
	// we don't run any test yet and this call panics
	coreClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	client, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	var networkPlugin string
	if networkConfig, err := client.ConfigV1().Networks().Get("cluster", metav1.GetOptions{}); err == nil {
		networkPlugin = networkConfig.Spec.NetworkType
	}

	infra, err := client.ConfigV1().Infrastructures().Get("cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	p := infra.Status.PlatformStatus
	if p == nil {
		return nil, fmt.Errorf("status.platformStatus must be set")
	}
	if p.Type == configv1.NonePlatformType {
		return &ClusterConfiguration{
			NetworkPlugin: networkPlugin,
		}, nil
	}

	masters, err := coreClient.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=",
	})
	if err != nil {
		return nil, err
	}
	zones := sets.NewString()
	for _, node := range masters.Items {
		zones.Insert(node.Labels["failure-domain.beta.kubernetes.io/zone"])
	}
	zones.Delete("")

	nonMasters, err := coreClient.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: "!node-role.kubernetes.io/master",
	})
	if err != nil {
		return nil, err
	}

	config := &ClusterConfiguration{
		MultiMaster:   len(masters.Items) > 1,
		MultiZone:     zones.Len() > 1,
		NetworkPlugin: networkPlugin,
	}
	if zones.Len() > 0 {
		config.Zone = zones.List()[0]
	}
	if len(nonMasters.Items) == 0 {
		config.NumNodes = len(nonMasters.Items)
	} else {
		config.NumNodes = len(masters.Items)
	}

	switch {
	case p.AWS != nil:
		config.ProviderName = "aws"
		config.Region = p.AWS.Region

	case p.GCP != nil:
		config.ProviderName = "gce"
		config.ProjectID = p.GCP.ProjectID
		config.Region = p.GCP.Region

	case p.Azure != nil:
		config.ProviderName = "azure"

		data, err := azure.LoadConfigFile()
		if err != nil {
			return nil, err
		}
		tmpFile, err := ioutil.TempFile("", "e2e-*")
		if err != nil {
			return nil, err
		}
		tmpFile.Close()
		if err := ioutil.WriteFile(tmpFile.Name(), data, 0600); err != nil {
			return nil, err
		}
		config.CloudConfigFile = tmpFile.Name()
	}

	return config, nil
}
