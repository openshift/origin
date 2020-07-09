// Package haproxy is inspired by https://github.com/prometheus/haproxy_exporter
package haproxy

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	_ "net/http/pprof"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	client_model "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func TestExporter_scrape(t *testing.T) {
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")
	scrapes := []string{
		`public,FRONTEND,,,0,2,20000,162,18770,30715,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,1,,,,0,160,1,0,1,0,,0,1,162,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,1,162,160,0,0,0,,,0,0,,,,,,,
public_ssl,FRONTEND,,,1,32,20000,200,928408,2060591,0,0,0,,,,,OPEN,,,,,,,,,1,3,0,,,,0,0,0,50,,,,,,,,,,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,tcp,,0,50,200,,0,0,0,,,,,,,,,,,
be_sni,fe_sni,0,0,1,32,,184,900961,1776021,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,4,1,,184,,2,0,,51,,,,,,,,,,,,,,3,0,,,,,68,,,2,0,0,734,,,,,,,,,,,,127.0.0.1:10444,,tcp,,,,,,,,0,184,0,,,0,,29,6,0,46392,
be_sni,BACKEND,0,0,1,32,2000,184,900961,1776021,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,4,0,,184,,1,0,,51,,,,,,,,,,,,,,3,0,0,0,0,0,68,,,2,0,0,734,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,184,0,,,,,29,6,0,46392,
fe_sni,FRONTEND,,,1,32,20000,184,1072234,2875407,0,0,37,,,,,OPEN,,,,,,,,,1,5,0,,,,0,0,0,53,,,,5,426,242,42,0,0,,0,135,715,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,51,184,0,0,0,0,,,0,0,,,,,,,
be_no_sni,fe_no_sni,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,6,1,,0,,2,0,,0,,,,,,,,,,,,,,0,0,,,,,-1,,,0,0,0,0,,,,,,,,,,,,127.0.0.1:10443,,tcp,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_no_sni,BACKEND,0,0,0,0,2000,0,0,0,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,6,0,,0,,1,0,,0,,,,,,,,,,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,0,0,,,,,0,0,0,0,
fe_no_sni,FRONTEND,,,0,0,20000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,7,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,0,0,0,0,0,0,,,0,0,,,,,,,
openshift_default,BACKEND,0,0,0,1,6000,1,38,3288,0,0,,1,0,0,0,UP,0,0,0,,0,802,,,1,8,0,,0,,1,0,,1,,,,0,0,0,0,1,0,,,,1,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,http,roundrobin,,,,,,,0,0,0,0,0,,,0,0,0,0,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-nm72j:oauth-openshift:10.129.0.42:6443,0,0,0,1,,3,20092,221613,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,1,,3,,2,0,,1,L4OK,,0,,,,,,,,,,,0,0,,,,,67,,,1,1,0,130,,,,Layer4 check passed,,2,3,4,,,,10.129.0.42:6443,,tcp,,,,,,,,0,3,0,,,0,,2,1,0,63237,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-z5rhm:oauth-openshift:10.130.64.11:6443,0,0,0,3,,13,7355,62957,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,2,,13,,2,0,,1,L4OK,,1,,,,,,,,,,,0,0,,,,,70,,,0,0,0,1493,,,,Layer4 check passed,,2,3,4,,,,10.130.64.11:6443,,tcp,,,,,,,,0,13,0,,,0,,0,1,0,59559,
be_tcp:openshift-authentication:oauth-openshift,BACKEND,0,0,0,4,1,16,27447,284570,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,16,0,,16,,1,0,,1,,,,,,,,,,,,,,0,0,0,0,0,0,67,,,1,1,0,1615,,,,,,,,,,,,,,tcp,source,,,,,,,0,16,0,,,,,2,1,0,63237,
be_secure:openshift-console:console,pod:console-6db7cbb464-gr787:console:10.129.0.43:8443,0,0,0,8,,236,505655,2344127,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,1,,0,,2,0,,57,L6OK,,1,5,226,1,4,0,0,,,,,0,0,,,,,11,,,0,0,2,1350,,,,Layer6 check passed,,2,3,4,,,,10.129.0.43:8443,7e4a3da6d0368ecb934a4910245f83b4,http,,,,,,,,0,15,221,,,0,,0,4,26,16533,
be_secure:openshift-console:console,pod:console-6db7cbb464-8s44k:console:10.130.64.12:8443,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,2,,0,,2,0,,0,L6OK,,1,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer6 check passed,,2,3,4,,,,10.130.64.12:8443,5b10765dbf34d04f53986cf7ac1bf19c,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_secure:openshift-console:console,BACKEND,0,0,0,8,1,236,505655,2344127,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,17,0,,0,,1,0,,57,,,,5,226,1,4,0,0,,,,236,0,0,0,0,0,0,11,,,0,0,2,1350,,,,,,,,,,,,,1e2670d92730b515ce3a1bb65da45062,http,leastconn,,,,,,,0,15,221,0,0,,,0,4,26,16533,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-vn6lh:downloads:10.128.0.30:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,1,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.128.0.30:8080,ce739475136fa468d51cfcf5aad91b68,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-g7nsm:downloads:10.129.5.61:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,2,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.129.5.61:8080,450630300ddc04605decdd966ea57de6,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,BACKEND,0,0,0,0,1,0,0,0,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,18,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,0,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,a663438294fbd72a8e16964e97c8ecde,http,leastconn,,,,,,,0,0,0,0,0,,,0,0,0,0,
`,
		// increase the count of connections on the second console pod by 5
		`public,FRONTEND,,,0,2,20000,162,18770,30715,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,1,,,,0,160,1,0,1,0,,0,1,162,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,1,162,160,0,0,0,,,0,0,,,,,,,
public_ssl,FRONTEND,,,1,32,20000,200,928408,2060591,0,0,0,,,,,OPEN,,,,,,,,,1,3,0,,,,0,0,0,50,,,,,,,,,,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,tcp,,0,50,200,,0,0,0,,,,,,,,,,,
be_sni,fe_sni,0,0,1,32,,184,900961,1776021,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,4,1,,184,,2,0,,51,,,,,,,,,,,,,,3,0,,,,,68,,,2,0,0,734,,,,,,,,,,,,127.0.0.1:10444,,tcp,,,,,,,,0,184,0,,,0,,29,6,0,46392,
be_sni,BACKEND,0,0,1,32,2000,184,900961,1776021,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,4,0,,184,,1,0,,51,,,,,,,,,,,,,,3,0,0,0,0,0,68,,,2,0,0,734,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,184,0,,,,,29,6,0,46392,
fe_sni,FRONTEND,,,1,32,20000,184,1072234,2875407,0,0,37,,,,,OPEN,,,,,,,,,1,5,0,,,,0,0,0,53,,,,5,426,242,42,0,0,,0,135,715,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,51,184,0,0,0,0,,,0,0,,,,,,,
be_no_sni,fe_no_sni,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,6,1,,0,,2,0,,0,,,,,,,,,,,,,,0,0,,,,,-1,,,0,0,0,0,,,,,,,,,,,,127.0.0.1:10443,,tcp,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_no_sni,BACKEND,0,0,0,0,2000,0,0,0,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,6,0,,0,,1,0,,0,,,,,,,,,,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,0,0,,,,,0,0,0,0,
fe_no_sni,FRONTEND,,,0,0,20000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,7,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,0,0,0,0,0,0,,,0,0,,,,,,,
openshift_default,BACKEND,0,0,0,1,6000,1,38,3288,0,0,,1,0,0,0,UP,0,0,0,,0,802,,,1,8,0,,0,,1,0,,1,,,,0,0,0,0,1,0,,,,1,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,http,roundrobin,,,,,,,0,0,0,0,0,,,0,0,0,0,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-nm72j:oauth-openshift:10.129.0.42:6443,0,0,0,1,,3,20092,221613,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,1,,3,,2,0,,1,L4OK,,0,,,,,,,,,,,0,0,,,,,67,,,1,1,0,130,,,,Layer4 check passed,,2,3,4,,,,10.129.0.42:6443,,tcp,,,,,,,,0,3,0,,,0,,2,1,0,63237,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-z5rhm:oauth-openshift:10.130.64.11:6443,0,0,0,3,,13,7355,62957,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,2,,13,,2,0,,1,L4OK,,1,,,,,,,,,,,0,0,,,,,70,,,0,0,0,1493,,,,Layer4 check passed,,2,3,4,,,,10.130.64.11:6443,,tcp,,,,,,,,0,13,0,,,0,,0,1,0,59559,
be_tcp:openshift-authentication:oauth-openshift,BACKEND,0,0,0,4,1,16,27447,284570,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,16,0,,16,,1,0,,1,,,,,,,,,,,,,,0,0,0,0,0,0,67,,,1,1,0,1615,,,,,,,,,,,,,,tcp,source,,,,,,,0,16,0,,,,,2,1,0,63237,
be_secure:openshift-console:console,pod:console-6db7cbb464-gr787:console:10.129.0.43:8443,0,0,0,8,,241,505655,2344127,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,1,,0,,2,0,,57,L6OK,,1,5,226,1,4,0,0,,,,,0,0,,,,,11,,,0,0,2,1350,,,,Layer6 check passed,,2,3,4,,,,10.129.0.43:8443,7e4a3da6d0368ecb934a4910245f83b4,http,,,,,,,,0,15,221,,,0,,0,4,26,16533,
be_secure:openshift-console:console,pod:console-6db7cbb464-8s44k:console:10.130.64.12:8443,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,2,,0,,2,0,,0,L6OK,,1,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer6 check passed,,2,3,4,,,,10.130.64.12:8443,5b10765dbf34d04f53986cf7ac1bf19c,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_secure:openshift-console:console,BACKEND,0,0,0,8,1,236,505655,2344127,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,17,0,,0,,1,0,,57,,,,5,226,1,4,0,0,,,,236,0,0,0,0,0,0,11,,,0,0,2,1350,,,,,,,,,,,,,1e2670d92730b515ce3a1bb65da45062,http,leastconn,,,,,,,0,15,221,0,0,,,0,4,26,16533,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-vn6lh:downloads:10.128.0.30:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,1,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.128.0.30:8080,ce739475136fa468d51cfcf5aad91b68,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-g7nsm:downloads:10.129.5.61:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,2,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.129.5.61:8080,450630300ddc04605decdd966ea57de6,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,BACKEND,0,0,0,0,1,0,0,0,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,18,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,0,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,a663438294fbd72a8e16964e97c8ecde,http,leastconn,,,,,,,0,0,0,0,0,,,0,0,0,0,
`,
		// simulate a reset metrics due to the router reloading:
		// * set first console pod connections to 3
		// * set fe_sni connections to 0
		`public,FRONTEND,,,0,2,20000,162,18770,30715,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,1,,,,0,160,1,0,1,0,,0,1,162,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,1,162,160,0,0,0,,,0,0,,,,,,,
public_ssl,FRONTEND,,,1,32,20000,200,928408,2060591,0,0,0,,,,,OPEN,,,,,,,,,1,3,0,,,,0,0,0,50,,,,,,,,,,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,tcp,,0,50,200,,0,0,0,,,,,,,,,,,
be_sni,fe_sni,0,0,1,32,,0,900961,1776021,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,4,1,,184,,2,0,,51,,,,,,,,,,,,,,3,0,,,,,68,,,2,0,0,734,,,,,,,,,,,,127.0.0.1:10444,,tcp,,,,,,,,0,184,0,,,0,,29,6,0,46392,
be_sni,BACKEND,0,0,1,32,2000,0,900961,1776021,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,4,0,,184,,1,0,,51,,,,,,,,,,,,,,3,0,0,0,0,0,68,,,2,0,0,734,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,184,0,,,,,29,6,0,46392,
fe_sni,FRONTEND,,,1,32,20000,0,1072234,2875407,0,0,37,,,,,OPEN,,,,,,,,,1,5,0,,,,0,0,0,53,,,,5,426,242,42,0,0,,0,135,715,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,51,184,0,0,0,0,,,0,0,,,,,,,
be_no_sni,fe_no_sni,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,6,1,,0,,2,0,,0,,,,,,,,,,,,,,0,0,,,,,-1,,,0,0,0,0,,,,,,,,,,,,127.0.0.1:10443,,tcp,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_no_sni,BACKEND,0,0,0,0,2000,0,0,0,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,6,0,,0,,1,0,,0,,,,,,,,,,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,0,0,,,,,0,0,0,0,
fe_no_sni,FRONTEND,,,0,0,20000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,7,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,0,0,0,0,0,0,,,0,0,,,,,,,
openshift_default,BACKEND,0,0,0,1,6000,1,38,3288,0,0,,1,0,0,0,UP,0,0,0,,0,802,,,1,8,0,,0,,1,0,,1,,,,0,0,0,0,1,0,,,,1,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,http,roundrobin,,,,,,,0,0,0,0,0,,,0,0,0,0,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-nm72j:oauth-openshift:10.129.0.42:6443,0,0,0,1,,3,20092,221613,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,1,,3,,2,0,,1,L4OK,,0,,,,,,,,,,,0,0,,,,,67,,,1,1,0,130,,,,Layer4 check passed,,2,3,4,,,,10.129.0.42:6443,,tcp,,,,,,,,0,3,0,,,0,,2,1,0,63237,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-z5rhm:oauth-openshift:10.130.64.11:6443,0,0,0,3,,13,7355,62957,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,2,,13,,2,0,,1,L4OK,,1,,,,,,,,,,,0,0,,,,,70,,,0,0,0,1493,,,,Layer4 check passed,,2,3,4,,,,10.130.64.11:6443,,tcp,,,,,,,,0,13,0,,,0,,0,1,0,59559,
be_tcp:openshift-authentication:oauth-openshift,BACKEND,0,0,0,4,1,16,27447,284570,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,16,0,,16,,1,0,,1,,,,,,,,,,,,,,0,0,0,0,0,0,67,,,1,1,0,1615,,,,,,,,,,,,,,tcp,source,,,,,,,0,16,0,,,,,2,1,0,63237,
be_secure:openshift-console:console,pod:console-6db7cbb464-gr787:console:10.129.0.43:8443,0,0,0,8,,3,505655,2344127,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,1,,0,,2,0,,57,L6OK,,1,5,226,1,4,0,0,,,,,0,0,,,,,11,,,0,0,2,1350,,,,Layer6 check passed,,2,3,4,,,,10.129.0.43:8443,7e4a3da6d0368ecb934a4910245f83b4,http,,,,,,,,0,15,221,,,0,,0,4,26,16533,
be_secure:openshift-console:console,pod:console-6db7cbb464-8s44k:console:10.130.64.12:8443,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,2,,0,,2,0,,0,L6OK,,1,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer6 check passed,,2,3,4,,,,10.130.64.12:8443,5b10765dbf34d04f53986cf7ac1bf19c,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_secure:openshift-console:console,BACKEND,0,0,0,8,1,236,505655,2344127,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,17,0,,0,,1,0,,57,,,,5,226,1,4,0,0,,,,236,0,0,0,0,0,0,11,,,0,0,2,1350,,,,,,,,,,,,,1e2670d92730b515ce3a1bb65da45062,http,leastconn,,,,,,,0,15,221,0,0,,,0,4,26,16533,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-vn6lh:downloads:10.128.0.30:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,1,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.128.0.30:8080,ce739475136fa468d51cfcf5aad91b68,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-g7nsm:downloads:10.129.5.61:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,2,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.129.5.61:8080,450630300ddc04605decdd966ea57de6,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,BACKEND,0,0,0,0,1,0,0,0,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,18,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,0,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,a663438294fbd72a8e16964e97c8ecde,http,leastconn,,,,,,,0,0,0,0,0,,,0,0,0,0,
`,
		// increment the first console pod to 4 connections
		`public,FRONTEND,,,0,2,20000,162,18770,30715,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,1,,,,0,160,1,0,1,0,,0,1,162,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,1,162,160,0,0,0,,,0,0,,,,,,,
public_ssl,FRONTEND,,,1,32,20000,200,928408,2060591,0,0,0,,,,,OPEN,,,,,,,,,1,3,0,,,,0,0,0,50,,,,,,,,,,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,tcp,,0,50,200,,0,0,0,,,,,,,,,,,
be_sni,fe_sni,0,0,1,32,,0,900961,1776021,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,4,1,,184,,2,0,,51,,,,,,,,,,,,,,3,0,,,,,68,,,2,0,0,734,,,,,,,,,,,,127.0.0.1:10444,,tcp,,,,,,,,0,184,0,,,0,,29,6,0,46392,
be_sni,BACKEND,0,0,1,32,2000,0,900961,1776021,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,4,0,,184,,1,0,,51,,,,,,,,,,,,,,3,0,0,0,0,0,68,,,2,0,0,734,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,184,0,,,,,29,6,0,46392,
fe_sni,FRONTEND,,,1,32,20000,0,1072234,2875407,0,0,37,,,,,OPEN,,,,,,,,,1,5,0,,,,0,0,0,53,,,,5,426,242,42,0,0,,0,135,715,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,51,184,0,0,0,0,,,0,0,,,,,,,
be_no_sni,fe_no_sni,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,6,1,,0,,2,0,,0,,,,,,,,,,,,,,0,0,,,,,-1,,,0,0,0,0,,,,,,,,,,,,127.0.0.1:10443,,tcp,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_no_sni,BACKEND,0,0,0,0,2000,0,0,0,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,6,0,,0,,1,0,,0,,,,,,,,,,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,0,0,,,,,0,0,0,0,
fe_no_sni,FRONTEND,,,0,0,20000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,7,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,0,0,0,0,0,0,,,0,0,,,,,,,
openshift_default,BACKEND,0,0,0,1,6000,1,38,3288,0,0,,1,0,0,0,UP,0,0,0,,0,802,,,1,8,0,,0,,1,0,,1,,,,0,0,0,0,1,0,,,,1,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,http,roundrobin,,,,,,,0,0,0,0,0,,,0,0,0,0,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-nm72j:oauth-openshift:10.129.0.42:6443,0,0,0,1,,3,20092,221613,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,1,,3,,2,0,,1,L4OK,,0,,,,,,,,,,,0,0,,,,,67,,,1,1,0,130,,,,Layer4 check passed,,2,3,4,,,,10.129.0.42:6443,,tcp,,,,,,,,0,3,0,,,0,,2,1,0,63237,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-z5rhm:oauth-openshift:10.130.64.11:6443,0,0,0,3,,13,7355,62957,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,2,,13,,2,0,,1,L4OK,,1,,,,,,,,,,,0,0,,,,,70,,,0,0,0,1493,,,,Layer4 check passed,,2,3,4,,,,10.130.64.11:6443,,tcp,,,,,,,,0,13,0,,,0,,0,1,0,59559,
be_tcp:openshift-authentication:oauth-openshift,BACKEND,0,0,0,4,1,16,27447,284570,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,16,0,,16,,1,0,,1,,,,,,,,,,,,,,0,0,0,0,0,0,67,,,1,1,0,1615,,,,,,,,,,,,,,tcp,source,,,,,,,0,16,0,,,,,2,1,0,63237,
be_secure:openshift-console:console,pod:console-6db7cbb464-gr787:console:10.129.0.43:8443,0,0,0,8,,4,505655,2344127,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,1,,0,,2,0,,57,L6OK,,1,5,226,1,4,0,0,,,,,0,0,,,,,11,,,0,0,2,1350,,,,Layer6 check passed,,2,3,4,,,,10.129.0.43:8443,7e4a3da6d0368ecb934a4910245f83b4,http,,,,,,,,0,15,221,,,0,,0,4,26,16533,
be_secure:openshift-console:console,pod:console-6db7cbb464-8s44k:console:10.130.64.12:8443,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,2,,0,,2,0,,0,L6OK,,1,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer6 check passed,,2,3,4,,,,10.130.64.12:8443,5b10765dbf34d04f53986cf7ac1bf19c,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_secure:openshift-console:console,BACKEND,0,0,0,8,1,236,505655,2344127,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,17,0,,0,,1,0,,57,,,,5,226,1,4,0,0,,,,236,0,0,0,0,0,0,11,,,0,0,2,1350,,,,,,,,,,,,,1e2670d92730b515ce3a1bb65da45062,http,leastconn,,,,,,,0,15,221,0,0,,,0,4,26,16533,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-vn6lh:downloads:10.128.0.30:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,1,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.128.0.30:8080,ce739475136fa468d51cfcf5aad91b68,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-g7nsm:downloads:10.129.5.61:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,2,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.129.5.61:8080,450630300ddc04605decdd966ea57de6,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,BACKEND,0,0,0,0,1,0,0,0,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,18,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,0,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,a663438294fbd72a8e16964e97c8ecde,http,leastconn,,,,,,,0,0,0,0,0,,,0,0,0,0,
`,
		// simulate a second reset metrics due to the router reloading:
		// * set first console pod connections to 1
		// * set fe_sni connections to 0
		`public,FRONTEND,,,0,2,20000,162,18770,30715,0,0,0,,,,,OPEN,,,,,,,,,1,2,0,,,,0,0,0,1,,,,0,160,1,0,1,0,,0,1,162,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,1,162,160,0,0,0,,,0,0,,,,,,,
public_ssl,FRONTEND,,,1,32,20000,200,928408,2060591,0,0,0,,,,,OPEN,,,,,,,,,1,3,0,,,,0,0,0,50,,,,,,,,,,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,tcp,,0,50,200,,0,0,0,,,,,,,,,,,
be_sni,fe_sni,0,0,1,32,,0,900961,1776021,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,4,1,,184,,2,0,,51,,,,,,,,,,,,,,3,0,,,,,68,,,2,0,0,734,,,,,,,,,,,,127.0.0.1:10444,,tcp,,,,,,,,0,184,0,,,0,,29,6,0,46392,
be_sni,BACKEND,0,0,1,32,2000,0,900961,1776021,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,4,0,,184,,1,0,,51,,,,,,,,,,,,,,3,0,0,0,0,0,68,,,2,0,0,734,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,184,0,,,,,29,6,0,46392,
fe_sni,FRONTEND,,,1,32,20000,0,1072234,2875407,0,0,37,,,,,OPEN,,,,,,,,,1,5,0,,,,0,0,0,53,,,,5,426,242,42,0,0,,0,135,715,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,51,184,0,0,0,0,,,0,0,,,,,,,
be_no_sni,fe_no_sni,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,802,,,1,6,1,,0,,2,0,,0,,,,,,,,,,,,,,0,0,,,,,-1,,,0,0,0,0,,,,,,,,,,,,127.0.0.1:10443,,tcp,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_no_sni,BACKEND,0,0,0,0,2000,0,0,0,0,0,,0,0,0,0,UP,1,1,0,,0,802,0,,1,6,0,,0,,1,0,,0,,,,,,,,,,,,,,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,tcp,roundrobin,,,,,,,0,0,0,,,,,0,0,0,0,
fe_no_sni,FRONTEND,,,0,0,20000,0,0,0,0,0,0,,,,,OPEN,,,,,,,,,1,7,0,,,,0,0,0,0,,,,0,0,0,0,0,0,,0,0,0,,,0,0,0,0,,,,,,,,,,,,,,,,,,,,,http,,0,0,0,0,0,0,0,,,0,0,,,,,,,
openshift_default,BACKEND,0,0,0,1,6000,1,38,3288,0,0,,1,0,0,0,UP,0,0,0,,0,802,,,1,8,0,,0,,1,0,,1,,,,0,0,0,0,1,0,,,,1,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,,http,roundrobin,,,,,,,0,0,0,0,0,,,0,0,0,0,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-nm72j:oauth-openshift:10.129.0.42:6443,0,0,0,1,,3,20092,221613,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,1,,3,,2,0,,1,L4OK,,0,,,,,,,,,,,0,0,,,,,67,,,1,1,0,130,,,,Layer4 check passed,,2,3,4,,,,10.129.0.42:6443,,tcp,,,,,,,,0,3,0,,,0,,2,1,0,63237,
be_tcp:openshift-authentication:oauth-openshift,pod:oauth-openshift-5844b98b58-z5rhm:oauth-openshift:10.130.64.11:6443,0,0,0,3,,13,7355,62957,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,16,2,,13,,2,0,,1,L4OK,,1,,,,,,,,,,,0,0,,,,,70,,,0,0,0,1493,,,,Layer4 check passed,,2,3,4,,,,10.130.64.11:6443,,tcp,,,,,,,,0,13,0,,,0,,0,1,0,59559,
be_tcp:openshift-authentication:oauth-openshift,BACKEND,0,0,0,4,1,16,27447,284570,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,16,0,,16,,1,0,,1,,,,,,,,,,,,,,0,0,0,0,0,0,67,,,1,1,0,1615,,,,,,,,,,,,,,tcp,source,,,,,,,0,16,0,,,,,2,1,0,63237,
be_secure:openshift-console:console,pod:console-6db7cbb464-gr787:console:10.129.0.43:8443,0,0,0,8,,1,505655,2344127,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,1,,0,,2,0,,57,L6OK,,1,5,226,1,4,0,0,,,,,0,0,,,,,11,,,0,0,2,1350,,,,Layer6 check passed,,2,3,4,,,,10.129.0.43:8443,7e4a3da6d0368ecb934a4910245f83b4,http,,,,,,,,0,15,221,,,0,,0,4,26,16533,
be_secure:openshift-console:console,pod:console-6db7cbb464-8s44k:console:10.130.64.12:8443,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,17,2,,0,,2,0,,0,L6OK,,1,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer6 check passed,,2,3,4,,,,10.130.64.12:8443,5b10765dbf34d04f53986cf7ac1bf19c,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_secure:openshift-console:console,BACKEND,0,0,0,8,1,236,505655,2344127,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,17,0,,0,,1,0,,57,,,,5,226,1,4,0,0,,,,236,0,0,0,0,0,0,11,,,0,0,2,1350,,,,,,,,,,,,,1e2670d92730b515ce3a1bb65da45062,http,leastconn,,,,,,,0,15,221,0,0,,,0,4,26,16533,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-vn6lh:downloads:10.128.0.30:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,1,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.128.0.30:8080,ce739475136fa468d51cfcf5aad91b68,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,pod:downloads-564948bf9c-g7nsm:downloads:10.129.5.61:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,256,1,0,0,0,802,0,,1,18,2,,0,,2,0,,0,L4OK,,0,0,0,0,0,0,0,,,,,0,0,,,,,-1,,,0,0,0,0,,,,Layer4 check passed,,2,3,4,,,,10.129.5.61:8080,450630300ddc04605decdd966ea57de6,http,,,,,,,,0,0,0,,,0,,0,0,0,0,
be_edge_http:openshift-console:downloads,BACKEND,0,0,0,0,1,0,0,0,0,0,,0,0,0,0,UP,512,2,0,,0,802,0,,1,18,0,,0,,1,0,,0,,,,0,0,0,0,0,0,,,,0,0,0,0,0,0,0,-1,,,0,0,0,0,,,,,,,,,,,,,a663438294fbd72a8e16964e97c8ecde,http,leastconn,,,,,,,0,0,0,0,0,,,0,0,0,0,
`,
	}
	var index int

	e, err := NewExporter(defaultOptions(PrometheusOptions{ScrapeURI: "http://localhost"}))
	if err != nil {
		t.Fatal(err)
	}
	e.fetch = func() (io.ReadCloser, error) {
		r := strings.NewReader(scrapes[index])
		if index < (len(scrapes) - 1) {
			index++
		}
		return ioutil.NopCloser(r), nil
	}
	r := prometheus.NewRegistry()
	if err := r.Register(e); err != nil {
		t.Fatal(err)
	}

	connectionsTotalIndex := 7
	secondConsolePodID := metricID{proxyType: "2", proxyName: "be_secure:openshift-console:console", serverName: "pod:console-6db7cbb464-gr787:console:10.129.0.43:8443"}

	// perform the first scrape
	f := gatherMetrics(t, r)
	if e.counterValues != nil {
		t.Fatal(e.counterValues)
	}
	mustHaveMetric(t, f, "haproxy_exporter_total_scrapes", 1)
	// this metric should stay the same across all runs because it is not a counter
	mustHaveMetric(t, f, "haproxy_server_max_sessions", 32, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 184, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 236, map[string]string{"namespace": "openshift-console", "pod": "console-6db7cbb464-gr787", "route": "console", "server": "10.129.0.43:8443", "service": "console"})

	// simulate reload
	e.CollectNow()
	if e.counterValues[secondConsolePodID][e.counterIndices[connectionsTotalIndex]] != 241 {
		t.Fatalf("incorrect counter: %#v", e.counterValues[secondConsolePodID])
	}

	e.lastScrape = nil
	f = gatherMetrics(t, r)
	if e.counterValues[secondConsolePodID][e.counterIndices[connectionsTotalIndex]] != 241 {
		t.Fatalf("incorrect counter: %#v", e.counterValues[secondConsolePodID])
	}

	mustHaveMetric(t, f, "haproxy_exporter_total_scrapes", 3)
	mustHaveMetric(t, f, "haproxy_server_max_sessions", 32, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 184, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 244, map[string]string{"namespace": "openshift-console", "pod": "console-6db7cbb464-gr787", "route": "console", "server": "10.129.0.43:8443", "service": "console"})

	now := time.Now()
	e.lastScrape = &now
	e.scrapeInterval = time.Hour
	f = gatherMetrics(t, r)
	if e.counterValues[secondConsolePodID][e.counterIndices[connectionsTotalIndex]] != 241 {
		t.Fatalf("incorrect counter: %#v", e.counterValues[secondConsolePodID])
	}

	mustHaveMetric(t, f, "haproxy_exporter_total_scrapes", 3)
	mustHaveMetric(t, f, "haproxy_server_max_sessions", 32, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 184, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 244, map[string]string{"namespace": "openshift-console", "pod": "console-6db7cbb464-gr787", "route": "console", "server": "10.129.0.43:8443", "service": "console"})

	// simulate second reload
	e.CollectNow()
	if e.counterValues[secondConsolePodID][e.counterIndices[connectionsTotalIndex]] != 245 {
		t.Fatalf("incorrect counter: %#v", e.counterValues[secondConsolePodID])
	}

	// expect no scrape due to the interval set by the last gather
	e.lastScrape = &now
	f = gatherMetrics(t, r)
	if e.counterValues[secondConsolePodID][e.counterIndices[connectionsTotalIndex]] != 245 {
		t.Fatalf("incorrect counter: %#v", e.counterValues[secondConsolePodID])
	}

	mustHaveMetric(t, f, "haproxy_exporter_total_scrapes", 4)
	mustHaveMetric(t, f, "haproxy_server_max_sessions", 32, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 184, map[string]string{"namespace": "", "pod": "", "route": "", "server": "fe_sni", "service": ""})
	mustHaveMetric(t, f, "haproxy_server_connections_total", 245, map[string]string{"namespace": "openshift-console", "pod": "console-6db7cbb464-gr787", "route": "console", "server": "10.129.0.43:8443", "service": "console"})
}

func mustHaveMetric(t *testing.T, families []*client_model.MetricFamily, name string, value float64, labels ...map[string]string) {
	t.Helper()
	if !hasMetric(families, name, value, labels...) {
		t.Fatalf("does not have metric %s%v=%f:\n\n%s", name, labels, value, mustMetricsToString(families, name))
	}
}

func gatherMetrics(t *testing.T, r *prometheus.Registry) []*client_model.MetricFamily {
	t.Helper()
	f, err := r.Gather()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func hasMetric(families []*client_model.MetricFamily, metric string, value float64, labels ...map[string]string) bool {
	for _, family := range families {
		if *family.Name != metric {
			continue
		}
		for _, m := range family.Metric {
			if !hasAllLabels(m.Label, labels) {
				continue
			}
			var v float64
			switch {
			case m.Counter != nil:
				v = *m.Counter.Value
			case m.Gauge != nil:
				v = *m.Gauge.Value
			case m.Untyped != nil:
				v = *m.Untyped.Value
			default:
				continue
			}
			if value == v {
				return true
			}
		}
	}
	return false
}

func mustMetricsToString(families []*client_model.MetricFamily, names ...string) string {
	s, err := metricsToString(families, names...)
	if err != nil {
		panic(err)
	}
	return s
}

func metricsToString(families []*client_model.MetricFamily, names ...string) (string, error) {
	buf := &bytes.Buffer{}
	e := expfmt.NewEncoder(buf, expfmt.FmtText)
	for _, family := range families {
		if !hasName(family, names) {
			continue
		}
		if err := e.Encode(family); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func hasAllLabels(pairs []*client_model.LabelPair, labels []map[string]string) bool {
	for _, labelSet := range labels {
		for k, v := range labelSet {
			var match bool
			for _, pair := range pairs {
				if *pair.Name == k && *pair.Value == v {
					match = true
					break
				}
			}
			if !match {
				return false
			}
		}
	}
	return true
}

func hasName(family *client_model.MetricFamily, names []string) bool {
	if len(names) == 0 {
		return true
	}
	var named string
	if family.Name != nil {
		named = *family.Name
	}
	for _, name := range names {
		if name == named {
			return true
		}
	}
	return false
}
