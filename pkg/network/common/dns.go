package common

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kexec "k8s.io/utils/exec"
)

const (
	dig = "dig"

	defaultTTL = 30 * time.Minute
)

type dnsValue struct {
	// All IPv4 addresses for a given domain name
	ips []net.IP
	// Time-to-live value from non-authoritative/cached name server for the domain
	ttl time.Duration
	// Holds (last dns lookup time + ttl), tells when to refresh IPs next time
	nextQueryTime time.Time
}

type DNS struct {
	// Runs shell commands
	execer kexec.Interface

	// Protects dnsMap operations
	lock sync.Mutex
	// Holds dns name and its corresponding information
	dnsMap map[string]dnsValue
}

func CheckDNSResolver() error {
	if _, err := exec.LookPath(dig); err != nil {
		return fmt.Errorf("%s is not installed", dig)
	}
	return nil
}

func NewDNS(execer kexec.Interface) *DNS {
	return &DNS{
		execer: execer,
		dnsMap: map[string]dnsValue{},
	}
}

func (d *DNS) Size() int {
	d.lock.Lock()
	defer d.lock.Unlock()

	return len(d.dnsMap)
}

func (d *DNS) Get(dns string) dnsValue {
	d.lock.Lock()
	defer d.lock.Unlock()

	data := dnsValue{}
	if res, ok := d.dnsMap[dns]; ok {
		data.ips = make([]net.IP, len(res.ips))
		copy(data.ips, res.ips)
		data.ttl = res.ttl
		data.nextQueryTime = res.nextQueryTime
	}
	return data
}

func (d *DNS) Add(dns string) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.dnsMap[dns] = dnsValue{}
	err, _ := d.updateOne(dns)
	if err != nil {
		delete(d.dnsMap, dns)
	}
	return err
}

func (d *DNS) Update() (error, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()

	errList := []error{}
	changed := false
	for dns := range d.dnsMap {
		err, updated := d.updateOne(dns)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		if updated {
			changed = true
		}
	}
	return kerrors.NewAggregate(errList), changed
}

func (d *DNS) updateOne(dns string) (error, bool) {
	// Due to lack of any go bindings for dns resolver that actually provides TTL value, we are relying on 'dig' shell command.
	// Output Format:
	// <domain-name>.		<<ttl from authoritative ns>	IN	A	<IP addr>
	out, err := d.execer.Command(dig, "+nocmd", "+noall", "+answer", "+ttlid", "a", dns).CombinedOutput()
	if err != nil || len(out) == 0 {
		return fmt.Errorf("failed to fetch IP addr and TTL value for domain: %q, err: %v", dns, err), false
	}
	outStr := strings.Trim(string(out[:]), "\n")

	res, ok := d.dnsMap[dns]
	if !ok {
		// Should not happen, all operations on dnsMap are synchronized by d.lock
		return fmt.Errorf("DNS value not found in dnsMap for domain: %q", dns), false
	}

	var minTTL time.Duration
	var ips []net.IP
	for _, line := range strings.Split(outStr, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 5 {
			continue
		}

		ttl, err := time.ParseDuration(fmt.Sprintf("%ss", fields[1]))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Invalid TTL value for domain: %q, err: %v, defaulting ttl=%s", dns, err, defaultTTL.String()))
			ttl = defaultTTL
		}
		if (minTTL.Seconds() == 0) || (minTTL.Seconds() > ttl.Seconds()) {
			minTTL = ttl
		}

		ip := net.ParseIP(fields[4])
		if ip != nil {
			ips = append(ips, ip)
		}
	}

	changed := false
	if !ipsEqual(res.ips, ips) {
		changed = true
	}
	res.ips = ips
	res.ttl = minTTL
	res.nextQueryTime = time.Now().Add(res.ttl)
	d.dnsMap[dns] = res
	return nil, changed
}

func (d *DNS) GetMinQueryTime() (time.Time, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()

	timeSet := false
	var minTime time.Time
	for _, res := range d.dnsMap {
		if (timeSet == false) || res.nextQueryTime.Before(minTime) {
			timeSet = true
			minTime = res.nextQueryTime
		}
	}

	return minTime, timeSet
}

func ipsEqual(oldips, newips []net.IP) bool {
	if len(oldips) != len(newips) {
		return false
	}

	for _, oldip := range oldips {
		found := false
		for _, newip := range newips {
			if oldip.Equal(newip) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
