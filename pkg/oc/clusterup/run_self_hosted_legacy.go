package clusterup

import (
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/oc/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/util"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

type componentInstallTemplate struct {
	ComponentImage string
	Template       componentinstall.Template
}

var (
	runlevelZeroLabel         = map[string]string{"openshift.io/run-level": "0"}
	runlevelOneLabel          = map[string]string{"openshift.io/run-level": "1"}
	runLevelOneKubeComponents = []componentInstallTemplate{
		{
			ComponentImage: "cluster-kube-apiserver-operator",
			Template: componentinstall.Template{
				Name:            "openshift-kube-apiserver-operator",
				Namespace:       "openshift-core-operators",
				NamespaceObj:    newNamespaceBytes("openshift-core-operators", runlevelZeroLabel),
				InstallTemplate: manifests.MustAsset("install/cluster-kube-apiserver-operator/install.yaml"),
			},
		},
		{
			ComponentImage: "service-serving-cert-signer",
			Template: componentinstall.Template{
				Name:            "openshift-service-cert-signer-operator",
				Namespace:       "openshift-core-operators",
				NamespaceObj:    newNamespaceBytes("openshift-core-operators", runlevelOneLabel),
				RBACTemplate:    manifests.MustAsset("install/openshift-service-cert-signer-operator/install-rbac.yaml"),
				InstallTemplate: manifests.MustAsset("install/openshift-service-cert-signer-operator/install.yaml"),
			},
		},
	}
	runLevelOneOpenShiftComponents = []componentInstallTemplate{
		{
			ComponentImage: "hypershift",
			Template: componentinstall.Template{
				Name:            "openshift-apiserver",
				Namespace:       "openshift-apiserver",
				NamespaceObj:    newNamespaceBytes("openshift-apiserver", runlevelOneLabel),
				InstallTemplate: manifests.MustAsset("install/openshift-apiserver/install.yaml"),
			},
		},
	}

	// componentsToInstall DOES NOT INSTALL IN ORDER.  They are installed separately and expected to come up
	// in any order and self-organize into something that works.  Remember, when the whole system crashes and restarts
	// you don't get to choose your restart order.  Plan accordingly.  No bugs or attempts at interlocks will be accepted
	// in cluster up.
	// TODO we can take a guess at readiness by making sure that pods in the namespace exist and all pods are healthy
	// TODO it's not perfect, but its fairly good as a starting point.
	componentsToInstall = []componentInstallTemplate{
		{
			ComponentImage: "hypershift",
			Template: componentinstall.Template{
				Name:              "openshift-controller-manager",
				Namespace:         "openshift-controller-manager",
				NamespaceObj:      newNamespaceBytes("openshift-controller-manager", nil),
				PrivilegedSANames: []string{"openshift-controller-manager"},
				RBACTemplate:      manifests.MustAsset("install/openshift-controller-manager/install-rbac.yaml"),
				InstallTemplate:   manifests.MustAsset("install/openshift-controller-manager/install.yaml"),
			},
		},
	}
)

// makeMasterConfig returns the directory where a generated masterconfig lives
func (c *ClusterUpConfig) makeMasterConfig() (string, error) {
	publicHost := c.GetPublicHostName()

	container := kubeapiserver.NewKubeAPIServerStartConfig()
	container.MasterImage = OpenShiftImages.Get("control-plane").ToPullSpec(c.ImageTemplate).String()
	container.Args = []string{
		"--write-config=/var/lib/origin/openshift.local.config",
		fmt.Sprintf("--master=%s", c.ServerIP),
		fmt.Sprintf("--images=%s", c.imageFormat()),
		fmt.Sprintf("--dns=0.0.0.0:%d", c.DNSPort),
		fmt.Sprintf("--public-master=https://%s:8443", publicHost),
		"--etcd-dir=/var/lib/etcd",
	}

	masterConfigDir, err := container.MakeMasterConfig(c.DockerClient(), path.Join(c.BaseDir, "legacy"))
	if err != nil {
		return "", fmt.Errorf("error creating master config: %v", err)
	}

	return masterConfigDir, nil
}

func newNamespaceBytes(namespace string, labels map[string]string) []byte {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: labels}}
	output, err := kruntime.Encode(legacyscheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion), ns)
	if err != nil {
		// coding error
		panic(err)
	}
	return output
}

func installComponentTemplates(templates []componentInstallTemplate, imageFormat, baseDir string, params map[string]string, dockerClient util.Interface) error {
	components := []componentinstall.Component{}
	cliImage := strings.Replace(imageFormat, "${component}", "cli", -1)
	for _, template := range templates {
		paramsWithImage := make(map[string]string)
		for k, v := range params {
			paramsWithImage[k] = v
		}
		if len(template.ComponentImage) > 0 {
			paramsWithImage["IMAGE"] = strings.Replace(imageFormat, "${component}", template.ComponentImage, -1)
		}

		components = append(components, template.Template.MakeReady(cliImage, baseDir, paramsWithImage))
	}

	return componentinstall.InstallComponents(components, dockerClient)
}
