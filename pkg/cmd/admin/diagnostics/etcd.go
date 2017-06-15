package diagnostics

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"

	"github.com/openshift/origin/pkg/cmd/server/etcd"
	clustdiags "github.com/openshift/origin/pkg/diagnostics/cluster"
	"github.com/openshift/origin/pkg/diagnostics/host"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableEtcdDiagnostics contains the names of etcd diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableEtcdDiagnostics = sets.NewString(
		clustdiags.EtcdWriteName,
	)
	// defaultSkipEtcdDiagnostics is a list of diagnostics to skip by default
	defaultSkipEtcdDiagnostics = sets.NewString(
		clustdiags.EtcdWriteName,
	)
)

// buildEtcdDiagnostics builds cluster Diagnostic objects if etcd is configured
func (o DiagnosticsOptions) buildEtcdDiagnostics() ([]types.Diagnostic, bool, error) {
	requestedDiagnostics := availableEtcdDiagnostics.Intersection(sets.NewString(o.RequestedDiagnostics...)).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, true, nil // don't waste time on discovery
	}

	v2Client, v3Client, found, err := o.findEtcdClients(o.MasterConfigLocation)
	if !found {
		o.Logger.Notice("DE2000", "Could not configure etcd clients against the current config, so etcd diagnostics will be skipped")
		return nil, true, err
	}

	diagnostics := []types.Diagnostic{}
	for _, diagnosticName := range requestedDiagnostics {
		var d types.Diagnostic
		switch diagnosticName {
		case clustdiags.EtcdWriteName:
			d = &clustdiags.EtcdWriteVolume{V2Client: v2Client, V3Client: v3Client}
		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
		diagnostics = append(diagnostics, d)
	}
	return diagnostics, true, nil
}

// findEtcdClients finds and loads etcd clients
func (o DiagnosticsOptions) findEtcdClients(configFile string) (etcdclient.Client, *clientv3.Client, bool, error) {
	r := types.NewDiagnosticResult("")
	masterConfig, err := host.GetMasterConfig(r, configFile)
	if err != nil {
		configErr := fmt.Errorf("Unreadable master config; skipping this diagnostic.")
		o.Logger.Error("DE2001", configErr.Error())
		return nil, nil, false, configErr
	}
	if len(masterConfig.EtcdClientInfo.URLs) == 0 {
		configErr := fmt.Errorf("No etcdClientInfo.urls defined; can't contact etcd")
		o.Logger.Error("DE2002", configErr.Error())
		return nil, nil, false, configErr
	}
	v2Client, err := etcd.MakeEtcdClient(masterConfig.EtcdClientInfo)
	if err != nil {
		configErr := fmt.Errorf("Unable to create an etcd v2 client: %v", err)
		o.Logger.Error("DE2003", configErr.Error())
		return nil, nil, false, configErr
	}
	config, err := etcd.MakeEtcdClientV3Config(masterConfig.EtcdClientInfo)
	if err != nil {
		configErr := fmt.Errorf("Unable to create an etcd v3 client config: %v", err)
		o.Logger.Error("DE2004", configErr.Error())
		return nil, nil, false, configErr
	}
	config.DialTimeout = 5 * time.Second
	v3Client, err := clientv3.New(*config)
	if err != nil {
		configErr := fmt.Errorf("Unable to create an etcd v3 client: %v", err)
		o.Logger.Error("DE2005", configErr.Error())
		return nil, nil, false, configErr
	}
	return v2Client, v3Client, true, nil
}
