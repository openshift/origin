package keepalived

import (
	"fmt"
	"strconv"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	dapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/ipfailover"
)

const defaultInterface = "eth0"
const libModulesVolumeName = "lib-modules"
const libModulesPath = "/lib/modules"

//  Get kube client configuration from a file containing credentials for
//  connecting to the master.
func getClientConfig(path string) (*restclient.Config, error) {
	if 0 == len(path) {
		return nil, nil
	}

	rules := &kclientcmd.ClientConfigLoadingRules{ExplicitPath: path, Precedence: []string{}}
	credentials, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("Could not load credentials from %q: %v", path, err)
	}

	config, err := kclientcmd.NewDefaultClientConfig(*credentials, &kclientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("Credentials %q error: %v", path, err)
	}

	if err = restclient.LoadTLSFiles(config); err != nil {
		return nil, fmt.Errorf("Unable to load certificate info using credentials from %q: %v", path, err)
	}

	return config, nil
}

//  Generate the IP failover monitor (keepalived) container environment entries.
func generateEnvEntries(name string, options *ipfailover.IPFailoverConfigCmdOptions, kconfig *restclient.Config) app.Environment {
	watchPort := strconv.Itoa(options.WatchPort)
	replicas := strconv.FormatInt(int64(options.Replicas), 10)
	VRRPIDOffset := strconv.Itoa(options.VRRPIDOffset)
	env := app.Environment{}

	if kconfig != nil {
		insecureStr := strconv.FormatBool(kconfig.Insecure)
		env.Add(app.Environment{
			"OPENSHIFT_MASTER":    kconfig.Host,
			"OPENSHIFT_CA_DATA":   string(kconfig.CAData),
			"OPENSHIFT_KEY_DATA":  string(kconfig.KeyData),
			"OPENSHIFT_CERT_DATA": string(kconfig.CertData),
			"OPENSHIFT_INSECURE":  insecureStr,
		})

	}
	env.Add(app.Environment{

		"OPENSHIFT_HA_CONFIG_NAME":       name,
		"OPENSHIFT_HA_VIRTUAL_IPS":       options.VirtualIPs,
		"OPENSHIFT_HA_NETWORK_INTERFACE": options.NetworkInterface,
		"OPENSHIFT_HA_MONITOR_PORT":      watchPort,
		"OPENSHIFT_HA_VRRP_ID_OFFSET":    VRRPIDOffset,
		"OPENSHIFT_HA_REPLICA_COUNT":     replicas,
		"OPENSHIFT_HA_USE_UNICAST":       "false",
		"OPENSHIFT_HA_IPTABLES_CHAIN":    options.IptablesChain,
		// "OPENSHIFT_HA_UNICAST_PEERS":     "127.0.0.1",
	})
	return env
}

//  Generate the IP failover monitor (keepalived) container configuration.
func generateFailoverMonitorContainerConfig(name string, options *ipfailover.IPFailoverConfigCmdOptions, env app.Environment) *kapi.Container {
	containerName := fmt.Sprintf("%s-%s", name, options.Type)

	imageName := fmt.Sprintf("%s-%s", options.Type, ipfailover.DefaultName)
	image := options.ImageTemplate.ExpandOrDie(imageName)

	//  Container port to expose the service interconnects between keepaliveds.
	ports := make([]kapi.ContainerPort, 1)
	ports[0] = kapi.ContainerPort{
		ContainerPort: int32(options.ServicePort),
		HostPort:      int32(options.ServicePort),
	}

	mounts := make([]kapi.VolumeMount, 1)
	mounts[0] = kapi.VolumeMount{
		Name:      libModulesVolumeName,
		ReadOnly:  true,
		MountPath: libModulesPath,
	}

	livenessProbe := &kapi.Probe{
		InitialDelaySeconds: 10,

		Handler: kapi.Handler{
			Exec: &kapi.ExecAction{
				Command: []string{"pgrep", "keepalived"},
			},
		},
	}

	privileged := true
	return &kapi.Container{
		Name:  containerName,
		Image: image,
		Ports: ports,
		SecurityContext: &kapi.SecurityContext{
			Privileged: &privileged,
		},
		ImagePullPolicy: kapi.PullIfNotPresent,
		VolumeMounts:    mounts,
		Env:             env.List(),
		LivenessProbe:   livenessProbe,
	}
}

//  Generate the IP failover monitor (keepalived) container configuration.
func generateContainerConfig(name string, options *ipfailover.IPFailoverConfigCmdOptions) ([]kapi.Container, error) {
	containers := make([]kapi.Container, 0)

	if len(options.VirtualIPs) < 1 {
		return containers, nil
	}

	config, err := getClientConfig(options.Credentials)
	if err != nil {
		return containers, err
	}

	env := generateEnvEntries(name, options, config)

	c := generateFailoverMonitorContainerConfig(name, options, env)
	if c != nil {
		containers = append(containers, *c)
	}

	return containers, nil
}

//  Generate the IP failover monitor (keepalived) container volume config.
func generateVolumeConfig() []kapi.Volume {
	//  The keepalived container needs access to the kernel modules
	//  directory in order to load the module.
	hostPath := &kapi.HostPathVolumeSource{Path: libModulesPath}
	src := kapi.VolumeSource{HostPath: hostPath}

	vol := kapi.Volume{Name: libModulesVolumeName, VolumeSource: src}
	return []kapi.Volume{vol}
}

//  Generates the node selector (if any) to use.
func generateNodeSelector(name string, selector map[string]string) map[string]string {
	// Check if the selector is default.
	selectorValue, ok := selector[ipfailover.DefaultName]
	if ok && len(selector) == 1 && selectorValue == name {
		return map[string]string{}
	}

	return selector
}

// GenerateDeploymentConfig generates an IP Failover deployment configuration.
func GenerateDeploymentConfig(name string, options *ipfailover.IPFailoverConfigCmdOptions, selector map[string]string) (*dapi.DeploymentConfig, error) {
	containers, err := generateContainerConfig(name, options)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"ipfailover": name,
	}
	podTemplate := &kapi.PodTemplateSpec{
		ObjectMeta: kapi.ObjectMeta{
			Labels: labels,
		},
		Spec: kapi.PodSpec{
			SecurityContext: &kapi.PodSecurityContext{
				HostNetwork: true,
			},
			NodeSelector:       generateNodeSelector(name, selector),
			Containers:         containers,
			Volumes:            generateVolumeConfig(),
			ServiceAccountName: options.ServiceAccount,
		},
	}
	return &dapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: dapi.DeploymentConfigSpec{
			Strategy: dapi.DeploymentStrategy{
				Type: dapi.DeploymentStrategyTypeRecreate,
			},
			// TODO: v0.1 requires a manual resize of the
			//       replicas to match current cluster state.
			//       In the future, the PerNodeController in
			//       kubernetes would remove the need for this
			//       manual intervention.
			Replicas: options.Replicas,
			Template: podTemplate,
			Triggers: []dapi.DeploymentTriggerPolicy{
				{Type: dapi.DeploymentTriggerOnConfigChange},
			},
		},
	}, nil
}
