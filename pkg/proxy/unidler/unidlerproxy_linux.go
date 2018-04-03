// +build linux

package unidler

import (
	"container/list"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/directxman12/go-nfnetlink/nfqueue"
	"github.com/golang/glog"
	"github.com/google/gopacket/layers"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	api "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/util/iptables"
)

type servicePortInfo struct {
	types.NamespacedName
	Port     int32
	PortName string
	IP       string
	Protocol api.Protocol
}

func (p *servicePortInfo) String() string {
	return fmt.Sprintf("port %s (%v/%s) on service %s (%v)", p.PortName, p.Port, p.Protocol, p.NamespacedName.String(), p.IP)
}

type ipPort struct {
	ip   [16]byte // enough for IPv6 (can't compare a slice, only an array)
	port uint16
}

func (p ipPort) String() string {
	return net.JoinHostPort(net.IP(p.ip[:]).String(), fmt.Sprintf("%d", p.port))
}

func ipPortFromNet(ip net.IP, port uint16) (*ipPort, error) {
	ip6 := ip.To16()
	if ip6 == nil {
		return nil, fmt.Errorf("unkown IP address length for IP %v", ip)
	}
	res := ipPort{
		port: port,
	}
	copy(res.ip[:], ip6[:])

	return &res, nil
}

func ipPortFromKube(ipRaw string, port int32) (*ipPort, error) {
	ip6 := net.ParseIP(ipRaw).To16()
	if ip6 == nil {
		return nil, fmt.Errorf("invalid IP address %q", ipRaw)
	}

	if port < 0 || port > math.MaxUint16 {
		return nil, fmt.Errorf("invalid port %v", port)
	}

	res := ipPort{
		port: uint16(port),
	}
	copy(res.ip[:], ip6[:])

	return &res, nil
}

// agedPacket holds tcp-quad connection information and packet age.  It's used in
// packet holder to identify and age-out old packets.
type packetQuad struct {
	src  *ipPort
	dest *ipPort
}

// packetHolder holds packets, dropping them after a timeout or a certain number of held packets
// so we don't keep them around for too long.  It is *not* threadsafe.
type packetHolder struct {
	// packetsByDest maps destIP:destPort --> srcIP:srcPort --> packet
	packetsByDest map[ipPort]map[ipPort]*nfqueue.NFQPacket
	// packetAges holds ages for packets (in the form of packetQuads
	// sorted by time). When a packet ages out, if it's still in packetsByDest,
	// it will be dropped. Otherwise, it's simply removed from the list.
	// The list is sorted by time.
	packetAges *list.List

	// lastTickElem is the element of packetAges we saw as of the last tick.
	// Anything older (after) this element will be dropped.
	lastTickElem *list.Element
}

func newPacketHolder() *packetHolder {
	return &packetHolder{
		packetsByDest: make(map[ipPort]map[ipPort]*nfqueue.NFQPacket),
		packetAges:    list.New(),
	}
}

// Hold holds on to a particular packet.  The packet *must* be
// already validated to have TCP and IP (v4 or v6) layers.
func (p *packetHolder) Hold(packet *nfqueue.NFQPacket) error {
	tcpLayer := packet.Packet.Layer(layers.LayerTypeTCP).(*layers.TCP)
	ipv4LayerRaw := packet.Packet.Layer(layers.LayerTypeIPv4)
	ipv6LayerRaw := packet.Packet.Layer(layers.LayerTypeIPv6)

	var srcIP net.IP
	var dstIP net.IP
	if ipv6LayerRaw != nil {
		ipv6Layer := ipv6LayerRaw.(*layers.IPv6)
		srcIP = ipv6Layer.SrcIP
		dstIP = ipv6Layer.DstIP
	} else {
		ipv4Layer := ipv4LayerRaw.(*layers.IPv4)
		srcIP = ipv4Layer.SrcIP
		dstIP = ipv4Layer.DstIP
	}

	srcIdent, err := ipPortFromNet(srcIP, uint16(tcpLayer.SrcPort))
	if err != nil {
		return err
	}

	dstIdent, err := ipPortFromNet(dstIP, uint16(tcpLayer.DstPort))
	if err != nil {
		return err
	}

	forDst, ok := p.packetsByDest[*dstIdent]
	if !ok {
		forDst = make(map[ipPort]*nfqueue.NFQPacket)
		p.packetsByDest[*dstIdent] = forDst
	}

	if oldPacket, existed := forDst[*srcIdent]; existed {
		glog.V(6).Infof("dropping old packet to %s", dstIdent)
		oldPacket.Drop()
	}

	forDst[*srcIdent] = packet

	p.packetAges.PushFront(&packetQuad{
		src:  srcIdent,
		dest: dstIdent,
	})
	glog.V(6).Infof("held packet to %s", dstIdent)

	return nil
}

// Tick times out old packets that were present last tick.  Timed out packets
// will be dropped if they haven't already been popped.
func (p *packetHolder) Tick() {
	if p.lastTickElem != nil {
		nextElem := p.lastTickElem
		for nextElem != nil {
			// traverse the list, starting with this packet, dropping them all
			ident := nextElem.Value.(*packetQuad)
			nextElem = nextElem.Next()

			// it's fine to read from a nil map
			packet, present := p.packetsByDest[*ident.dest][*ident.src]
			if !present {
				continue
			}
			packet.Drop()
			delete(p.packetsByDest[*ident.dest], *ident.src)
			if len(p.packetsByDest[*ident.dest]) == 0 {
				delete(p.packetsByDest, *ident.dest)
			}
		}
		// TODO(directxman12): instrument this section with metrics
	}

	p.lastTickElem = p.packetAges.Front()
}

// Pop takes and removes all packets for a particular destination IP and port.
// It may return nil if there are no held packets.
func (p *packetHolder) Pop(destIP string, port int32) ([]*nfqueue.NFQPacket, error) {
	dstIdent, err := ipPortFromKube(destIP, port)
	if err != nil {
		return nil, err
	}

	forDst, ok := p.packetsByDest[*dstIdent]
	if !ok {
		return nil, nil
	}
	res := make([]*nfqueue.NFQPacket, 0, len(forDst))
	for _, packet := range forDst {
		res = append(res, packet)
		glog.V(10).Infof("popped packet to %s", dstIdent)
	}
	delete(p.packetsByDest, *dstIdent)
	return res, nil
}

// Close drops all held packets.
func (p *packetHolder) Close() {
	for _, packetsBySrc := range p.packetsByDest {
		for _, packet := range packetsBySrc {
			packet.Drop()
		}
	}
}

// jumpArgs constructs the arguements to iptables used to jump from
// the prerouting and output chains to the unidling chain
func jumpArgs(markBit uint) []string {
	markVal := uint32(1) << markBit
	markWithMask := fmt.Sprintf("%#08x/%#08x", markVal, markVal)

	return []string{
		"-m", "comment", "--comment", "handle OpenShift idled services",
		"-m", "mark", "!", "--mark", markWithMask,
		"-j", string(iptablesContainerUnidlingChain),
	}
}

// CleanupLeftovers cleans up leftover iptables rules from the unidler proxier.
func CleanupLeftovers(ipt iptables.Interface, markBit uint) error {
	args := jumpArgs(markBit)

	// drop the unidling chain jump rules
	if err := ipt.DeleteRule(iptables.TableNAT, iptables.ChainOutput, args...); err != nil {
		return err
	}
	if err := ipt.DeleteRule(iptables.TableNAT, iptables.ChainPrerouting, args...); err != nil {
		return err
	}

	// flush the chain
	if err := ipt.FlushChain(iptables.TableNAT, iptablesContainerUnidlingChain); err != nil {
		return err
	}

	// remove the actual chain
	if err := ipt.DeleteChain(iptables.TableNAT, iptablesContainerUnidlingChain); err != nil {
		return err
	}

	return nil
}

var iptablesContainerUnidlingChain iptables.Chain = "ORIGIN-UNIDLING-CONTAINER"

// NewUnidlerProxier creates a new proxier that sends unidling signals for services backed by
// idled scalables.  It watches for syn packets, then holds those syn packets until the corresponding
// service is removed from its list of services, resending the syn packet through the iptables chains.
// The given mark bit is used to prevent looping when resending the packet, and should be unique.
// The waitForRelease function is used to block releasing of packets until underlying network
// infrastructure is in place (some proxiers, such as iptables, don't sync immediately).
func NewUnidlerProxier(iptables iptables.Interface, queueNumber uint16, markBit uint, signaler NeedPodsSignaler, waitForRelease func()) (*UnidlerProxy, error) {
	markVal := uint32(1) << markBit
	proxy := &UnidlerProxy{
		iptables: iptables,
		queueNum: queueNumber,
		markBit:  markBit,
		mark:     markVal,

		ipCache:     make(map[string]types.NamespacedName),
		heldPackets: newPacketHolder(),
		// with the linux defaults, a client won't abort the connection
		// for just over 2 minutes-ish (max number of retries, plus some
		// extra time for the last retry to resolve)
		packetTimeout: 3 * time.Minute,

		signaler: signaler,

		waitForRelease: waitForRelease,
	}
	if err := proxy.iptablesInit(); err != nil {
		return nil, err
	}
	if err := proxy.iptablesFlush(); err != nil {
		return nil, err
	}
	return proxy, nil
}

// TODO(directxman12): we may need to consider a max number of packets as well,
// if we start running into perf issues.

// UnidlerProxy is a "proxy" that uses an nfqueue to intercept syn packets
// and send wakeup calls to Idlers.
type UnidlerProxy struct {
	iptables iptables.Interface
	queueNum uint16
	markBit  uint
	mark     uint32

	// ipCache maps IPs to service names
	ipCacheMu sync.RWMutex
	ipCache   map[string]types.NamespacedName

	// heldPackets contain all held packets by service-port-name
	heldPackets *packetHolder
	// packetTimeout determines the max amount of time that we hold
	// a packet for.  The *sending* OS might make have a shorter
	// duration that it will send retries for, so this really only
	// determines the potential max time.
	packetTimeout time.Duration

	signaler NeedPodsSignaler

	// waitForRelease is used to wait until underlying network infrastructure
	// is in place before releasing the packets.
	waitForRelease func()
}

func (p *UnidlerProxy) RunUntil(stopCh <-chan struct{}) error {
	// TODO(directxman12): we can actually configure the maxium queue length,
	// if needed (there's a netlink message). Keep an eye on this in case we need to.
	queue := nfqueue.NewNFQueue(p.queueNum)
	packetChan, err := queue.Open()
	if err != nil {
		return err
	}
	go func() {
		glog.V(4).Infof("Listening for packets on NFQueue %v", p.queueNum)
		defer glog.V(4).Infof("Done listening for packets on NFQueue %v", p.queueNum)
		defer queue.Close()
		defer p.heldPackets.Close()

		timeoutTicker := time.NewTicker(5 * time.Minute)
		defer timeoutTicker.Stop()

		for {
			select {
			case packet, ok := <-packetChan:
				if !ok {
					utilruntime.HandleError(fmt.Errorf("nfqueue packet channel closed unexpectedly"))
					return
				}
				if err := p.processPacket(packet); err != nil {
					utilruntime.HandleError(err)
				}
			case <-stopCh:
				return
			case <-timeoutTicker.C:
				p.heldPackets.Tick()
			}
		}
	}()

	return nil
}

// processPacket processes the given packet, potentialy saving it for
// later when the scalables wake up.
func (p *UnidlerProxy) processPacket(packet *nfqueue.NFQPacket) error {
	// only deal with TCP syn packets or udp packet for services that we know about
	tcpLayerRaw := packet.Packet.Layer(layers.LayerTypeTCP)
	udpLayerRaw := packet.Packet.Layer(layers.LayerTypeUDP)

	if tcpLayerRaw != nil {
		return p.processTCPPacket(packet)
	}
	if udpLayerRaw != nil {
		return p.processUDPPacket(packet)
	}

	glog.V(8).Infof("non-tcp/non-udp packet made its way to unidling proxy, accepting")
	packet.Accept()
	return nil
}

func (p *UnidlerProxy) getServiceForPacket(packet *nfqueue.NFQPacket) (svcName types.NamespacedName, knownService bool, ipPacket bool) {
	ipv4LayerRaw := packet.Packet.Layer(layers.LayerTypeIPv4)
	ipv6LayerRaw := packet.Packet.Layer(layers.LayerTypeIPv6)
	if ipv4LayerRaw == nil && ipv6LayerRaw == nil {
		return types.NamespacedName{}, false, false
	}

	// No dual stack support yet, so the service IP will either be v4 or v6
	p.ipCacheMu.RLock()
	defer p.ipCacheMu.RUnlock()

	var ipString string
	if ipv6LayerRaw != nil {
		ipString = ipv6LayerRaw.(*layers.IPv6).DstIP.String()
		svcName, knownService = p.ipCache[ipString]
	}
	if !knownService && ipv4LayerRaw != nil {
		ipString = ipv4LayerRaw.(*layers.IPv4).DstIP.String()
		svcName, knownService = p.ipCache[ipString]
	}

	if !knownService {
		glog.V(8).Infof("packet for unknown service %s made its way to unidling proxy", ipString)
	}

	return svcName, knownService, true
}

func (p *UnidlerProxy) processUDPPacket(packet *nfqueue.NFQPacket) error {
	svcName, knownService, ipPacket := p.getServiceForPacket(packet)

	if !ipPacket {
		glog.V(8).Infof("non-ip (v4 and v6) packet made its way to unidling proxy, accepting")
		packet.Accept()
		return nil
	}
	if !knownService {
		packet.Accept()
		return nil
	}
	// since UDP is lossy anyway, we just expect you to keep retrying yourself, so
	// we don't hang on to the packets
	glog.V(6).Infof("dropping udp packet for service %s and sending wakeup call", svcName)
	packet.Drop()
	return p.signaler.NeedPods(svcName)
}

func (p *UnidlerProxy) processTCPPacket(packet *nfqueue.NFQPacket) error {
	tcpLayerRaw := packet.Packet.Layer(layers.LayerTypeTCP)
	tcpLayer := tcpLayerRaw.(*layers.TCP)

	if !tcpLayer.SYN || tcpLayer.ACK {
		// ignore anything that's not just a syn packet
		glog.V(8).Infof("non-syn packet made its way to unidling proxy, accepting")
		packet.Accept()
		return nil
	}

	svcName, knownService, ipPacket := p.getServiceForPacket(packet)

	if !ipPacket {
		glog.V(8).Infof("non-ip (v4 and v6) packet made its way to unidling proxy, accepting")
		packet.Accept()
		return nil
	}

	if !knownService {
		packet.Accept()
		return nil
	}

	// hold the packet until later, because when we backoff,
	// there'll be one backoff period *before* timeout without
	// a retry, which gives us a *lot* longer unidling period
	// (around 2 minutes with the linux kernel default).
	glog.V(6).Infof("holding on to syn packet for service %s and sending wakeup call", svcName)
	if err := p.signaler.NeedPods(svcName); err != nil {
		utilruntime.HandleError(err)
	}
	return p.heldPackets.Hold(packet)
}

func (p *UnidlerProxy) iptablesInit() error {
	if _, err := p.iptables.EnsureChain(iptables.TableNAT, iptablesContainerUnidlingChain); err != nil {
		return err
	}
	args := jumpArgs(p.markBit)
	if _, err := p.iptables.EnsureRule(iptables.Prepend, iptables.TableNAT, iptables.ChainPrerouting, args...); err != nil {
		return err
	}
	if _, err := p.iptables.EnsureRule(iptables.Prepend, iptables.TableNAT, iptables.ChainOutput, args...); err != nil {
		return err
	}

	return nil
}

func (p *UnidlerProxy) iptablesFlush() error {
	if err := p.iptables.FlushChain(iptables.TableNAT, iptablesContainerUnidlingChain); err != nil {
		return err
	}

	return nil
}

func (p *UnidlerProxy) iptablesRuleArgs(info *servicePortInfo) []string {
	return []string{
		"-m", "comment", "--comment", info.String(),
		"-p", strings.ToLower(string(info.Protocol)),
		"-m", strings.ToLower(string(info.Protocol)),
		"--dport", fmt.Sprintf("%d", info.Port),
		"-d", fmt.Sprintf("%s/32", info.IP),
		"-j", "NFQUEUE", "--queue-num", fmt.Sprintf("%d", p.queueNum),
	}
}

func (p *UnidlerProxy) addServiceOnPort(info *servicePortInfo) error {
	existed, err := p.iptables.EnsureRule(iptables.Append, iptables.TableNAT, iptablesContainerUnidlingChain, p.iptablesRuleArgs(info)...)
	if err != nil {
		return err
	}
	if !existed {
		glog.V(3).Infof("Opened iptables from-container portal for %s to nfqueue %v", info.String(), p.queueNum)
	}

	return nil
}

func (p *UnidlerProxy) releasePackets(info *servicePortInfo) error {
	packets, err := p.heldPackets.Pop(info.IP, info.Port)
	if err != nil {
		return fmt.Errorf("unable to release held packets for %s: %v", info.String(), err)
	}
	go func() {
		p.waitForRelease()
		for _, packet := range packets {
			packet.Repeat(p.mark)
		}
	}()
	return nil
}

func (p *UnidlerProxy) removeServiceOnPort(info *servicePortInfo) error {
	// release all held packets, since we've either got the indication that we're good to go
	// unidling-wise, or we don't care about this port any more.  We release first since the
	// mark will take care of loops, and we want to avoid iptables lag.
	packetsErr := p.releasePackets(info)

	if err := p.iptables.DeleteRule(iptables.TableNAT, iptablesContainerUnidlingChain, p.iptablesRuleArgs(info)...); err != nil {
		return err
	}
	glog.V(3).Infof("Removed iptables from-container portal for %s to nfqueue %v", info.String(), p.queueNum)

	return packetsErr
}

func (p *UnidlerProxy) OnServiceAdd(service *api.Service) {
	// we don't care about services that aren't "normal" (i.e. loadbalancer, headless, etc)
	if !kapihelper.IsServiceIPSet(service) {
		return
	}
	svcName := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}

	p.ipCacheMu.Lock()
	p.ipCache[service.Spec.ClusterIP] = svcName
	p.ipCacheMu.Unlock()

	for _, port := range service.Spec.Ports {
		info := servicePortInfo{
			NamespacedName: svcName,
			PortName:       port.Name,
			Port:           port.Port,
			IP:             service.Spec.ClusterIP,
			Protocol:       port.Protocol,
		}
		p.addServiceOnPort(&info)
	}
}

func (p *UnidlerProxy) OnServiceUpdate(oldService, service *api.Service) {
	svcName := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}

	if !kapihelper.IsServiceIPSet(oldService) && kapihelper.IsServiceIPSet(service) {
		p.ipCacheMu.Lock()
		p.ipCache[service.Spec.ClusterIP] = svcName
		p.ipCacheMu.Unlock()
	} else if kapihelper.IsServiceIPSet(oldService) && !kapihelper.IsServiceIPSet(service) {
		p.ipCacheMu.Lock()
		delete(p.ipCache, service.Spec.ClusterIP)
		p.ipCacheMu.Unlock()
	}

	portsToDelete := make(map[api.ServicePort]struct{})
	if kapihelper.IsServiceIPSet(oldService) {
		for _, oldPort := range service.Spec.Ports {
			portsToDelete[oldPort] = struct{}{}
		}
	}

	if kapihelper.IsServiceIPSet(service) {
		for _, newPort := range service.Spec.Ports {
			if _, exists := portsToDelete[newPort]; exists {
				delete(portsToDelete, newPort)
				continue
			}
			info := servicePortInfo{
				NamespacedName: svcName,
				PortName:       newPort.Name,
				Port:           newPort.Port,
				IP:             service.Spec.ClusterIP,
				Protocol:       newPort.Protocol,
			}
			p.addServiceOnPort(&info)
		}
	}

	for port := range portsToDelete {
		info := servicePortInfo{
			NamespacedName: svcName,
			PortName:       port.Name,
			Port:           port.Port,
			IP:             service.Spec.ClusterIP,
			Protocol:       port.Protocol,
		}
		p.removeServiceOnPort(&info)
	}
}

func (p *UnidlerProxy) OnServiceDelete(service *api.Service) {
	// we don't care about services that aren't "normal" (i.e. loadbalancer, headless, etc)
	if !kapihelper.IsServiceIPSet(service) {
		return
	}
	svcName := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}

	p.ipCacheMu.Lock()
	delete(p.ipCache, service.Spec.ClusterIP)
	p.ipCacheMu.Unlock()

	for _, port := range service.Spec.Ports {
		info := servicePortInfo{
			NamespacedName: svcName,
			PortName:       port.Name,
			Port:           port.Port,
			IP:             service.Spec.ClusterIP,
			Protocol:       port.Protocol,
		}
		p.removeServiceOnPort(&info)
	}
}

func (p *UnidlerProxy) OnServiceSynced() {}
