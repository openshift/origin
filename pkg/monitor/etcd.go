package monitor

import (
	"context"
	"fmt"
	"time"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

/*
Under ideal conditions, the following etcd term/election timeline is expected:

term  elections  reason
1     -          no bootstrap member
2     1          bootstrap member elected
3     2          bootstrap member removed, master elected
4     3          new etcd pod rev to drop bootstrap member from config

Elections 1 and 2 occur during etcd pivot and before Prometheus is scraping any
metrics, and so will be invisible. Election 3 is the first election that should
be collected in the metrics data. Any other elections are suspicious and could
indicate a problem (e.g. IO contention, packet loss) that we want to investigate.

So, only 1 leader change is expected to be observed unless the test is either
disruptive or an upgrade.
*/
const ExpectedLeaderChanges = 1

func startEtcdMonitoring(ctx context.Context, m Recorder, prometheus prometheusv1.API) {
	expectedCount := ExpectedLeaderChanges
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		lastCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, _, err := prometheus.Query(context.Background(), "max by (job) (etcd_server_leader_changes_seen_total)", time.Now())
				if err != nil {
					// TODO: How do we log from here?
					fmt.Printf("etcd monitor couldn't query prometheus: %v\n", err)
					break
				}
				count := 0
				if vec, isVector := result.(model.Vector); isVector {
					if len(vec) > 0 {
						count = int(vec[0].Value)
					}
				}
				if count != lastCount {
					switch {
					case count <= expectedCount:
						m.Record(Condition{
							Level:   Info,
							Locator: "etcd",
							Message: fmt.Sprintf("observed expected leader election, expected total %d, last observed %d", expectedCount, count),
						})
					case count > expectedCount:
						m.Record(Condition{
							Level:   Error,
							Locator: "etcd",
							Message: fmt.Sprintf("observed unexpected leader election, expected total %d, last observed %d", expectedCount, count),
						})
					}
				}
				lastCount = count
			}
		}
	}()
}
