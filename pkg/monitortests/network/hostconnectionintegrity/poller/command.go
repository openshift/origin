package poller

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Options are the command options.
type Options struct {
	genericclioptions.IOStreams

	MyNodeName        string
	Namespace         string
	StopConfigMap     string
	Peers             []string
	ControlPlaneNodes []string
	APIIntURL         string
	ProbeInterval     time.Duration
	ProbeTimeout      time.Duration
	MaxPeers          int
}

// NewCommand returns the watch-established-connections command.
func NewCommand(streams genericclioptions.IOStreams) *cobra.Command {
	o := &Options{
		IOStreams:     streams,
		ProbeInterval: 500 * time.Millisecond,
		ProbeTimeout:  2 * time.Second,
		MaxPeers:      3,
	}
	cmd := &cobra.Command{
		Use:   "watch-established-connections",
		Short: "Keep long-lived host-network connections open and report resets and silent stalls",
		Long: `Keep one long-lived connection open to every peer kubelet, to the api-int endpoint and (on
control-plane nodes) to the local kube-apiserver. Probe over it periodically and report, as one-line JSON
intervals on stdout, every episode where the established connection was reset or silently stopped passing
traffic while a new connection to the same target still worked.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&o.MyNodeName, "my-node-name", o.MyNodeName, "Name of the node the poller runs on.")
	cmd.Flags().StringVar(&o.Namespace, "namespace", os.Getenv("POD_NAMESPACE"), "Namespace of the stop configmap.")
	cmd.Flags().StringVar(&o.StopConfigMap, "stop-configmap", "stop-collecting", "Stop when this configmap exists in --namespace.")
	cmd.Flags().StringSliceVar(&o.Peers, "peers", o.Peers, "Comma separated nodeName=IP pairs of all nodes; the poller skips itself.")
	cmd.Flags().StringSliceVar(&o.ControlPlaneNodes, "control-plane-nodes", o.ControlPlaneNodes, "Names of control-plane nodes.")
	cmd.Flags().StringVar(&o.APIIntURL, "api-int-url", o.APIIntURL, "Internal API URL (infrastructure status.apiServerInternalURI).")
	cmd.Flags().DurationVar(&o.ProbeInterval, "probe-interval", o.ProbeInterval, "Time between probes.")
	cmd.Flags().IntVar(&o.MaxPeers, "max-peers", o.MaxPeers, "Number of peer kubelets probed from this node (the next N nodes in sorted name order, as a ring).")
	cmd.Flags().DurationVar(&o.ProbeTimeout, "probe-timeout", o.ProbeTimeout, "Timeout of a single probe.")
	return cmd
}

// Validate checks the options.
func (o *Options) Validate() error {
	if len(o.MyNodeName) == 0 {
		return fmt.Errorf("--my-node-name is required")
	}
	if len(o.Namespace) == 0 {
		return fmt.Errorf("--namespace (or POD_NAMESPACE) is required")
	}
	if o.MaxPeers < 1 {
		return fmt.Errorf("--max-peers must be at least 1")
	}
	if _, err := ParsePeers(o.Peers); err != nil {
		return err
	}
	return nil
}

// ParsePeers parses nodeName=IP pairs.
func ParsePeers(peers []string) (map[string]string, error) {
	ret := map[string]string{}
	for _, p := range peers {
		name, ip, ok := strings.Cut(p, "=")
		if !ok || len(name) == 0 || net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("invalid --peers entry %q, expected nodeName=IP", p)
		}
		ret[name] = ip
	}
	return ret, nil
}

// RingPeers returns the maxPeers nodes following myNodeName in sorted name order (wrapping around). Every node
// is probed by, and probes, the same number of peers, so all nodes are covered as source and target while the
// number of connections grows linearly with the cluster size.
func RingPeers(myNodeName string, peers map[string]string, maxPeers int) []string {
	names := make([]string, 0, len(peers))
	for n := range peers {
		names = append(names, n)
	}
	sort.Strings(names)
	idx := sort.SearchStrings(names, myNodeName)
	ret := []string{}
	for i := 1; i < len(names) && len(ret) < maxPeers; i++ {
		n := names[(idx+i)%len(names)]
		if n == myNodeName {
			continue
		}
		ret = append(ret, n)
	}
	return ret
}

// BuildTargets returns the targets probed from myNodeName.
func BuildTargets(myNodeName string, peers map[string]string, maxPeers int, controlPlaneNodes []string, apiIntURL string) []Target {
	targets := []Target{}
	for _, name := range RingPeers(myNodeName, peers, maxPeers) {
		ip := peers[name]
		targets = append(targets, Target{
			Backend: BackendPeerKubelet,
			Name:    name,
			URL:     fmt.Sprintf("https://%s/healthz", net.JoinHostPort(ip, "10250")),
		})
	}
	if len(apiIntURL) > 0 {
		targets = append(targets, Target{
			Backend: BackendAPIIntSelf,
			Name:    "api-int",
			URL:     strings.TrimSuffix(apiIntURL, "/") + "/readyz",
		})
	}
	for _, cp := range controlPlaneNodes {
		if cp == myNodeName {
			targets = append(targets, Target{
				Backend: BackendLocalhostAPIServer,
				Name:    "localhost",
				URL:     "https://localhost:6443/readyz",
			})
			break
		}
	}
	return targets
}

// Run runs the poller until the stop configmap appears or the process is signalled.
func (o *Options) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	peers, _ := ParsePeers(o.Peers)
	targets := BuildTargets(o.MyNodeName, peers, o.MaxPeers, o.ControlPlaneNodes, o.APIIntURL)
	emit := JSONEmitter(o.Out)
	klog.Infof("node/%s probing %d targets every %v (timeout %v)", o.MyNodeName, len(targets), o.ProbeInterval, o.ProbeTimeout)

	pollCtx, stopPolling := context.WithCancel(ctx)
	defer stopPolling()
	var wg sync.WaitGroup
	for _, t := range targets {
		w := &Watcher{
			NodeName:    o.MyNodeName,
			Target:      t,
			Established: NewHTTPProber(t.URL, true, o.ProbeTimeout),
			Fresh:       NewHTTPProber(t.URL, false, o.ProbeTimeout),
			Interval:    o.ProbeInterval,
			Now:         time.Now,
			Emit:        emit,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Run(pollCtx)
		}()
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
		case <-ticker.C:
			getCtx, cancelGet := context.WithTimeout(ctx, 3*time.Second)
			_, err := kubeClient.CoreV1().ConfigMaps(o.Namespace).Get(getCtx, o.StopConfigMap, metav1.GetOptions{})
			cancelGet()
			if err != nil {
				if !apierrors.IsNotFound(err) {
					klog.Warningf("unable to check stop configmap: %v", err)
				}
				continue
			}
			klog.Infof("found configmap %s/%s, stopping", o.Namespace, o.StopConfigMap)
		}
		break
	}
	stopPolling()
	wg.Wait()
	// Keep the process around so the pod logs can be collected; it is terminated by namespace deletion.
	<-ctx.Done()
	return nil
}
