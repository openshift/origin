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
