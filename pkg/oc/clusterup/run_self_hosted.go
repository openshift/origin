package clusterup

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/controlplane-operator"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/bootkube"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/etcd"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubelet"
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
		Image:           c.etcdImage(),
		ImagePullPolicy: c.pullPolicy,
		StaticPodDir:    configDirs.podManifestDir,
		TlsDir:          filepath.Join(configDirs.assetsDir, "master"),
		EtcdDataDir:     c.HostDataDir,
	}
	if err := etcdCmd.Start(); err != nil {
		return err
	}

	bk := &bootkube.BootkubeRunConfig{
		BootkubeImage:        c.bootkubeImage(),
		StaticPodManifestDir: configDirs.podManifestDir,
		AssetsDir:            configDirs.assetsDir,
		ContainerBinds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", filepath.Join(c.BaseDir, "kubernetes")),
		},
	}
	glog.Info("Bootkube phase-1 kube-apiserver is ready. Going to call bootkube start ...")

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

	glog.Info("Waiting for bootkube phase-2 kubernetes control plane to be ready ...")
	if err := waitForHealthyKubeAPIServer(clientConfig); err != nil {
		return err
	}
	glog.Info("Bootkube phase-2 kube-apiserver is ready. Going to start operators ...")

	/***************************************************************************************/
	/* Everything below is legacy bootstrapping of components, to be replaced by operators */
	/***************************************************************************************/

	// If we're only supposed to install kubernetes, don't install anything else
	if c.KubeOnly {
		return nil
	}

	templateSubstitutionValues := map[string]string{
		//"MASTER_CONFIG_HOST_PATH":                       configDirs.masterConfigDir,
		//"OPENSHIFT_APISERVER_CONFIG_HOST_PATH":          configDirs.openshiftAPIServerConfigDir,
		//"OPENSHIFT_CONTROLLER_MANAGER_CONFIG_HOST_PATH": configDirs.openshiftControllerConfigDir,
		//"NODE_CONFIG_HOST_PATH":                         configDirs.nodeConfigDir,
		//"KUBEDNS_CONFIG_HOST_PATH":                      configDirs.kubeDNSConfigDir,
		"OPENSHIFT_PULL_POLICY": c.pullPolicy,
		"LOGLEVEL":              fmt.Sprintf("%d", c.ServerLogLevel),
	}

	runLevelOneComponents := append([]componentInstallTemplate{}, runLevelOneKubeComponents...)
	if !c.KubeOnly {
		//runLevelOneComponents = append(runLevelOneComponents, runLevelOneOpenShiftComponents...)
	}
	err = installComponentTemplates(
		runLevelOneComponents,
		c.ImageTemplate.Format,
		c.BaseDir,
		templateSubstitutionValues,
		c.GetDockerClient(),
	)
	if err != nil {
		return err
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
		// Directory where assets ared rendered to
		assetsDir: filepath.Join(c.BaseDir, "bootkube"),
		// Directory where bootkube copy the bootstrap secrets
		kubernetesDir: filepath.Join(c.BaseDir, "kubernetes"),
		// Directory that kubelet scans for static manifests
		podManifestDir: filepath.Join(c.BaseDir, "kubernetes/manifests"),
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

	if err := bk.RemoveApiserver(configs.kubernetesDir); err != nil {
		return nil, err
	}

	// LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY LEGACY
	// TRANSITION TRANSITION TRANSITION TRANSITION TRANSITION TRANSITION TRANSITION TRANSITION

	// copy bootkube-render files to operatpr render input dir, simulating what c.makeMasterConfig would generate
	// TODO: generate tls files without bootkube-render
	masterDir := filepath.Join(configs.assetsDir, "master")
	legacyBootkubeMapping := map[string]string{
		"ca.crt":                     path.Join(configs.assetsDir, "tls", "ca.crt"),
		"admin.crt":                  path.Join(configs.assetsDir, "tls", "admin.crt"),
		"admin.key":                  path.Join(configs.assetsDir, "tls", "admin.key"),
		"openshift-master.crt":       path.Join(configs.assetsDir, "tls", "admin.crt"),
		"openshift-master.key":       path.Join(configs.assetsDir, "tls", "admin.key"),
		"master.server.crt":          path.Join(configs.assetsDir, "tls", "apiserver.crt"),
		"master.server.key":          path.Join(configs.assetsDir, "tls", "apiserver.key"),
		"master.etcd-client-ca.crt":  path.Join(configs.assetsDir, "tls", "etcd-client-ca.crt"), // this does not exist in legacy cluster-up, but might be necessary for etcd access
		"master.etcd-client.crt":     path.Join(configs.assetsDir, "tls", "etcd-client.crt"),
		"master.etcd-client.key":     path.Join(configs.assetsDir, "tls", "etcd-client.key"),
		"serviceaccounts.public.key": path.Join(configs.assetsDir, "tls", "service-account.pub"),
		"frontproxy-ca.crt":          path.Join(configs.assetsDir, "tls", "apiserver.crt"), // this does not exist in bootkube, but might be necessary for aggregated apiserver authn
		"openshift-aggregator.crt":   path.Join(configs.assetsDir, "tls", "apiserver.crt"), // this does not exist in bootkube, but might be necessary for aggregated apiserver authn
		"openshift-aggregator.key":   path.Join(configs.assetsDir, "tls", "apiserver.key"), // this does not exist in bootkube, but might be necessary for aggregated apiserver authn
		"master.kubelet-client.crt":  path.Join(configs.assetsDir, "tls", "apiserver.crt"),
		"master.kubelet-client.key":  path.Join(configs.assetsDir, "tls", "apiserver.key"),
		"master.proxy-client.crt":    path.Join(configs.assetsDir, "tls", "apiserver.crt"),
		"master.proxy-client.key":    path.Join(configs.assetsDir, "tls", "apiserver.key"),
	}
	if _, err := os.Stat(masterDir); os.IsNotExist(err) {
		if err := os.MkdirAll(masterDir, 0755); err != nil {
			return nil, err
		}
	}
	for legacy, bootkubeFile := range legacyBootkubeMapping {
		dest := path.Join(masterDir, legacy)
		if err := admin.CopyFile(bootkubeFile, dest, 0644); err != nil {
			return nil, fmt.Errorf("failed to copy bootkube tls file %q to %q: %v", bootkubeFile, dest, err)
		}
	}

	// create initial configs
	apiserverConfigOverride := filepath.Join(masterDir, "kube-apiserver-config-overrides.yaml")
	if err := ioutil.WriteFile(apiserverConfigOverride,
		[]byte(`apiVersion: kubecontrolplane.config.openshift.io/v1
kind: KubeAPIServerConfig
`), 0644); err != nil {
		return nil, err
	}

	// generate kube-apiserver manifests using the corresponding operator render command
	ok := controlplaneoperator.RenderConfig{
		OperatorImage:   "openshift/origin-cluster-kube-apiserver-operator:latest",
		AssetInputDir:   masterDir,
		AssetsOutputDir: configs.assetsDir,
		ConfigOutputDir: masterDir, // we put config, overrides and certs+keys in one dir
		ConfigFileName:  "kube-apiserver-config.yaml",
		ConfigOverrides: apiserverConfigOverride,
		ContainerBinds:  nil,
	}
	if _, err := ok.RunRender("kube-apiserver", c.hypershiftImage(), c.GetDockerClient(), hostIP); err != nil {
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
