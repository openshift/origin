package clusterup

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/assets"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/controlplane-operator"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/bootkube"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/etcd"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/kubelet"
)

func (c *ClusterUpConfig) StartSelfHosted(out io.Writer) error {
	// BuildConfig will:
	// 1. Render the TLS files (certicates/secrets/kubeconfig/etc.)
	// 2. Render the bootstrap (phase 1) control plane manifests (kube api server, controller manager and scheduler)
	// 3. Render the phase 2 control plane manifests
	configDirs, err := c.BuildConfig()
	if err != nil {
		return err
	}

	fmt.Fprintf(c.Out, "Starting self-hosted OpenShift cluster ...")

	dockerRoot, err := c.Docker().DockerRoot()
	if err != nil {
		return err
	}

	kubeletConfig := kubelet.NewKubeletRunConfig()
	kubeletConfig.HostPersistentVolumesDir = c.HostPersistentVolumesDir
	kubeletConfig.HostVolumesDir = c.HostVolumesDir
	kubeletConfig.DockerRoot = dockerRoot
	kubeletConfig.UseNsenterMount = true
	kubeletConfig.NodeImage = OpenShiftImages.Get("node").ToPullSpec(c.ImageTemplate).String()
	kubeletConfig.PodImage = OpenShiftImages.Get("pod").ToPullSpec(c.ImageTemplate).String()

	if _, err := kubeletConfig.StartKubelet(c.DockerClient(), configDirs.podManifestDir, configDirs.assetsDir, c.BaseDir); err != nil {
		return err
	}

	etcdCmd := &etcd.EtcdConfig{
		Image:           OpenShiftImages.Get("etcd").ToPullSpec(c.ImageTemplate).String(),
		ImagePullPolicy: c.pullPolicy,
		StaticPodDir:    configDirs.podManifestDir,
		TlsDir:          filepath.Join(configDirs.assetsDir, "tls"),
		EtcdDataDir:     c.HostDataDir,
	}
	if err := etcdCmd.Start(); err != nil {
		return err
	}

	glog.Info("Bootkube phase-1 kube-apiserver is ready. Going to call bootkube start ...")
	bk := &bootkube.BootkubeRunConfig{
		BootkubeImage:        OpenShiftImages.Get("bootkube").ToPullSpec(c.ImageTemplate).String(),
		StaticPodManifestDir: configDirs.podManifestDir,
		AssetsDir:            configDirs.assetsDir,
		ContainerBinds: []string{
			fmt.Sprintf("%s:/etc/kubernetes:z", filepath.Join(c.BaseDir, "kubernetes")),
		},
	}
	if _, err := bk.RunStart(c.DockerClient()); err != nil {
		return err
	}

	clientConfigBuilder, err := kclientcmd.LoadFromFile(filepath.Join(c.BaseDir, "assets", "auth", "admin.kubeconfig"))
	if err != nil {
		return err
	}

	overrides := &kclientcmd.ConfigOverrides{}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*clientConfigBuilder, overrides)
	clientConfig, err := defaultCfg.ClientConfig()
	if err != nil {
		return err
	}

	clientConfig.Host = c.ServerIP + ":6443"

	glog.Info("Waiting for bootkube phase-2 kubernetes control plane to be ready ...")
	if err := waitForHealthyKubeAPIServer(clientConfig); err != nil {
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
		assetsDir: filepath.Join(c.BaseDir, "assets"),
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
	if _, err := os.Stat(filepath.Join(configs.kubernetesDir, "lock")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(configs.kubernetesDir, "lock"), 0777); err != nil {
			return nil, err
		}
	}
	// We need to make the directory world writeable to bypass selinux
	if err := os.Chmod(filepath.Join(configs.kubernetesDir, "lock"), 0777); err != nil {
		return nil, err
	}

	// If --public-hostname is specified, use that instead of 127.0.0.1
	hostIP, err := c.determineIP()
	if err != nil {
		return nil, err
	}

	certs, err := assets.NewTLSAssetsRenderer(c.GetPublicHostName()).Render()
	if err != nil {
		return nil, err
	}
	if err := certs.WriteFiles(filepath.Join(configs.assetsDir)); err != nil {
		return nil, err
	}

	// prepare config
	configDir := filepath.Join(configs.assetsDir, "config")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, err
		}
	}

	// Overrides allow to tweak the api server config
	apiserverConfigOverride := filepath.Join(configDir, "kube-apiserver-config-overrides.yaml")
	if err := ioutil.WriteFile(apiserverConfigOverride,
		[]byte(`apiVersion: kubecontrolplane.config.openshift.io/v1
kind: KubeAPIServerConfig
`), 0644); err != nil {
		return nil, err
	}

	// Overrides allow to tweak the controller manager config
	controllerManagerConfigOverride := filepath.Join(configDir, "kube-controller-manager-config-overrides.yaml")
	if err := ioutil.WriteFile(controllerManagerConfigOverride,
		[]byte(`apiVersion: kubecontrolplane.config.openshift.io/v1
kind: KubeControllerManagerConfig
`), 0644); err != nil {
		return nil, err
	}

	// generate kube-apiserver manifests using the corresponding operator render command
	apiserverConfig := controlplaneoperator.RenderConfig{
		OperatorImage:   OpenShiftImages.Get("cluster-kube-apiserver-operator").ToPullSpec(c.ImageTemplate).String(),
		AssetInputDir:   filepath.Join(configs.assetsDir, "tls"),
		AssetsOutputDir: configs.assetsDir,
		ConfigOutputDir: configDir,
		ConfigFileName:  "kube-apiserver-config.yaml",
		ConfigOverrides: apiserverConfigOverride,
		AdditionalFlags: []string{
			fmt.Sprintf("--manifest-etcd-server-urls=https://127.0.0.1:2379"),
			fmt.Sprintf("--manifest-lock-host-path=%s", filepath.Join(configs.kubernetesDir, "lock")),
		},
	}
	if _, err := apiserverConfig.RunRender("kube-apiserver", OpenShiftImages.Get("hypershift").ToPullSpec(c.ImageTemplate).String(), c.DockerClient(), hostIP); err != nil {
		return nil, err
	}

	// generate kube-controller-manager manifests using the corresponding operator render command
	// TODO: This will also render checkpointer manifests. Those should be owned by their operators but for now they are owned
	//       by controller manager operator.
	controllerConfig := controlplaneoperator.RenderConfig{
		OperatorImage:   OpenShiftImages.Get("cluster-kube-controller-manager-operator").ToPullSpec(c.ImageTemplate).String(),
		AssetInputDir:   filepath.Join(configs.assetsDir, "tls"),
		AssetsOutputDir: configs.assetsDir,
		ConfigOutputDir: configDir,
		ConfigFileName:  "kube-controller-manager-config.yaml",
		ConfigOverrides: controllerManagerConfigOverride,
	}
	if _, err := controllerConfig.RunRender("kube-controller-manager", OpenShiftImages.Get("hyperkube").ToPullSpec(c.ImageTemplate).String(), c.DockerClient(), hostIP); err != nil {
		return nil, err
	}

	// generate kube-scheduler manifests using the corresponding operator render command
	schedulerConfig := controlplaneoperator.RenderConfig{
		OperatorImage:   OpenShiftImages.Get("cluster-kube-scheduler-operator").ToPullSpec(c.ImageTemplate).String(),
		AssetInputDir:   filepath.Join(configs.assetsDir, "tls"),
		AssetsOutputDir: configs.assetsDir,
		ConfigOutputDir: configDir,
		ConfigFileName:  "kube-scheduler-config.yaml",
		ConfigOverrides: controllerManagerConfigOverride,
	}
	if _, err := schedulerConfig.RunRender("kube-scheduler", OpenShiftImages.Get("hyperkube").ToPullSpec(c.ImageTemplate).String(), c.DockerClient(), hostIP); err != nil {
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
