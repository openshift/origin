package common

import (
	"net"
	"sync"
	"time"
)

type fakeDNSReply struct {
	name string
	ips  []net.IP
	// ttl is *not* honored, we need something for get
	ttl time.Duration

	hasBeenUpdated bool

	nextQueryTime time.Time
	delay         time.Duration
}

type FakeDNS struct {

	// Protects dnsReplies operations
	lock sync.Mutex
	// Holds DNS name and its corresponding information
	dnsReplies []fakeDNSReply
}

func NewFakeDNS(dnsReplies []fakeDNSReply) *FakeDNS {
	return &FakeDNS{
		dnsReplies: dnsReplies,
	}
}

// Add: Not implemented
func (f *FakeDNS) Add(dns string) error {
	return nil
}

func (f *FakeDNS) Size() int {
	return 0
}

func (f *FakeDNS) Get(dns string) dnsValue {
	data := dnsValue{}
	return data
}
func (f *FakeDNS) Delete(dns string) {

}
func (f *FakeDNS) SetUpdating(dns string) error {
	return nil
}

// Update always assumes that if there is a reply the IP list always changes
func (f *FakeDNS) Update(dns string) (bool, error) {
	f.lock.Lock()
	delay := 0 * time.Second
	changed := false
	for i := range f.dnsReplies {
		if f.dnsReplies[i].name != dns {
			continue
		}
		if !f.dnsReplies[i].hasBeenUpdated {
			f.dnsReplies[i].hasBeenUpdated = true
			delay = f.dnsReplies[i].delay
			changed = true
			break
		}
	}
	f.lock.Unlock()
	time.Sleep(delay)
	return changed, nil
}

func (f *FakeDNS) GetNextQueryTime() (time.Time, string, bool) {
	f.lock.Lock()
	defer f.lock.Unlock()
	timeSet := false
	var minTime time.Time
	var dns string

	for i := range f.dnsReplies {
		if !f.dnsReplies[i].hasBeenUpdated {
			timeSet = true
			dns = f.dnsReplies[i].name
			minTime = f.dnsReplies[i].nextQueryTime
			return minTime, dns, timeSet
		}
	}

	return minTime, dns, timeSet
}
