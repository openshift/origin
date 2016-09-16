package integration

import (
	"strings"
	"testing"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"golang.org/x/net/context"

	"github.com/openshift/origin/pkg/util/leaderlease"
	testutil "github.com/openshift/origin/test/util"
)

func TestLeaderLeaseAcquire(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	c, err := testutil.MakeNewEtcdClient()
	if err != nil {
		t.Fatal(err)
	}
	client := etcdclient.NewKeysAPI(c)

	key := "/random/key"
	held := make(chan struct{})
	go func() {
		<-held
		if _, err := client.Delete(context.Background(), key, nil); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Deleted key")
	}()

	lease := leaderlease.NewEtcd(c, key, "holder", 10)
	ch := make(chan error, 1)
	go lease.AcquireAndHold(ch)

	<-ch
	glog.Infof("Lease acquired")
	close(held)
	if err, ok := <-ch; err == nil || !ok || !strings.Contains(err.Error(), "the lease has been lost") {
		t.Errorf("Expected error and open channel when lease was swapped: %v %t", err, ok)
	}
	<-ch
	glog.Infof("Lease lost")

	select {
	case _, ok := <-held:
		if ok {
			t.Error("did not acquire the lease")
		}
	default:
		t.Error("lease is still open")
	}
}

func TestLeaderLeaseWait(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	c, err := testutil.MakeNewEtcdClient()
	if err != nil {
		t.Fatal(err)
	}
	client := etcdclient.NewKeysAPI(c)
	key := "/random/key"

	if _, err := client.Set(context.Background(), key, "other", &etcdclient.SetOptions{TTL: time.Second, PrevExist: etcdclient.PrevNoExist}); err != nil {
		t.Fatal(err)
	}

	held := make(chan struct{})
	go func() {
		<-held
		if _, err := client.Delete(context.Background(), key, nil); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Deleted key")
	}()

	lease := leaderlease.NewEtcd(c, key, "holder", 10)
	ch := make(chan error, 1)
	go lease.AcquireAndHold(ch)

	<-ch
	glog.Infof("Lease acquired")
	close(held)
	if err, ok := <-ch; err == nil || !ok || !strings.Contains(err.Error(), "the lease has been lost") {
		t.Errorf("Expected error and open channel when lease was swapped: %v %t", err, ok)
	}
	<-ch
	glog.Infof("Lease lost")

	select {
	case _, ok := <-held:
		if ok {
			t.Error("did not acquire the lease")
		}
	default:
		t.Error("lease is still open")
	}
}

func TestLeaderLeaseSwapWhileWaiting(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	c, err := testutil.MakeNewEtcdClient()
	if err != nil {
		t.Fatal(err)
	}
	client := etcdclient.NewKeysAPI(c)
	key := "/random/key"

	if _, err := client.Set(context.Background(), key, "holder", &etcdclient.SetOptions{TTL: 10 * time.Second, PrevExist: etcdclient.PrevNoExist}); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)
		if _, err := client.Set(context.Background(), key, "other", &etcdclient.SetOptions{TTL: 10 * time.Second}); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Changed key ownership")
	}()

	lease := leaderlease.NewEtcd(c, key, "other", 10)
	ch := make(chan error, 1)
	go lease.AcquireAndHold(ch)

	<-ch
	glog.Infof("Lease acquired")
	lease.Release()
	if err, ok := <-ch; err == nil || !ok || !strings.Contains(err.Error(), "the lease has been lost") {
		t.Errorf("Expected error and open channel when lease was swapped: %v %t", err, ok)
	}
	<-ch
	glog.Infof("Lease gone")
}

func TestLeaderLeaseReacquire(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	c, err := testutil.MakeNewEtcdClient()
	if err != nil {
		t.Fatal(err)
	}
	client := etcdclient.NewKeysAPI(c)
	key := "/random/key"

	if _, err := client.Set(context.Background(), key, "holder", &etcdclient.SetOptions{TTL: time.Second, PrevExist: etcdclient.PrevNoExist}); err != nil {
		t.Fatal(err)
	}

	held := make(chan struct{})
	go func() {
		<-held
		if _, err := client.Delete(context.Background(), key, nil); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Deleted key")
	}()

	lease := leaderlease.NewEtcd(c, key, "holder", 1)
	ch := make(chan error, 1)
	go lease.AcquireAndHold(ch)

	<-ch
	glog.Infof("Lease acquired")
	time.Sleep(2 * time.Second)
	close(held)
	<-ch
	glog.Infof("Lease lost")

	select {
	case _, ok := <-held:
		if ok {
			t.Error("did not acquire the lease")
		}
	default:
		t.Error("lease is still open")
	}
}
