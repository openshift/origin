package api

type EventType string

const (
	Added   EventType = "ADDED"
	Deleted EventType = "DELETED"
)

type SubnetRegistry interface {
	InitSubnets() error
	GetSubnets() (*[]Subnet, error)
	GetSubnet(minion string) (*Subnet, error)
	DeleteSubnet(minion string) error
	CreateSubnet(sn string, sub *Subnet) error
	WatchSubnets(receiver chan *SubnetEvent, stop chan bool) error

	InitMinions() error
	GetMinions() (*[]string, error)
	CreateMinion(minion string, data string) error
	WatchMinions(receiver chan *MinionEvent, stop chan bool) error

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

	GetServicesNetwork() (string, error)
	GetServices() (*[]Service, error)
	WatchServices(receiver chan *ServiceEvent, stop chan bool) error
}

type SubnetEvent struct {
	Type   EventType
	Minion string
	Sub    Subnet
}

type MinionEvent struct {
	Type   EventType
	Minion string
}

type Subnet struct {
	Minion string
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
