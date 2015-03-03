package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/coreos/go-etcd/etcd"
	log "github.com/golang/glog"
)

type EventType string

const (
	Added   EventType = "ADDED"
	Deleted EventType = "DELETED"
)

type SubnetRegistry interface {
	InitSubnets() error
	InitMinions() error
	GetSubnets() (*[]Subnet, error)
	GetSubnet(minion string) (*Subnet, error)
	DeleteSubnet(minion string) error
	CreateSubnet(sn string, sub *Subnet) (*etcd.Response, error)
	CreateMinion(minion string, data string) error
	WatchSubnets(rev uint64, receiver chan *SubnetEvent, stop chan bool) error
	GetMinions() (*[]string, error)
	WatchMinions(rev uint64, receiver chan *MinionEvent, stop chan bool) error
	CheckEtcdIsAlive(seconds uint64) bool
}

type EtcdConfig struct {
	Endpoints  []string
	Keyfile    string
	Certfile   string
	CAFile     string
	SubnetPath string
	MinionPath string
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

type EtcdSubnetRegistry struct {
	mux     sync.Mutex
	cli     *etcd.Client
	etcdCfg *EtcdConfig
}

func newMinionEvent(action, key, value string) *MinionEvent {
	min := &MinionEvent{}
	switch action {
	case "delete", "deleted", "expired":
		min.Type = Deleted
	default:
		min.Type = Added
	}

	if key != "" {
		_, min.Minion = path.Split(key)
		return min
	}

	fmt.Printf("Error decoding minion event: nil key (%s,%s,%s).\n", action, key, value)
	return nil
}

func newSubnetEvent(resp *etcd.Response) *SubnetEvent {
	var value string
	_, minkey := path.Split(resp.Node.Key)
	var t EventType
	switch resp.Action {
	case "deleted", "delete", "expired":
		t = Deleted
		value = resp.PrevNode.Value
	default:
		t = Added
		value = resp.Node.Value
	}
	var sub Subnet
	if err := json.Unmarshal([]byte(value), &sub); err == nil {
		return &SubnetEvent{
			Type:   t,
			Minion: minkey,
			Sub:    sub,
		}
	}
	log.Errorf("Failed to unmarshal response: %v", resp)
	return nil
}

func newEtcdClient(c *EtcdConfig) (*etcd.Client, error) {
	if c.Keyfile != "" || c.Certfile != "" || c.CAFile != "" {
		return etcd.NewTLSClient(c.Endpoints, c.Certfile, c.Keyfile, c.CAFile)
	} else {
		return etcd.NewClient(c.Endpoints), nil
	}
}

func (sub *EtcdSubnetRegistry) CheckEtcdIsAlive(seconds uint64) bool {
	for {
		status := sub.client().SyncCluster()
		log.Infof("Etcd cluster status: %v", status)
		if status {
			return status
		}
		if seconds <= 0 {
			break
		}
		time.Sleep(5 * time.Second)
		seconds -= 5
	}
	return false
}

func NewEtcdSubnetRegistry(config *EtcdConfig) (SubnetRegistry, error) {
	r := &EtcdSubnetRegistry{
		etcdCfg: config,
	}

	var err error
	r.cli, err = newEtcdClient(config)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (sub *EtcdSubnetRegistry) InitSubnets() error {
	key := sub.etcdCfg.SubnetPath
	_, err := sub.client().SetDir(key, 0)
	return err
}

func (sub *EtcdSubnetRegistry) InitMinions() error {
	key := sub.etcdCfg.MinionPath
	_, err := sub.client().SetDir(key, 0)
	return err
}

func (sub *EtcdSubnetRegistry) GetMinions() (*[]string, error) {
	key := sub.etcdCfg.MinionPath
	resp, err := sub.client().Get(key, false, true)
	if err != nil {
		return nil, err
	}

	if resp.Node.Dir == false {
		return nil, errors.New("Minion path is not a directory")
	}

	minions := make([]string, 0)

	for _, node := range resp.Node.Nodes {
		if node.Key == "" {
			log.Errorf("Error unmarshalling GetMinions response node %s", node.Key)
			continue
		}
		_, minion := path.Split(node.Key)
		minions = append(minions, minion)
	}
	return &minions, nil
}

func (sub *EtcdSubnetRegistry) GetSubnets() (*[]Subnet, error) {
	key := sub.etcdCfg.SubnetPath
	resp, err := sub.client().Get(key, false, true)
	if err != nil {
		return nil, err
	}

	if resp.Node.Dir == false {
		return nil, errors.New("Subnet path is not a directory")
	}

	subnets := make([]Subnet, 0)

	for _, node := range resp.Node.Nodes {
		var s Subnet
		err := json.Unmarshal([]byte(node.Value), &s)
		if err != nil {
			log.Errorf("Error unmarshalling GetSubnets response for node %s: %s", node.Value, err.Error())
			continue
		}
		subnets = append(subnets, s)
	}
	return &subnets, err
}

func (sub *EtcdSubnetRegistry) GetSubnet(minionip string) (*Subnet, error) {
	key := path.Join(sub.etcdCfg.SubnetPath, minionip)
	resp, err := sub.client().Get(key, false, false)
	if err == nil {
		log.Infof("Unmarshalling response: %s", resp.Node.Value)
		var sub Subnet
		if err = json.Unmarshal([]byte(resp.Node.Value), &sub); err == nil {
			return &sub, nil
		}
		return nil, err
	}
	return nil, err
}

func (sub *EtcdSubnetRegistry) DeleteSubnet(minion string) error {
	key := path.Join(sub.etcdCfg.SubnetPath, minion)
	_, err := sub.client().Delete(key, false)
	return err
}

func (sub *EtcdSubnetRegistry) CreateMinion(minion string, data string) error {
	key := path.Join(sub.etcdCfg.MinionPath, minion)
	_, err := sub.client().Get(key, false, false)
	if err != nil {
		// good, it does not exist, write it
		_, err = sub.client().Create(key, data, 0)
		if err != nil {
			log.Errorf("Failed to write new subnet to etcd: %v", err)
			return err
		}
	}

	return nil
}

func (sub *EtcdSubnetRegistry) CreateSubnet(minion string, subnet *Subnet) (*etcd.Response, error) {
	subbytes, _ := json.Marshal(subnet)
	data := string(subbytes)
	log.Infof("Minion subnet structure: %s", data)
	key := path.Join(sub.etcdCfg.SubnetPath, minion)
	resp, err := sub.client().Create(key, data, 0)
	if err != nil {
		resp, err = sub.client().Update(key, data, 0)
		if err != nil {
			log.Errorf("Failed to write new subnet to etcd: %v", err)
			return nil, err
		}
	}

	return resp, nil
}

func (sub *EtcdSubnetRegistry) WatchMinions(rev uint64, receiver chan *MinionEvent, stop chan bool) error {
	key := sub.etcdCfg.MinionPath
	log.Infof("Watching %s for new minions.", key)
	for {
		resp, err := sub.watch(key, rev, stop)
		if resp == nil && err == nil {
			continue
		}
		rev = resp.Node.ModifiedIndex + 1
		log.Infof("Issuing a minion event: %v", resp)
		minevent := newMinionEvent(resp.Action, resp.Node.Key, resp.Node.Value)
		receiver <- minevent
	}
}

func (sub *EtcdSubnetRegistry) watch(key string, rev uint64, stop chan bool) (*etcd.Response, error) {
	rawResp, err := sub.client().RawWatch(key, rev, true, nil, stop)

	if err != nil {
		if err == etcd.ErrWatchStoppedByUser {
			return nil, err
		} else {
			log.Warningf("Temporary error while watching %s: %v\n", key, err)
			time.Sleep(time.Second)
			sub.resetClient()
			return nil, nil
		}
	}

	if len(rawResp.Body) == 0 {
		// etcd timed out, go back but recreate the client as the underlying
		// http transport gets hosed (http://code.google.com/p/go/issues/detail?id=8648)
		sub.resetClient()
		return nil, nil
	}

	return rawResp.Unmarshal()
}

func (sub *EtcdSubnetRegistry) WatchSubnets(rev uint64, receiver chan *SubnetEvent, stop chan bool) error {
	for {
		key := sub.etcdCfg.SubnetPath
		resp, err := sub.watch(key, rev, stop)
		if resp == nil && err == nil {
			continue
		}
		rev = resp.Node.ModifiedIndex + 1
		if err != nil && err == etcd.ErrWatchStoppedByUser {
			log.Infof("New subnet event error: %v", err)
			return err
		}
		subevent := newSubnetEvent(resp)
		log.Infof("New subnet event: %v, %v", subevent, resp)
		receiver <- subevent
	}
}

func (sub *EtcdSubnetRegistry) client() *etcd.Client {
	sub.mux.Lock()
	defer sub.mux.Unlock()
	return sub.cli
}

func (sub *EtcdSubnetRegistry) resetClient() {
	sub.mux.Lock()
	defer sub.mux.Unlock()

	var err error
	sub.cli, err = newEtcdClient(sub.etcdCfg)
	if err != nil {
		panic(fmt.Errorf("resetClient: error recreating etcd client: %v", err))
	}
}
