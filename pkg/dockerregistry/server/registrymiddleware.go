package server

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/api/v2"
	regmw "github.com/docker/distribution/registry/middleware/registry"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
)

// enumRepoKind determines what kind of repositories will registry enumerate.
type enumRepoKind int

const (
	// enumRepoExisting makes Repositories fetch repository names from
	// imageStreams folder in etcd store.
	enumRepoExisting = iota
	// enumRepoDeletion makes Repositories fetch repository names from
	// imageStreamDeletions folder in etcd store.
	enumRepoDeletion
	// enumRepoLocal makes Repositories walk registry's storage.
	enumRepoLocal
)

func init() {
	regmw.Register("openshift", regmw.InitFunc(newRegistry))
}

// Sort namespaces by name
type ByNSName []kapi.Namespace

func (a ByNSName) Len() int           { return len(a) }
func (a ByNSName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByNSName) Less(i, j int) bool { return a[i].Name < a[j].Name }

type registry struct {
	distribution.Namespace

	ctx          context.Context
	kubeClient   kclient.Interface
	osClient     client.Interface
	registryAddr string
	enumRepoKind enumRepoKind
}

// newRegistry returns a new registry middleware.
func newRegistry(ctx context.Context, reg distribution.Namespace, options map[string]interface{}) (distribution.Namespace, error) {
	registryAddr := os.Getenv("REGISTRY_URL")
	if len(registryAddr) == 0 {
		return nil, errors.New("REGISTRY_URL is required")
	}

	kclient, err := NewRegistryKubernetesClient()
	if err != nil {
		return nil, err
	}
	osClient, err := NewRegistryOpenShiftClient()
	if err != nil {
		return nil, err
	}

	return &registry{
		Namespace:    reg,
		kubeClient:   kclient,
		osClient:     osClient,
		registryAddr: registryAddr,
	}, nil
}

// Scope returns the namespace scope for a registry. The registry
// will only serve repositories contained within this scope.
func (reg *registry) Scope() distribution.Scope {
	return distribution.GlobalScope
}

// Repository returns an instance of the repository tied to the registry.
// Instances should not be shared between goroutines but are cheap to
// allocate. In general, they should be request scoped.
func (reg *registry) Repository(ctx context.Context, name string) (distribution.Repository, error) {
	if err := v2.ValidateRepositoryName(name); err != nil {
		return nil, distribution.ErrRepositoryNameInvalid{
			Name:   name,
			Reason: err,
		}
	}

	repo, err := reg.Namespace.Repository(ctx, name)
	if err != nil {
		return nil, err
	}

	nameParts := strings.SplitN(repo.Name(), "/", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", repo.Name())
	}

	return &repository{
		Repository:     repo,
		ctx:            ctx,
		registryClient: reg.osClient,
		registryAddr:   reg.registryAddr,
		namespace:      nameParts[0],
		name:           nameParts[1],
	}, nil
}

// Repositories fills 'repos' with a lexigraphically sorted catalog of repositories
// up to the size of 'repos' and returns the value 'n' for the number of entries
// which were filled.  'last' contains an offset in the catalog, and 'err' will be
// set to io.EOF if there are no more entries to obtain.
// Returned repository names will either be fetched from etcd or local storage based
// on enumRepoKind setting.
func (reg *registry) Repositories(ctx context.Context, repos []string, last string) (n int, err error) {
	if reg.enumRepoKind == enumRepoLocal {
		return reg.Namespace.Repositories(ctx, repos, last)
	}

	lastNS := ""
	lastName := ""
	if last != "" {
		nameParts := strings.SplitN(last, "/", 2)
		if len(nameParts) != 2 {
			return 0, fmt.Errorf("invalid repository name %q: it must be of the format <project>/<name>", last)
		}
		lastNS = nameParts[0]
		lastName = nameParts[1]
	}

	if len(repos) == 0 {
		return
	}

	if reg.enumRepoKind == enumRepoExisting {
		nsList, err := reg.kubeClient.Namespaces().List(nil, nil)
		if err != nil {
			return 0, err
		}

		sort.Sort(ByNSName(nsList.Items))

		for _, ns := range nsList.Items {
			if lastNS != "" && ns.Name < lastNS {
				continue
			}

			isList, err := reg.osClient.ImageStreams(ns.Name).List(labels.Everything(), fields.Everything())
			if err != nil {
				log.Warnf("Failed to list image streams of %q namespace: %v", ns.Name, err)
				continue
			}
			isNames := make([]string, 0, len(isList.Items))
			for _, is := range isList.Items {
				if lastNS == ns.Name && is.Name <= lastName {
					continue
				}
				isNames = append(isNames, is.Name)
			}

			sort.Sort(sort.StringSlice(isNames))

			for _, name := range isNames {
				repos[n] = path.Join(ns.Name, name)
				n++
				if n >= len(repos) {
					break
				}
			}
		}
	} else {
		isdList, err := reg.osClient.ImageStreamDeletions().List(labels.Everything(), fields.Everything())
		if err != nil {
			return 0, fmt.Errorf("Failed to list image stream deletions: %v", err)
		}
		isNames := make([]string, 0, len(isdList.Items))
		for _, isd := range isdList.Items {
			isNameParts := strings.SplitN(isd.Name, ":", 2)
			if len(isNameParts) != 2 {
				log.Warnf("invalid name of image stream deletion %q expected <namespace>:<name>", isd.Name)
				continue
			}
			name := path.Join(isNameParts...)
			if name > last {
				isNames = append(isNames, name)
			}
		}

		sort.Sort(sort.StringSlice(isNames))

		for _, name := range isNames {
			repos[n] = path.Join(name, name)
			n++
			if n >= len(repos) {
				break
			}
		}
	}

	return
}
