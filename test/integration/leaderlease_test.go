package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/util/leaderlease"
	testutil "github.com/openshift/origin/test/util"
)

func TestLeaderLeaseAcquire(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	client := testutil.NewEtcdClient()

	key := "/random/key"
	held := make(chan struct{})
	go func() {
		<-held
		if _, err := client.Delete(key, false); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Deleted key")
	}()

	lease := leaderlease.NewEtcd(client, key, "holder", 10)
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
	client := testutil.NewEtcdClient()
	key := "/random/key"

	if _, err := client.Create(key, "other", 1); err != nil {
		t.Fatal(err)
	}

	held := make(chan struct{})
	go func() {
		<-held
		if _, err := client.Delete(key, false); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Deleted key")
	}()

	lease := leaderlease.NewEtcd(client, key, "holder", 10)
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
	client := testutil.NewEtcdClient()
	key := "/random/key"

	if _, err := client.Create(key, "holder", 10); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)
		if _, err := client.Set(key, "other", 10); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Changed key ownership")
	}()

	lease := leaderlease.NewEtcd(client, key, "other", 10)
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
	client := testutil.NewEtcdClient()
	key := "/random/key"

	if _, err := client.Create(key, "holder", 1); err != nil {
		t.Fatal(err)
	}

	held := make(chan struct{})
	go func() {
		<-held
		if _, err := client.Delete(key, false); err != nil {
			t.Fatal(err)
		}
		glog.Infof("Deleted key")
	}()

	lease := leaderlease.NewEtcd(client, key, "holder", 1)
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
