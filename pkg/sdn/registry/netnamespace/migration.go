package netnamespace

import (
	"fmt"
	"path"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/openshift-sdn/pkg/netid"
	"github.com/openshift/origin/pkg/sdn/api"
)

const (
	etcdNetNamespacePrefix string = "/registry/sdnnetnamespaces"

	// Marker for NetNamespace migration
	NetNamespaceMigrationAnnotation string = "pod.network.openshift.io/netns.migrated"
)

type Migrate struct {
	etcdHelper storage.Interface
	client     kclient.NamespaceInterface
}

func NewMigrate(etcdHelper storage.Interface, client kclient.NamespaceInterface) *Migrate {
	return &Migrate{
		etcdHelper: etcdHelper,
		client:     client,
	}
}

func (m *Migrate) Run() error {
	var netnsList api.NetNamespaceList

	err := m.etcdHelper.List(context.TODO(), etcdNetNamespacePrefix, "", storage.Everything, &netnsList)
	if err != nil {
		return fmt.Errorf("list network namespaces failed: %v, please retry the operation", err)
	}

	errList := []error{}
	for _, netns := range netnsList.Items {
		if err := m.migrateNetNamespace(&netns); err != nil {
			errList = append(errList, err)
			continue
		}
	}

	if len(errList) > 0 {
		errList = append(errList, fmt.Errorf("unexpected errors during NetNamespace migration, please retry the operation"))
	}
	return kerrors.NewAggregate(errList)
}

func (m *Migrate) migrateNetNamespace(netns *api.NetNamespace) error {
	// Get corresponding namespace for the NetNamespace object
	ns, err := m.client.Get(netns.NetName)
	// If corresponding namespace no longer exists, nothing to migrate in this case.
	if errors.IsNotFound(err) {
		err = m.deleteNetNamespace(netns)
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get namespace %q failed: %v", netns.NetName, err)
	}

	// Set netid as namespace annotation
	if err := netid.SetVNID(ns, netns.NetID); err != nil {
		return fmt.Errorf("set netid %d for namespace %q failed: %v", netns.NetID, ns.Name, err)
	}

	// Update namespace
	updatedNs, err := m.client.Update(ns)
	if errors.IsNotFound(err) {
		err = m.deleteNetNamespace(netns)
		if err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("update namespace %q failed: %v", ns.Name, err)
	}

	// Validate
	if id, err := netid.GetVNID(updatedNs); err != nil {
		return fmt.Errorf("failed to migrate netid %d from NetNamespace to annotation on namespace %q", netns.NetID, ns.Name)
	} else if id != netns.NetID {
		return fmt.Errorf("failed to migrate netid from NetNamespace to namespace %q, expected netid %d but got %d", ns.Name, netns.NetID, id)
	} else {
		glog.Infof("migrated netid %d from NetNamespace to annotation on namespace %q", netns.NetID, ns.Name)
	}

	// Delete processed NetNamespace object
	err = m.deleteNetNamespace(netns)
	if err != nil {
		return err
	}
	return nil
}

func (m *Migrate) deleteNetNamespace(netns *api.NetNamespace) error {
	// Persist network namespace as migrated so that new nodes can ignore
	// watching obsoleted NetNamespace resource.
	if err := m.setNetNamespaceMigration(netns); err != nil {
		return err
	}

	var out api.NetNamespace
	if err := m.etcdHelper.Delete(context.TODO(), path.Join(etcdNetNamespacePrefix, netns.ObjectMeta.Name), &out, nil); err != nil {
		return fmt.Errorf("failed to delete NetNamespace: %v, error: %v", netns, err)
	}
	return nil
}

func (m *Migrate) setNetNamespaceMigration(netns *api.NetNamespace) error {
	if netns.Annotations == nil {
		netns.Annotations = make(map[string]string)
	}
	netns.Annotations[NetNamespaceMigrationAnnotation] = "true"

	err := m.etcdHelper.GuaranteedUpdate(context.TODO(), path.Join(etcdNetNamespacePrefix, netns.ObjectMeta.Name), netns, true, nil, storage.SimpleUpdate(func(input runtime.Object) (output runtime.Object, err error) {
		existing := input.(*api.NetNamespace)
		if existing.ResourceVersion != netns.ResourceVersion {
			return nil, fmt.Errorf("failed to update NetNamespace: %v, resource version mismatch", netns)
		}
		return netns, nil
	}))
	if err != nil {
		return fmt.Errorf("failed to update NetNamespace: %v, error: %v", netns, err)
	}
	return nil
}

func IsNetNamespaceMigrated(netns *api.NetNamespace) bool {
	migrated := false
	if netns.Annotations != nil {
		_, migrated = netns.Annotations[NetNamespaceMigrationAnnotation]
	}
	return migrated
}
