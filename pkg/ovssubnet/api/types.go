package api

type EventType string

const (
	Added   EventType = "ADDED"
	Deleted EventType = "DELETED"
)

type SubnetRegistry interface {
	GetSubnets() ([]Subnet, string, error)
	GetSubnet(nodeName string) (*Subnet, error)
	DeleteSubnet(nodeName string) error
	CreateSubnet(sn string, sub *Subnet) error
	WatchSubnets(receiver chan<- *SubnetEvent, ready chan<- bool, startVersion <-chan string, stop <-chan bool) error

	GetNodes() ([]Node, string, error)
	WatchNodes(receiver chan<- *NodeEvent, ready chan<- bool, startVersion <-chan string, stop <-chan bool) error

	WriteNetworkConfig(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error
	GetClusterNetworkCIDR() (string, error)

	GetNamespaces() ([]string, string, error)
	WatchNamespaces(receiver chan<- *NamespaceEvent, ready chan<- bool, startVersion <-chan string, stop <-chan bool) error

	WatchNetNamespaces(receiver chan<- *NetNamespaceEvent, ready chan<- bool, startVersion <-chan string, stop <-chan bool) error
	GetNetNamespaces() ([]NetNamespace, string, error)
	GetNetNamespace(name string) (NetNamespace, error)
	WriteNetNamespace(name string, id uint) error
	DeleteNetNamespace(name string) error

	GetServicesNetworkCIDR() (string, error)
	GetServices() ([]Service, string, error)
	WatchServices(receiver chan<- *ServiceEvent, ready chan<- bool, startVersion <-chan string, stop <-chan bool) error
}

type Subnet struct {
	NodeIP     string
	SubnetCIDR string
}

type SubnetEvent struct {
	Type     EventType
	NodeName string
	Subnet   Subnet
}

type Node struct {
	Name string
	IP   string
}

type NodeEvent struct {
	Type EventType
	Node Node
}

type NetNamespace struct {
	Name  string
	NetID uint
}

type NetNamespaceEvent struct {
	Type  EventType
	Name  string
	NetID uint
}

type NamespaceEvent struct {
	Type EventType
	Name string
}

type ServiceProtocol string

const (
	TCP ServiceProtocol = "TCP"
	UDP ServiceProtocol = "UDP"
)

type Service struct {
	Name      string
	Namespace string
	IP        string
	Protocol  ServiceProtocol
	Port      uint
}

type ServiceEvent struct {
	Type    EventType
	Service Service
}
