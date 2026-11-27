package networking

import (
	"encoding/base64"
	"net/url"
	"reflect"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// internalCorefile is the on-prem Corefile as runtimecfg renders it for
// dnsRecordsType Internal.
const internalCorefile = `. {
    errors
    bufsize 512
    health :18080
    forward . 192.168.111.1 {
        policy sequential
    }
    cache 30
    reload
    template IN A ostest.test.metalkube.org {
        match .*[.]apps.ostest.test.metalkube.org
        answer "{{ .Name }} 60 in {{ .Type }} 192.168.111.4"
        fallthrough
    }
    template IN AAAA ostest.test.metalkube.org {
        match .*[.]apps.ostest.test.metalkube.org
        fallthrough
    }
    template IN A ostest.test.metalkube.org {
        match ^api.ostest.test.metalkube.org
        answer "{{ .Name }} 60 in {{ .Type }} 192.168.111.5"
        fallthrough
    }
    template IN AAAA ostest.test.metalkube.org {
        match ^api.ostest.test.metalkube.org
        fallthrough
    }
    template IN A ostest.test.metalkube.org {
        match ^api-int.ostest.test.metalkube.org
        answer "{{ .Name }} 60 in {{ .Type }} 192.168.111.5"
        fallthrough
    }
    template IN AAAA ostest.test.metalkube.org {
        match ^api-int.ostest.test.metalkube.org
        fallthrough
    }
    hosts {
        192.168.111.20 master-0 master-0.ostest.test.metalkube.org
        fallthrough
    }
}
`

// externalCorefile is the on-prem Corefile for dnsRecordsType External: MCO
// drops every template block.
const externalCorefile = `. {
    errors
    bufsize 512
    health :18080
    forward . 192.168.111.1 {
        policy sequential
    }
    cache 30
    reload
    
    hosts {
        192.168.111.20 master-0 master-0.ostest.test.metalkube.org
        fallthrough
    }
}
`

func TestParseOnPremDNSCorefile(t *testing.T) {
	for _, test := range []struct {
		name      string
		corefile  string
		want      onPremDNSCorefileRecords
		wantHosts bool
	}{
		{name: "internal", corefile: internalCorefile, want: onPremDNSCorefileRecords{API: true, APIInt: true, Apps: true}, wantHosts: true},
		{name: "external", corefile: externalCorefile, wantHosts: true},
		{name: "api-int only", corefile: "template IN A d {\n match ^api-int.d\n}\n", want: onPremDNSCorefileRecords{APIInt: true}},
		{name: "empty"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, hosts := parseOnPremDNSCorefile(test.corefile)
			if got != test.want || hosts != test.wantHosts {
				t.Errorf("got %s hosts=%t, want %s hosts=%t", got, hosts, test.want, test.wantHosts)
			}
		})
	}
}

func TestCheckOnPremDNSCorefile(t *testing.T) {
	for _, test := range []struct {
		name     string
		corefile string
		mode     configv1.DNSRecordsType
		wantErr  string
	}{
		{name: "internal matches", corefile: internalCorefile, mode: configv1.DNSRecordsTypeInternal},
		{name: "external matches", corefile: externalCorefile, mode: configv1.DNSRecordsTypeExternal},
		{name: "external with records", corefile: internalCorefile, mode: configv1.DNSRecordsTypeExternal, wantErr: "still answers cluster records"},
		{name: "internal without records", corefile: externalCorefile, mode: configv1.DNSRecordsTypeInternal, wantErr: "does not answer every cluster record"},
		{name: "no hosts block", corefile: "match ^api.d\nmatch ^api-int.d\nmatch .*[.]apps.d\n", mode: configv1.DNSRecordsTypeInternal, wantErr: "no hosts block"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkOnPremDNSCorefile(test.corefile, test.mode)
			switch {
			case test.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)):
				t.Errorf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

func TestOnPremDNSPlatformFromStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		ps     *configv1.PlatformStatus
		want   onPremDNSPlatform
		wantOK bool
	}{
		{name: "nil"},
		{name: "aws", ps: &configv1.PlatformStatus{Type: configv1.AWSPlatformType, AWS: &configv1.AWSPlatformStatus{}}},
		{name: "baremetal without member", ps: &configv1.PlatformStatus{Type: configv1.BareMetalPlatformType}},
		{
			name: "baremetal defaults",
			ps: &configv1.PlatformStatus{Type: configv1.BareMetalPlatformType, BareMetal: &configv1.BareMetalPlatformStatus{
				APIServerInternalIPs: []string{"192.168.111.5"},
				IngressIPs:           []string{"192.168.111.4"},
			}},
			want: onPremDNSPlatform{
				Type: configv1.BareMetalPlatformType, ShortName: "kni", LoadBalancer: configv1.LoadBalancerTypeOpenShiftManagedDefault,
				APIVIPs: []string{"192.168.111.5"}, IngressVIPs: []string{"192.168.111.4"},
			},
			wantOK: true,
		},
		{
			name: "vsphere external",
			ps: &configv1.PlatformStatus{Type: configv1.VSpherePlatformType, VSphere: &configv1.VSpherePlatformStatus{
				DNSRecordsType: configv1.DNSRecordsTypeExternal,
				LoadBalancer:   &configv1.VSpherePlatformLoadBalancer{Type: configv1.LoadBalancerTypeUserManaged},
			}},
			want: onPremDNSPlatform{
				Type: configv1.VSpherePlatformType, ShortName: "vsphere",
				DNSRecordsType: configv1.DNSRecordsTypeExternal, LoadBalancer: configv1.LoadBalancerTypeUserManaged,
			},
			wantOK: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := onPremDNSPlatformFromStatus(test.ps)
			if ok != test.wantOK {
				t.Fatalf("ok = %t, want %t", ok, test.wantOK)
			}
			if ok && !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestOnPremDNSPlatformEffective(t *testing.T) {
	if got := (onPremDNSPlatform{}).Effective(); got != configv1.DNSRecordsTypeInternal {
		t.Errorf("empty dnsRecordsType: got %q, want Internal", got)
	}
	if got := (onPremDNSPlatform{DNSRecordsType: configv1.DNSRecordsTypeExternal}).Effective(); got != configv1.DNSRecordsTypeExternal {
		t.Errorf("External: got %q", got)
	}
}

func TestOnPremDNSValidationKeys(t *testing.T) {
	for _, test := range []struct {
		name string
		ps   *configv1.PlatformStatus
		want []string
	}{
		{name: "nil", want: []string{"baremetal", "openstack", "vsphere", "nutanix"}},
		{name: "aws", ps: &configv1.PlatformStatus{Type: configv1.AWSPlatformType}, want: []string{"baremetal", "openstack", "vsphere", "nutanix"}},
		{name: "baremetal", ps: &configv1.PlatformStatus{Type: configv1.BareMetalPlatformType}, want: []string{"openstack", "vsphere", "nutanix"}},
		{name: "vsphere", ps: &configv1.PlatformStatus{Type: configv1.VSpherePlatformType}, want: []string{"baremetal", "openstack", "nutanix"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := onPremDNSValidationKeys(test.ps); !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestOnPremDNSStatusPatch(t *testing.T) {
	for _, test := range []struct {
		name string
		lb   configv1.PlatformLoadBalancerType
		want string
	}{
		{name: "with load balancer", lb: configv1.LoadBalancerTypeUserManaged, want: `{"status":{"platformStatus":{"openstack":{"dnsRecordsType":"External","loadBalancer":{"type":"UserManaged"}}}}}`},
		{name: "without load balancer", want: `{"status":{"platformStatus":{"openstack":{"dnsRecordsType":"External"}}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := onPremDNSStatusPatch("openstack", "External", test.lb)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestDNSRecordsTypeForStatusKey(t *testing.T) {
	ps := &configv1.PlatformStatus{
		BareMetal: &configv1.BareMetalPlatformStatus{DNSRecordsType: configv1.DNSRecordsTypeExternal},
		Nutanix:   &configv1.NutanixPlatformStatus{DNSRecordsType: configv1.DNSRecordsTypeInternal},
	}
	for key, want := range map[string]configv1.DNSRecordsType{
		"baremetal": configv1.DNSRecordsTypeExternal,
		"nutanix":   configv1.DNSRecordsTypeInternal,
		"openstack": "",
		"unknown":   "",
	} {
		if got := dnsRecordsTypeForStatusKey(ps, key); got != want {
			t.Errorf("%s: got %q, want %q", key, got, want)
		}
	}
	if got := dnsRecordsTypeForStatusKey(nil, "baremetal"); got != "" {
		t.Errorf("nil status: got %q", got)
	}
}

func TestOnPremDNSClusterDomain(t *testing.T) {
	for _, test := range []struct {
		url, want string
		wantErr   bool
	}{
		{url: "https://api-int.ostest.test.metalkube.org:6443", want: "ostest.test.metalkube.org"},
		{url: "https://api.ostest.test.metalkube.org:6443", wantErr: true},
		{url: "https://api-int.:6443", wantErr: true},
		{url: "", wantErr: true},
	} {
		infra := &configv1.Infrastructure{Status: configv1.InfrastructureStatus{APIServerInternalURL: test.url}}
		got, err := onPremDNSClusterDomain(infra)
		if (err != nil) != test.wantErr || got != test.want {
			t.Errorf("%q: got %q, %v; want %q, error=%t", test.url, got, err, test.want, test.wantErr)
		}
	}
}

func TestCorefileTemplateFromRenderedMachineConfig(t *testing.T) {
	mc := func(files ...interface{}) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "rendered-master-abc"},
			"spec": map[string]interface{}{"config": map[string]interface{}{
				"storage": map[string]interface{}{"files": files},
			}},
		}}
	}
	file := func(path, source string) interface{} {
		return map[string]interface{}{"path": path, "contents": map[string]interface{}{"source": source}}
	}

	got, found, err := corefileTemplateFromRenderedMachineConfig(mc(
		file("/etc/other", "data:,x"),
		file(onPremDNSCorefileTemplatePath, "data:text/plain;charset=utf-8;base64,"+base64.StdEncoding.EncodeToString([]byte(externalCorefile))),
	))
	if err != nil || !found || got != externalCorefile {
		t.Errorf("base64: got found=%t err=%v corefile=%q", found, err, got)
	}

	got, found, err = corefileTemplateFromRenderedMachineConfig(mc(file(onPremDNSCorefileTemplatePath, "data:,"+url.PathEscape("hosts {\n}\n"))))
	if err != nil || !found || got != "hosts {\n}\n" {
		t.Errorf("url-encoded: got found=%t err=%v corefile=%q", found, err, got)
	}

	if _, found, err := corefileTemplateFromRenderedMachineConfig(mc(file("/etc/other", "data:,x"))); found || err != nil {
		t.Errorf("missing file: got found=%t err=%v", found, err)
	}

	if _, found, err := corefileTemplateFromRenderedMachineConfig(mc(file(onPremDNSCorefileTemplatePath, "not-a-data-url"))); !found || err == nil {
		t.Errorf("bad source: got found=%t err=%v", found, err)
	}
}

func TestParseGetentAhosts(t *testing.T) {
	out := `192.168.111.5   STREAM api-int.ostest.test.metalkube.org
192.168.111.5   DGRAM
192.168.111.5   RAW
fd2e:6f44:5dd8:c956::5 STREAM
`
	want := []string{"192.168.111.5", "fd2e:6f44:5dd8:c956::5"}
	if got := parseGetentAhosts(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := parseGetentAhosts("Starting pod/debug ...\n"); len(got) != 0 {
		t.Errorf("noise: got %v", got)
	}
}

func TestCheckOnPremDNSResolution(t *testing.T) {
	for _, test := range []struct {
		name    string
		addrs   []string
		vips    []string
		wantErr string
	}{
		{name: "unresolved", wantErr: "does not resolve"},
		{name: "any address without vips", addrs: []string{"10.0.0.1"}},
		{name: "matches vips", addrs: []string{"192.168.111.5", "fd2e:6f44:5dd8:c956::5"}, vips: []string{"192.168.111.5", "fd2e:6f44:5dd8:c956:0::5"}},
		{name: "unexpected address", addrs: []string{"192.168.111.5", "10.0.0.1"}, vips: []string{"192.168.111.5"}, wantErr: "expected only the VIPs"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkOnPremDNSResolution("api-int.example", test.addrs, test.vips)
			switch {
			case test.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)):
				t.Errorf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

// TestOnPremDNSRecordsTags guards the frozen tag set: renaming it resets the
// test history in Sippy and the FeatureGate promotion check looks for it.
func TestOnPremDNSRecordsTags(t *testing.T) {
	for _, want := range []string{"[OCPFeatureGate:OnPremDNSRecords]", `[Jira:"Networking / On-Prem DNS"]`, "[sig-network]"} {
		if !strings.Contains(onPremDNSRecordsTags, want) {
			t.Errorf("onPremDNSRecordsTags lacks %s", want)
		}
	}
	// The API validation tests must not require the MCO API, which HyperShift
	// guest clusters do not serve.
	if strings.Contains(onPremDNSRecordsAPITags, "machineconfiguration.openshift.io") {
		t.Errorf("onPremDNSRecordsAPITags must not require the MCO API: %s", onPremDNSRecordsAPITags)
	}
}
