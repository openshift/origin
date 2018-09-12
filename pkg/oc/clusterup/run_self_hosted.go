package clusterup

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/bootkube"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/etcd"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubelet"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
)

func (c *ClusterUpConfig) StartSelfHosted(out io.Writer) error {
	configDirs, err := c.BuildConfig()
	if err != nil {
		return err
	}

	dockerRoot, err := c.DockerHelper().DockerRoot()
	if err != nil {
		return err
	}

	// if we're supposed to write the config, we'll do that and then exit
	if c.WriteConfig {
		fmt.Printf("Wrote config to: %q\n", c.BaseDir)
		return nil
	}

	kubeletConfig := kubelet.NewKubeletRunConfig()
	kubeletConfig.HostPersistentVolumesDir = c.HostPersistentVolumesDir
	kubeletConfig.HostVolumesDir = c.HostVolumesDir
	kubeletConfig.HTTPProxy = c.HTTPProxy
	kubeletConfig.HTTPSProxy = c.HTTPSProxy
	kubeletConfig.NoProxy = c.NoProxy
	kubeletConfig.DockerRoot = dockerRoot
	kubeletConfig.UseNsenterMount = c.UseNsenterMount
	kubeletConfig.NodeImage = c.hyperkubeImage()
	kubeletConfig.PodImage = c.podImage()

	if _, err := kubeletConfig.StartKubelet(c.GetDockerClient(), configDirs.podManifestDir, configDirs.assetsDir, c.BaseDir); err != nil {
		return err
	}

	etcdCmd := &etcd.EtcdConfig{
		EtcdImage:      c.etcdImage(),
		AssetsDir:      configDirs.assetsDir,
		EtcdDataDir:    c.HostDataDir,
		ContainerBinds: []string{},
	}
	if _, err := etcdCmd.Start(c.GetDockerClient()); err != nil {
		return err
	}

	bk := &bootkube.BootkubeRunConfig{
		BootkubeImage:        c.bootkubeImage(),
		StaticPodManifestDir: configDirs.podManifestDir,
		AssetsDir:            configDirs.assetsDir,
		ContainerBinds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", filepath.Join(c.BaseDir, "kubernetes")),
			fmt.Sprintf("%s:/static-pod-manifests:z", filepath.Join(c.BaseDir, "static-pod-manifests")),
		},
	}

	if _, err := bk.RunStart(c.GetDockerClient()); err != nil {
		return err
	}

	clientConfigBuilder, err := kclientcmd.LoadFromFile(filepath.Join(c.BaseDir, "bootkube", "auth", "kubeconfig"))
	if err != nil {
		return err
	}

	overrides := &kclientcmd.ConfigOverrides{}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*clientConfigBuilder, overrides)
	clientConfig, err := defaultCfg.ClientConfig()
	if err != nil {
		return err
	}

	clientConfig.Host = c.ServerIP + ":8443"

	glog.Info("Waiting for the kubernetes control plane to be ready ...")
	if err := waitForHealthyKubeAPIServer(clientConfig); err != nil {
		return err
	}

	// If we're only supposed to install kubernetes, don't install anything else
	if c.KubeOnly {
		return nil
	}

	return nil
}

type configDirs struct {
	podManifestDir string
	assetsDir      string
	kubernetesDir  string
}

func (c *ClusterUpConfig) BuildConfig() (*configDirs, error) {
	configs := &configDirs{
		// Directory where bootkube renders the static pod manifests
		assetsDir: filepath.Join(c.BaseDir, "bootkube"),
		// Directory that kubelet scans for static manifests
		podManifestDir: filepath.Join(c.BaseDir, "static-pod-manifests"),
		// Directory where bootkube copy the bootstrap secrets
		kubernetesDir: filepath.Join(c.BaseDir, "kubernetes"),
	}

	if _, err := os.Stat(configs.assetsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configs.assetsDir, 0755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.kubernetesDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configs.kubernetesDir, 0755); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.podManifestDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configs.podManifestDir, 0755); err != nil {
			return nil, err
		}
	}

	bk := bootkube.BootkubeRunConfig{
		BootkubeImage:        c.bootkubeImage(),
		StaticPodManifestDir: configs.podManifestDir,
		AssetsDir:            configs.assetsDir,
		ContainerBinds:       []string{},
	}

	// If --public-hostname is specified, use that instead of 127.0.0.1
	hostIP, err := c.determineIP()
	if err != nil {
		return nil, err
	}

	if _, err := bk.RunRender(c.GetDockerClient(), hostIP); err != nil {
		return nil, err
	}

	if err := bk.PostRenderSubstitutions(configs.kubernetesDir, c.hyperkubeImage(), c.nodeImage()); err != nil {
		return nil, err
	}

	return configs, nil
}

func waitForHealthyKubeAPIServer(clientConfig *rest.Config) error {
	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	var lastResponseError error
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
		if err != nil {
			return false, nil
		}

		healthStatus := 0
		resp := discoveryClient.RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if resp.Error() != nil {
			glog.V(4).Infof("Server isn't healthy yet.  Waiting a little while. %v", resp.Error())
			lastResponseError = resp.Error()
			return false, nil
		}
		content, _ := resp.Raw()
		healthzContent = string(content)
		if healthStatus != http.StatusOK {
			glog.V(4).Infof("Server isn't healthy yet.  Waiting a little while. %v", healthStatus)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		glog.Errorf("API server error: %v (%s)", lastResponseError, healthzContent)
	}

	return err
}
