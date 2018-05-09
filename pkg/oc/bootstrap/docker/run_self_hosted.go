package docker

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubelet"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/staticpods"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/host"

	// install our apis into the legacy scheme
	_ "github.com/openshift/origin/pkg/api/install"
)

type staticInstall struct {
	Location       string
	ComponentImage string
}

type componentInstallTemplate struct {
	ComponentImage string
	Template       componentinstall.Template
}

var (
	// staticPodInstalls should only include those pods that *must* be run statically because they
	// bring up the services required to run the workload controllers.
	// etcd, kube-apiserver, kube-controller-manager, kube-scheduler (this is because sig-scheduling is expanding the scheduler responsibilities)
	staticPodInstalls = []staticInstall{
		{
			Location:       "install/etcd/etcd.yaml",
			ComponentImage: "control-plane",
		},
		{
			Location:       "install/kube-apiserver/apiserver.yaml",
			ComponentImage: "hypershift",
		},
		{
			Location:       "install/kube-controller-manager/kube-controller-manager.yaml",
			ComponentImage: "hyperkube",
		},
		{
			Location:       "install/kube-scheduler/kube-scheduler.yaml",
			ComponentImage: "hyperkube",
		},
	}

	runlevelOneLabel      = map[string]string{"openshift.io/run-level": "1"}
	runLevelOneComponents = []componentInstallTemplate{
		{
			ComponentImage: "control-plane",
			Template: componentinstall.Template{
				Name:            "kube-proxy",
				Namespace:       "kube-proxy",
				NamespaceObj:    newNamespaceBytes("kube-proxy", runlevelOneLabel),
				InstallTemplate: bootstrap.MustAsset("install/kube-proxy/install.yaml"),
			},
		},
		{
			ComponentImage: "control-plane",
			Template: componentinstall.Template{
				Name:            "kube-dns",
				Namespace:       "kube-dns",
				NamespaceObj:    newNamespaceBytes("kube-dns", runlevelOneLabel),
				InstallTemplate: bootstrap.MustAsset("install/kube-dns/install.yaml"),
			},
		},
		{
			ComponentImage: "hypershift",
			Template: componentinstall.Template{
				Name:            "openshift-apiserver",
				Namespace:       "openshift-apiserver",
				NamespaceObj:    newNamespaceBytes("openshift-apiserver", runlevelOneLabel),
				InstallTemplate: bootstrap.MustAsset("install/openshift-apiserver/install.yaml"),
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
				RBACTemplate:      bootstrap.MustAsset("install/openshift-controller-manager/install-rbac.yaml"),
				InstallTemplate:   bootstrap.MustAsset("install/openshift-controller-manager/install.yaml"),
			},
		},
	}
)

func (c *ClusterUpConfig) StartSelfHosted(out io.Writer) error {
	configDirs, err := c.BuildConfig()
	if err != nil {
		return err
	}
	// if we're supposed to write the config, we'll do that and then exit
	if c.WriteConfig {
		fmt.Printf("Wrote config to: %q\n", c.BaseDir)
		return nil
	}

	kubeletFlags, err := c.makeKubeletFlags(out, configDirs.nodeConfigDir)
	if err != nil {
		return err
	}
	glog.V(2).Infof("kubeletflags := %s\n", kubeletFlags)

	kubeletContainerID, err := c.startKubelet(out, configDirs.masterConfigDir, configDirs.nodeConfigDir, configDirs.podManifestDir, kubeletFlags)
	if err != nil {
		return err
	}
	glog.V(2).Infof("started kubelet in container %q\n", kubeletContainerID)

	templateSubstitutionValues := map[string]string{
		"MASTER_CONFIG_HOST_PATH":                       configDirs.masterConfigDir,
		"OPENSHIFT_APISERVER_CONFIG_HOST_PATH":          configDirs.openshiftAPIServerConfigDir,
		"OPENSHIFT_CONTROLLER_MANAGER_CONFIG_HOST_PATH": configDirs.openshiftControllerConfigDir,
		"NODE_CONFIG_HOST_PATH":                         configDirs.nodeConfigDir,
		"KUBEDNS_CONFIG_HOST_PATH":                      configDirs.kubeDNSConfigDir,
		"OPENSHIFT_PULL_POLICY":                         c.pullPolicy,
		"LOGLEVEL":                                      fmt.Sprintf("%d", c.ServerLogLevel),
	}

	clientConfigBuilder, err := kclientcmd.LoadFromFile(filepath.Join(c.LocalDirFor(kubeapiserver.KubeAPIServerDirName), "admin.kubeconfig"))
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
	// wait for the apiserver to be ready
	glog.Info("Waiting for the kube-apiserver to be ready ...")
	if err := waitForHealthyKubeAPIServer(clientConfig); err != nil {
		return err
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

	installContext, err := componentinstall.NewComponentInstallContext(c.cliImage(), c.imageFormat(), c.pullPolicy, c.BaseDir, c.ServerLogLevel)
	if err != nil {
		return err
	}

	// wait for the openshift apiserver before we create the rest of the components, since they may rely on openshift resources
	aggregatorClient, err := aggregatorclient.NewForConfig(installContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	err = componentinstall.WaitForAPIs(aggregatorClient,
		"v1.apps.openshift.io",
		"v1.authorization.openshift.io",
		"v1.build.openshift.io",
		"v1.image.openshift.io",
		"v1.network.openshift.io",
		"v1.oauth.openshift.io",
		"v1.project.openshift.io",
		"v1.quota.openshift.io",
		"v1.route.openshift.io",
		"v1.security.openshift.io",
		"v1.template.openshift.io",
		"v1.user.openshift.io",
	)
	if err != nil {
		return err
	}
	glog.Info("openshift-apiserver available")

	go watchAPIServices(aggregatorClient)

	err = installComponentTemplates(
		componentsToInstall,
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
	masterConfigDir              string
	openshiftAPIServerConfigDir  string
	openshiftControllerConfigDir string
	nodeConfigDir                string
	kubeDNSConfigDir             string
	podManifestDir               string
	baseDir                      string
	err                          error
}

// LocalDirFor returns a local directory path for the given component.
func (c *ClusterUpConfig) LocalDirFor(componentName string) string {
	return filepath.Join(c.BaseDir, componentName)
}

// RemoteDirFor returns a directory path on remote host
func (c *ClusterUpConfig) RemoteDirFor(componentName string) string {
	return filepath.Join(host.RemoteHostOriginDir, c.BaseDir, componentName)
}

func (c *ClusterUpConfig) copyToRemote(source, component string) (string, error) {
	if err := c.hostHelper.CopyToHost(source, c.RemoteDirFor(component)); err != nil {
		return "", err
	}
	return c.RemoteDirFor(component), nil
}

func (c *ClusterUpConfig) BuildConfig() (*configDirs, error) {
	configs := &configDirs{
		masterConfigDir:              filepath.Join(c.BaseDir, kubeapiserver.KubeAPIServerDirName),
		openshiftAPIServerConfigDir:  filepath.Join(c.BaseDir, kubeapiserver.OpenShiftAPIServerDirName),
		openshiftControllerConfigDir: filepath.Join(c.BaseDir, kubeapiserver.OpenShiftControllerManagerDirName),
		nodeConfigDir:                filepath.Join(c.BaseDir, kubelet.NodeConfigDirName),
		kubeDNSConfigDir:             filepath.Join(c.BaseDir, kubelet.KubeDNSDirName),
		podManifestDir:               filepath.Join(c.BaseDir, kubelet.PodManifestDirName),
	}

	originalMasterConfigDir := configs.masterConfigDir
	originalNodeConfigDir := configs.nodeConfigDir
	var err error

	if _, err := os.Stat(configs.masterConfigDir); os.IsNotExist(err) {
		_, err = c.makeMasterConfig()
		if err != nil {
			return nil, err
		}
	}
	if c.isRemoteDocker {
		configs.masterConfigDir, err = c.copyToRemote(configs.masterConfigDir, kubeapiserver.KubeAPIServerDirName)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.openshiftAPIServerConfigDir); os.IsNotExist(err) {
		_, err = c.makeOpenShiftAPIServerConfig(originalMasterConfigDir)
		if err != nil {
			return nil, err
		}
	}
	if c.isRemoteDocker {
		configs.openshiftAPIServerConfigDir, err = c.copyToRemote(configs.openshiftAPIServerConfigDir, kubeapiserver.OpenShiftAPIServerDirName)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.openshiftControllerConfigDir); os.IsNotExist(err) {
		_, err = c.makeOpenShiftControllerConfig(originalMasterConfigDir)
		if err != nil {
			return nil, err
		}
	}
	if c.isRemoteDocker {
		configs.openshiftControllerConfigDir, err = c.copyToRemote(configs.openshiftControllerConfigDir, kubeapiserver.OpenShiftControllerManagerDirName)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.nodeConfigDir); os.IsNotExist(err) {
		_, err = c.makeNodeConfig(configs.masterConfigDir)
		if err != nil {
			return nil, err
		}
	}
	if c.isRemoteDocker {
		configs.nodeConfigDir, err = c.copyToRemote(configs.nodeConfigDir, kubelet.NodeConfigDirName)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.kubeDNSConfigDir); os.IsNotExist(err) {
		_, err = c.makeKubeDNSConfig(originalNodeConfigDir)
		if err != nil {
			return nil, err
		}

	}
	if c.isRemoteDocker {
		configs.kubeDNSConfigDir, err = c.copyToRemote(configs.kubeDNSConfigDir, kubelet.KubeDNSDirName)
		if err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(configs.podManifestDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configs.podManifestDir, 0755); err != nil {
			return nil, err
		}

	}

	substitutions := map[string]string{
		"/path/to/master/config-dir":              configs.masterConfigDir,
		"/path/to/openshift-apiserver/config-dir": configs.openshiftAPIServerConfigDir,
		"ETCD_VOLUME":                             "emptyDir:\n",
		"OPENSHIFT_PULL_POLICY":                   c.pullPolicy,
	}

	if len(c.HostDataDir) > 0 {
		substitutions["ETCD_VOLUME"] = `hostPath:
      path: ` + c.HostDataDir + "\n"
	}

	glog.V(2).Infof("Creating static pod definitions in %q", configs.podManifestDir)
	for _, staticPod := range staticPodInstalls {
		if len(staticPod.ComponentImage) > 0 {
			substitutions["IMAGE"] = c.ImageTemplate.ExpandOrDie(staticPod.ComponentImage)
		} else {
			delete(substitutions, "IMAGE")
		}
		glog.V(3).Infof("Substitutions: %#v", substitutions)
		if err := staticpods.UpsertStaticPod(staticPod.Location, substitutions, configs.podManifestDir); err != nil {
			return nil, err
		}
	}

	if c.isRemoteDocker {
		configs.podManifestDir, err = c.copyToRemote(configs.podManifestDir, kubelet.PodManifestDirName)
		if err != nil {
			return nil, err
		}
	}
	glog.V(2).Infof("configLocations = %#v", *configs)

	return configs, nil
}

// makeMasterConfig returns the directory where a generated masterconfig lives
func (c *ClusterUpConfig) makeMasterConfig() (string, error) {
	publicHost := c.GetPublicHostName()

	container := kubeapiserver.NewKubeAPIServerStartConfig()
	container.MasterImage = c.openshiftImage()
	container.Args = []string{
		"--write-config=/var/lib/origin/openshift.local.config",
		fmt.Sprintf("--master=%s", c.ServerIP),
		fmt.Sprintf("--images=%s", c.imageFormat()),
		fmt.Sprintf("--dns=0.0.0.0:%d", c.DNSPort),
		fmt.Sprintf("--public-master=https://%s:8443", publicHost),
		"--etcd-dir=/var/lib/etcd",
	}

	masterConfigDir, err := container.MakeMasterConfig(c.GetDockerClient(), c.BaseDir)
	if err != nil {
		return "", fmt.Errorf("error creating master config: %v", err)
	}

	return masterConfigDir, nil
}

// makeNodeConfig returns the directory where a generated nodeconfig lives
func (c *ClusterUpConfig) makeNodeConfig(masterConfigDir string) (string, error) {
	defaultNodeName := "localhost"

	container := kubelet.NewNodeStartConfig()
	container.ContainerBinds = append(container.ContainerBinds, masterConfigDir+":/var/lib/origin/openshift.local.masterconfig:z")
	container.CLIImage = c.cliImage()
	container.NodeImage = c.nodeImage()
	container.Args = []string{
		fmt.Sprintf("--certificate-authority=%s", "/var/lib/origin/openshift.local.masterconfig/ca.crt"),
		fmt.Sprintf("--dns-bind-address=0.0.0.0:%d", c.DNSPort),
		fmt.Sprintf("--hostnames=%s", defaultNodeName),
		fmt.Sprintf("--hostnames=%s", "127.0.0.1"),
		fmt.Sprintf("--images=%s", c.imageFormat()),
		fmt.Sprintf("--node=%s", defaultNodeName),
		fmt.Sprintf("--node-client-certificate-authority=%s", "/var/lib/origin/openshift.local.masterconfig/ca.crt"),
		fmt.Sprintf("--signer-cert=%s", "/var/lib/origin/openshift.local.masterconfig/ca.crt"),
		fmt.Sprintf("--signer-key=%s", "/var/lib/origin/openshift.local.masterconfig/ca.key"),
		fmt.Sprintf("--signer-serial=%s", "/var/lib/origin/openshift.local.masterconfig/ca.serial.txt"),
		fmt.Sprintf("--volume-dir=%s", c.HostVolumesDir),
	}

	nodeConfigDir, err := container.MakeNodeConfig(c.GetDockerClient(), c.BaseDir)
	if err != nil {
		return "", fmt.Errorf("error creating node config: %v", err)
	}

	return nodeConfigDir, nil
}

// makeKubeletFlags returns the kubelet flags
func (c *ClusterUpConfig) makeKubeletFlags(out io.Writer, nodeConfigDir string) ([]string, error) {
	container := kubelet.NewKubeletStartFlags()
	container.ContainerBinds = append(container.ContainerBinds, nodeConfigDir+":/var/lib/origin/openshift.local.config/node:z")
	container.NodeImage = c.nodeImage()
	container.UseSharedVolume = !c.UseNsenterMount

	kubeletFlags, err := container.MakeKubeletFlags(c.GetDockerClient(), c.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("error creating node config: %v", err)
	}

	// TODO make this non-broken, but for now spaces are evil
	flags := strings.Split(strings.TrimSpace(kubeletFlags), " ")

	if driverName, err := c.DockerHelper().CgroupDriver(); err == nil && driverName == "cgroupfs" {
		flags = append(flags, "--cgroup-driver=cgroupfs")
	}

	// TODO: OSX snowflake
	// Default changed in kube 1.10. This ultimately breaks Docker For Mac as every hostPath
	// mount must be shared or rslave.
	if runtime.GOOS == "darwin" {
		flags = append(flags, "--feature-gates=MountPropagation=false")
	}

	return flags, nil
}

func (c *ClusterUpConfig) makeKubeDNSConfig(nodeConfig string) (string, error) {
	return kubelet.MakeKubeDNSConfig(nodeConfig, c.BaseDir)
}

func (c *ClusterUpConfig) makeOpenShiftAPIServerConfig(masterConfigDir string) (string, error) {
	return kubeapiserver.MakeOpenShiftAPIServerConfig(masterConfigDir, c.RoutingSuffix, c.BaseDir)
}

func (c *ClusterUpConfig) makeOpenShiftControllerConfig(masterConfigDir string) (string, error) {
	return kubeapiserver.MakeOpenShiftControllerConfig(masterConfigDir, c.BaseDir)
}

// startKubelet returns the container id
func (c *ClusterUpConfig) startKubelet(out io.Writer, masterConfigDir, nodeConfigDir, podManifestDir string, kubeletFlags []string) (string, error) {
	dockerRoot, err := c.DockerHelper().DockerRoot()
	if err != nil {
		return "", err
	}

	// here's a cool thing.  The kubelet flags specify a --root-dir which is the *real* HostVolumesDir
	hostVolumeDir := c.HostVolumesDir
	for i, flag := range kubeletFlags {
		if strings.HasPrefix(flag, "--root-dir=") {
			hostVolumeDir = strings.TrimLeft(flag, "--root-dir=")
			// TODO: Figure out if we need this on Windows as well
			if runtime.GOOS != "linux" {
				kubeletFlags[i] = "--root-dir=" + c.HostVolumesDir
			}
		}
	}

	container := kubelet.NewKubeletRunConfig()
	container.ContainerBinds = append(container.ContainerBinds, nodeConfigDir+":/var/lib/origin/openshift.local.config/node:z")
	container.ContainerBinds = append(container.ContainerBinds, masterConfigDir+":/var/lib/origin/openshift.local.config/master:z")
	container.ContainerBinds = append(container.ContainerBinds, podManifestDir+":/var/lib/origin/pod-manifests:z")
	container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%s:/var/lib/etcd:z", c.HostDataDir))
	container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s", c.HostPersistentVolumesDir))
	container.Environment = append(container.Environment, fmt.Sprintf("OPENSHIFT_PV_DIR=%s", c.HostPersistentVolumesDir))
	if !c.UseNsenterMount {
		hostVolumeDirSource := hostVolumeDir
		// TODO: Figure out if we need this on Windows as well
		if runtime.GOOS != "linux" {
			hostVolumeDirSource = c.HostVolumesDir
		}
		container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%s:%s:shared", hostVolumeDirSource, hostVolumeDir))
		container.Environment = append(container.Environment, "OPENSHIFT_CONTAINERIZED=false")
	} else {
		container.ContainerBinds = append(container.ContainerBinds, "/:/rootfs:ro")
		container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s:rslave", hostVolumeDir))
	}
	container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s", dockerRoot))
	// Kubelet needs to be able to write to
	// /sys/devices/virtual/net/vethXXX/brport/hairpin_mode, so make this rw, not ro.
	container.ContainerBinds = append(container.ContainerBinds, "/sys/devices/virtual/net:/sys/devices/virtual/net:rw")

	container.NodeImage = c.nodeImage()
	container.HTTPProxy = c.HTTPProxy
	container.HTTPSProxy = c.HTTPSProxy
	container.NoProxy = c.NoProxy

	actualKubeletFlags := []string{}
	for _, curr := range kubeletFlags {
		if curr == "--cluster-dns=" {
			continue
		}
		if curr == "--pod-manifest-path=" {
			continue
		}
		actualKubeletFlags = append(actualKubeletFlags, curr)
	}
	container.Args = append(actualKubeletFlags, "--pod-manifest-path=/var/lib/origin/pod-manifests")
	container.Args = append(container.Args, "--cluster-dns=172.30.0.2")
	container.Args = append(container.Args, fmt.Sprintf("--v=%d", c.ServerLogLevel))
	glog.V(1).Info(strings.Join(container.Args, " "))

	kubeletContainerID, err := container.StartKubelet(c.DockerHelper().Client(), c.BaseDir)
	if err != nil {
		return "", fmt.Errorf("error creating node config: %v", err)
	}
	return kubeletContainerID, nil
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

func watchAPIServices(aggregatorClient aggregatorclient.Interface) {
	watch, err := aggregatorClient.ApiregistrationV1beta1().APIServices().Watch(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	watchCh := watch.ResultChan()
	for {
		select {
		case watchEvent, ok := <-watchCh:
			if !ok {
				glog.V(5).Infof("channel closed, restablishing")
				watch, err := aggregatorClient.ApiregistrationV1beta1().APIServices().Watch(metav1.ListOptions{})
				if err != nil {
					panic(err)
				}
				watchCh = watch.ResultChan()
			}
			if watchEvent.Object == nil {
				glog.V(5).Infof("observed %q without an object", watchEvent.Type)
				break
			}
			encoder := json.NewYAMLSerializer(json.DefaultMetaFactory, legacyscheme.Scheme, legacyscheme.Scheme)
			output, err := kruntime.Encode(encoder, watchEvent.Object)
			if err != nil {
				utilruntime.HandleError(err)
				continue
			}
			glog.V(5).Infof("observed %q with\n%v", watchEvent.Type, string(output))
		}
	}
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

func installComponentTemplates(templates []componentInstallTemplate, imageFormat, baseDir string, params map[string]string, dockerClient dockerhelper.Interface) error {
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
