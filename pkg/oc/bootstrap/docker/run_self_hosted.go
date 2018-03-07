package docker

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/util/retry"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrorutils "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	securityclient "github.com/openshift/client-go/security/clientset/versioned"
	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/bulk"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/bootstrap"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubelet"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/staticpods"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/tmpformac"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateclientshim "github.com/openshift/origin/pkg/template/client/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

var (
	// staticPodLocations should only include those pods that *must* be run statically because they
	// bring up the services required to run the workload controllers.
	// etcd, kube-apiserver, kube-controller-manager, kube-scheduler (this is because sig-scheduling is expanding the scheduler responsibilities)
	staticPodLocations = []string{
		"install/etcd/etcd.yaml",
		"install/kube-apiserver/apiserver.yaml",
		"install/kube-controller-manager/kube-controller-manager.yaml",
		"install/kube-scheduler/kube-scheduler.yaml",
	}

	// componentsToInstall DOES NOT INSTALL IN ORDER.  They are installed separately and expected to come up
	// in any order and self-organize into something that works.  Remember, when the whole system crashes and restarts
	// you don't get to choose your restart order.  Plan accordingly.  No bugs or attempts at interlocks will be accepted
	// in cluster up.
	// TODO we can take a guess at readiness by making sure that pods in the namespace exist and all pods are healthy
	// TODO it's not perfect, but its fairly good as a starting point.
	componentsToInstall = []componentRequest{
		{location: "openshift-controller-manager", privilegedSANames: []string{"openshift-controller-manager"}},
		{location: "kube-proxy", privilegedSANames: []string{"kube-proxy"}},
		{location: "kube-dns", privilegedSANames: []string{"kube-dns"}},
	}
)

type componentRequest struct {
	// location is the name of the folder in install and the name of the namespace you're installed into
	location          string
	privilegedSANames []string
}

func (c *ClientStartConfig) StartSelfHosted(out io.Writer) error {
	if c.PortForwarding {
		err := openshift.CheckSocat()
		if err != nil {
			return err
		}
	}

	var (
		masterConfigDir  string
		nodeConfigDir    string
		kubeDNSConfigDir string
		podManifestDir   string
		err              error
	)

	switch {
	case len(c.HostConfigDir) > 0 && !c.WriteConfig:
		masterConfigDir = filepath.Join(c.HostConfigDir, kubeapiserver.KubeAPIServerDirName, "master")
		nodeConfigDir = filepath.Join(c.HostConfigDir, kubelet.NodeConfigDirName)
		kubeDNSConfigDir = filepath.Join(c.HostConfigDir, kubelet.KubeDNSDirName)
		podManifestDir = filepath.Join(c.HostConfigDir, kubelet.PodManifestDirName)

	case len(c.HostConfigDir) == 0 && c.WriteConfig:
		return fmt.Errorf("cannot write a config without a hostconfigdir")

	default:
		// we need to generate the config
		masterConfigDir, err = c.makeMasterConfig(out)
		if err != nil {
			return err
		}
		nodeConfigDir, kubeDNSConfigDir, err = c.makeNodeConfig(out, masterConfigDir)
		if err != nil {
			return err
		}
		kubeDNSConfigDir, err = c.makeKubeDNSConfig(out, kubeDNSConfigDir)
		if err != nil {
			return err
		}
		podManifestDir, err = tmpformac.TempDir(kubelet.PodManifestDirName)
		if err != nil {
			return err
		}

	}
	glog.V(2).Infof("master-config at %q, node-config at %q, kube-dns-config: %q", masterConfigDir, nodeConfigDir, kubeDNSConfigDir)

	// if we're supposed to write the config, we'll do that and then exit
	if c.WriteConfig {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		absHostDir, err := cmdutil.MakeAbs(c.HostConfigDir, cwd)
		if err != nil {
			return err
		}
		if err := CopyDirectory(masterConfigDir, path.Join(absHostDir, kubeapiserver.KubeAPIServerDirName, "master")); err != nil {
			return err
		}
		if err := CopyDirectory(nodeConfigDir, path.Join(absHostDir, kubelet.NodeConfigDirName)); err != nil {
			return err
		}
		if err := CopyDirectory(kubeDNSConfigDir, path.Join(absHostDir, kubelet.KubeDNSDirName)); err != nil {
			return err
		}
		if err := CopyDirectory(kubeDNSConfigDir, path.Join(absHostDir, kubelet.PodManifestDirName)); err != nil {
			return err
		}

		fmt.Printf("Wrote config to: %q\n", c.HostConfigDir)
		return nil
	}

	kubeletFlags, err := c.makeKubeletFlags(out, nodeConfigDir)
	if err != nil {
		return err
	}
	glog.V(2).Infof("kubeletflags := %s\n", kubeletFlags)

	kubeletContainerID, err := c.startKubelet(out, masterConfigDir, nodeConfigDir, podManifestDir, kubeletFlags)
	if err != nil {
		return err
	}
	glog.V(2).Infof("started kubelet in container %q\n", kubeletContainerID)

	substitutions := map[string]string{
		"/path/to/master/config-dir": masterConfigDir,
		"ETCD_VOLUME":                "emptyDir:\n",
	}
	if len(c.HostDataDir) > 0 {
		substitutions["ETCD_VOLUME"] = `hostPath:
      path: ` + c.HostDataDir + "\n"
	}
	for _, staticPodLocation := range staticPodLocations {
		if err := staticpods.UpsertStaticPod(staticPodLocation, substitutions, podManifestDir); err != nil {
			return err
		}
	}

	clientConfigBuilder, err := kclientcmd.LoadFromFile(filepath.Join(masterConfigDir, "admin.kubeconfig"))
	if err != nil {
		return err
	}
	overrides := &kclientcmd.ConfigOverrides{}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*clientConfigBuilder, overrides)
	clientConfig, err := defaultCfg.ClientConfig()
	if err != nil {
		return err
	}

	// wait for the apiserver to be ready
	waitForHealthyKubeAPIServer(clientConfig)

	templateSubstitutionValues := map[string]string{
		"MASTER_CONFIG_HOST_PATH":  masterConfigDir,
		"NODE_CONFIG_HOST_PATH":    nodeConfigDir,
		"KUBEDNS_CONFIG_HOST_PATH": kubeDNSConfigDir,
	}

	waitGroup := sync.WaitGroup{}
	for i := range componentsToInstall {
		component := componentsToInstall[i]
		glog.Infof("Installing %q...", component.location)
		waitGroup.Add(1)

		go func() {
			defer utilruntime.HandleCrash()
			defer waitGroup.Done()

			if err := component.install(clientConfig, templateSubstitutionValues); err != nil {
				panic(err)
			}
		}()
	}
	waitGroup.Wait()
	glog.Info("components installed")

	// TODO remove this linkage.  State like this doesn't belong on the struct and should be passed through for each invocation
	c.LocalConfigDir = path.Dir(masterConfigDir)

	return nil
}

// makeMasterConfig returns the directory where a generated masterconfig lives
func (c *ClientStartConfig) makeMasterConfig(out io.Writer) (string, error) {
	publicHost := c.PublicHostname
	if len(publicHost) == 0 {
		publicHost = c.ServerIP
	}

	imageRunningHelper := run.NewRunHelper(c.DockerHelper())
	container := kubeapiserver.NewKubeAPIServerStartConfig()
	// TODO follow the args pattern of the others
	container.MasterImage = c.openshiftImage()
	container.Args = []string{
		"--write-config=/var/lib/origin/openshift.local.config",
		"--master=127.0.0.1",
		fmt.Sprintf("--images=%s", c.imageFormat()),
		fmt.Sprintf("--dns=0.0.0.0:%d", c.DNSPort),
		fmt.Sprintf("--public-master=https://%s:8443", publicHost),
		"--etcd-dir=/var/lib/etcd",
	}

	masterConfigDir, err := container.MakeMasterConfig(c.GetDockerClient(out), imageRunningHelper.New(), out)
	if err != nil {
		return "", fmt.Errorf("error creating master config: %v", err)
	}

	return masterConfigDir, nil
}

// makeNodeConfig returns the directory where a generated nodeconfig lives
func (c *ClientStartConfig) makeNodeConfig(out io.Writer, masterConfigDir string) (string, string, error) {
	defaultNodeName := "localhost"

	imageRunningHelper := run.NewRunHelper(c.DockerHelper())
	container := kubelet.NewNodeStartConfig()
	container.ContainerBinds = append(container.ContainerBinds, masterConfigDir+":/var/lib/origin/openshift.local.masterconfig:z")
	container.NodeImage = c.openshiftImage()
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

	nodeConfigDir, err := container.MakeNodeConfig(c.GetDockerClient(out), imageRunningHelper.New(), out)
	if err != nil {
		return "", "", fmt.Errorf("error creating node config: %v", err)
	}
	kubeDNSConfigDir, err := container.MakeKubeDNSConfig(c.GetDockerClient(out), imageRunningHelper.New(), out)
	if err != nil {
		return "", "", fmt.Errorf("error creating node config: %v", err)
	}

	return nodeConfigDir, kubeDNSConfigDir, nil
}

// makeKubeletFlags returns the kubelet flags
func (c *ClientStartConfig) makeKubeletFlags(out io.Writer, nodeConfigDir string) ([]string, error) {
	imageRunningHelper := run.NewRunHelper(c.DockerHelper())
	container := kubelet.NewKubeletStartFlags()
	container.ContainerBinds = append(container.ContainerBinds, nodeConfigDir+":/var/lib/origin/openshift.local.config/node:z")
	container.Environment = c.Environment
	container.NodeImage = c.openshiftImage()
	container.Environment = c.Environment
	container.UseSharedVolume = !c.UseNsenterMount

	kubeletFlags, err := container.MakeKubeletFlags(imageRunningHelper.New(), out)
	if err != nil {
		return nil, fmt.Errorf("error creating node config: %v", err)
	}

	// TODO make this non-broken, but for now spaces are evil
	flags := strings.Split(strings.TrimSpace(kubeletFlags), " ")

	if driverName, err := c.DockerHelper().CgroupDriver(); err == nil && driverName == "cgroupfs" {
		flags = append(flags, "--cgroup-driver=cgroupfs")
	}

	return flags, nil
}

// makeKubeDNSConfig mutates some pieces of the kubedns dir.
// TODO This should be building the whole thing eventually
func (c *ClientStartConfig) makeKubeDNSConfig(out io.Writer, kubeDNSConfigDir string) (string, error) {
	return kubelet.MutateKubeDNSConfig(kubeDNSConfigDir, out)
}

// startKubelet returns the container id
func (c *ClientStartConfig) startKubelet(out io.Writer, masterConfigDir, nodeConfigDir, podManifestDir string, kubeletFlags []string) (string, error) {
	dockerRoot, err := c.DockerHelper().DockerRoot()
	if err != nil {
		return "", err
	}

	imageRunningHelper := run.NewRunHelper(c.DockerHelper())
	container := kubelet.NewKubeletRunConfig()
	container.ContainerBinds = append(container.ContainerBinds, nodeConfigDir+":/var/lib/origin/openshift.local.config/node:z")
	container.ContainerBinds = append(container.ContainerBinds, masterConfigDir+":/var/lib/origin/openshift.local.config/master:z")
	container.ContainerBinds = append(container.ContainerBinds, podManifestDir+":/var/lib/origin/pod-manifests:z")
	if len(c.HostDataDir) > 0 {
		container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%s:/var/lib/etcd:z", c.HostDataDir))
	}
	if len(c.HostPersistentVolumesDir) > 0 {
		container.ContainerBinds = append(container.ContainerBinds, fmt.Sprintf("%[1]s:%[1]s", c.HostPersistentVolumesDir))
		container.Environment = append(container.Environment, fmt.Sprintf("OPENSHIFT_PV_DIR=%s", c.HostPersistentVolumesDir))
	}
	container.NodeImage = c.openshiftImage()
	container.Environment = c.Environment
	container.DockerRoot = dockerRoot
	container.UseSharedVolume = !c.UseNsenterMount
	container.HostVolumesDir = c.HostVolumesDir
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
	glog.V(1).Info(strings.Join(container.Args, " "))

	// update DNS resolution to point at the master (for now).  Do this by grabbing the local and prepending to it.
	// this is probably broken somewhere for some reason and a bad idea of other reasons, but it gets us moving
	if existingResolveConf, err := ioutil.ReadFile("/etc/resolv.conf"); err == nil {
		newResolveConf := append([]byte("nameserver 172.30.0.2\n"), existingResolveConf...)
		if err := ioutil.WriteFile(path.Join(nodeConfigDir, "resolv.conf"), newResolveConf, 0644); err != nil {
			return "", err
		}
		if err := ioutil.WriteFile(path.Join(nodeConfigDir, "resolv.conf"), existingResolveConf, 0644); err != nil {
			return "", err
		}
		//container.Args = append(kubeletFlags, "--cluster-dns=172.30.0.2")

	} else {
		// TOD this may not be fatal after it sort of works.
		return "", err
	}

	kubeletContainerID, err := container.MakeNodeConfig(imageRunningHelper.New(), out)
	if err != nil {
		return "", fmt.Errorf("error creating node config: %v", err)
	}

	return kubeletContainerID, nil
}

func waitForHealthyKubeAPIServer(clientConfig *rest.Config) error {
	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
		if err != nil {
			return false, nil
		}

		healthStatus := 0
		resp := discoveryClient.RESTClient().Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if resp.Error() != nil {
			glog.V(5).Infof("Server isn't healthy yet.  Waiting a little while. %v", resp.Error())
			return false, nil
		}
		content, _ := resp.Raw()
		healthzContent = string(content)
		if healthStatus != http.StatusOK {
			glog.V(5).Infof("Server isn't healthy yet.  Waiting a little while. %v", healthStatus)
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		glog.Error(healthzContent)
	}

	return err
}

func (c componentRequest) install(clientConfig *rest.Config, substitutionValues map[string]string) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	if _, err := kubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.location}}); err != nil && !kapierrors.IsAlreadyExists(err) {
		return err
	}

	for _, saName := range c.privilegedSANames {
		// this has to be built here, because the first thing we will install is the openshift apiserver, which hosts our SCCs,
		// which means its namespace skips the SCC check, and we won't be able to create a working client
		securityClient, err := securityclient.NewForConfig(clientConfig)
		if err != nil {
			return err
		}
		// TODO use patch
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			privilegedSCC, err := securityClient.SecurityV1().SecurityContextConstraints().Get("privileged", metav1.GetOptions{})
			if err != nil {
				return err
			}
			privilegedSCC.Users = append(privilegedSCC.Users, serviceaccount.MakeUsername(c.location, saName))
			if _, err := securityClient.SecurityV1().SecurityContextConstraints().Update(privilegedSCC); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	template, err := processTemplate(clientConfig, path.Join("install", c.location, "install.yaml"), c.location, substitutionValues)
	if err != nil {
		return err
	}

	if err := createObjects(clientConfig, template.Objects, c.location); err != nil {
		return err
	}

	return nil
}

func createObjects(clientConfig *rest.Config, inObjs []runtime.Object, namespace string) error {
	// we end up with a bunch of unknowns. We can actually roundtrip them back in a unstructured and I think this works
	// TODO unbork this.
	objs := []runtime.Object{}
	for _, inObj := range inObjs {
		buf := &bytes.Buffer{}
		if err := unstructured.UnstructuredJSONScheme.Encode(inObj, buf); err != nil {
			return nil
		}
		unstructuredObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, buf.Bytes())
		if err != nil {
			return err
		}
		objs = append(objs, unstructuredObj)
	}

	mapper, typer := legacyscheme.Registry.RESTMapper(), legacyscheme.Scheme

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		return err
	}

	// Create objects
	bulkOperation := &bulk.Bulk{}

	bulkOperation.DynamicMapper = &resource.Mapper{
		RESTMapper:  mapper,
		ObjectTyper: typer,
		ClientMapper: resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
			return unstructuredClientForMapping(mapping, clientConfig)
		}),
	}
	bulkOperation.Mapper = &resource.Mapper{
		RESTMapper:   mapper,
		ObjectTyper:  typer,
		ClientMapper: bulk.ClientMapperFromConfig(clientConfig),
	}
	bulkOperation.PreferredSerializationOrder = bulk.PreferredSerializationOrder(discoveryClient)
	bulkOperation.Op = bulk.Create

	itemsToCreate := &kapi.List{
		Items: objs,
	}
	if errs := bulkOperation.Run(itemsToCreate, namespace); len(errs) > 0 {
		filteredErrs := []error{}
		for _, err := range errs {
			filteredErrs = append(filteredErrs, err)
		}
		if len(filteredErrs) == 0 {
			return nil
		}
		err = kerrorutils.NewAggregate(filteredErrs)
		return err
	}

	return nil
}

func processTemplate(clientConfig *rest.Config, location, namespace string, substitutionValues map[string]string) (*templateapi.Template, error) {
	data, err := bootstrap.Asset(location)
	if err != nil {
		return nil, err
	}
	templateObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), data)
	if err != nil {
		return nil, err
	}
	template := templateObj.(*templateapi.Template)

	injectUserVars(template, substitutionValues)

	templateClient, err := templateclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	templateProcessor := templateclientshim.NewTemplateProcessorClient(templateClient.Template().RESTClient(), namespace)

	return templateProcessor.Process(template)
}

func injectUserVars(t *templateapi.Template, values map[string]string) {
	for param, val := range values {
		v := getParameterByName(t, param)
		if v != nil {
			v.Value = val
			v.Generate = ""
		}
	}
}

func getParameterByName(t *templateapi.Template, name string) *templateapi.Parameter {
	for i, param := range t.Parameters {
		if param.Name == name {
			return &(t.Parameters[i])
		}
	}
	return nil
}

// TODO this is useful for building a generic bulk operations, try to make it more generic
func unstructuredClientForMapping(mapping *meta.RESTMapping, clientConfig *rest.Config) (resource.RESTClient, error) {
	cfg := *clientConfig

	cfg.APIPath = "/apis"
	if mapping.GroupVersionKind.Group == api.GroupName {
		cfg.APIPath = "/api"
	}
	gv := mapping.GroupVersionKind.GroupVersion()
	cfg.ContentConfig = dynamic.ContentConfig()
	cfg.GroupVersion = &gv

	return rest.RESTClientFor(&cfg)
}
