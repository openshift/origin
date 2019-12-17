//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package metrics

import (
	"log"
	"net/http"

	"github.com/heketi/heketi/apps"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	app apps.Application
}

const (
	namespace = "heketi"
)

const (
	KB uint64 = 1024
)

var (
	up = promDesc(
		"up",
		"Is heketi running?",
		nil,
	)

	clusterCount = promDesc(
		"cluster_count",
		"Number of clusters",
		nil,
	)

	volumesCount = promDesc(
		"volumes_count",
		"Number of volumes on cluster",
		[]string{"cluster"},
	)

	blockVolumesCount = promDesc(
		"block_volumes_count",
		"Number of block volumes on cluster",
		[]string{"cluster"},
	)

	nodesCount = promDesc(
		"nodes_count",
		"Number of nodes on cluster",
		[]string{"cluster"},
	)

	deviceCount = promDesc(
		"device_count",
		"Number of devices on host",
		[]string{"cluster", "hostname"},
	)

	deviceSize = promDesc(
		"device_size",
		"Total size of the device",
		[]string{"cluster", "hostname", "device"},
	)

	deviceFree = promDesc(
		"device_free",
		"Amount of Free space available on the device",
		[]string{"cluster", "hostname", "device"},
	)

	deviceUsed = promDesc(
		"device_used",
		"Amount of space used on the device",
		[]string{"cluster", "hostname", "device"},
	)

	deviceSizeInBytes = promDesc(
		"device_size_bytes",
		"Total size of the device in bytes",
		[]string{"cluster", "hostname", "device"},
	)

	deviceFreeInBytes = promDesc(
		"device_free_bytes",
		"Amount of Free space available on the device in bytes",
		[]string{"cluster", "hostname", "device"},
	)

	deviceUsedInBytes = promDesc(
		"device_used_bytes",
		"Amount of space used on the device in bytes",
		[]string{"cluster", "hostname", "device"},
	)

	brickCount = promDesc(
		"device_brick_count",
		"Number of bricks on device",
		[]string{"cluster", "hostname", "device"},
	)

	staleCount = promDesc(
		"operations_stale_count",
		"Number of Stale Operations",
		nil,
	)

	failedCount = promDesc(
		"operations_failed_count",
		"Number of Failed Operations",
		nil,
	)

	newCount = promDesc(
		"operations_new_count",
		"Number of New Operations",
		nil,
	)

	totalCount = promDesc(
		"operations_total_count",
		"Total Number of Operations",
		nil,
	)

	inFlightCount = promDesc(
		"operations_inFlight_count",
		"Number of in flight Operations",
		nil,
	)
)

func promDesc(name, help string, variableLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", name),
		help,
		variableLabels,
		nil,
	)
}

// Describe all the metrics exported by Heketi exporter. It implements prometheus.Collector.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterCount
	ch <- volumesCount
	ch <- blockVolumesCount
	ch <- nodesCount
	ch <- deviceCount
	ch <- deviceSize
	ch <- deviceFree
	ch <- deviceUsed
	ch <- deviceSizeInBytes
	ch <- deviceFreeInBytes
	ch <- deviceUsedInBytes
	ch <- brickCount
	/* following metrics are grabbed from operations list, gives number of stale|failed|new|total|inFlight operations */
	ch <- staleCount
	ch <- failedCount
	ch <- newCount
	ch <- totalCount
	ch <- inFlightCount

}

// Collect metrics from heketi app
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	topinfo, err := m.app.TopologyInfo()

	upVal := 0.0
	if err == nil {
		upVal = 1.0
	}

	ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, upVal)

	//Do not collect further metrics if heketi is down
	if err != nil {
		log.Println("Can't collect topology info for metrics: " + err.Error())
		return
	}

	ch <- prometheus.MustNewConstMetric(
		clusterCount,
		prometheus.GaugeValue,
		float64(len(topinfo.ClusterList)),
	)

	opinfo, err := m.app.AppOperationsInfo()
	if err != nil {
		log.Println("Can't collect Operations info for metrics: " + err.Error())
	} else {
		ch <- prometheus.MustNewConstMetric(
			staleCount,
			prometheus.GaugeValue,
			float64(opinfo.Stale))

		ch <- prometheus.MustNewConstMetric(
			failedCount,
			prometheus.GaugeValue,
			float64(opinfo.Failed))

		ch <- prometheus.MustNewConstMetric(
			newCount,
			prometheus.GaugeValue,
			float64(opinfo.New))

		ch <- prometheus.MustNewConstMetric(
			totalCount,
			prometheus.GaugeValue,
			float64(opinfo.Total))

		ch <- prometheus.MustNewConstMetric(
			inFlightCount,
			prometheus.GaugeValue,
			float64(opinfo.InFlight))
	}

	for _, cluster := range topinfo.ClusterList {
		ch <- prometheus.MustNewConstMetric(
			volumesCount,
			prometheus.GaugeValue,
			float64(len(cluster.Volumes)),
			cluster.Id,
		)

		ch <- prometheus.MustNewConstMetric(
			blockVolumesCount,
			prometheus.GaugeValue,
			float64(len(cluster.BlockVolumes)),
			cluster.Id,
		)

		ch <- prometheus.MustNewConstMetric(
			nodesCount,
			prometheus.GaugeValue,
			float64(len(cluster.Nodes)),
			cluster.Id,
		)
		for _, node := range cluster.Nodes {
			ch <- prometheus.MustNewConstMetric(
				deviceCount,
				prometheus.GaugeValue,
				float64(len(node.DevicesInfo)),
				cluster.Id,
				node.Hostnames.Manage[0],
			)
			for _, device := range node.DevicesInfo {
				ch <- prometheus.MustNewConstMetric(
					deviceSize,
					prometheus.GaugeValue,
					float64(device.Storage.Total),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceFree,
					prometheus.GaugeValue,
					float64(device.Storage.Free),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceUsed, prometheus.GaugeValue,
					float64(device.Storage.Used),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceSizeInBytes,
					prometheus.GaugeValue,
					float64(device.Storage.Total*KB),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceFreeInBytes,
					prometheus.GaugeValue,
					float64(device.Storage.Free*KB),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					deviceUsedInBytes, prometheus.GaugeValue,
					float64(device.Storage.Used*KB),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)

				ch <- prometheus.MustNewConstMetric(
					brickCount,
					prometheus.GaugeValue,
					float64(len(device.Bricks)),
					cluster.Id,
					node.Hostnames.Manage[0],
					device.Name,
				)
			}
		}
	}
}

func NewMetricsHandler(app apps.Application) http.HandlerFunc {
	m := &Metrics{
		app: app,
	}
	prometheus.MustRegister(m)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
}
