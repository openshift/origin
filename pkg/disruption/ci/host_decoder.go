package ci

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/sampler"

	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// NewAPIServerIdentityToHostNameDecoder returns a new
// HostNameDecoder instance that is capable of decoding the
// APIServerIdentity into the human readable hostname.
func NewAPIServerIdentityToHostNameDecoder(kubeClient kubernetes.Interface) (*apiServerIdentityDecoder, error) {
	client := kubeClient.CoordinationV1().Leases(metav1.NamespaceSystem)
	return &apiServerIdentityDecoder{client: client}, nil
}

var _ backend.HostNameDecoder = &apiServerIdentityDecoder{}
var _ sampler.Runner = &apiServerIdentityDecoder{}

type apiServerIdentityDecoder struct {
	client coordinationv1.LeaseInterface

	once sync.Once
	ctx  context.Context

	lock  sync.RWMutex
	hosts map[string]string
}

func (t *apiServerIdentityDecoder) Decode(encoded string) string {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if host, ok := t.hosts[encoded]; ok {
		return host
	}
	return encoded
}

func (t *apiServerIdentityDecoder) update(current map[string]string) {
	needUpdate := func() bool {
		t.lock.RLock()
		defer t.lock.RUnlock()
		if reflect.DeepEqual(t.hosts, current) {
			return false
		}
		return true
	}()
	if !needUpdate {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	// TODO: do we need to preserve the old host names in the map?
	t.hosts = current
}

func (t *apiServerIdentityDecoder) Run(stop context.Context) context.Context {
	t.once.Do(func() {
		t.ctx = t.run(stop)
	})
	return t.ctx
}

func (t *apiServerIdentityDecoder) run(stop context.Context) context.Context {
	done, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		framework.Logf("APIServerIdentity: host name decoder starting")
		defer framework.Logf("APIServerIdentity: host name decoder done")

		for {
			func() {
				ctx, cancel := context.WithTimeout(stop, 30*time.Second)
				defer cancel()
				current, err := retrieve(ctx, t.client)
				if err != nil {
					framework.Logf("error while trying to retrieve leases: %v", err)
					return
				}
				t.update(current)
			}()

			select {
			case <-stop.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}

func retrieve(ctx context.Context, client coordinationv1.LeaseInterface) (map[string]string, error) {
	label := labels.SelectorFromSet(map[string]string{"apiserver.kubernetes.io/identity": "kube-apiserver"})
	leases, err := client.List(ctx, metav1.ListOptions{LabelSelector: label.String()})
	if err != nil {
		return nil, fmt.Errorf("APIServerIdentity: LIST failed with error: %v", err)
	}
	if len(leases.Items) == 0 {
		return nil, fmt.Errorf("no leases object found with APIServerIdentity")
	}

	hosts := map[string]string{}
	for _, obj := range leases.Items {
		hosts[obj.Name] = obj.Labels["kubernetes.io/hostname"]
	}
	return hosts, nil
}
