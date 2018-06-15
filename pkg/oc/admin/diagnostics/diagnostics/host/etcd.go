package host

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/coreos/etcd/clientv3"

	"bytes"

	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

// EtcdWriteVolume is a Diagnostic to check the writes occurring against etcd
// and organize them by volume.
type EtcdWriteVolume struct {
	MasterConfigLocation string
	V3Client             *clientv3.Client
	durationSpec         string
	duration             time.Duration
}

const (
	EtcdWriteName   = "EtcdWriteVolume"
	DurationParam   = "duration"
	DurationDefault = "1m"
)

func (d *EtcdWriteVolume) Name() string {
	return EtcdWriteName
}

func (d *EtcdWriteVolume) Description() string {
	return fmt.Sprintf("Check the volume of writes against etcd over a time period and classify them by operation and key")
}

func (d *EtcdWriteVolume) Requirements() (client bool, host bool) {
	return false, true
}

func (d *EtcdWriteVolume) AvailableParameters() []types.Parameter {
	return []types.Parameter{
		{DurationParam, "How long to perform the write test", &d.durationSpec, DurationDefault},
	}
}

func (d *EtcdWriteVolume) Complete(logger *log.Logger) error {
	v3Client, found, err := findEtcdClients(d.MasterConfigLocation, logger)
	if err != nil || !found {
		return err
	}
	d.V3Client = v3Client

	// determine the duration to run the check from either the deprecated env var, the flag, or the default
	s := os.Getenv("ETCD_WRITE_VOLUME_DURATION") // deprecated way
	if d.durationSpec != "" {
		s = d.durationSpec
	}
	if s == "" {
		s = DurationDefault
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("EtcdWriteVolume duration '%s' could not be parsed: %v", s, err)
	}
	d.duration = duration
	return nil
}

func (d *EtcdWriteVolume) CanRun() (bool, error) {
	if d.V3Client == nil {
		return false, fmt.Errorf("must have a V3 etcd client")
	}
	return true, nil
}

func (d *EtcdWriteVolume) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(EtcdWriteName)

	ctx := context.Background()
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(d.duration))
	defer cancel()

	keyStats := &keyCounter{}
	stats := &lockedKeyCounter{KeyCounter: keyStats}

	ch := d.V3Client.Watch(ctx, "/", clientv3.WithKeysOnly(), clientv3.WithPrefix())
	for resource := range ch {
		for _, evt := range resource.Events {
			if evt.Kv == nil {
				continue
			}
			action := fmt.Sprintf("v3:%s", evt.Type)
			stats.Inc(strings.Split(action+"/"+strings.TrimPrefix(string(evt.Kv.Key), "/"), "/"))
		}
	}

	bins := keyStats.Bins("", "/")
	sort.Sort(DescendingBins(bins))

	buf := &bytes.Buffer{}
	tw := tabwriter.NewWriter(buf, 0, 0, 1, ' ', 0)
	fmt.Fprintf(tw, "/\t%6d\t100.0%%\n", keyStats.count)
	for _, b := range bins {
		fmt.Fprintf(tw, "%s\t%6d\t%5.1f%%\n", b.Name, b.Count, float64(b.Count)/float64(keyStats.count)*100)
	}
	tw.Flush()
	r.Info("DEw2004", fmt.Sprintf("Measured %.1f writes/sec\n", float64(keyStats.count)/float64(d.duration/time.Second))+buf.String())

	return r
}

// findEtcdClients finds and loads etcd clients
func findEtcdClients(configFile string, logger *log.Logger) (*clientv3.Client, bool, error) {
	masterConfig, err := GetMasterConfig(configFile, logger)
	if err != nil {
		configErr := fmt.Errorf("Unreadable master config; skipping this diagnostic.")
		logger.Error("DE2001", configErr.Error())
		return nil, false, configErr
	}
	if len(masterConfig.EtcdClientInfo.URLs) == 0 {
		configErr := fmt.Errorf("No etcdClientInfo.urls defined; can't contact etcd")
		logger.Error("DE2002", configErr.Error())
		return nil, false, configErr
	}
	config, err := etcd.MakeEtcdClientV3Config(masterConfig.EtcdClientInfo)
	if err != nil {
		configErr := fmt.Errorf("Unable to create an etcd v3 client config: %v", err)
		logger.Error("DE2004", configErr.Error())
		return nil, false, configErr
	}
	config.DialTimeout = 5 * time.Second
	v3Client, err := clientv3.New(*config)
	if err != nil {
		configErr := fmt.Errorf("Unable to create an etcd v3 client: %v", err)
		logger.Error("DE2005", configErr.Error())
		return nil, false, configErr
	}
	return v3Client, true, nil
}

type KeyCounter interface {
	Inc(key []string)
}

type lockedKeyCounter struct {
	lock sync.Mutex
	KeyCounter
}

func (c *lockedKeyCounter) Inc(key []string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.KeyCounter.Inc(key)
}

type keyCounter struct {
	count    int
	children map[string]*keyCounter
}

func (b *keyCounter) Inc(key []string) {
	b.count++
	if len(key) == 0 {
		return
	}
	if b.children == nil {
		b.children = make(map[string]*keyCounter)
	}
	child, ok := b.children[key[0]]
	if !ok {
		child = &keyCounter{}
		b.children[key[0]] = child
	}
	child.Inc(key[1:])
}

type Bin struct {
	Name  string
	Count int
}

func (b *keyCounter) Bins(parent, separator string) []Bin {
	var bins []Bin
	for k, v := range b.children {
		childKey := parent + separator + k
		bins = append(bins, Bin{Name: childKey, Count: v.count})
		bins = append(bins, v.Bins(childKey, separator)...)
	}
	return bins
}

type DescendingBins []Bin

func (m DescendingBins) Len() int      { return len(m) }
func (m DescendingBins) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m DescendingBins) Less(i, j int) bool {
	if m[i].Name < m[j].Name {
		return true
	}
	return false
}
