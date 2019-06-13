package openshift_sdn

import (
	"io/ioutil"

	kclientv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	sdnnode "github.com/openshift/sdn/pkg/network/node"
)

const openshiftCNIFile string = "/etc/cni/net.d/80-openshift-network.conf"

// initSDN sets up the sdn process.
func (sdn *OpenShiftSDN) initSDN() error {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: sdn.informers.KubeClient.CoreV1().Events("")})
	sdn.sdnRecorder = eventBroadcaster.NewRecorder(scheme.Scheme, kclientv1.EventSource{Component: "openshift-sdn", Host: sdn.NodeConfig.NodeName})

	var err error
	sdn.OsdnNode, err = sdnnode.New(&sdnnode.OsdnNodeConfig{
		PluginName:         sdn.NodeConfig.NetworkConfig.NetworkPluginName,
		Hostname:           sdn.NodeConfig.NodeName,
		SelfIP:             sdn.NodeConfig.NodeIP,
		MTU:                sdn.NodeConfig.NetworkConfig.MTU,
		NetworkClient:      sdn.informers.NetworkClient,
		KClient:            sdn.informers.KubeClient,
		KubeInformers:      sdn.informers.KubeInformers,
		NetworkInformers:   sdn.informers.NetworkInformers,
		IPTablesSyncPeriod: sdn.ProxyConfig.IPTables.SyncPeriod.Duration,
		MasqueradeBit:      sdn.ProxyConfig.IPTables.MasqueradeBit,
		ProxyMode:          sdn.ProxyConfig.Mode,
		Recorder:           sdn.sdnRecorder,
	})
	return err
}

// runSDN starts the sdn node process. Returns.
func (sdn *OpenShiftSDN) runSDN() error {
	return sdn.OsdnNode.Start()
}

func (sdn *OpenShiftSDN) writeConfigFile() error {
	// Make an event that openshift-sdn started
	sdn.sdnRecorder.Eventf(&kclientv1.ObjectReference{Kind: "Node", Name: sdn.NodeConfig.NodeName}, kclientv1.EventTypeNormal, "Starting", "openshift-sdn done initializing node networking.")

	// Write our CNI config file out to disk to signal to kubelet that
	// our network plugin is ready
	return ioutil.WriteFile(openshiftCNIFile, []byte(`
{
  "cniVersion": "0.3.1",
  "name": "openshift-sdn",
  "type": "openshift-sdn"
}
`), 0644)
}
