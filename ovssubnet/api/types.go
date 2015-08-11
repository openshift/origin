package api

type EventType string

const (
	Added   EventType = "ADDED"
	Deleted EventType = "DELETED"
)

type SubnetRegistry interface {
	InitSubnets() error
	GetSubnets() (*[]Subnet, error)
	GetSubnet(node string) (*Subnet, error)
	DeleteSubnet(node string) error
	CreateSubnet(sn string, sub *Subnet) error
	WatchSubnets(receiver chan *SubnetEvent, stop chan bool) error

	InitNodes() error
	GetNodes() (*[]string, error)
	CreateNode(node string, data string) error
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
	Type EventType
	Node string
	Sub  Subnet
}

type NodeEvent struct {
	Type   EventType
	Node   string
	NodeIP string
}

type Subnet struct {
	NodeIP string
	Sub    string
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
