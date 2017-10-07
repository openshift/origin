package dns

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	godbus "github.com/godbus/dbus"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	utildbus "k8s.io/kubernetes/pkg/util/dbus"
)

func Test_dnsmasqMonitor_run(t *testing.T) {
	m := &dnsmasqMonitor{
		dnsIP:     "127.0.0.1",
		dnsDomain: "test.domain",
	}
	m.initMetrics()
	conn := utildbus.NewFakeConnection()
	//fake := utildbus.NewFake(conn, nil)

	callCh := make(chan string, 1)
	conn.AddObject(dbusDnsmasqInterface, dbusDnsmasqPath, func(method string, args ...interface{}) ([]interface{}, error) {
		defer func() { callCh <- method }()
		switch method {
		case "uk.org.thekelleys.SetDomainServers":
			if len(args) != 1 {
				t.Errorf("unexpected args: %v", args)
				return nil, fmt.Errorf("unexpected args")
			}
			if arr, ok := args[0].([]string); !ok || !reflect.DeepEqual([]string{"/in-addr.arpa/127.0.0.1", "/test.domain/127.0.0.1"}, arr) {
				t.Errorf("unexpected args: %v", args)
				return nil, fmt.Errorf("unexpected args")
			}
			return nil, nil
		default:
			t.Errorf("unexpected method: %v", method)
			return nil, fmt.Errorf("unexpected method")
		}
	})

	stopCh := make(chan struct{})
	go m.run(conn, stopCh)

	// should always set on startup
	if s := <-callCh; s != "uk.org.thekelleys.SetDomainServers" {
		t.Errorf("unexpected call: %s", s)
	}
	select {
	case s := <-callCh:
		t.Fatalf("got an unexpected second call: %s", s)
	default:
	}

	// restart and ensure we get a set
	conn.EmitSignal(dbusDnsmasqInterface, dbusDnsmasqPath, dbusDnsmasqInterface, "Up")
	if s := <-callCh; s != "uk.org.thekelleys.SetDomainServers" {
		t.Errorf("unexpected call: %s", s)
	}

	// send a bogus signal and check whether anything was invoked
	conn.EmitSignal(dbusDnsmasqInterface, dbusDnsmasqPath, dbusDnsmasqInterface, "Ignore")
	select {
	case s := <-callCh:
		t.Fatalf("got an unexpected second call: %s", s)
	default:
	}

	// shutdown, send one more bogus signal to ensure the channel is empty and the goroutines are done
	close(stopCh)
	conn.EmitSignal(dbusDnsmasqInterface, dbusDnsmasqPath, dbusDnsmasqInterface, "Ignore")
	select {
	case s := <-callCh:
		t.Fatalf("got an unexpected second call: %s", s)
	default:
	}
}

type threadsafeDBusConn struct {
	lock sync.Mutex
	conn *utildbus.DBusFakeConnection
}

func (c *threadsafeDBusConn) BusObject() utildbus.Object {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.conn.BusObject()
}

func (c *threadsafeDBusConn) Object(name, path string) utildbus.Object {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.conn.Object(name, path)
}

func (c *threadsafeDBusConn) Signal(ch chan<- *godbus.Signal) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.conn.Signal(ch)
}

func (c *threadsafeDBusConn) EmitSignal(name, path, iface, signal string, args ...interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.conn.EmitSignal(name, path, iface, signal, args...)
}

func Test_dnsmasqMonitor_run_metrics(t *testing.T) {
	m := &dnsmasqMonitor{dnsIP: "127.0.0.1", dnsDomain: "test.domain"}
	m.initMetrics()
	fakeConn := utildbus.NewFakeConnection()
	conn := &threadsafeDBusConn{conn: fakeConn}

	callCh := make(chan string, 1)
	fakeConn.AddObject(dbusDnsmasqInterface, dbusDnsmasqPath, func(method string, args ...interface{}) ([]interface{}, error) {
		defer func() { callCh <- method }()
		switch method {
		case "uk.org.thekelleys.SetDomainServers":
			return nil, fmt.Errorf("unable to send error")
		default:
			t.Errorf("unexpected method: %v", method)
			return nil, fmt.Errorf("unexpected method")
		}
	})

	// stops the test
	stopCh := make(chan struct{})
	// prevents the test from exiting until all values are checked
	exitCh := make(chan struct{})
	go func() {
		m.run(conn, stopCh)
		expectCounterValue(t, 2, m.metricRestart)
		expectCounterVecValue(t, 1, m.metricError, "periodic")
		expectCounterVecValue(t, 2, m.metricError, "restart")
		close(exitCh)
	}()

	// should always set on startup
	if s := <-callCh; s != "uk.org.thekelleys.SetDomainServers" {
		t.Errorf("unexpected call: %s", s)
	}

	conn.EmitSignal(dbusDnsmasqInterface, dbusDnsmasqPath, dbusDnsmasqInterface, "Up")
	if s := <-callCh; s != "uk.org.thekelleys.SetDomainServers" {
		t.Errorf("unexpected call: %s", s)
	}
	conn.EmitSignal(dbusDnsmasqInterface, dbusDnsmasqPath, dbusDnsmasqInterface, "Up")
	if s := <-callCh; s != "uk.org.thekelleys.SetDomainServers" {
		t.Errorf("unexpected call: %s", s)
	}

	// shutdown, send one more bogus signal to ensure the channel is empty and the goroutines are done
	close(stopCh)
	conn.EmitSignal(dbusDnsmasqInterface, dbusDnsmasqPath, dbusDnsmasqInterface, "Ignore")
	select {
	case s := <-callCh:
		t.Fatalf("got an unexpected second call: %s", s)
	default:
	}
	<-exitCh
}

func expectCounterVecValue(t *testing.T, expect float64, vec *prometheus.CounterVec, labels ...string) {
	// loop a number of times to let the value stabilize, because the metric is incremented in a goroutine
	// we cannot signal from
	for i := 0; ; i++ {
		c, err := vec.GetMetricWithLabelValues(labels...)
		if err != nil {
			t.Error(err)
		}
		m := &dto.Metric{}
		if err := c.Write(m); err != nil {
			t.Error(err)
		}
		if m.Counter.GetValue() == expect {
			break
		}
		if m.Counter.GetValue() > expect || i > 100 {
			t.Errorf("%v: value %f != expected %f", labels, m.Counter.GetValue(), expect)
		}
		time.Sleep(time.Millisecond)
	}
}

func expectCounterValue(t *testing.T, expect float64, c prometheus.Counter) {
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		t.Error(err)
	}
	if m.Counter.GetValue() != expect {
		t.Errorf("value %f != expected %f", m.Counter.GetValue(), expect)
	}
}
