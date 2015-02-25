package server

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strconv"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/capabilities"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
	"github.com/openshift/origin/pkg/cmd/server/origin"

	// Admission control plugins from upstream Kubernetes
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/limitranger"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/namespace/exists"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/resourcedefaults"
	_ "github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/resourcequota"
)

const (
	unauthenticatedUsername = "system:anonymous"

	authenticatedGroup   = "system:authenticated"
	unauthenticatedGroup = "system:unauthenticated"
)

func (cfg Config) startMaster() error {
	// Allow privileged containers
	// TODO: make this configurable and not the default https://github.com/openshift/origin/issues/662
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
	})

	cfg.MintNodeCerts()
	cfg.MintSystemClientCert("admin", "system:cluster-admins")
	cfg.MintSystemClientCert("openshift-deployer", "system:deployers")
	cfg.MintSystemClientCert("openshift-client")
	cfg.MintSystemClientCert("kube-client")

	openshiftConfig, err := cfg.BuildOriginMasterConfig()
	if err != nil {
		return err
	}
	//	 must start policy caching immediately
	openshiftConfig.RunPolicyCache()

	authConfig, err := cfg.BuildAuthConfig()
	if err != nil {
		return err
	}

	glog.Infof("  Nodes: %v", cfg.GetNodeList())

	if cfg.StartKube {
		kubeConfig, err := cfg.BuildKubernetesMasterConfig(openshiftConfig.RequestContextMapper, openshiftConfig.KubeClient())
		if err != nil {
			return err
		}
		kubeConfig.EnsurePortalFlags()

		openshiftConfig.Run([]origin.APIInstaller{kubeConfig}, []origin.APIInstaller{authConfig})

		kubeConfig.RunScheduler()
		kubeConfig.RunReplicationController()
		kubeConfig.RunEndpointController()
		kubeConfig.RunMinionController()
		kubeConfig.RunResourceQuotaManager()

	} else {
		kubeAddr, err := cfg.GetKubernetesAddress()
		if err != nil {
			return err
		}
		proxy := &kubernetes.ProxyConfig{
			KubernetesAddr: kubeAddr,
			ClientConfig:   &openshiftConfig.KubeClientConfig,
		}

		openshiftConfig.Run([]origin.APIInstaller{proxy}, []origin.APIInstaller{authConfig})
	}

	// TODO: recording should occur in individual components
	record.StartRecording(openshiftConfig.KubeClient().Events(""), kapi.EventSource{Component: "master"})

	openshiftConfig.RunAssetServer()
	openshiftConfig.RunBuildController()
	openshiftConfig.RunBuildImageChangeTriggerController()
	openshiftConfig.RunDeploymentController()
	openshiftConfig.RunDeploymentConfigController()
	openshiftConfig.RunDeploymentConfigChangeController()
	openshiftConfig.RunDeploymentImageChangeTriggerController()

	return nil
}

// run launches the appropriate startup modes or returns an error.
func start(cfg Config, args []string) error {
	if cfg.WriteConfigOnly {
		return nil
	}

	switch {
	case cfg.StartMaster && cfg.StartNode:
		glog.Infof("Starting an OpenShift all-in-one, reachable at %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())
		if cfg.MasterPublicAddr.Provided {
			glog.Infof("OpenShift master public address is %s", cfg.MasterPublicAddr.String())
		}

	case cfg.StartMaster:
		glog.Infof("Starting an OpenShift master, reachable at %s (etcd: %s)", cfg.MasterAddr.String(), cfg.EtcdAddr.String())
		if cfg.MasterPublicAddr.Provided {
			glog.Infof("OpenShift master public address is %s", cfg.MasterPublicAddr.String())
		}

	case cfg.StartNode:
		glog.Infof("Starting an OpenShift node, connecting to %s", cfg.MasterAddr.String())

	}

	if cfg.StartEtcd {
		if err := cfg.RunEtcd(); err != nil {
			return err
		}
	}

	if env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			glog.Infof("Starting profiling endpoint at http://127.0.0.1:6060/debug/pprof/")
			glog.Fatal(http.ListenAndServe("127.0.0.1:6060", nil))
		}()
	}

	if cfg.StartMaster {
		if err := cfg.startMaster(); err != nil {
			return err
		}
	}

	if cfg.StartNode {
		kubeClient, _, err := cfg.GetKubeClient()
		if err != nil {
			return err
		}

		if !cfg.StartMaster {
			// TODO: recording should occur in individual components
			record.StartRecording(kubeClient.Events(""), kapi.EventSource{Component: "node"})
		}

		nodeConfig, err := cfg.BuildKubernetesNodeConfig()
		if err != nil {
			return err
		}

		nodeConfig.EnsureVolumeDir()
		nodeConfig.EnsureDocker(cfg.Docker)
		nodeConfig.RunProxy()
		nodeConfig.RunKubelet()
	}

	select {}

	return nil
}

func envInt(key string, defaultValue int32, minValue int32) int32 {
	value, err := strconv.ParseInt(env(key, fmt.Sprintf("%d", defaultValue)), 10, 32)
	if err != nil || int32(value) < minValue {
		glog.Warningf("Invalid %s. Defaulting to %d.", key, defaultValue)
		return defaultValue
	}
	return int32(value)
}

// env returns an environment variable or a default value if not specified.
func env(key string, defaultValue string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

func (cfg Config) RunEtcd() error {
	etcdAddr, err := cfg.GetEtcdAddress()
	if err != nil {
		return err
	}

	etcdConfig := &etcd.Config{
		BindAddr:     cfg.BindAddr.Host,
		PeerBindAddr: cfg.BindAddr.Host,
		MasterAddr:   etcdAddr.Host,
		EtcdDir:      cfg.EtcdDir,
	}

	etcdConfig.Run()

	return nil
}

func getHost(theURL url.URL) string {
	host, _, err := net.SplitHostPort(theURL.Host)
	if err != nil {
		return theURL.Host
	}

	return host
}

func getPort(theURL url.URL) int {
	_, port, err := net.SplitHostPort(theURL.Host)
	if err != nil {
		return 0
	}

	intport, _ := strconv.Atoi(port)
	return intport
}
