package platformidentification

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

// Control plane topologies, in the vocabulary used by JobType.Topology and
// ClusterData.Topology.
const (
	TopologyHighlyAvailable = "ha"
	TopologySingleReplica   = "single"
	TopologyDualReplica     = "dual"
	TopologyExternal        = "external"
)

// topologyReadBackoff spreads four Infrastructure reads over roughly 7s. Callers
// resolve the topology once per run and change how strictly they judge failures
// based on the answer, so it is worth waiting out a brief apiserver blip.
var topologyReadBackoff = wait.Backoff{
	Duration: 1 * time.Second,
	Factor:   2.0,
	Jitter:   0.1,
	Steps:    4,
}

// IsReducedTopology reports whether topology names a control plane with fewer
// than three members — DualReplica (two-node fencing) or SingleReplica (SNO).
// Those clusters lose quorum whenever a single node reboots, so API errors while
// a node recovers are expected rather than symptomatic.
//
// An unknown topology is not reduced; use ResolveReducedTopology to resolve one
// that has not been read yet.
func IsReducedTopology(topology string) bool {
	return topology == TopologyDualReplica || topology == TopologySingleReplica
}

// GetControlPlaneTopology returns the cluster's control plane topology, using
// the same vocabulary as JobType.Topology.
//
// It reads the Infrastructure CR directly rather than going through
// BuildClusterData, which reports no topology at all when any of its unrelated
// ClusterVersion, Network, or architecture lookups fail.
func GetControlPlaneTopology(ctx context.Context, clientConfig *rest.Config) (string, error) {
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return "", fmt.Errorf("couldn't build config client: %w", err)
	}
	return controlPlaneTopology(ctx, configClient)
}

// controlPlaneTopology is the client-injectable half of GetControlPlaneTopology.
func controlPlaneTopology(ctx context.Context, configClient configclient.ConfigV1Interface) (string, error) {
	var infrastructure *configv1.Infrastructure
	var lastErr error
	waitErr := wait.ExponentialBackoffWithContext(ctx, topologyReadBackoff, func(ctx context.Context) (bool, error) {
		infrastructure, lastErr = configClient.Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		switch {
		case lastErr == nil:
			return true, nil
		case kapierrs.IsNotFound(lastErr):
			// The resource does not exist on this cluster; retrying cannot help.
			return false, lastErr
		default:
			return false, nil
		}
	})
	if waitErr != nil {
		if lastErr == nil {
			lastErr = waitErr
		}
		return "", fmt.Errorf("couldn't read infrastructures/cluster: %w", lastErr)
	}

	switch topology := infrastructure.Status.ControlPlaneTopology; topology {
	case configv1.HighlyAvailableTopologyMode:
		return TopologyHighlyAvailable, nil
	case configv1.SingleReplicaTopologyMode:
		return TopologySingleReplica, nil
	case configv1.DualReplicaTopologyMode:
		return TopologyDualReplica, nil
	case configv1.ExternalTopologyMode:
		return TopologyExternal, nil
	default:
		return "", fmt.Errorf("unrecognized control plane topology %q", topology)
	}
}

// ResolveReducedTopology reads the control plane topology and reports whether it
// is reduced (see IsReducedTopology).
//
// A topology that cannot be resolved is not classified as reduced. In that case
// err is non-nil and topology is empty, so callers can report why the topology
// was unavailable without downgrading a potential HA failure to a flake.
func ResolveReducedTopology(ctx context.Context, clientConfig *rest.Config) (reduced bool, topology string, err error) {
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return false, "", fmt.Errorf("couldn't build config client: %w", err)
	}
	return resolveReducedTopology(ctx, configClient)
}

// resolveReducedTopology is the client-injectable half of ResolveReducedTopology.
func resolveReducedTopology(ctx context.Context, configClient configclient.ConfigV1Interface) (bool, string, error) {
	topology, err := controlPlaneTopology(ctx, configClient)
	if err != nil {
		return false, "", err
	}
	return IsReducedTopology(topology), topology, nil
}
