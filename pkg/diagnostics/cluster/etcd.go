package cluster

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"

	"bytes"

	"github.com/openshift/origin/pkg/diagnostics/types"
)

// EtcdWriteVolume is a Diagnostic to check the writes occurring against etcd
// and organize them by volume.
type EtcdWriteVolume struct {
	V2Client etcdclient.Client
	V3Client *clientv3.Client
}

const (
	EtcdWriteName = "EtcdWriteVolume"
)

func (d *EtcdWriteVolume) duration() time.Duration {
	s := os.Getenv("ETCD_WRITE_VOLUME_DURATION")
	if len(s) == 0 {
		s = "1m"
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Errorf("ETCD_WRITE_VOLUME_DURATION could not be parsed: %v", err))
	}
	return duration
}

func (d *EtcdWriteVolume) Name() string {
	return EtcdWriteName
}

func (d *EtcdWriteVolume) Description() string {
	return fmt.Sprintf("Check the volume of writes against etcd and classify them by operation and key for %s", d.duration())
}

func (d *EtcdWriteVolume) CanRun() (bool, error) {
	if d.V2Client == nil {
		return false, fmt.Errorf("must have a V2 etcd client")
	}
	if d.V3Client == nil {
		return false, fmt.Errorf("must have a V3 etcd client")
	}
	return true, nil
}

func (d *EtcdWriteVolume) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(EtcdWriteName)

	var wg sync.WaitGroup

	duration := d.duration()
	ctx := context.Background()
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(duration))
	defer cancel()

	keyStats := &keyCounter{}
	stats := &lockedKeyCounter{KeyCounter: keyStats}

	wg.Add(2)
	go func() {
		defer wg.Done()
		keys := etcdclient.NewKeysAPI(d.V2Client)
		w := keys.Watcher("/", &etcdclient.WatcherOptions{Recursive: true})
		for {
			evt, err := w.Next(ctx)
			if err != nil {
				if err != context.DeadlineExceeded {
					r.Error("DEw2001", err, fmt.Sprintf("Unable to get a v2 watch event, stopping early: %v", err))
				}
				return
			}
			node := evt.Node
			if node == nil {
				node = evt.PrevNode
			}
			if node == nil {
				continue
			}
			action := fmt.Sprintf("v2:%s", evt.Action)
			stats.Inc(strings.Split(action+"/"+strings.TrimPrefix(evt.Node.Key, "/"), "/"))
		}
	}()
	go func() {
		defer wg.Done()
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
	}()
	wg.Wait()

	bins := keyStats.Bins("", "/")
	sort.Sort(DescendingBins(bins))

	buf := &bytes.Buffer{}
	tw := tabwriter.NewWriter(buf, 0, 0, 1, ' ', 0)
	fmt.Fprintf(tw, "/\t%6d\t100.0%%\n", keyStats.count)
	for _, b := range bins {
		fmt.Fprintf(tw, "%s\t%6d\t%5.1f%%\n", b.Name, b.Count, float64(b.Count)/float64(keyStats.count)*100)
	}
	tw.Flush()
	r.Info("DEw2004", fmt.Sprintf("Measured %.1f writes/sec\n", float64(keyStats.count)/float64(duration/time.Second))+buf.String())

	return r
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
