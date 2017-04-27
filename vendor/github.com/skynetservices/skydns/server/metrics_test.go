// Copyright (c) 2014 The SkyDNS Authors. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

package server

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"

	"github.com/skynetservices/skydns/metrics"

	"github.com/miekg/dns"
)

func query(n string, t uint16) {
	m := new(dns.Msg)
	m.SetQuestion(n, t)
	dns.Exchange(m, "127.0.0.1:"+StrPort)
}

func scrape(t *testing.T, key string) int {
	resp, err := http.Get("http://localhost:12300/metrics")
	if err != nil {
		t.Logf("could not get metrics")
		return -1
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return -1
	}

	// Find value for key.
	n := bytes.Index(body, []byte(key))
	if n == -1 {
		return -1
	}

	i := n
	for i < len(body) {
		if body[i] == '\n' {
			break
		}
		if body[i] == ' ' {
			n = i + 1
		}
		i++
	}
	value, err := strconv.Atoi(string(body[n:i]))
	if err != nil {
		t.Fatal("failed to get value")
	}
	return value
}

func TestMetrics(t *testing.T) {
	s := newTestServer(t, false)
	defer s.Stop()

	metrics.Port = "12300"
	metrics.Subsystem = "test"
	metrics.Namespace = "test"
	metrics.Metrics()

	query("miek.nl.", dns.TypeMX)
	v0 := scrape(t, "test_test_dns_request_count_total{system=\"recursive\"}")
	query("miek.nl.", dns.TypeMX)
	v1 := scrape(t, "test_test_dns_request_count_total{system=\"recursive\"}")

	if v1 != v0+1 {
		t.Fatalf("expecting %d, got %d", v0+1, v1)
	}
}
