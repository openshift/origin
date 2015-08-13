package osdn

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/watch"

	osdnapi "github.com/openshift/openshift-sdn/ovssubnet/api"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	"github.com/openshift/origin/pkg/sdn/api"
)

type OsdnRegistryInterface struct {
	oClient osclient.Interface
	kClient kclient.Interface
}

func NewOsdnRegistryInterface(osClient *osclient.Client, kClient *kclient.Client) OsdnRegistryInterface {
	return OsdnRegistryInterface{osClient, kClient}
}

func (oi *OsdnRegistryInterface) InitSubnets() error {
	return nil
}

func (oi *OsdnRegistryInterface) GetSubnets() (*[]osdnapi.Subnet, error) {
	hostSubnetList, err := oi.oClient.HostSubnets().List()
	if err != nil {
		return nil, err
	}
	// convert HostSubnet to osdnapi.Subnet
	subList := make([]osdnapi.Subnet, 0)
	for _, subnet := range hostSubnetList.Items {
		subList = append(subList, osdnapi.Subnet{NodeIP: subnet.HostIP, Sub: subnet.Subnet})
	}
	return &subList, nil
}

func (oi *OsdnRegistryInterface) GetSubnet(node string) (*osdnapi.Subnet, error) {
	hs, err := oi.oClient.HostSubnets().Get(node)
	if err != nil {
		return nil, err
	}
	return &osdnapi.Subnet{NodeIP: hs.HostIP, Sub: hs.Subnet}, nil
}

func (oi *OsdnRegistryInterface) DeleteSubnet(node string) error {
	return oi.oClient.HostSubnets().Delete(node)
}

func (oi *OsdnRegistryInterface) CreateSubnet(node string, sub *osdnapi.Subnet) error {
	hs := &api.HostSubnet{
		TypeMeta:   kapi.TypeMeta{Kind: "HostSubnet"},
		ObjectMeta: kapi.ObjectMeta{Name: node},
		Host:       node,
		HostIP:     sub.NodeIP,
		Subnet:     sub.Sub,
	}
	_, err := oi.oClient.HostSubnets().Create(hs)
	return err
}

func (oi *OsdnRegistryInterface) WatchSubnets(receiver chan *osdnapi.SubnetEvent, stop chan bool) error {
	subnetEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.oClient.HostSubnets().List()
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.oClient.HostSubnets().Watch(resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &api.HostSubnet{}, subnetEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := subnetEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added, watch.Modified:
			// create SubnetEvent
			hs := obj.(*api.HostSubnet)
			receiver <- &osdnapi.SubnetEvent{Type: osdnapi.Added, Node: hs.Host, Sub: osdnapi.Subnet{NodeIP: hs.HostIP, Sub: hs.Subnet}}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			hs := obj.(*api.HostSubnet)
			receiver <- &osdnapi.SubnetEvent{Type: osdnapi.Deleted, Node: hs.Host, Sub: osdnapi.Subnet{NodeIP: hs.HostIP, Sub: hs.Subnet}}
		}
	}
}

func (oi *OsdnRegistryInterface) InitNodes() error {
	// return no error, as this gets initialized by apiserver
	return nil
}

func (oi *OsdnRegistryInterface) GetNodes() (*[]string, error) {
	nodes, err := oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	// convert kapi.NodeList to []string
	nodeList := make([]string, 0)
	for _, node := range nodes.Items {
		nodeList = append(nodeList, node.Name)
	}
	return &nodeList, nil
}

func (oi *OsdnRegistryInterface) CreateNode(node string, data string) error {
	return fmt.Errorf("Feature not supported in native mode. SDN cannot create/register nodes.")
}

func (oi *OsdnRegistryInterface) getNodeAddressMap() (map[types.UID]string, error) {
	nodeAddressMap := map[types.UID]string{}

	nodes, err := oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nodeAddressMap, err
	}
	for _, node := range nodes.Items {
		if len(node.Status.Addresses) > 0 {
			nodeAddressMap[node.ObjectMeta.UID] = node.Status.Addresses[0].Address
		}
	}
	return nodeAddressMap, nil
}

func (oi *OsdnRegistryInterface) WatchNodes(receiver chan *osdnapi.NodeEvent, stop chan bool) error {
	nodeEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.kClient.Nodes().List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Nodes().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &kapi.Node{}, nodeEventQueue, 4*time.Minute).Run()

	nodeAddressMap, err := oi.getNodeAddressMap()
	if err != nil {
		return err
	}

	for {
		eventType, obj, err := nodeEventQueue.Pop()
		if err != nil {
			return err
		}
		node := obj.(*kapi.Node)
		nodeIP := ""
		if len(node.Status.Addresses) > 0 {
			nodeIP = node.Status.Addresses[0].Address
		}

		switch eventType {
		case watch.Added:
			receiver <- &osdnapi.NodeEvent{Type: osdnapi.Added, Node: node.ObjectMeta.Name, NodeIP: nodeIP}
			nodeAddressMap[node.ObjectMeta.UID] = nodeIP
		case watch.Modified:
			oldNodeIP, ok := nodeAddressMap[node.ObjectMeta.UID]
			if ok && oldNodeIP != nodeIP {
				// Node Added event will handle update subnet if there is ip mismatch
				receiver <- &osdnapi.NodeEvent{Type: osdnapi.Added, Node: node.ObjectMeta.Name, NodeIP: nodeIP}
				nodeAddressMap[node.ObjectMeta.UID] = nodeIP
			}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			receiver <- &osdnapi.NodeEvent{Type: osdnapi.Deleted, Node: node.ObjectMeta.Name}
			delete(nodeAddressMap, node.ObjectMeta.UID)
		}
	}
}

func (oi *OsdnRegistryInterface) WriteNetworkConfig(network string, subnetLength uint) error {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	if err == nil {
		if cn.Network == network && cn.HostSubnetLength == int(subnetLength) {
			return nil
		} else {
			return fmt.Errorf("A network already exists and does not match the new network's parameters - Existing: (%s, %d); New: (%s, %d) ", cn.Network, cn.HostSubnetLength, network, subnetLength)
		}
	}
	cn = &api.ClusterNetwork{
		TypeMeta:         kapi.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta:       kapi.ObjectMeta{Name: "default"},
		Network:          network,
		HostSubnetLength: int(subnetLength),
	}
	_, err = oi.oClient.ClusterNetwork().Create(cn)
	return err
}

func (oi *OsdnRegistryInterface) GetContainerNetwork() (string, error) {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	return cn.Network, err
}

func (oi *OsdnRegistryInterface) GetSubnetLength() (uint64, error) {
	cn, err := oi.oClient.ClusterNetwork().Get("default")
	return uint64(cn.HostSubnetLength), err
}

func (oi *OsdnRegistryInterface) CheckEtcdIsAlive(seconds uint64) bool {
	// always assumed to be true as we run through the apiserver
	return true
}

func (oi *OsdnRegistryInterface) WatchNamespaces(receiver chan *osdnapi.NamespaceEvent, stop chan bool) error {
	nsEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.kClient.Namespaces().List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.kClient.Namespaces().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &kapi.Namespace{}, nsEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := nsEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added:
			// we should ignore the modified event because status updates cause unnecessary noise
			// the only time we would care about modified would be if the node changes its IP address
			// and hence all nodes need to update their vtep entries for the respective subnet
			// create nodeEvent
			ns := obj.(*kapi.Namespace)
			receiver <- &osdnapi.NamespaceEvent{Type: osdnapi.Added, Name: ns.ObjectMeta.Name}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			ns := obj.(*kapi.Namespace)
			receiver <- &osdnapi.NamespaceEvent{Type: osdnapi.Deleted, Name: ns.ObjectMeta.Name}
		}
	}
}

func (oi *OsdnRegistryInterface) WatchNetNamespaces(receiver chan *osdnapi.NetNamespaceEvent, stop chan bool) error {
	netNsEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	listWatch := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return oi.oClient.NetNamespaces().List()
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return oi.oClient.NetNamespaces().Watch(resourceVersion)
		},
	}
	cache.NewReflector(listWatch, &api.NetNamespace{}, netNsEventQueue, 4*time.Minute).Run()

	for {
		eventType, obj, err := netNsEventQueue.Pop()
		if err != nil {
			return err
		}
		switch eventType {
		case watch.Added:
			// we should ignore the modified event because status updates cause unnecessary noise
			// the only time we would care about modified would be if the node changes its IP address
			// and hence all nodes need to update their vtep entries for the respective subnet
			// create nodeEvent
			netns := obj.(*api.NetNamespace)
			receiver <- &osdnapi.NetNamespaceEvent{Type: osdnapi.Added, Name: netns.NetName, NetID: netns.NetID}
		case watch.Deleted:
			// TODO: There is a chance that a Delete event will not get triggered.
			// Need to use a periodic sync loop that lists and compares.
			netns := obj.(*api.NetNamespace)
			receiver <- &osdnapi.NetNamespaceEvent{Type: osdnapi.Deleted, Name: netns.NetName}
		}
	}
}

func (oi *OsdnRegistryInterface) GetNetNamespaces() ([]osdnapi.NetNamespace, error) {
	netNamespaceList, err := oi.oClient.NetNamespaces().List()
	if err != nil {
		return nil, err
	}
	// convert api.NetNamespace to osdnapi.NetNamespace
	nsList := make([]osdnapi.NetNamespace, 0)
	for _, netns := range netNamespaceList.Items {
		nsList = append(nsList, osdnapi.NetNamespace{Name: netns.Name, NetID: netns.NetID})
	}
	return nsList, nil
}

func (oi *OsdnRegistryInterface) GetNetNamespace(name string) (osdnapi.NetNamespace, error) {
	netns, err := oi.oClient.NetNamespaces().Get(name)
	if err != nil {
		return osdnapi.NetNamespace{}, err
	}
	return osdnapi.NetNamespace{Name: netns.Name, NetID: netns.NetID}, nil
}

func (oi *OsdnRegistryInterface) WriteNetNamespace(name string, id uint) error {
	netns := &api.NetNamespace{
		TypeMeta:   kapi.TypeMeta{Kind: "NetNamespace"},
		ObjectMeta: kapi.ObjectMeta{Name: name},
		NetName:    name,
		NetID:      id,
	}
	_, err := oi.oClient.NetNamespaces().Create(netns)
	return err
}

func (oi *OsdnRegistryInterface) DeleteNetNamespace(name string) error {
	return oi.oClient.NetNamespaces().Delete(name)
}
