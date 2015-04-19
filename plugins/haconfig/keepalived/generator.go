package keepalived

import (
	"fmt"
	"strconv"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/golang/glog"

	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/haconfig"
)

const defaultInterface = "eth0"
const libModulesVolumeName = "lib-modules"
const libModulesPath = "/lib/modules"

//  Get kube client configuration from a file containing credentials for
//  connecting to the master.
func getClientConfig(path string) *kclient.Config {
	if 0 == len(path) {
		glog.Fatalf("You must specify a .kubeconfig file path containing credentials for connecting to the master with --credentials")
	}

	rules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: path, Precedence: []string{}}
	credentials, err := rules.Load()
	if err != nil {
		glog.Fatalf("Could not load credentials from %q: %v", path, err)
	}

	config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		glog.Fatalf("Credentials %q error: %v", path, err)
	}

	if err := kclient.LoadTLSFiles(config); err != nil {
		glog.Fatalf("Unable to load certificate info using credentials from %q: %v", path, err)
	}

	return config
}

//  Generate the HA failover monitor (keepalived) container environment entries.
func generateEnvEntries(name string, options *haconfig.HAConfigCmdOptions, kconfig *kclient.Config) app.Environment {
	watchPort := strconv.Itoa(options.WatchPort)
	replicas := strconv.Itoa(options.Replicas)
	insecureStr := strconv.FormatBool(kconfig.Insecure)
	unicastStr := strconv.FormatBool(options.UseUnicast)

	return app.Environment{
		"OPENSHIFT_MASTER":    kconfig.Host,
		"OPENSHIFT_CA_DATA":   string(kconfig.CAData),
		"OPENSHIFT_KEY_DATA":  string(kconfig.KeyData),
		"OPENSHIFT_CERT_DATA": string(kconfig.CertData),
		"OPENSHIFT_INSECURE":  insecureStr,

		"OPENSHIFT_HA_CONFIG_NAME":       name,
		"OPENSHIFT_HA_VIRTUAL_IPS":       options.VirtualIPs,
		"OPENSHIFT_HA_NETWORK_INTERFACE": options.NetworkInterface,
		"OPENSHIFT_HA_MONITOR_PORT":      watchPort,
		"OPENSHIFT_HA_REPLICA_COUNT":     replicas,
		"OPENSHIFT_HA_USE_UNICAST":       unicastStr,
		// "OPENSHIFT_HA_UNICAST_PEERS":     "127.0.0.1",
	}
}

//  Generate the HA failover monitor (keepalived) container configuration.
func generateFailoverMonitorContainerConfig(name string, options *haconfig.HAConfigCmdOptions, env app.Environment) *kapi.Container {
	containerName := fmt.Sprintf("%s-%s", name, options.Type)

	imageName := fmt.Sprintf("%s-%s", options.Type, haconfig.DefaultName)
	image := options.ImageTemplate.ExpandOrDie(imageName)

	ports := make([]kapi.ContainerPort, 1)
	ports[0] = kapi.ContainerPort{
		ContainerPort: options.ServicePort,
		HostPort:      options.ServicePort,
	}

	mounts := make([]kapi.VolumeMount, 1)
	mounts[0] = kapi.VolumeMount{
		Name:      libModulesVolumeName,
		ReadOnly:  true,
		MountPath: libModulesPath,
	}

	return &kapi.Container{
		Name:            containerName,
		Image:           image,
		Ports:           ports,
		Privileged:      true,
		ImagePullPolicy: kapi.PullIfNotPresent,
		VolumeMounts:    mounts,
		Env:             env.List(),
	}
}

//  Generate the HA failover monitor (keepalived) container configuration.
func generateContainerConfig(name string, options *haconfig.HAConfigCmdOptions) []kapi.Container {
	containers := make([]kapi.Container, 0)

	if len(options.VirtualIPs) < 1 {
		return containers
	}

	config := getClientConfig(options.Credentials)
	env := generateEnvEntries(name, options, config)

	c := generateFailoverMonitorContainerConfig(name, options, env)
	if c != nil {
		containers = append(containers, *c)
	}

	return containers
}

//  Generate the HA failover monitor (keepalived) container volume config.
func generateVolumeConfig() []kapi.Volume {
	hostPath := &kapi.HostPathVolumeSource{Path: libModulesPath}
	src := kapi.VolumeSource{HostPath: hostPath}

	vol := kapi.Volume{Name: libModulesVolumeName, VolumeSource: src}
	return []kapi.Volume{vol}
}

//  Generate the HA deployment template.
func generateDeploymentTemplate(name string, options *haconfig.HAConfigCmdOptions, selector map[string]string) dapi.DeploymentTemplate {
	podTemplate := &kapi.PodTemplateSpec{
		ObjectMeta: kapi.ObjectMeta{Labels: selector},
		Spec: kapi.PodSpec{
			HostNetwork: true,
			Containers:  generateContainerConfig(name, options),
			Volumes:     generateVolumeConfig(),
		},
	}

	return dapi.DeploymentTemplate{
		Strategy: dapi.DeploymentStrategy{
			Type: dapi.DeploymentStrategyTypeRecreate,
		},
		ControllerTemplate: kapi.ReplicationControllerSpec{
			// TODO: v0.1 requires a manual resize to the
			//       replicas to match current cluster state.
			//       v0.1+ could do this with either a watcher
			//       that updates the replica count or better
			//       yet, some way to kubernetes to say run
			//       this ReplicationController on each and
			//       every node that matches the selector.
			Replicas: options.Replicas,
			Selector: selector,
			Template: podTemplate,
		},
	}
}

//  Generate the HA deployment configuration.
func GenerateDeploymentConfig(name string, options *haconfig.HAConfigCmdOptions, selector map[string]string) *dapi.DeploymentConfig {

	return &dapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: selector,
		},
		Triggers: []dapi.DeploymentTriggerPolicy{
			{Type: dapi.DeploymentTriggerOnConfigChange},
		},
		Template: generateDeploymentTemplate(name, options, selector),
	}
}
