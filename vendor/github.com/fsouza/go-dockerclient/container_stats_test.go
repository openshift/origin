package docker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestStats(t *testing.T) {
	t.Parallel()
	jsonStats1 := `{
       "read" : "2015-01-08T22:57:31.547920715Z",
       "network" : {
          "rx_dropped" : 0,
          "rx_bytes" : 648,
          "rx_errors" : 0,
          "tx_packets" : 8,
          "tx_dropped" : 0,
          "rx_packets" : 8,
          "tx_errors" : 0,
          "tx_bytes" : 648
       },
	   "networks" : {
		   "eth0":{
			   "rx_dropped" : 0,
			   "rx_bytes" : 648,
			   "rx_errors" : 0,
			   "tx_packets" : 8,
			   "tx_dropped" : 0,
			   "rx_packets" : 8,
			   "tx_errors" : 0,
			   "tx_bytes" : 648
		   }
	   },
       "memory_stats" : {
          "stats" : {
             "total_pgmajfault" : 0,
             "cache" : 0,
             "mapped_file" : 0,
             "total_inactive_file" : 0,
             "pgpgout" : 414,
             "rss" : 6537216,
             "total_mapped_file" : 0,
             "writeback" : 0,
             "unevictable" : 0,
             "pgpgin" : 477,
             "total_unevictable" : 0,
             "pgmajfault" : 0,
             "total_rss" : 6537216,
             "total_rss_huge" : 6291456,
             "total_writeback" : 0,
             "total_inactive_anon" : 0,
             "rss_huge" : 6291456,
	     "hierarchical_memory_limit": 189204833,
             "total_pgfault" : 964,
             "total_active_file" : 0,
             "active_anon" : 6537216,
             "total_active_anon" : 6537216,
             "total_pgpgout" : 414,
             "total_cache" : 0,
             "inactive_anon" : 0,
             "active_file" : 0,
             "pgfault" : 964,
             "inactive_file" : 0,
             "total_pgpgin" : 477,
             "swap" : 47312896,
             "hierarchical_memsw_limit" : 1610612736
          },
          "max_usage" : 6651904,
          "usage" : 6537216,
          "failcnt" : 0,
          "limit" : 67108864
       },
       "blkio_stats": {
          "io_service_bytes_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 428795731968
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 388177920
             }
          ],
          "io_serviced_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 25994442
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 1734
             }
          ],
          "io_queue_recursive": [],
          "io_service_time_recursive": [],
          "io_wait_time_recursive": [],
          "io_merged_recursive": [],
          "io_time_recursive": [],
          "sectors_recursive": []
       },
       "cpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       },
       "precpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       }
    }`
	// 1 second later, cache is 100
	jsonStats2 := `{
       "read" : "2015-01-08T22:57:32.547920715Z",
	   "networks" : {
		   "eth0":{
			   "rx_dropped" : 0,
			   "rx_bytes" : 648,
			   "rx_errors" : 0,
			   "tx_packets" : 8,
			   "tx_dropped" : 0,
			   "rx_packets" : 8,
			   "tx_errors" : 0,
			   "tx_bytes" : 648
		   }
	   },
	   "memory_stats" : {
          "stats" : {
             "total_pgmajfault" : 0,
             "cache" : 100,
             "mapped_file" : 0,
             "total_inactive_file" : 0,
             "pgpgout" : 414,
             "rss" : 6537216,
             "total_mapped_file" : 0,
             "writeback" : 0,
             "unevictable" : 0,
             "pgpgin" : 477,
             "total_unevictable" : 0,
             "pgmajfault" : 0,
             "total_rss" : 6537216,
             "total_rss_huge" : 6291456,
             "total_writeback" : 0,
             "total_inactive_anon" : 0,
             "rss_huge" : 6291456,
             "total_pgfault" : 964,
             "total_active_file" : 0,
             "active_anon" : 6537216,
             "total_active_anon" : 6537216,
             "total_pgpgout" : 414,
             "total_cache" : 0,
             "inactive_anon" : 0,
             "active_file" : 0,
             "pgfault" : 964,
             "inactive_file" : 0,
             "total_pgpgin" : 477,
             "swap" : 47312896,
             "hierarchical_memsw_limit" : 1610612736
          },
          "max_usage" : 6651904,
          "usage" : 6537216,
          "failcnt" : 0,
          "limit" : 67108864
       },
       "blkio_stats": {
          "io_service_bytes_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 428795731968
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 388177920
             }
          ],
          "io_serviced_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 25994442
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 1734
             }
          ],
          "io_queue_recursive": [],
          "io_service_time_recursive": [],
          "io_wait_time_recursive": [],
          "io_merged_recursive": [],
          "io_time_recursive": [],
          "sectors_recursive": []
       },
       "cpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       },
       "precpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000,
		  "online_cpus": 4
       }
    }`
	var expected1 Stats
	var expected2 Stats
	err := json.Unmarshal([]byte(jsonStats1), &expected1)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal([]byte(jsonStats2), &expected2)
	if err != nil {
		t.Fatal(err)
	}
	id := "4fa6e0f0"

	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonStats1))
		w.Write([]byte(jsonStats2))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	errC := make(chan error, 1)
	statsC := make(chan *Stats)
	done := make(chan bool)
	defer close(done)
	go func() {
		errC <- client.Stats(StatsOptions{ID: id, Stats: statsC, Stream: true, Done: done})
		close(errC)
	}()
	var resultStats []*Stats
	for {
		stats, ok := <-statsC
		if !ok {
			break
		}
		resultStats = append(resultStats, stats)
	}
	err = <-errC
	if err != nil {
		t.Fatal(err)
	}
	if len(resultStats) != 2 {
		t.Fatalf("Stats: Expected 2 results. Got %d.", len(resultStats))
	}
	if !reflect.DeepEqual(resultStats[0], &expected1) {
		t.Errorf("Stats: Expected:\n%+v\nGot:\n%+v", expected1, resultStats[0])
	}
	if !reflect.DeepEqual(resultStats[1], &expected2) {
		t.Errorf("Stats: Expected:\n%+v\nGot:\n%+v", expected2, resultStats[1])
	}
	if req.Method != http.MethodGet {
		t.Errorf("Stats: wrong HTTP method. Want GET. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/containers/" + id + "/stats"))
	if req.URL.Path != u.Path {
		t.Errorf("Stats: wrong HTTP path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestStatsContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	statsC := make(chan *Stats)
	done := make(chan bool)
	defer close(done)
	err := client.Stats(StatsOptions{ID: "abef348", Stats: statsC, Stream: true, Done: done})
	expectNoSuchContainer(t, "abef348", err)
}
