package api

type EventType string

const (
	Added   EventType = "ADDED"
	Deleted EventType = "DELETED"
)

type SubnetRegistry interface {
	InitSubnets() error
	GetSubnets() (*[]Subnet, error)
	GetSubnet(nodeName string) (*Subnet, error)
	DeleteSubnet(nodeName string) error
	CreateSubnet(sn string, sub *Subnet) error
	WatchSubnets(receiver chan *SubnetEvent, stop chan bool) error

	InitNodes() error
	GetNodes() (*[]string, error)
	CreateNode(nodeName string, data string) error
	WatchNodes(receiver chan *NodeEvent, stop chan bool) error

	WriteNetworkConfig(network string, subnetLength uint) error
	GetContainerNetwork() (string, error)
	GetSubnetLength() (uint64, error)
	CheckEtcdIsAlive(seconds uint64) bool

	WatchNamespaces(receiver chan *NamespaceEvent, stop chan bool) error
	WatchNetNamespaces(receiver chan *NetNamespaceEvent, stop chan bool) error
	GetNetNamespaces() ([]NetNamespace, error)
	GetNetNamespace(name string) (NetNamespace, error)
	WriteNetNamespace(name string, id uint) error
	DeleteNetNamespace(name string) error
}

type SubnetEvent struct {
	Type     EventType
	NodeName string
	Subnet   Subnet
}

type NodeEvent struct {
	Type     EventType
	NodeName string
	NodeIP   string
}

type Subnet struct {
	NodeIP   string
	SubnetIP string
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
